package extensions

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"
)

type HostCallHandler func(ctx context.Context, method string, params json.RawMessage) (interface{}, error)

type HostClient struct {
	token   string
	input   io.WriteCloser
	output  io.ReadCloser
	process *exec.Cmd
	handler HostCallHandler

	writeMu   sync.Mutex
	mu        sync.Mutex
	pending   map[string]chan rpcMessage
	closed    chan struct{}
	counter   atomic.Uint64
	closeOnce sync.Once
	markOnce  sync.Once
}

func StartHostProcess(executable string, handler HostCallHandler) (*HostClient, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("create extension host token: %w", err)
	}
	token := hex.EncodeToString(tokenBytes)
	command := exec.Command(executable, "extension-host")
	configureExtensionHostCommand(command)
	command.Env = append(os.Environ(), "ADOMNIA_EXTENSION_HOST_TOKEN="+token)
	input, err := command.StdinPipe()
	if err != nil {
		return nil, err
	}
	output, err := command.StdoutPipe()
	if err != nil {
		return nil, err
	}
	command.Stderr = os.Stderr
	if err := command.Start(); err != nil {
		return nil, fmt.Errorf("start extension host: %w", err)
	}
	client := NewHostClient(output, input, token, handler)
	client.process = command
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var initialized InitializeHostResult
	if err := client.Request(ctx, "initialize", InitializeHostRequest{ProtocolVersion: HostProtocolVersion}, &initialized); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("initialize extension host: %w", err)
	}
	if initialized.ProtocolVersion != HostProtocolVersion {
		_ = client.Close()
		return nil, fmt.Errorf("extension host protocol mismatch: %s", initialized.ProtocolVersion)
	}
	return client, nil
}

func NewHostClient(output io.ReadCloser, input io.WriteCloser, token string, handler HostCallHandler) *HostClient {
	client := &HostClient{
		token: token, input: input, output: output, handler: handler,
		pending: map[string]chan rpcMessage{}, closed: make(chan struct{}),
	}
	go client.readLoop()
	return client
}

func (c *HostClient) Request(ctx context.Context, method string, params, result interface{}) error {
	id := fmt.Sprintf("parent-%d", c.counter.Add(1))
	rawParams, err := marshalRPCValue(params)
	if err != nil {
		return err
	}
	response := make(chan rpcMessage, 1)
	c.mu.Lock()
	select {
	case <-c.closed:
		c.mu.Unlock()
		return fmt.Errorf("extension host is closed")
	default:
	}
	c.pending[id] = response
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
	}()
	if err := c.write(rpcMessage{JSONRPC: "2.0", ID: id, Method: method, Params: rawParams, Token: c.token}); err != nil {
		return err
	}
	select {
	case message := <-response:
		if message.Error != nil {
			return &RPCError{Code: message.Error.Code, Message: message.Error.Message}
		}
		if result == nil || len(message.Result) == 0 {
			return nil
		}
		if err := json.Unmarshal(message.Result, result); err != nil {
			return fmt.Errorf("decode extension host response: %w", err)
		}
		return nil
	case <-ctx.Done():
		cancelParams, _ := marshalRPCValue(CancelHostRequest{ID: id})
		_ = c.write(rpcMessage{JSONRPC: "2.0", Method: "$/cancelRequest", Params: cancelParams, Token: c.token})
		return ctx.Err()
	case <-c.closed:
		return fmt.Errorf("extension host closed while handling %s", method)
	}
}

func (c *HostClient) Close() error {
	var closeErr error
	c.closeOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		_ = c.Request(ctx, "shutdown", map[string]any{}, nil)
		cancel()
		_ = c.input.Close()
		_ = c.output.Close()
		if c.process != nil {
			done := make(chan error, 1)
			go func() { done <- c.process.Wait() }()
			select {
			case closeErr = <-done:
			case <-time.After(2 * time.Second):
				_ = c.process.Process.Kill()
				closeErr = <-done
			}
		}
		c.markClosed()
	})
	return closeErr
}

func (c *HostClient) readLoop() {
	scanner := bufio.NewScanner(c.output)
	scanner.Buffer(make([]byte, 64*1024), maxRPCMessageBytes)
	for scanner.Scan() {
		var message rpcMessage
		if err := json.Unmarshal(scanner.Bytes(), &message); err != nil || message.Token != c.token {
			continue
		}
		if message.Method != "" {
			go c.handleHostCall(message)
			continue
		}
		c.mu.Lock()
		pending := c.pending[message.ID]
		c.mu.Unlock()
		if pending != nil {
			select {
			case pending <- message:
			default:
			}
		}
	}
	c.markClosed()
}

func (c *HostClient) handleHostCall(message rpcMessage) {
	if c.handler == nil {
		_ = c.write(rpcMessage{JSONRPC: "2.0", ID: message.ID, Error: &rpcError{Code: -32601, Message: "host API is unavailable"}, Token: c.token})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := c.handler(ctx, message.Method, message.Params)
	response := rpcMessage{JSONRPC: "2.0", ID: message.ID, Token: c.token}
	if err != nil {
		response.Error = &rpcError{Code: -32000, Message: err.Error()}
	} else {
		response.Result, err = marshalRPCValue(result)
		if err != nil {
			response.Error = &rpcError{Code: -32603, Message: err.Error()}
		}
	}
	_ = c.write(response)
}

func (c *HostClient) write(message rpcMessage) error {
	data, err := json.Marshal(message)
	if err != nil {
		return err
	}
	if len(data) > maxRPCMessageBytes {
		return fmt.Errorf("extension host message exceeds %d bytes", maxRPCMessageBytes)
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if _, err := c.input.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write extension host message: %w", err)
	}
	return nil
}

func (c *HostClient) markClosed() {
	c.markOnce.Do(func() {
		close(c.closed)
	})
}
