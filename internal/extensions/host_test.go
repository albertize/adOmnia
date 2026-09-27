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
	notifications := []string{}
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
		case "window.notify":
			notifications = append(notifications, params["message"].(string))
			return map[string]bool{"ok": true}, nil
		case "domains.get":
			return []map[string]any{{"id": "collection-1"}}, nil
		case "variables.getAll":
			return map[string]string{"baseUrl": "https://example.test"}, nil
		case "variables.resolve":
			return "https://example.test/users", nil
		case "log.write":
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
	manifest.Permissions = []string{"notifications", "globalState", "responses.read", "collections.read", "variables.read"}
	manifest.ActivationEvents = append(manifest.ActivationEvents, "onResponse")
	source := `
		export async function activate(api) {
			api.context.subscriptions.add(api.commands.registerCommand("test.extension.run", async () => {
				const count = await api.globalState.get("count", 0)
				await api.globalState.set("count", count + 1)
				await api.window.notify("ran", "success")
				const collections = await api.collections.list()
				const resolved = await api.variables.resolve('{{baseUrl}}/users')
				return { count: count + 1, collectionCount: collections.length, resolved }
			}))
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
	if data["count"] != float64(1) || data["collectionCount"] != int64(1) && data["collectionCount"] != float64(1) || data["resolved"] != "https://example.test/users" {
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
	mu.Lock()
	if state["count"] != float64(1) || len(notifications) != 1 || notifications[0] != "ran" {
		t.Fatalf("state=%#v notifications=%#v", state, notifications)
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
