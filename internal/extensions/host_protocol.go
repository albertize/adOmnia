package extensions

import (
	"encoding/json"
	"fmt"
)

const (
	HostProtocolVersion = "1.0"
	maxRPCMessageBytes  = 8 << 20
)

type rpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      string          `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
	Token   string          `json:"token"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// RPCError preserves the stable protocol error code across the subprocess boundary.
type RPCError struct {
	Code    int
	Message string
}

func (e *RPCError) Error() string { return e.Message }

type CancelHostRequest struct {
	ID string `json:"id"`
}

type InitializeHostRequest struct {
	ProtocolVersion string `json:"protocolVersion"`
}

type InitializeHostResult struct {
	ProtocolVersion string `json:"protocolVersion"`
	Runtime         string `json:"runtime"`
}

type ActivateHostRequest struct {
	Manifest Manifest       `json:"manifest"`
	Settings map[string]any `json:"settings"`
	Source   string         `json:"source"`
}

type ExecuteCommandRequest struct {
	ExtensionID string         `json:"extensionId"`
	CommandID   string         `json:"commandId"`
	Args        map[string]any `json:"args,omitempty"`
	Source      string         `json:"source,omitempty"`
}

type DispatchEventRequest struct {
	ExtensionID string         `json:"extensionId"`
	Event       string         `json:"event"`
	Payload     map[string]any `json:"payload"`
}

type EvaluateAssertionsRequest struct {
	ExtensionID string         `json:"extensionId"`
	Payload     map[string]any `json:"payload"`
}

type AssertionProviderResult struct {
	ExtensionID string `json:"extensionId,omitempty"`
	ProviderID  string `json:"providerId"`
	Label       string `json:"label"`
	Passed      bool   `json:"passed"`
	Actual      string `json:"actual,omitempty"`
	Expected    string `json:"expected,omitempty"`
	Message     string `json:"message,omitempty"`
}

type EvaluateVariableProvidersRequest struct {
	ExtensionID string         `json:"extensionId"`
	Context     map[string]any `json:"context"`
}

type VariableProviderResult struct {
	ExtensionID string            `json:"extensionId,omitempty"`
	ProviderID  string            `json:"providerId"`
	Values      map[string]string `json:"values"`
}

type DeactivateHostRequest struct {
	ExtensionID string `json:"extensionId"`
}

type HostExecutionResult struct {
	Success  bool        `json:"success"`
	Data     interface{} `json:"data,omitempty"`
	Modified bool        `json:"modified,omitempty"`
	Error    string      `json:"error,omitempty"`
	TimeMS   float64     `json:"timeMs"`
}

func marshalRPCValue(value interface{}) (json.RawMessage, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode RPC value: %w", err)
	}
	return data, nil
}
