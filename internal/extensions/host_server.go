package extensions

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dop251/goja"
)

var (
	v2ExportFunctionPattern = regexp.MustCompile(`(?m)^([ \t]*)export\s+(?:(async)\s+)?function\s+([A-Za-z_$][A-Za-z0-9_$]*)`)
	v2ExportVariablePattern = regexp.MustCompile(`(?m)^([ \t]*)export\s+(const|let|var)\s+([A-Za-z_$][A-Za-z0-9_$]*)`)
)

type hostServer struct {
	token   string
	input   io.Reader
	output  io.Writer
	writeMu sync.Mutex
	counter atomic.Uint64

	pendingMu sync.Mutex
	pending   map[string]chan rpcMessage
	requests  chan rpcMessage
	done      chan struct{}
	runtimes  map[string]*extensionRuntime

	operationMu sync.Mutex
	activeVM    map[string]*goja.Runtime
	cancelled   map[string]bool
}

type extensionRuntime struct {
	manifest           Manifest
	settings           map[string]any
	vm                 *goja.Runtime
	module             *goja.Object
	commands           map[string]goja.Callable
	events             map[string][]goja.Callable
	assertionProviders map[string]goja.Callable
	variableProviders  map[string]goja.Callable
	disposables        []goja.Callable
	timers             map[int64]runtimeTimer
	nextTimerID        int64
}

type runtimeTimer struct {
	due      time.Time
	callback goja.Callable
	args     []goja.Value
}

// RunExtensionHost runs the private child-process protocol. It must be invoked
// before Wails initialization and receives its authentication token only via
// the inherited environment.
func RunExtensionHost(input io.Reader, output io.Writer) error {
	token := os.Getenv("ADOMNIA_EXTENSION_HOST_TOKEN")
	if len(token) < 32 {
		return fmt.Errorf("extension host token is missing")
	}
	server := &hostServer{
		token: token, input: input, output: output,
		pending: map[string]chan rpcMessage{}, requests: make(chan rpcMessage, 64),
		done: make(chan struct{}), runtimes: map[string]*extensionRuntime{},
		activeVM: map[string]*goja.Runtime{}, cancelled: map[string]bool{},
	}
	go server.readLoop()
	return server.dispatchLoop()
}

func (s *hostServer) readLoop() {
	scanner := bufio.NewScanner(s.input)
	scanner.Buffer(make([]byte, 64*1024), maxRPCMessageBytes)
	for scanner.Scan() {
		var message rpcMessage
		if json.Unmarshal(scanner.Bytes(), &message) != nil || message.Token != s.token {
			continue
		}
		if message.Method == "$/cancelRequest" {
			var request CancelHostRequest
			if json.Unmarshal(message.Params, &request) == nil && request.ID != "" {
				s.cancelOperation(request.ID)
			}
			continue
		}
		if message.Method != "" {
			select {
			case s.requests <- message:
			case <-s.done:
				return
			}
			continue
		}
		s.pendingMu.Lock()
		pending := s.pending[message.ID]
		s.pendingMu.Unlock()
		if pending != nil {
			select {
			case pending <- message:
			default:
			}
		}
	}
	close(s.done)
}

func (s *hostServer) dispatchLoop() error {
	for {
		select {
		case message := <-s.requests:
			result, err, shutdown := s.dispatch(message.ID, message.Method, message.Params)
			response := rpcMessage{JSONRPC: "2.0", ID: message.ID, Token: s.token}
			if err != nil {
				code := -32000
				var protocolError *RPCError
				if errors.As(err, &protocolError) {
					code = protocolError.Code
				}
				response.Error = &rpcError{Code: code, Message: err.Error()}
			} else {
				response.Result, err = marshalRPCValue(result)
				if err != nil {
					response.Error = &rpcError{Code: -32603, Message: err.Error()}
				}
			}
			if writeErr := s.write(response); writeErr != nil {
				return writeErr
			}
			if shutdown {
				return nil
			}
		case <-s.done:
			return nil
		}
	}
}

func (s *hostServer) dispatch(operationID, method string, params json.RawMessage) (interface{}, error, bool) {
	switch method {
	case "initialize":
		var request InitializeHostRequest
		if err := json.Unmarshal(params, &request); err != nil {
			return nil, err, false
		}
		if request.ProtocolVersion != HostProtocolVersion {
			return nil, fmt.Errorf("unsupported protocol version %s", request.ProtocolVersion), false
		}
		return InitializeHostResult{ProtocolVersion: HostProtocolVersion, Runtime: "goja"}, nil, false
	case "activate":
		var request ActivateHostRequest
		if err := json.Unmarshal(params, &request); err != nil {
			return nil, err, false
		}
		return s.activate(operationID, request), nil, false
	case "executeCommand":
		var request ExecuteCommandRequest
		if err := json.Unmarshal(params, &request); err != nil {
			return nil, err, false
		}
		return s.executeCommand(operationID, request), nil, false
	case "dispatchEvent":
		var request DispatchEventRequest
		if err := json.Unmarshal(params, &request); err != nil {
			return nil, err, false
		}
		return s.dispatchEvent(operationID, request), nil, false
	case "evaluateAssertions":
		var request EvaluateAssertionsRequest
		if err := json.Unmarshal(params, &request); err != nil {
			return nil, err, false
		}
		return s.evaluateAssertions(operationID, request), nil, false
	case "evaluateVariableProviders":
		var request EvaluateVariableProvidersRequest
		if err := json.Unmarshal(params, &request); err != nil {
			return nil, err, false
		}
		return s.evaluateVariableProviders(operationID, request), nil, false
	case "deactivate":
		var request DeactivateHostRequest
		if err := json.Unmarshal(params, &request); err != nil {
			return nil, err, false
		}
		return map[string]bool{"ok": true}, s.deactivate(operationID, request.ExtensionID), false
	case "shutdown":
		for id := range s.runtimes {
			_ = s.deactivate(operationID, id)
		}
		return map[string]bool{"ok": true}, nil, true
	default:
		return nil, &RPCError{Code: -32601, Message: fmt.Sprintf("unknown extension host method: %s", method)}, false
	}
}

func (s *hostServer) activate(operationID string, request ActivateHostRequest) HostExecutionResult {
	started := time.Now()
	if existing := s.runtimes[request.Manifest.ID]; existing != nil {
		return HostExecutionResult{Success: true, TimeMS: elapsedMS(started)}
	}
	if int64(len(request.Source)) > MaxPackageBytes {
		return executionFailure(started, fmt.Errorf("extension source exceeds memory budget"))
	}
	vm := goja.New()
	s.beginOperation(operationID, vm)
	defer s.finishOperation(operationID)
	module := vm.NewObject()
	exports := vm.NewObject()
	_ = module.Set("exports", exports)
	_ = vm.Set("module", module)
	_ = vm.Set("exports", exports)
	runtime := &extensionRuntime{
		manifest: request.Manifest, settings: request.Settings, vm: vm, module: module,
		commands: map[string]goja.Callable{}, events: map[string][]goja.Callable{}, assertionProviders: map[string]goja.Callable{}, variableProviders: map[string]goja.Callable{}, timers: map[int64]runtimeTimer{},
	}
	if err := installRuntimeTimers(runtime); err != nil {
		return executionFailure(started, err)
	}
	api, err := s.buildAPI(runtime)
	if err != nil {
		return executionFailure(started, err)
	}
	if err := vm.Set("adomnia", api); err != nil {
		return executionFailure(started, err)
	}
	if err := runWithTimeout(vm, 10*time.Second, func() error {
		_, err := vm.RunString(transformV2Module(request.Source))
		return err
	}); err != nil {
		return executionFailure(started, fmt.Errorf("load extension: %w", err))
	}
	activateValue := module.Get("exports").ToObject(vm).Get("activate")
	activate, ok := goja.AssertFunction(activateValue)
	if !ok {
		return executionFailure(started, fmt.Errorf("extension does not export activate(api)"))
	}
	if err := runWithTimeout(vm, 10*time.Second, func() error {
		value, err := activate(goja.Undefined(), api)
		if err != nil {
			return err
		}
		_, err = resolveRuntimePromise(runtime, value, "activation returned a pending promise")
		return err
	}); err != nil {
		return executionFailure(started, fmt.Errorf("activate extension: %w", err))
	}
	s.runtimes[request.Manifest.ID] = runtime
	return HostExecutionResult{Success: true, TimeMS: elapsedMS(started)}
}

func (s *hostServer) executeCommand(operationID string, request ExecuteCommandRequest) HostExecutionResult {
	started := time.Now()
	runtime := s.runtimes[request.ExtensionID]
	if runtime == nil {
		return executionFailure(started, fmt.Errorf("extension is not active: %s", request.ExtensionID))
	}
	s.beginOperation(operationID, runtime.vm)
	defer s.finishOperation(operationID)
	handler := runtime.commands[request.CommandID]
	if handler == nil {
		return executionFailure(started, fmt.Errorf("command is not registered: %s", request.CommandID))
	}
	var result interface{}
	err := runWithTimeout(runtime.vm, 10*time.Second, func() error {
		value, err := handler(goja.Undefined(), runtime.vm.ToValue(request.Args), runtime.vm.ToValue(map[string]any{"source": request.Source}))
		if err != nil {
			return err
		}
		value, err = resolveRuntimePromise(runtime, value, "command returned a pending promise")
		if err != nil {
			return err
		}
		if goja.IsUndefined(value) || goja.IsNull(value) {
			return nil
		}
		data, err := json.Marshal(value.Export())
		if err != nil {
			return fmt.Errorf("command returned non-serializable data: %w", err)
		}
		if int64(len(data)) > MaxPackageBytes {
			return fmt.Errorf("command result exceeds memory budget")
		}
		return json.Unmarshal(data, &result)
	})
	if err != nil {
		return executionFailure(started, err)
	}
	return HostExecutionResult{Success: true, Data: result, TimeMS: elapsedMS(started)}
}

func (s *hostServer) dispatchEvent(operationID string, request DispatchEventRequest) HostExecutionResult {
	started := time.Now()
	runtime := s.runtimes[request.ExtensionID]
	if runtime == nil {
		return executionFailure(started, fmt.Errorf("extension is not active: %s", request.ExtensionID))
	}
	s.beginOperation(operationID, runtime.vm)
	defer s.finishOperation(operationID)
	payload := request.Payload
	modified := false
	for _, handler := range runtime.events[request.Event] {
		var next map[string]any
		err := runWithTimeout(runtime.vm, 10*time.Second, func() error {
			value, err := handler(goja.Undefined(), runtime.vm.ToValue(payload))
			if err != nil {
				return err
			}
			value, err = resolveRuntimePromise(runtime, value, "event handler returned a pending promise")
			if err != nil {
				return err
			}
			if goja.IsUndefined(value) || goja.IsNull(value) {
				return nil
			}
			exported, ok := value.Export().(map[string]interface{})
			if !ok {
				return nil
			}
			isModified, _ := exported["modified"].(bool)
			if !isModified {
				return nil
			}
			data, ok := exported["data"].(map[string]interface{})
			if !ok {
				return fmt.Errorf("modified event result requires object data")
			}
			next = data
			return nil
		})
		if err != nil {
			return executionFailure(started, err)
		}
		if next != nil {
			payload = next
			modified = true
		}
	}
	return HostExecutionResult{Success: true, Data: payload, Modified: modified, TimeMS: elapsedMS(started)}
}

func (s *hostServer) evaluateAssertions(operationID string, request EvaluateAssertionsRequest) HostExecutionResult {
	started := time.Now()
	runtime := s.runtimes[request.ExtensionID]
	if runtime == nil {
		return executionFailure(started, fmt.Errorf("extension is not active: %s", request.ExtensionID))
	}
	s.beginOperation(operationID, runtime.vm)
	defer s.finishOperation(operationID)
	providerIDs := make([]string, 0, len(runtime.assertionProviders))
	for id := range runtime.assertionProviders {
		providerIDs = append(providerIDs, id)
	}
	sort.Strings(providerIDs)
	results := make([]AssertionProviderResult, 0)
	for _, providerID := range providerIDs {
		handler := runtime.assertionProviders[providerID]
		var exported interface{}
		err := runWithTimeout(runtime.vm, 10*time.Second, func() error {
			value, err := handler(goja.Undefined(), runtime.vm.ToValue(request.Payload))
			if err != nil {
				return err
			}
			value, err = resolveRuntimePromise(runtime, value, "assertion provider returned a pending promise")
			if err != nil {
				return err
			}
			if goja.IsUndefined(value) || goja.IsNull(value) {
				return nil
			}
			exported = value.Export()
			return nil
		})
		if err != nil {
			return executionFailure(started, fmt.Errorf("assertion provider %s failed: %w", providerID, err))
		}
		if exported == nil {
			continue
		}
		data, err := json.Marshal(exported)
		if err != nil || len(data) > maxStateValueBytes {
			return executionFailure(started, fmt.Errorf("assertion provider %s returned invalid or oversized data", providerID))
		}
		var entries []AssertionProviderResult
		if len(data) > 0 && data[0] == '[' {
			err = json.Unmarshal(data, &entries)
		} else {
			var entry AssertionProviderResult
			err = json.Unmarshal(data, &entry)
			entries = []AssertionProviderResult{entry}
		}
		if err != nil {
			return executionFailure(started, fmt.Errorf("assertion provider %s returned invalid data: %w", providerID, err))
		}
		for index := range entries {
			entries[index].ProviderID = providerID
			if strings.TrimSpace(entries[index].Label) == "" {
				return executionFailure(started, fmt.Errorf("assertion provider %s returned a result without a label", providerID))
			}
		}
		results = append(results, entries...)
		if len(results) > 200 {
			return executionFailure(started, fmt.Errorf("assertion providers exceed 200 results"))
		}
	}
	return HostExecutionResult{Success: true, Data: results, TimeMS: elapsedMS(started)}
}

func (s *hostServer) evaluateVariableProviders(operationID string, request EvaluateVariableProvidersRequest) HostExecutionResult {
	started := time.Now()
	runtime := s.runtimes[request.ExtensionID]
	if runtime == nil {
		return executionFailure(started, fmt.Errorf("extension is not active: %s", request.ExtensionID))
	}
	s.beginOperation(operationID, runtime.vm)
	defer s.finishOperation(operationID)
	providerIDs := make([]string, 0, len(runtime.variableProviders))
	for id := range runtime.variableProviders {
		providerIDs = append(providerIDs, id)
	}
	sort.Strings(providerIDs)
	results := make([]VariableProviderResult, 0, len(providerIDs))
	for _, providerID := range providerIDs {
		handler := runtime.variableProviders[providerID]
		var values map[string]string
		err := runWithTimeout(runtime.vm, 10*time.Second, func() error {
			value, err := handler(goja.Undefined(), runtime.vm.ToValue(request.Context))
			if err != nil {
				return err
			}
			value, err = resolveRuntimePromise(runtime, value, "variable provider returned a pending promise")
			if err != nil {
				return err
			}
			if goja.IsUndefined(value) || goja.IsNull(value) {
				values = map[string]string{}
				return nil
			}
			data, err := json.Marshal(value.Export())
			if err != nil || len(data) > maxStateValueBytes {
				return fmt.Errorf("variable provider returned invalid or oversized data")
			}
			if err := json.Unmarshal(data, &values); err != nil {
				return fmt.Errorf("variable provider must return a string map: %w", err)
			}
			return nil
		})
		if err != nil {
			return executionFailure(started, fmt.Errorf("variable provider %s failed: %w", providerID, err))
		}
		results = append(results, VariableProviderResult{ProviderID: providerID, Values: values})
	}
	return HostExecutionResult{Success: true, Data: results, TimeMS: elapsedMS(started)}
}

func (s *hostServer) deactivate(operationID, extensionID string) error {
	runtime := s.runtimes[extensionID]
	if runtime == nil {
		return nil
	}
	s.beginOperation(operationID, runtime.vm)
	defer s.finishOperation(operationID)
	deactivateValue := runtime.module.Get("exports").ToObject(runtime.vm).Get("deactivate")
	err := runWithTimeout(runtime.vm, 5*time.Second, func() error {
		var firstErr error
		if deactivate, ok := goja.AssertFunction(deactivateValue); ok {
			value, callErr := deactivate(goja.Undefined())
			if callErr == nil {
				_, callErr = resolveRuntimePromise(runtime, value, "deactivation returned a pending promise")
			}
			firstErr = callErr
		}
		for index := len(runtime.disposables) - 1; index >= 0; index-- {
			if _, disposeErr := runtime.disposables[index](goja.Undefined()); disposeErr != nil && firstErr == nil {
				firstErr = disposeErr
			}
		}
		return firstErr
	})
	clear(runtime.timers)
	delete(s.runtimes, extensionID)
	return err
}

func (s *hostServer) beginOperation(id string, vm *goja.Runtime) {
	if id == "" {
		return
	}
	s.operationMu.Lock()
	s.activeVM[id] = vm
	cancelled := s.cancelled[id]
	s.operationMu.Unlock()
	if cancelled {
		vm.Interrupt("extension operation cancelled")
	}
}

func (s *hostServer) finishOperation(id string) {
	if id == "" {
		return
	}
	s.operationMu.Lock()
	delete(s.activeVM, id)
	delete(s.cancelled, id)
	s.operationMu.Unlock()
}

func (s *hostServer) cancelOperation(id string) {
	s.operationMu.Lock()
	s.cancelled[id] = true
	vm := s.activeVM[id]
	s.operationMu.Unlock()
	if vm != nil {
		vm.Interrupt("extension operation cancelled")
	}
}

func (s *hostServer) buildAPI(runtime *extensionRuntime) (*goja.Object, error) {
	vm := runtime.vm
	api := vm.NewObject()
	contextObject := vm.NewObject()
	_ = contextObject.Set("id", runtime.manifest.ID)
	_ = contextObject.Set("version", runtime.manifest.Version)
	subscriptions := vm.NewObject()
	_ = subscriptions.Set("add", func(call goja.FunctionCall) goja.Value {
		disposable := call.Argument(0)
		if goja.IsUndefined(disposable) || goja.IsNull(disposable) {
			return goja.Undefined()
		}
		dispose, ok := goja.AssertFunction(disposable.ToObject(vm).Get("dispose"))
		if !ok {
			panic(vm.NewGoError(fmt.Errorf("subscription must expose dispose()")))
		}
		runtime.disposables = append(runtime.disposables, dispose)
		return goja.Undefined()
	})
	_ = contextObject.Set("subscriptions", subscriptions)
	_ = api.Set("context", contextObject)

	allowedCommands := map[string]struct{}{}
	for _, command := range runtime.manifest.Contributes.Commands {
		allowedCommands[command.ID] = struct{}{}
	}
	commands := vm.NewObject()
	_ = commands.Set("registerCommand", func(call goja.FunctionCall) goja.Value {
		id := call.Argument(0).String()
		if _, ok := allowedCommands[id]; !ok {
			panic(vm.NewGoError(fmt.Errorf("command is not declared in manifest: %s", id)))
		}
		handler, ok := goja.AssertFunction(call.Argument(1))
		if !ok {
			panic(vm.NewGoError(fmt.Errorf("command handler must be a function")))
		}
		runtime.commands[id] = handler
		disposable := vm.NewObject()
		_ = disposable.Set("dispose", func(goja.FunctionCall) goja.Value {
			delete(runtime.commands, id)
			return goja.Undefined()
		})
		return disposable
	})
	_ = commands.Set("executeCommand", func(call goja.FunctionCall) goja.Value {
		id := call.Argument(0).String()
		if _, ok := allowedCommands[id]; !ok {
			panic(vm.NewGoError(fmt.Errorf("cross-extension command execution is not supported: %s", id)))
		}
		handler := runtime.commands[id]
		if handler == nil {
			panic(vm.NewGoError(fmt.Errorf("command is not registered: %s", id)))
		}
		value, err := handler(goja.Undefined(), call.Argument(1), vm.ToValue(map[string]string{"source": "extension"}))
		if err != nil {
			panic(vm.NewGoError(err))
		}
		value, err = resolveRuntimePromise(runtime, value, "nested command returned a pending promise")
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return value
	})
	_ = api.Set("commands", commands)

	events := vm.NewObject()
	registerEvent := func(event string, value goja.Value) goja.Value {
		if !manifestHasActivation(runtime.manifest, event) {
			panic(vm.NewGoError(fmt.Errorf("event is not declared in activationEvents: %s", event)))
		}
		if permission := eventPermission(event); permission != "" && !hasPermission(runtime.manifest.Permissions, permission) {
			panic(vm.NewGoError(fmt.Errorf("permission denied: %s requires %s", event, permission)))
		}
		handler, ok := goja.AssertFunction(value)
		if !ok {
			panic(vm.NewGoError(fmt.Errorf("event handler must be a function")))
		}
		runtime.events[event] = append(runtime.events[event], handler)
		disposable := vm.NewObject()
		_ = disposable.Set("dispose", func(goja.FunctionCall) goja.Value {
			delete(runtime.events, event)
			return goja.Undefined()
		})
		return disposable
	}
	_ = events.Set("on", func(call goja.FunctionCall) goja.Value {
		return registerEvent(call.Argument(0).String(), call.Argument(1))
	})
	for method, event := range map[string]string{"onRequest": "onRequest", "onResponse": "onResponse", "onSend": "onSend", "onSave": "onSave", "onImport": "onImport", "onExport": "onExport", "onThemeChange": "onThemeChange", "onEnvironmentChange": "onEnvChange", "onTabOpen": "onTabOpen", "onTabClose": "onTabClose", "onWorkspaceOpen": "onWorkspaceOpen", "onWorkspaceClose": "onWorkspaceClose", "onBrowserNetwork": "onBrowserNetwork", "onMockHit": "onMockHit", "onProxyTraffic": "onProxyTraffic", "onFlowProgress": "onFlowProgress", "onFlowComplete": "onFlowComplete", "onDatabaseComplete": "onDatabaseComplete", "onBrokerPublishComplete": "onBrokerPublishComplete", "onDocumentReadComplete": "onDocumentReadComplete", "onDocumentWriteComplete": "onDocumentWriteComplete", "onAIComplete": "onAIComplete"} {
		eventName := event
		_ = events.Set(method, func(call goja.FunctionCall) goja.Value { return registerEvent(eventName, call.Argument(0)) })
	}
	_ = api.Set("events", events)

	requests := vm.NewObject()
	_ = requests.Set("getActive", func(goja.FunctionCall) goja.Value {
		if !hasPermission(runtime.manifest.Permissions, "requests.read") {
			panic(vm.NewGoError(fmt.Errorf("permission denied: requests.read")))
		}
		result, err := s.hostCall("requests.getActive", map[string]any{"extensionId": runtime.manifest.ID})
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(result)
	})
	_ = requests.Set("execute", func(call goja.FunctionCall) goja.Value {
		if !hasPermission(runtime.manifest.Permissions, "requests.execute") {
			panic(vm.NewGoError(fmt.Errorf("permission denied: requests.execute")))
		}
		result, err := s.hostCall("requests.execute", map[string]any{"extensionId": runtime.manifest.ID, "request": call.Argument(0).Export()})
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(result)
	})
	_ = api.Set("requests", requests)

	responses := vm.NewObject()
	_ = responses.Set("getActive", func(goja.FunctionCall) goja.Value {
		if !hasPermission(runtime.manifest.Permissions, "responses.read") {
			panic(vm.NewGoError(fmt.Errorf("permission denied: responses.read")))
		}
		result, err := s.hostCall("responses.getActive", map[string]any{"extensionId": runtime.manifest.ID})
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(result)
	})
	_ = api.Set("responses", responses)

	assertions := vm.NewObject()
	_ = assertions.Set("registerProvider", func(call goja.FunctionCall) goja.Value {
		if !hasPermission(runtime.manifest.Permissions, "assertions.provide") {
			panic(vm.NewGoError(fmt.Errorf("permission denied: assertions.provide")))
		}
		if !manifestHasActivation(runtime.manifest, "onAssertions") {
			panic(vm.NewGoError(fmt.Errorf("assertion providers require onAssertions activation")))
		}
		id := call.Argument(0).String()
		if !contributionIDPattern.MatchString(id) || !strings.HasPrefix(id, runtime.manifest.ID+".") {
			panic(vm.NewGoError(fmt.Errorf("assertion provider id is not owned by extension: %s", id)))
		}
		handler, ok := goja.AssertFunction(call.Argument(1))
		if !ok {
			panic(vm.NewGoError(fmt.Errorf("assertion provider handler must be a function")))
		}
		runtime.assertionProviders[id] = handler
		disposable := vm.NewObject()
		_ = disposable.Set("dispose", func(goja.FunctionCall) goja.Value {
			delete(runtime.assertionProviders, id)
			return goja.Undefined()
		})
		return disposable
	})
	_ = api.Set("assertions", assertions)

	variables := vm.NewObject()
	_ = variables.Set("getAll", func(goja.FunctionCall) goja.Value {
		if !hasPermission(runtime.manifest.Permissions, "variables.read") {
			panic(vm.NewGoError(fmt.Errorf("permission denied: variables.read")))
		}
		result, err := s.hostCall("variables.getAll", map[string]any{"extensionId": runtime.manifest.ID})
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(result)
	})
	_ = variables.Set("registerProvider", func(call goja.FunctionCall) goja.Value {
		if !hasPermission(runtime.manifest.Permissions, "variables.provide") {
			panic(vm.NewGoError(fmt.Errorf("permission denied: variables.provide")))
		}
		if !manifestHasActivation(runtime.manifest, "onVariables") {
			panic(vm.NewGoError(fmt.Errorf("variable providers require onVariables activation")))
		}
		id := call.Argument(0).String()
		if !contributionIDPattern.MatchString(id) || !strings.HasPrefix(id, runtime.manifest.ID+".") {
			panic(vm.NewGoError(fmt.Errorf("variable provider id is not owned by extension: %s", id)))
		}
		handler, ok := goja.AssertFunction(call.Argument(1))
		if !ok {
			panic(vm.NewGoError(fmt.Errorf("variable provider handler must be a function")))
		}
		runtime.variableProviders[id] = handler
		disposable := vm.NewObject()
		_ = disposable.Set("dispose", func(goja.FunctionCall) goja.Value {
			delete(runtime.variableProviders, id)
			return goja.Undefined()
		})
		return disposable
	})
	_ = variables.Set("resolve", func(call goja.FunctionCall) goja.Value {
		if !hasPermission(runtime.manifest.Permissions, "variables.read") {
			panic(vm.NewGoError(fmt.Errorf("permission denied: variables.read")))
		}
		result, err := s.hostCall("variables.resolve", map[string]any{"extensionId": runtime.manifest.ID, "value": call.Argument(0).String()})
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(result)
	})
	_ = api.Set("variables", variables)

	mockAPI := vm.NewObject()
	_ = mockAPI.Set("getSnapshot", func(goja.FunctionCall) goja.Value {
		if !hasPermission(runtime.manifest.Permissions, "mock.read") {
			panic(vm.NewGoError(fmt.Errorf("permission denied: mock.read")))
		}
		result, err := s.hostCall("mock.getSnapshot", map[string]any{"extensionId": runtime.manifest.ID})
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(result)
	})
	_ = mockAPI.Set("clearHits", func(goja.FunctionCall) goja.Value {
		if !hasPermission(runtime.manifest.Permissions, "mock.control") {
			panic(vm.NewGoError(fmt.Errorf("permission denied: mock.control")))
		}
		if _, err := s.hostCall("mock.clearHits", map[string]any{"extensionId": runtime.manifest.ID}); err != nil {
			panic(vm.NewGoError(err))
		}
		return goja.Undefined()
	})
	_ = mockAPI.Set("stop", func(goja.FunctionCall) goja.Value {
		if !hasPermission(runtime.manifest.Permissions, "mock.control") {
			panic(vm.NewGoError(fmt.Errorf("permission denied: mock.control")))
		}
		if _, err := s.hostCall("mock.stop", map[string]any{"extensionId": runtime.manifest.ID}); err != nil {
			panic(vm.NewGoError(err))
		}
		return goja.Undefined()
	})
	_ = api.Set("mock", mockAPI)

	documentsAPI := vm.NewObject()
	_ = documentsAPI.Set("readPdfText", func(call goja.FunctionCall) goja.Value {
		if !hasPermission(runtime.manifest.Permissions, "documents.readContents") {
			panic(vm.NewGoError(fmt.Errorf("permission denied: documents.readContents")))
		}
		if !manifestHasActivation(runtime.manifest, "onDocumentReadComplete") {
			panic(vm.NewGoError(fmt.Errorf("document text reading requires onDocumentReadComplete activation")))
		}
		result, err := s.hostCall("documents.startReadText", map[string]any{"extensionId": runtime.manifest.ID, "projectId": call.Argument(0).String(), "options": call.Argument(1).Export()})
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(result)
	})
	_ = documentsAPI.Set("exportPdf", func(call goja.FunctionCall) goja.Value {
		if !hasPermission(runtime.manifest.Permissions, "documents.write") {
			panic(vm.NewGoError(fmt.Errorf("permission denied: documents.write")))
		}
		if !manifestHasActivation(runtime.manifest, "onDocumentWriteComplete") {
			panic(vm.NewGoError(fmt.Errorf("document export requires onDocumentWriteComplete activation")))
		}
		result, err := s.hostCall("documents.startExport", map[string]any{"extensionId": runtime.manifest.ID, "projectId": call.Argument(0).String(), "options": call.Argument(1).Export()})
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(result)
	})
	_ = documentsAPI.Set("cancel", func(call goja.FunctionCall) goja.Value {
		if !hasPermission(runtime.manifest.Permissions, "documents.readContents") && !hasPermission(runtime.manifest.Permissions, "documents.write") {
			panic(vm.NewGoError(fmt.Errorf("permission denied: document jobs")))
		}
		if _, err := s.hostCall("documents.cancel", map[string]any{"extensionId": runtime.manifest.ID, "jobId": call.Argument(0).String()}); err != nil {
			panic(vm.NewGoError(err))
		}
		return goja.Undefined()
	})
	_ = documentsAPI.Set("listPdfProjects", func(goja.FunctionCall) goja.Value {
		if !hasPermission(runtime.manifest.Permissions, "documents.read") {
			panic(vm.NewGoError(fmt.Errorf("permission denied: documents.read")))
		}
		result, err := s.hostCall("documents.listPdfProjects", map[string]any{"extensionId": runtime.manifest.ID})
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(result)
	})
	_ = api.Set("documents", documentsAPI)

	aiAPI := vm.NewObject()
	_ = aiAPI.Set("complete", func(call goja.FunctionCall) goja.Value {
		if !hasPermission(runtime.manifest.Permissions, "ai.execute") {
			panic(vm.NewGoError(fmt.Errorf("permission denied: ai.execute")))
		}
		if !manifestHasActivation(runtime.manifest, "onAIComplete") {
			panic(vm.NewGoError(fmt.Errorf("AI completion requires onAIComplete activation")))
		}
		result, err := s.hostCall("ai.startComplete", map[string]any{"extensionId": runtime.manifest.ID, "systemPrompt": call.Argument(0).String(), "userPrompt": call.Argument(1).String(), "options": call.Argument(2).Export()})
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(result)
	})
	_ = aiAPI.Set("cancel", func(call goja.FunctionCall) goja.Value {
		if !hasPermission(runtime.manifest.Permissions, "ai.execute") {
			panic(vm.NewGoError(fmt.Errorf("permission denied: ai.execute")))
		}
		if _, err := s.hostCall("ai.cancel", map[string]any{"extensionId": runtime.manifest.ID, "jobId": call.Argument(0).String()}); err != nil {
			panic(vm.NewGoError(err))
		}
		return goja.Undefined()
	})
	_ = api.Set("ai", aiAPI)

	databasesAPI := vm.NewObject()
	_ = databasesAPI.Set("execute", func(call goja.FunctionCall) goja.Value {
		if !hasPermission(runtime.manifest.Permissions, "databases.execute") {
			panic(vm.NewGoError(fmt.Errorf("permission denied: databases.execute")))
		}
		if !manifestHasActivation(runtime.manifest, "onDatabaseComplete") {
			panic(vm.NewGoError(fmt.Errorf("database execution requires onDatabaseComplete activation")))
		}
		result, err := s.hostCall("databases.startQuery", map[string]any{"extensionId": runtime.manifest.ID, "connectionId": call.Argument(0).String(), "query": call.Argument(1).String(), "options": call.Argument(2).Export()})
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(result)
	})
	_ = databasesAPI.Set("cancel", func(call goja.FunctionCall) goja.Value {
		if !hasPermission(runtime.manifest.Permissions, "databases.execute") {
			panic(vm.NewGoError(fmt.Errorf("permission denied: databases.execute")))
		}
		if _, err := s.hostCall("databases.cancel", map[string]any{"extensionId": runtime.manifest.ID, "jobId": call.Argument(0).String()}); err != nil {
			panic(vm.NewGoError(err))
		}
		return goja.Undefined()
	})
	_ = databasesAPI.Set("listConnections", func(goja.FunctionCall) goja.Value {
		if !hasPermission(runtime.manifest.Permissions, "databases.read") {
			panic(vm.NewGoError(fmt.Errorf("permission denied: databases.read")))
		}
		result, err := s.hostCall("databases.listConnections", map[string]any{"extensionId": runtime.manifest.ID})
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(result)
	})
	_ = api.Set("databases", databasesAPI)

	brokersAPI := vm.NewObject()
	_ = brokersAPI.Set("publish", func(call goja.FunctionCall) goja.Value {
		if !hasPermission(runtime.manifest.Permissions, "brokers.publish") {
			panic(vm.NewGoError(fmt.Errorf("permission denied: brokers.publish")))
		}
		if !manifestHasActivation(runtime.manifest, "onBrokerPublishComplete") {
			panic(vm.NewGoError(fmt.Errorf("broker publishing requires onBrokerPublishComplete activation")))
		}
		result, err := s.hostCall("brokers.startPublish", map[string]any{"extensionId": runtime.manifest.ID, "connectionId": call.Argument(0).String(), "destination": call.Argument(1).String(), "message": call.Argument(2).String(), "options": call.Argument(3).Export()})
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(result)
	})
	_ = brokersAPI.Set("cancel", func(call goja.FunctionCall) goja.Value {
		if !hasPermission(runtime.manifest.Permissions, "brokers.publish") {
			panic(vm.NewGoError(fmt.Errorf("permission denied: brokers.publish")))
		}
		if _, err := s.hostCall("brokers.cancel", map[string]any{"extensionId": runtime.manifest.ID, "jobId": call.Argument(0).String()}); err != nil {
			panic(vm.NewGoError(err))
		}
		return goja.Undefined()
	})
	_ = brokersAPI.Set("listConnections", func(goja.FunctionCall) goja.Value {
		if !hasPermission(runtime.manifest.Permissions, "brokers.read") {
			panic(vm.NewGoError(fmt.Errorf("permission denied: brokers.read")))
		}
		result, err := s.hostCall("brokers.listConnections", map[string]any{"extensionId": runtime.manifest.ID})
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(result)
	})
	_ = api.Set("brokers", brokersAPI)

	flowsAPI := vm.NewObject()
	_ = flowsAPI.Set("list", func(goja.FunctionCall) goja.Value {
		if !hasPermission(runtime.manifest.Permissions, "flows.read") {
			panic(vm.NewGoError(fmt.Errorf("permission denied: flows.read")))
		}
		result, err := s.hostCall("flows.list", map[string]any{"extensionId": runtime.manifest.ID})
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(result)
	})
	_ = flowsAPI.Set("execute", func(call goja.FunctionCall) goja.Value {
		if !hasPermission(runtime.manifest.Permissions, "flows.execute") {
			panic(vm.NewGoError(fmt.Errorf("permission denied: flows.execute")))
		}
		if !manifestHasActivation(runtime.manifest, "onFlowComplete") {
			panic(vm.NewGoError(fmt.Errorf("flow execution requires onFlowComplete activation")))
		}
		result, err := s.hostCall("flows.start", map[string]any{"extensionId": runtime.manifest.ID, "flowId": call.Argument(0).String(), "options": call.Argument(1).Export()})
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(result)
	})
	_ = flowsAPI.Set("executeStress", func(call goja.FunctionCall) goja.Value {
		if !hasPermission(runtime.manifest.Permissions, "flows.execute") {
			panic(vm.NewGoError(fmt.Errorf("permission denied: flows.execute")))
		}
		if !manifestHasActivation(runtime.manifest, "onFlowComplete") {
			panic(vm.NewGoError(fmt.Errorf("flow execution requires onFlowComplete activation")))
		}
		result, err := s.hostCall("flows.startStress", map[string]any{"extensionId": runtime.manifest.ID, "flowId": call.Argument(0).String(), "options": call.Argument(1).Export()})
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(result)
	})
	_ = flowsAPI.Set("cancel", func(call goja.FunctionCall) goja.Value {
		if !hasPermission(runtime.manifest.Permissions, "flows.execute") {
			panic(vm.NewGoError(fmt.Errorf("permission denied: flows.execute")))
		}
		_, err := s.hostCall("flows.cancel", map[string]any{"extensionId": runtime.manifest.ID, "jobId": call.Argument(0).String()})
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return goja.Undefined()
	})
	_ = flowsAPI.Set("get", func(call goja.FunctionCall) goja.Value {
		if !hasPermission(runtime.manifest.Permissions, "flows.read") {
			panic(vm.NewGoError(fmt.Errorf("permission denied: flows.read")))
		}
		result, err := s.hostCall("flows.get", map[string]any{"extensionId": runtime.manifest.ID, "id": call.Argument(0).String()})
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(result)
	})
	_ = api.Set("flows", flowsAPI)

	proxyAPI := vm.NewObject()
	_ = proxyAPI.Set("getSnapshot", func(goja.FunctionCall) goja.Value {
		if !hasPermission(runtime.manifest.Permissions, "proxy.read") {
			panic(vm.NewGoError(fmt.Errorf("permission denied: proxy.read")))
		}
		result, err := s.hostCall("proxy.getSnapshot", map[string]any{"extensionId": runtime.manifest.ID})
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(result)
	})
	_ = proxyAPI.Set("clearTraffic", func(goja.FunctionCall) goja.Value {
		if !hasPermission(runtime.manifest.Permissions, "proxy.control") {
			panic(vm.NewGoError(fmt.Errorf("permission denied: proxy.control")))
		}
		if _, err := s.hostCall("proxy.clearTraffic", map[string]any{"extensionId": runtime.manifest.ID}); err != nil {
			panic(vm.NewGoError(err))
		}
		return goja.Undefined()
	})
	_ = proxyAPI.Set("stop", func(goja.FunctionCall) goja.Value {
		if !hasPermission(runtime.manifest.Permissions, "proxy.control") {
			panic(vm.NewGoError(fmt.Errorf("permission denied: proxy.control")))
		}
		if _, err := s.hostCall("proxy.stop", map[string]any{"extensionId": runtime.manifest.ID}); err != nil {
			panic(vm.NewGoError(err))
		}
		return goja.Undefined()
	})
	_ = api.Set("proxy", proxyAPI)

	diagnostics := vm.NewObject()
	_ = diagnostics.Set("set", func(call goja.FunctionCall) goja.Value {
		_, err := s.hostCall("diagnostics.set", map[string]any{"extensionId": runtime.manifest.ID, "entries": call.Argument(0).Export()})
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return goja.Undefined()
	})
	_ = api.Set("diagnostics", diagnostics)

	progress := vm.NewObject()
	_ = progress.Set("report", func(call goja.FunctionCall) goja.Value {
		_, err := s.hostCall("progress.report", map[string]any{"extensionId": runtime.manifest.ID, "progress": call.Argument(0).Export()})
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return goja.Undefined()
	})
	_ = api.Set("progress", progress)

	for domain, permission := range map[string]string{
		"environments": "environments.read",
		"collections":  "collections.read",
		"tabs":         "tabs.read",
		"workspace":    "workspace.read",
		"browserDebug": "browserDebug.read",
	} {
		domainName, requiredPermission := domain, permission
		domainAPI := vm.NewObject()
		get := func(selector string) func(goja.FunctionCall) goja.Value {
			return func(goja.FunctionCall) goja.Value {
				if !hasPermission(runtime.manifest.Permissions, requiredPermission) {
					panic(vm.NewGoError(fmt.Errorf("permission denied: %s", requiredPermission)))
				}
				result, err := s.hostCall("domains.get", map[string]any{"extensionId": runtime.manifest.ID, "domain": domainName, "selector": selector})
				if err != nil {
					panic(vm.NewGoError(err))
				}
				return vm.ToValue(result)
			}
		}
		_ = domainAPI.Set("getActive", get("active"))
		_ = domainAPI.Set("list", get("list"))
		_ = domainAPI.Set("getSnapshot", get("snapshot"))
		action := func(permission, name string, payload func(goja.FunctionCall) map[string]any) func(goja.FunctionCall) goja.Value {
			return func(call goja.FunctionCall) goja.Value {
				if !hasPermission(runtime.manifest.Permissions, permission) {
					panic(vm.NewGoError(fmt.Errorf("permission denied: %s", permission)))
				}
				_, err := s.hostCall("domains.action", map[string]any{"extensionId": runtime.manifest.ID, "domain": domainName, "action": name, "payload": payload(call)})
				if err != nil {
					panic(vm.NewGoError(err))
				}
				return goja.Undefined()
			}
		}
		switch domainName {
		case "collections":
			_ = domainAPI.Set("importCollection", action("collections.write", "import", func(call goja.FunctionCall) map[string]any {
				return map[string]any{"collection": call.Argument(0).Export()}
			}))
			_ = domainAPI.Set("addRequest", action("collections.write", "addRequest", func(call goja.FunctionCall) map[string]any {
				return map[string]any{"collectionId": call.Argument(0).String(), "parentId": call.Argument(1).Export(), "request": call.Argument(2).Export()}
			}))
		case "environments":
			_ = domainAPI.Set("setActive", action("environments.write", "setActive", func(call goja.FunctionCall) map[string]any { return map[string]any{"id": call.Argument(0).Export()} }))
		case "tabs":
			_ = domainAPI.Set("open", action("tabs.write", "open", func(call goja.FunctionCall) map[string]any {
				return map[string]any{"request": call.Argument(0).Export(), "collectionId": call.Argument(1).Export()}
			}))
			_ = domainAPI.Set("close", action("tabs.write", "close", func(call goja.FunctionCall) map[string]any { return map[string]any{"id": call.Argument(0).String()} }))
			_ = domainAPI.Set("setActive", action("tabs.write", "setActive", func(call goja.FunctionCall) map[string]any { return map[string]any{"id": call.Argument(0).String()} }))
		case "browserDebug":
			_ = domainAPI.Set("clear", action("browserDebug.control", "clear", func(goja.FunctionCall) map[string]any { return map[string]any{} }))
			_ = domainAPI.Set("select", action("browserDebug.control", "select", func(call goja.FunctionCall) map[string]any { return map[string]any{"id": call.Argument(0).Export()} }))
		}
		_ = api.Set(domain, domainAPI)
	}

	declaredViews := map[string]struct{}{}
	for _, view := range runtime.manifest.Contributes.Views {
		if view.Renderer == "declarative" {
			declaredViews[view.ID] = struct{}{}
		}
	}
	views := vm.NewObject()
	_ = views.Set("setState", func(call goja.FunctionCall) goja.Value {
		viewID := call.Argument(0).String()
		if _, ok := declaredViews[viewID]; !ok {
			panic(vm.NewGoError(fmt.Errorf("declarative view is not declared: %s", viewID)))
		}
		_, err := s.hostCall("views.setState", map[string]any{"extensionId": runtime.manifest.ID, "viewId": viewID, "state": call.Argument(1).Export()})
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return goja.Undefined()
	})
	_ = api.Set("views", views)

	configuration := vm.NewObject()
	_ = configuration.Set("get", func(call goja.FunctionCall) goja.Value {
		key := call.Argument(0).String()
		if value, ok := runtime.settings[key]; ok {
			return vm.ToValue(value)
		}
		return call.Argument(1)
	})
	_ = configuration.Set("onDidChange", func(call goja.FunctionCall) goja.Value {
		declared := false
		for _, event := range runtime.manifest.ActivationEvents {
			if strings.HasPrefix(event, "onConfiguration:") {
				declared = true
				break
			}
		}
		if !declared {
			panic(vm.NewGoError(fmt.Errorf("onDidChange requires an onConfiguration:<key> activation event")))
		}
		handler, ok := goja.AssertFunction(call.Argument(0))
		if !ok {
			panic(vm.NewGoError(fmt.Errorf("configuration change handler must be a function")))
		}
		wrapped := func(this goja.Value, args ...goja.Value) (goja.Value, error) {
			if len(args) == 0 {
				return handler(this, vm.NewArray())
			}
			payload, _ := args[0].Export().(map[string]any)
			return handler(this, vm.ToValue(payload["keys"]))
		}
		runtime.events["onConfiguration"] = append(runtime.events["onConfiguration"], wrapped)
		disposable := vm.NewObject()
		_ = disposable.Set("dispose", func(goja.FunctionCall) goja.Value { delete(runtime.events, "onConfiguration"); return goja.Undefined() })
		return disposable
	})
	_ = api.Set("configuration", configuration)

	for _, scope := range []string{"global", "workspace"} {
		scopeName := scope
		requiredPermission := scope + "State"
		state := vm.NewObject()
		_ = state.Set("get", func(call goja.FunctionCall) goja.Value {
			if !hasPermission(runtime.manifest.Permissions, requiredPermission) {
				panic(vm.NewGoError(fmt.Errorf("permission denied: %s", requiredPermission)))
			}
			result, err := s.hostCall("state.get", map[string]any{"extensionId": runtime.manifest.ID, "scope": scopeName, "key": call.Argument(0).String(), "fallback": call.Argument(1).Export()})
			if err != nil {
				panic(vm.NewGoError(err))
			}
			return vm.ToValue(result)
		})
		_ = state.Set("set", func(call goja.FunctionCall) goja.Value {
			if !hasPermission(runtime.manifest.Permissions, requiredPermission) {
				panic(vm.NewGoError(fmt.Errorf("permission denied: %s", requiredPermission)))
			}
			_, err := s.hostCall("state.set", map[string]any{"extensionId": runtime.manifest.ID, "scope": scopeName, "key": call.Argument(0).String(), "value": call.Argument(1).Export()})
			if err != nil {
				panic(vm.NewGoError(err))
			}
			return goja.Undefined()
		})
		_ = state.Set("delete", func(call goja.FunctionCall) goja.Value {
			if !hasPermission(runtime.manifest.Permissions, requiredPermission) {
				panic(vm.NewGoError(fmt.Errorf("permission denied: %s", requiredPermission)))
			}
			_, err := s.hostCall("state.delete", map[string]any{"extensionId": runtime.manifest.ID, "scope": scopeName, "key": call.Argument(0).String()})
			if err != nil {
				panic(vm.NewGoError(err))
			}
			return goja.Undefined()
		})
		_ = api.Set(scope+"State", state)
	}

	secrets := vm.NewObject()
	_ = secrets.Set("get", func(call goja.FunctionCall) goja.Value {
		if !hasPermission(runtime.manifest.Permissions, "secrets.own") {
			panic(vm.NewGoError(fmt.Errorf("permission denied: secrets.own")))
		}
		result, err := s.hostCall("secrets.get", map[string]any{"extensionId": runtime.manifest.ID, "key": call.Argument(0).String(), "fallback": call.Argument(1).Export()})
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(result)
	})
	_ = secrets.Set("set", func(call goja.FunctionCall) goja.Value {
		if !hasPermission(runtime.manifest.Permissions, "secrets.own") {
			panic(vm.NewGoError(fmt.Errorf("permission denied: secrets.own")))
		}
		_, err := s.hostCall("secrets.set", map[string]any{"extensionId": runtime.manifest.ID, "key": call.Argument(0).String(), "value": call.Argument(1).String()})
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return goja.Undefined()
	})
	_ = secrets.Set("delete", func(call goja.FunctionCall) goja.Value {
		if !hasPermission(runtime.manifest.Permissions, "secrets.own") {
			panic(vm.NewGoError(fmt.Errorf("permission denied: secrets.own")))
		}
		_, err := s.hostCall("secrets.delete", map[string]any{"extensionId": runtime.manifest.ID, "key": call.Argument(0).String()})
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return goja.Undefined()
	})
	_ = api.Set("secrets", secrets)

	logging := vm.NewObject()
	for _, level := range []string{"debug", "info", "warn", "error"} {
		logLevel := level
		_ = logging.Set(level, func(call goja.FunctionCall) goja.Value {
			_, err := s.hostCall("log.write", map[string]any{"extensionId": runtime.manifest.ID, "level": logLevel, "message": call.Argument(0).String(), "fields": call.Argument(1).Export()})
			if err != nil {
				panic(vm.NewGoError(err))
			}
			return goja.Undefined()
		})
	}
	_ = api.Set("logging", logging)

	window := vm.NewObject()
	_ = window.Set("notify", func(call goja.FunctionCall) goja.Value {
		if !hasPermission(runtime.manifest.Permissions, "notifications") {
			panic(vm.NewGoError(fmt.Errorf("permission denied: notifications")))
		}
		_, err := s.hostCall("window.notify", map[string]any{"extensionId": runtime.manifest.ID, "message": call.Argument(0).String(), "type": call.Argument(1).String()})
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return goja.Undefined()
	})
	_ = api.Set("window", window)
	return api, nil
}

func (s *hostServer) hostCall(method string, params interface{}) (interface{}, error) {
	id := fmt.Sprintf("child-%d", s.counter.Add(1))
	raw, err := marshalRPCValue(params)
	if err != nil {
		return nil, err
	}
	response := make(chan rpcMessage, 1)
	s.pendingMu.Lock()
	s.pending[id] = response
	s.pendingMu.Unlock()
	defer func() {
		s.pendingMu.Lock()
		delete(s.pending, id)
		s.pendingMu.Unlock()
	}()
	if err := s.write(rpcMessage{JSONRPC: "2.0", ID: id, Method: method, Params: raw, Token: s.token}); err != nil {
		return nil, err
	}
	select {
	case message := <-response:
		if message.Error != nil {
			return nil, fmt.Errorf("%s", message.Error.Message)
		}
		if len(message.Result) == 0 {
			return nil, nil
		}
		var result interface{}
		if err := json.Unmarshal(message.Result, &result); err != nil {
			return nil, err
		}
		return result, nil
	case <-time.After(10 * time.Second):
		return nil, fmt.Errorf("host API call timed out: %s", method)
	case <-s.done:
		return nil, fmt.Errorf("extension host closed")
	}
}

func (s *hostServer) write(message rpcMessage) error {
	data, err := json.Marshal(message)
	if err != nil {
		return err
	}
	if len(data) > maxRPCMessageBytes {
		return fmt.Errorf("RPC message exceeds %d bytes", maxRPCMessageBytes)
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err = s.output.Write(append(data, '\n'))
	return err
}

func runWithTimeout(vm *goja.Runtime, timeout time.Duration, operation func() error) error {
	timer := time.AfterFunc(timeout, func() { vm.Interrupt("extension execution timed out") })
	defer timer.Stop()
	err := operation()
	vm.ClearInterrupt()
	return err
}

func installRuntimeTimers(runtime *extensionRuntime) error {
	vm := runtime.vm
	if err := vm.Set("setTimeout", func(call goja.FunctionCall) goja.Value {
		callback, ok := goja.AssertFunction(call.Argument(0))
		if !ok {
			panic(vm.NewGoError(fmt.Errorf("setTimeout callback must be a function")))
		}
		delay := call.Argument(1).ToInteger()
		if delay < 0 {
			delay = 0
		}
		if delay > 60_000 {
			delay = 60_000
		}
		runtime.nextTimerID++
		id := runtime.nextTimerID
		args := append([]goja.Value(nil), call.Arguments[2:]...)
		runtime.timers[id] = runtimeTimer{due: time.Now().Add(time.Duration(delay) * time.Millisecond), callback: callback, args: args}
		return vm.ToValue(id)
	}); err != nil {
		return err
	}
	return vm.Set("clearTimeout", func(call goja.FunctionCall) goja.Value {
		delete(runtime.timers, call.Argument(0).ToInteger())
		return goja.Undefined()
	})
}

func resolveRuntimePromise(runtime *extensionRuntime, value goja.Value, pendingMessage string) (goja.Value, error) {
	promise, ok := value.Export().(*goja.Promise)
	if !ok {
		return value, nil
	}
	for promise.State() == goja.PromiseStatePending {
		var nextID int64
		var next runtimeTimer
		for id, timer := range runtime.timers {
			if nextID == 0 || timer.due.Before(next.due) {
				nextID, next = id, timer
			}
		}
		if nextID == 0 {
			return value, fmt.Errorf("%s", pendingMessage)
		}
		if wait := time.Until(next.due); wait > 0 {
			if wait > 5*time.Millisecond {
				wait = 5 * time.Millisecond
			}
			time.Sleep(wait)
			continue
		}
		delete(runtime.timers, nextID)
		if _, err := next.callback(goja.Undefined(), next.args...); err != nil {
			return value, err
		}
	}
	if promise.State() == goja.PromiseStateRejected {
		return promise.Result(), fmt.Errorf("%s", promise.Result().String())
	}
	return promise.Result(), nil
}

func transformV2Module(source string) string {
	exported := []string{}
	source = v2ExportFunctionPattern.ReplaceAllStringFunc(source, func(match string) string {
		parts := v2ExportFunctionPattern.FindStringSubmatch(match)
		exported = append(exported, parts[3])
		if parts[2] == "async" {
			return parts[1] + "async function " + parts[3]
		}
		return parts[1] + "function " + parts[3]
	})
	source = v2ExportVariablePattern.ReplaceAllStringFunc(source, func(match string) string {
		parts := v2ExportVariablePattern.FindStringSubmatch(match)
		exported = append(exported, parts[3])
		return parts[1] + parts[2] + " " + parts[3]
	})
	for _, name := range exported {
		source += fmt.Sprintf("\nmodule.exports[%q] = %s;", name, name)
	}
	return source
}

func manifestHasActivation(manifest Manifest, event string) bool {
	for _, activation := range manifest.ActivationEvents {
		if activation == event {
			return true
		}
	}
	return false
}

func eventPermission(event string) string {
	switch event {
	case "onRequest", "onSend":
		return "requests.read"
	case "onResponse":
		return "responses.read"
	case "onEnvChange":
		return "environments.read"
	case "onTabOpen", "onTabClose":
		return "tabs.read"
	case "onWorkspaceOpen", "onWorkspaceClose":
		return "workspace.read"
	case "onAssertions":
		return "assertions.provide"
	case "onVariables":
		return "variables.provide"
	case "onBrowserNetwork":
		return "browserDebug.read"
	case "onMockHit":
		return "mock.read"
	case "onProxyTraffic":
		return "proxy.read"
	case "onFlowProgress", "onFlowComplete":
		return "flows.execute"
	case "onDatabaseComplete":
		return "databases.execute"
	case "onBrokerPublishComplete":
		return "brokers.publish"
	case "onDocumentReadComplete":
		return "documents.readContents"
	case "onDocumentWriteComplete":
		return "documents.write"
	case "onAIComplete":
		return "ai.execute"
	default:
		return ""
	}
}

func hasPermission(permissions []string, wanted string) bool {
	for _, permission := range permissions {
		if permission == wanted {
			return true
		}
	}
	return false
}

func executionFailure(started time.Time, err error) HostExecutionResult {
	return HostExecutionResult{Success: false, Error: err.Error(), TimeMS: elapsedMS(started)}
}

func elapsedMS(started time.Time) float64 {
	return float64(time.Since(started).Microseconds()) / 1000
}
