package extensions

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"testing"
	"time"
)

func TestExtensionHostActivatesCommandsStateNotificationsAndEvents(t *testing.T) {
	t.Setenv("ADOMNIA_EXTENSION_HOST_TOKEN", "0123456789abcdef0123456789abcdef")
	parentInput, childOutput := io.Pipe()
	childInput, parentOutput := io.Pipe()

	var mu sync.Mutex
	state := map[string]interface{}{}
	secrets := map[string]string{}
	notifications := []string{}
	logs := []string{}
	handler := func(_ context.Context, method string, raw json.RawMessage) (interface{}, error) {
		var params map[string]interface{}
		_ = json.Unmarshal(raw, &params)
		mu.Lock()
		defer mu.Unlock()
		switch method {
		case "state.get":
			if value, ok := state[params["key"].(string)]; ok {
				return value, nil
			}
			return params["fallback"], nil
		case "state.set":
			state[params["key"].(string)] = params["value"]
			return map[string]bool{"ok": true}, nil
		case "state.delete":
			delete(state, params["key"].(string))
			return map[string]bool{"ok": true}, nil
		case "secrets.get":
			if value, ok := secrets[params["key"].(string)]; ok {
				return value, nil
			}
			return params["fallback"], nil
		case "secrets.set":
			secrets[params["key"].(string)] = params["value"].(string)
			return map[string]bool{"ok": true}, nil
		case "secrets.delete":
			delete(secrets, params["key"].(string))
			return map[string]bool{"ok": true}, nil
		case "window.notify":
			notifications = append(notifications, params["message"].(string))
			return map[string]bool{"ok": true}, nil
		case "domains.get":
			return []map[string]any{{"id": "collection-1"}}, nil
		case "mock.getSnapshot":
			return map[string]any{"running": false, "hits": []any{}}, nil
		case "proxy.getSnapshot":
			return map[string]any{"running": false, "entries": []any{}}, nil
		case "documents.startReadText":
			return map[string]any{"jobId": "document-read-1"}, nil
		case "documents.startExport":
			return map[string]any{"jobId": "document-write-1"}, nil
		case "documents.cancel":
			return map[string]bool{"accepted": true}, nil
		case "documents.listPdfProjects":
			return []map[string]any{{"id": "pdf-1"}}, nil
		case "databases.startQuery":
			return map[string]any{"jobId": "database-1"}, nil
		case "databases.cancel":
			return map[string]bool{"accepted": true}, nil
		case "databases.listConnections":
			return []map[string]any{{"id": "db-1"}}, nil
		case "brokers.startPublish":
			return map[string]any{"jobId": "broker-job-1"}, nil
		case "brokers.cancel":
			return map[string]bool{"accepted": true}, nil
		case "brokers.listConnections":
			return []map[string]any{{"id": "broker-1"}}, nil
		case "flows.start":
			return map[string]any{"jobId": "job-1"}, nil
		case "flows.startStress":
			return map[string]any{"jobId": "stress-1"}, nil
		case "flows.cancel":
			return map[string]bool{"accepted": true}, nil
		case "flows.list":
			return []map[string]any{{"id": "flow-1"}}, nil
		case "flows.get":
			return map[string]any{"id": "flow-1"}, nil
		case "mock.clearHits", "mock.stop", "proxy.clearTraffic", "proxy.stop":
			return map[string]bool{"ok": true}, nil
		case "variables.getAll":
			return map[string]string{"baseUrl": "https://example.test"}, nil
		case "variables.resolve":
			return "https://example.test/users", nil
		case "log.write":
			logs = append(logs, params["message"].(string))
			return map[string]bool{"ok": true}, nil
		default:
			return nil, nil
		}
	}
	serverDone := make(chan error, 1)
	go func() { serverDone <- RunExtensionHost(childInput, childOutput) }()
	client := NewHostClient(parentInput, parentOutput, "0123456789abcdef0123456789abcdef", handler)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var initialized InitializeHostResult
	if err := client.Request(ctx, "initialize", InitializeHostRequest{ProtocolVersion: HostProtocolVersion}, &initialized); err != nil {
		t.Fatalf("initialize error = %v", err)
	}
	manifest := validTestManifest()
	manifest.Permissions = []string{"notifications", "globalState", "secrets.own", "responses.read", "collections.read", "variables.read", "variables.provide", "assertions.provide", "browserDebug.read", "browserDebug.control", "mock.read", "mock.control", "proxy.read", "proxy.control", "flows.read", "flows.execute", "databases.read", "databases.execute", "brokers.read", "brokers.publish", "documents.read", "documents.readContents", "documents.write"}
	manifest.ActivationEvents = append(manifest.ActivationEvents, "onResponse", "onAssertions", "onVariables", "onBrowserNetwork", "onMockHit", "onProxyTraffic", "onFlowProgress", "onFlowComplete", "onDatabaseComplete", "onBrokerPublishComplete", "onDocumentReadComplete", "onDocumentWriteComplete")
	source := `
		export async function activate(api) {
			api.context.subscriptions.add({ dispose() { api.logging.info('disposed') } })
			api.context.subscriptions.add(api.commands.registerCommand("test.extension.run", async () => {
				await new Promise((resolve) => setTimeout(resolve, 5))
				const count = await api.globalState.get("count", 0)
				await api.globalState.set("count", count + 1)
				await api.window.notify("ran", "success")
				await api.secrets.set("token", "local-secret")
				const secret = await api.secrets.get("token")
				const collections = await api.collections.list()
				const browserEntries = await api.browserDebug.list()
				await api.browserDebug.clear()
				const mockSnapshot = await api.mock.getSnapshot()
				const proxySnapshot = await api.proxy.getSnapshot()
				await api.mock.clearHits()
				await api.proxy.clearTraffic()
				const documents = await api.documents.listPdfProjects()
				const documentRead = await api.documents.readPdfText('pdf-1')
				const documentWrite = await api.documents.exportPdf('pdf-1')
				const databases = await api.databases.listConnections()
				const databaseJob = await api.databases.execute('db-1', 'SELECT 1')
				const brokers = await api.brokers.listConnections()
				const brokerJob = await api.brokers.publish('broker-1', 'events', 'hello')
				const flows = await api.flows.list()
				const flow = await api.flows.get('flow-1')
				const flowJob = await api.flows.execute('flow-1')
				const stressJob = await api.flows.executeStress('flow-1', { vus: 1, mode: 'iterations', iterations: 1 })
				const resolved = await api.variables.resolve('{{baseUrl}}/users')
				return { count: count + 1, collectionCount: collections.length, browserCount: browserEntries.length, mockRunning: mockSnapshot.running, proxyRunning: proxySnapshot.running, documentCount: documents.length, documentReadJobId: documentRead.jobId, documentWriteJobId: documentWrite.jobId, databaseCount: databases.length, databaseJobId: databaseJob.jobId, brokerCount: brokers.length, brokerJobId: brokerJob.jobId, flowCount: flows.length, flowId: flow.id, flowJobId: flowJob.jobId, stressJobId: stressJob.jobId, resolved, secret }
			}))
			api.context.subscriptions.add(api.variables.registerProvider('test.extension.dynamic', (context) => ({ providedUrl: context.url })))
			api.context.subscriptions.add(api.assertions.registerProvider('test.extension.status', (payload) => ({
				label: 'Status is successful', passed: payload.response.status < 400,
				actual: String(payload.response.status), expected: '< 400'
			})))
			api.events.onBrowserNetwork(() => undefined)
			api.events.onMockHit(() => undefined)
			api.events.onProxyTraffic(() => undefined)
			api.events.onFlowProgress(() => undefined)
			api.events.onFlowComplete((payload) => { payload.received = true; return { modified: true, data: payload } })
			api.events.onDatabaseComplete(() => undefined)
			api.events.onBrokerPublishComplete(() => undefined)
			api.events.onDocumentReadComplete(() => undefined)
			api.events.onDocumentWriteComplete(() => undefined)
			api.context.subscriptions.add(api.events.onResponse((payload) => {
				payload.tagged = true
				return { modified: true, data: payload }
			}))
		}
	`
	var activation HostExecutionResult
	if err := client.Request(ctx, "activate", ActivateHostRequest{Manifest: manifest, Settings: map[string]any{}, Source: source}, &activation); err != nil || !activation.Success {
		t.Fatalf("activate result=%#v err=%v", activation, err)
	}
	var command HostExecutionResult
	if err := client.Request(ctx, "executeCommand", ExecuteCommandRequest{ExtensionID: manifest.ID, CommandID: "test.extension.run", Source: "test"}, &command); err != nil || !command.Success {
		t.Fatalf("command result=%#v err=%v", command, err)
	}
	data := command.Data.(map[string]interface{})
	if data["count"] != float64(1) || data["collectionCount"] != int64(1) && data["collectionCount"] != float64(1) || data["browserCount"] != int64(1) && data["browserCount"] != float64(1) || data["mockRunning"] != false || data["proxyRunning"] != false || data["documentCount"] != int64(1) && data["documentCount"] != float64(1) || data["documentReadJobId"] != "document-read-1" || data["documentWriteJobId"] != "document-write-1" || data["databaseCount"] != int64(1) && data["databaseCount"] != float64(1) || data["databaseJobId"] != "database-1" || data["brokerCount"] != int64(1) && data["brokerCount"] != float64(1) || data["brokerJobId"] != "broker-job-1" || data["flowCount"] != int64(1) && data["flowCount"] != float64(1) || data["flowId"] != "flow-1" || data["flowJobId"] != "job-1" || data["stressJobId"] != "stress-1" || data["resolved"] != "https://example.test/users" || data["secret"] != "local-secret" {
		t.Fatalf("command data = %#v", data)
	}
	var event HostExecutionResult
	if err := client.Request(ctx, "dispatchEvent", DispatchEventRequest{ExtensionID: manifest.ID, Event: "onResponse", Payload: map[string]any{"status": 200}}, &event); err != nil || !event.Success || !event.Modified {
		t.Fatalf("event result=%#v err=%v", event, err)
	}
	eventData := event.Data.(map[string]interface{})
	if eventData["tagged"] != true {
		t.Fatalf("event data = %#v", eventData)
	}
	var flowEvent HostExecutionResult
	if err := client.Request(ctx, "dispatchEvent", DispatchEventRequest{ExtensionID: manifest.ID, Event: "onFlowComplete", Payload: map[string]any{"jobId": "job-1"}}, &flowEvent); err != nil || !flowEvent.Success || !flowEvent.Modified || flowEvent.Data.(map[string]any)["received"] != true {
		t.Fatalf("flow event result=%#v err=%v", flowEvent, err)
	}
	var databaseEvent HostExecutionResult
	if err := client.Request(ctx, "dispatchEvent", DispatchEventRequest{ExtensionID: manifest.ID, Event: "onDatabaseComplete", Payload: map[string]any{"jobId": "database-1", "success": true}}, &databaseEvent); err != nil || !databaseEvent.Success {
		t.Fatalf("database event result=%#v err=%v", databaseEvent, err)
	}
	var brokerEvent HostExecutionResult
	if err := client.Request(ctx, "dispatchEvent", DispatchEventRequest{ExtensionID: manifest.ID, Event: "onBrokerPublishComplete", Payload: map[string]any{"jobId": "broker-job-1", "success": true}}, &brokerEvent); err != nil || !brokerEvent.Success {
		t.Fatalf("broker event result=%#v err=%v", brokerEvent, err)
	}
	for _, documentEvent := range []string{"onDocumentReadComplete", "onDocumentWriteComplete"} {
		var result HostExecutionResult
		if err := client.Request(ctx, "dispatchEvent", DispatchEventRequest{ExtensionID: manifest.ID, Event: documentEvent, Payload: map[string]any{"success": true}}, &result); err != nil || !result.Success {
			t.Fatalf("%s result=%#v err=%v", documentEvent, result, err)
		}
	}
	var variableResults HostExecutionResult
	if err := client.Request(ctx, "evaluateVariableProviders", EvaluateVariableProvidersRequest{ExtensionID: manifest.ID, Context: map[string]any{"url": "https://provided.test"}}, &variableResults); err != nil || !variableResults.Success {
		t.Fatalf("variable providers result=%#v err=%v", variableResults, err)
	}
	encodedVariables, _ := json.Marshal(variableResults.Data)
	var provided []VariableProviderResult
	if err := json.Unmarshal(encodedVariables, &provided); err != nil || len(provided) != 1 || provided[0].Values["providedUrl"] != "https://provided.test" || provided[0].ProviderID != "test.extension.dynamic" {
		t.Fatalf("variable provider results=%#v err=%v", provided, err)
	}
	var providerResults HostExecutionResult
	if err := client.Request(ctx, "evaluateAssertions", EvaluateAssertionsRequest{ExtensionID: manifest.ID, Payload: map[string]any{"response": map[string]any{"status": 204}}}, &providerResults); err != nil || !providerResults.Success {
		t.Fatalf("assertion providers result=%#v err=%v", providerResults, err)
	}
	encodedResults, _ := json.Marshal(providerResults.Data)
	var assertions []AssertionProviderResult
	if err := json.Unmarshal(encodedResults, &assertions); err != nil || len(assertions) != 1 || !assertions[0].Passed || assertions[0].ProviderID != "test.extension.status" {
		t.Fatalf("assertion provider results=%#v err=%v", assertions, err)
	}
	mu.Lock()
	if state["count"] != float64(1) || secrets["token"] != "local-secret" || len(notifications) != 1 || notifications[0] != "ran" {
		t.Fatalf("state=%#v secrets=%#v notifications=%#v", state, secrets, notifications)
	}
	mu.Unlock()
	var deactivated map[string]bool
	if err := client.Request(ctx, "deactivate", DeactivateHostRequest{ExtensionID: manifest.ID}, &deactivated); err != nil {
		t.Fatalf("deactivate error = %v", err)
	}
	mu.Lock()
	if len(logs) != 1 || logs[0] != "disposed" {
		t.Fatalf("disposable logs=%#v", logs)
	}
	mu.Unlock()

	if err := client.Close(); err != nil {
		t.Fatalf("client.Close() error = %v", err)
	}
	select {
	case err := <-serverDone:
		if err != nil {
			t.Fatalf("server error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("extension host did not stop")
	}
}

func TestExtensionHostCancellationInterruptsJavaScriptAndKeepsHostUsable(t *testing.T) {
	t.Setenv("ADOMNIA_EXTENSION_HOST_TOKEN", "fedcba9876543210fedcba9876543210")
	parentInput, childOutput := io.Pipe()
	childInput, parentOutput := io.Pipe()
	serverDone := make(chan error, 1)
	go func() { serverDone <- RunExtensionHost(childInput, childOutput) }()
	client := NewHostClient(parentInput, parentOutput, "fedcba9876543210fedcba9876543210", nil)
	defer func() { _ = client.Close(); <-serverDone }()

	manifest := validTestManifest()
	manifest.Contributes.Commands = append(manifest.Contributes.Commands, CommandContribution{ID: "test.extension.ping", Title: "Ping"})
	manifest.ActivationEvents = append(manifest.ActivationEvents, "onCommand:test.extension.ping")
	source := `export function activate(api) {
		api.commands.registerCommand("test.extension.run", () => { while (true) {} })
		api.commands.registerCommand("test.extension.ping", () => ({ pong: true }))
	}`
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var activation HostExecutionResult
	if err := client.Request(ctx, "activate", ActivateHostRequest{Manifest: manifest, Source: source}, &activation); err != nil || !activation.Success {
		t.Fatalf("activate result=%#v err=%v", activation, err)
	}

	commandCtx, commandCancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	err := client.Request(commandCtx, "executeCommand", ExecuteCommandRequest{ExtensionID: manifest.ID, CommandID: "test.extension.run"}, &HostExecutionResult{})
	commandCancel()
	if err == nil {
		t.Fatal("infinite command was not cancelled")
	}

	var ping HostExecutionResult
	if err := client.Request(ctx, "executeCommand", ExecuteCommandRequest{ExtensionID: manifest.ID, CommandID: "test.extension.ping"}, &ping); err != nil || !ping.Success {
		t.Fatalf("host unusable after cancellation: result=%#v err=%v", ping, err)
	}
}

func TestExtensionHostReturnsStructuredMethodError(t *testing.T) {
	t.Setenv("ADOMNIA_EXTENSION_HOST_TOKEN", "00112233445566778899aabbccddeeff")
	parentInput, childOutput := io.Pipe()
	childInput, parentOutput := io.Pipe()
	serverDone := make(chan error, 1)
	go func() { serverDone <- RunExtensionHost(childInput, childOutput) }()
	client := NewHostClient(parentInput, parentOutput, "00112233445566778899aabbccddeeff", nil)
	defer func() { _ = client.Close(); <-serverDone }()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err := client.Request(ctx, "missingMethod", map[string]any{}, nil)
	var rpcErr *RPCError
	if !errors.As(err, &rpcErr) || rpcErr.Code != -32601 {
		t.Fatalf("structured error = %#v", err)
	}
}

func TestExtensionHostRejectsUndeclaredCommand(t *testing.T) {
	t.Setenv("ADOMNIA_EXTENSION_HOST_TOKEN", "abcdef0123456789abcdef0123456789")
	parentInput, childOutput := io.Pipe()
	childInput, parentOutput := io.Pipe()
	serverDone := make(chan error, 1)
	go func() { serverDone <- RunExtensionHost(childInput, childOutput) }()
	client := NewHostClient(parentInput, parentOutput, "abcdef0123456789abcdef0123456789", nil)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var activation HostExecutionResult
	source := `export function activate(api) { api.commands.registerCommand("other.command", () => true) }`
	if err := client.Request(ctx, "activate", ActivateHostRequest{Manifest: validTestManifest(), Source: source}, &activation); err != nil {
		t.Fatal(err)
	}
	if activation.Success {
		t.Fatalf("activation unexpectedly succeeded: %#v", activation)
	}
	_ = client.Close()
	<-serverDone
}
