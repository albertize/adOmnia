package extensions

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"adomnia/internal/storage"
	"adomnia/internal/vault"
)

func TestServiceRejectsCorruptedAndOversizedState(t *testing.T) {
	if storage.DB() != nil {
		storage.Close()
	}
	dataDir := t.TempDir()
	if err := storage.Open(dataDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(storage.Close)
	service := NewService(dataDir)
	instance := ExtensionInstance{Manifest: Manifest{ID: "test.state", Permissions: []string{"globalState"}}, Grants: []string{"globalState"}}
	if err := storage.Put(extensionsBucket, "state/global/test.state/broken", []byte("{not-json")); err != nil {
		t.Fatal(err)
	}
	if _, err := service.handleStateCall(instance, "state.get", map[string]any{"scope": "global", "key": "broken"}); err == nil {
		t.Fatal("corrupted state was accepted")
	}
	oversized := strings.Repeat("x", maxStateValueBytes+1)
	if _, err := service.handleStateCall(instance, "state.set", map[string]any{"scope": "global", "key": "large", "value": oversized}); err == nil {
		t.Fatal("oversized state was accepted")
	}
	if _, err := service.handleStateCall(instance, "state.set", map[string]any{"scope": "global", "key": "healthy", "value": "ok"}); err != nil {
		t.Fatalf("service unusable after bad state: %v", err)
	}
}

func TestServiceStoresExtensionSecretsEncryptedAndVaultBound(t *testing.T) {
	if storage.DB() != nil {
		storage.Close()
	}
	dataDir := t.TempDir()
	if err := storage.Open(dataDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(storage.Close)
	vault.Lock()
	t.Cleanup(vault.Lock)
	if err := vault.Unlock("extension-test-passphrase"); err != nil {
		t.Fatal(err)
	}
	service := NewService(dataDir)
	instance := ExtensionInstance{Manifest: Manifest{ID: "test.secret", Permissions: []string{"secrets.own"}}, Grants: []string{"secrets.own"}}
	if _, err := service.handleSecretCall(instance, "secrets.set", map[string]interface{}{"key": "token", "value": "plain-secret"}); err != nil {
		t.Fatal(err)
	}
	ciphertext, err := storage.Get(extensionsBucket, "secret/test.secret/token")
	if err != nil {
		t.Fatal(err)
	}
	if len(ciphertext) == 0 || strings.Contains(string(ciphertext), "plain-secret") {
		t.Fatalf("secret was not encrypted at rest: %q", ciphertext)
	}
	value, err := service.handleSecretCall(instance, "secrets.get", map[string]interface{}{"key": "token"})
	if err != nil || value != "plain-secret" {
		t.Fatalf("secret=%#v err=%v", value, err)
	}
	vault.Lock()
	if _, err := service.handleSecretCall(instance, "secrets.get", map[string]interface{}{"key": "token"}); err == nil {
		t.Fatal("secret was available while the Vault was locked")
	}
	if _, err := service.handleSecretCall(ExtensionInstance{Manifest: Manifest{ID: "test.denied"}}, "secrets.get", map[string]interface{}{"key": "token"}); err == nil {
		t.Fatal("secret permission was not enforced")
	}
}

func TestServiceBrokersGrantedDomainActions(t *testing.T) {
	if storage.DB() != nil {
		storage.Close()
	}
	dataDir := t.TempDir()
	if err := storage.Open(dataDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(storage.Close)
	root := writeValidExtension(t)
	manifest, err := LoadManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	manifest.Permissions = append(manifest.Permissions, "tabs.write", "browserDebug.control", "mock.read", "mock.control", "proxy.read", "proxy.control")
	data, _ := json.MarshalIndent(manifest, "", "  ")
	if err := os.WriteFile(filepath.Join(root, ManifestFileName), data, 0644); err != nil {
		t.Fatal(err)
	}
	service := NewService(dataDir)
	if err := service.Init(); err != nil {
		t.Fatal(err)
	}
	installed, err := service.InstallDirectory(root, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetGrants(installed.Manifest.ID, installed.Manifest.Permissions); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Enable(installed.Manifest.ID); err != nil {
		t.Fatal(err)
	}
	var received ExtensionDomainAction
	ConfigureDomainActionNotifier(service, func(action ExtensionDomainAction) { received = action })
	params, _ := json.Marshal(map[string]any{"extensionId": installed.Manifest.ID, "domain": "tabs", "action": "close", "payload": map[string]any{"id": "tab-1"}})
	if _, err := service.handleHostCall(context.Background(), "domains.action", params); err != nil {
		t.Fatal(err)
	}
	if received.ExtensionID != installed.Manifest.ID || received.Domain != "tabs" || received.Action != "close" || received.Payload["id"] != "tab-1" {
		t.Fatalf("action=%#v", received)
	}
	params, _ = json.Marshal(map[string]any{"extensionId": installed.Manifest.ID, "domain": "browserDebug", "action": "clear", "payload": map[string]any{}})
	if _, err := service.handleHostCall(context.Background(), "domains.action", params); err != nil {
		t.Fatal(err)
	}
	if received.Domain != "browserDebug" || received.Action != "clear" {
		t.Fatalf("browser action=%#v", received)
	}
	for _, method := range []string{"mock.getSnapshot", "mock.clearHits", "mock.stop", "proxy.getSnapshot", "proxy.clearTraffic", "proxy.stop"} {
		params, _ = json.Marshal(map[string]any{"extensionId": installed.Manifest.ID})
		if _, err := service.handleHostCall(context.Background(), method, params); err != nil {
			t.Fatalf("%s: %v", method, err)
		}
	}
	params, _ = json.Marshal(map[string]any{"extensionId": installed.Manifest.ID, "domain": "tabs", "action": "deleteAll", "payload": map[string]any{}})
	if _, err := service.handleHostCall(context.Background(), "domains.action", params); err == nil {
		t.Fatal("unsupported domain action accepted")
	}
}

func TestRedactEnvironmentVariablesRemovesValues(t *testing.T) {
	value := map[string]any{
		"active": map[string]any{"variables": []any{map[string]any{"key": "token", "value": "secret"}}},
		"items":  []any{map[string]any{"variables": []any{map[string]any{"key": "url", "value": "private"}}}},
	}
	redactEnvironmentVariables(value)
	active := value["active"].(map[string]any)["variables"].([]any)[0].(map[string]any)
	item := value["items"].([]any)[0].(map[string]any)["variables"].([]any)[0].(map[string]any)
	if active["value"] != "" || item["value"] != "" {
		t.Fatalf("values were not redacted: %#v", value)
	}
}

func TestServiceRecoversAfterExtensionHostTermination(t *testing.T) {
	if storage.DB() != nil {
		storage.Close()
	}
	dataDir := t.TempDir()
	if err := storage.Open(dataDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(storage.Close)
	root := filepath.Join(t.TempDir(), "recover")
	if _, err := Scaffold(root, ScaffoldOptions{ID: "test.recover", Name: "Recover", Publisher: "test", Template: "minimal"}); err != nil {
		t.Fatal(err)
	}
	service := NewService(dataDir)
	if err := service.Init(); err != nil {
		t.Fatal(err)
	}
	service.executable = buildTestDesktopExecutable(t)
	t.Cleanup(service.Shutdown)
	installed, err := service.InstallDirectory(root, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetGrants(installed.Manifest.ID, installed.Manifest.Permissions); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Enable(installed.Manifest.ID); err != nil {
		t.Fatal(err)
	}
	if result, err := service.ExecuteCommand(installed.Manifest.ID, "test.recover.hello", "{}", "test"); err != nil || !result.Success {
		t.Fatalf("first command result=%#v err=%v", result, err)
	}
	service.mu.Lock()
	oldHost := service.host
	service.mu.Unlock()
	if oldHost == nil || oldHost.process == nil {
		t.Fatal("host process was not started")
	}
	if err := oldHost.process.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-oldHost.closed:
	case <-time.After(3 * time.Second):
		t.Fatal("host did not report termination")
	}
	_ = oldHost.Close()
	if result, err := service.ExecuteCommand(installed.Manifest.ID, "test.recover.hello", "{}", "test"); err != nil || !result.Success {
		t.Fatalf("recovered command result=%#v err=%v", result, err)
	}
}

func TestServiceDispatchesWorkbenchAndDomainLifecycleEvents(t *testing.T) {
	if storage.DB() != nil {
		storage.Close()
	}
	dataDir := t.TempDir()
	if err := storage.Open(dataDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(storage.Close)
	root := filepath.Join(t.TempDir(), "events")
	if _, err := Scaffold(root, ScaffoldOptions{ID: "test.events", Name: "Events", Publisher: "test", Template: "minimal"}); err != nil {
		t.Fatal(err)
	}
	manifest, err := LoadManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	manifest.ActivationEvents = []string{"onStartup", "onCommand:test.events.seen", "onWorkspaceOpen", "onWorkspaceClose", "onEnvChange", "onThemeChange", "onTabOpen", "onTabClose", "onSave", "onImport", "onExport", "onAssertions", "onVariables", "onBrowserNetwork"}
	manifest.Permissions = []string{"workspace.read", "environments.read", "tabs.read", "assertions.provide", "variables.provide", "browserDebug.read"}
	manifest.Contributes.Commands = []CommandContribution{{ID: "test.events.seen", Title: "Seen events"}}
	manifestData, _ := json.MarshalIndent(manifest, "", "  ")
	if err := os.WriteFile(filepath.Join(root, ManifestFileName), manifestData, 0644); err != nil {
		t.Fatal(err)
	}
	source := `
let seen = []
export function activate(api) {
  const record = (event) => (payload) => { seen.push({ event, payload }) }
  api.events.onWorkspaceOpen(record('onWorkspaceOpen'))
  api.events.onWorkspaceClose(record('onWorkspaceClose'))
  api.events.onEnvironmentChange(record('onEnvChange'))
  api.events.onThemeChange(record('onThemeChange'))
  api.events.onTabOpen(record('onTabOpen'))
  api.events.onTabClose(record('onTabClose'))
  api.events.onSave(record('onSave'))
  api.events.onImport(record('onImport'))
  api.events.onExport(record('onExport'))
  api.events.onBrowserNetwork(record('onBrowserNetwork'))
  api.variables.registerProvider('test.events.dynamic', (context) => ({ providedHost: context.host }))
  api.assertions.registerProvider('test.events.status', (payload) => ({ label: 'Status below 400', passed: payload.response.status < 400, actual: String(payload.response.status), expected: '< 400' }))
  api.commands.registerCommand('test.events.seen', () => ({ events: seen }))
}`
	if err := os.WriteFile(filepath.Join(root, manifest.Main), []byte(source), 0644); err != nil {
		t.Fatal(err)
	}

	service := NewService(dataDir)
	if err := service.Init(); err != nil {
		t.Fatal(err)
	}
	service.executable = buildTestDesktopExecutable(t)
	t.Cleanup(service.Shutdown)
	installed, err := service.InstallDirectory(root, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetGrants(installed.Manifest.ID, installed.Manifest.Permissions); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Enable(installed.Manifest.ID); err != nil {
		t.Fatal(err)
	}

	setContext := func(value map[string]any) {
		data, marshalErr := json.Marshal(value)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		if err := service.SetDomainContext(string(data)); err != nil {
			t.Fatal(err)
		}
	}
	setContext(map[string]any{
		"workspace":    map[string]any{"active": map[string]any{"id": "workspace-1"}},
		"environments": map[string]any{"active": map[string]any{"id": "env-1"}},
		"theme":        map[string]any{"id": "dark"},
		"tabs":         map[string]any{"items": []any{map[string]any{"id": "tab-1"}}},
		"browserDebug": map[string]any{"items": []any{map[string]any{"id": "browser-1", "url": "https://one.test"}}},
	})
	waitForExtensionEvents(t, service, installed.Manifest.ID, 5)
	setContext(map[string]any{
		"workspace":    map[string]any{"active": map[string]any{"id": "workspace-2"}},
		"environments": map[string]any{"active": map[string]any{"id": "env-2"}},
		"theme":        map[string]any{"id": "light"},
		"tabs":         map[string]any{"items": []any{map[string]any{"id": "tab-2"}}},
		"browserDebug": map[string]any{"items": []any{map[string]any{"id": "browser-1", "url": "https://one.test"}, map[string]any{"id": "browser-2", "url": "https://two.test"}}},
	})
	waitForExtensionEvents(t, service, installed.Manifest.ID, 12)
	for _, event := range []string{"onSave", "onImport", "onExport"} {
		payload, _ := json.Marshal(map[string]any{"source": "integration-test"})
		if err := service.NotifyWorkbenchEvent(event, string(payload)); err != nil {
			t.Fatalf("notify %s: %v", event, err)
		}
	}
	events := waitForExtensionEvents(t, service, installed.Manifest.ID, 15)
	seen := map[string]bool{}
	for _, item := range events {
		if entry, ok := item.(map[string]any); ok {
			if name, ok := entry["event"].(string); ok {
				seen[name] = true
			}
		}
	}
	for _, expected := range []string{"onWorkspaceOpen", "onWorkspaceClose", "onEnvChange", "onThemeChange", "onTabOpen", "onTabClose", "onBrowserNetwork", "onSave", "onImport", "onExport"} {
		if !seen[expected] {
			t.Errorf("event %s was not dispatched; got %#v", expected, events)
		}
	}
	if err := service.NotifyWorkbenchEvent("onUnexpected", `{}`); err == nil {
		t.Fatal("unsupported workbench event accepted")
	}
	variableResults, err := service.EvaluateVariableProviders(`{"host":"provided.test"}`)
	if err != nil || len(variableResults) != 1 || variableResults[0].Values["providedHost"] != "provided.test" || variableResults[0].ExtensionID != installed.Manifest.ID {
		t.Fatalf("variable provider results=%#v err=%v", variableResults, err)
	}
	providerResults, err := service.EvaluateAssertions(`{"response":{"status":204}}`)
	if err != nil || len(providerResults) != 1 || !providerResults[0].Passed || providerResults[0].ExtensionID != installed.Manifest.ID || providerResults[0].ProviderID != "test.events.status" {
		t.Fatalf("assertion provider results=%#v err=%v", providerResults, err)
	}
}

func waitForExtensionEvents(t *testing.T, service *Service, extensionID string, minimum int) []any {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		result, err := service.ExecuteCommand(extensionID, "test.events.seen", `{}`, "test")
		if err == nil {
			if data, ok := result.Data.(map[string]any); ok {
				if events, ok := data["events"].([]any); ok && len(events) >= minimum {
					return events
				}
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d extension events", minimum)
	return nil
}

func TestServiceWebviewCSPAllowsOnlyGrantedNetworkOrigin(t *testing.T) {
	if storage.DB() != nil {
		storage.Close()
	}
	dataDir := t.TempDir()
	if err := storage.Open(dataDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(storage.Close)
	root := filepath.Join(t.TempDir(), "webview-network")
	if _, err := Scaffold(root, ScaffoldOptions{ID: "test.webview-network", Name: "Webview Network", Publisher: "test", Template: "webview"}); err != nil {
		t.Fatal(err)
	}
	manifest, err := LoadManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	manifest.Permissions = append(manifest.Permissions, "network:https://api.example.test")
	data, _ := json.MarshalIndent(manifest, "", "  ")
	if err := os.WriteFile(filepath.Join(root, ManifestFileName), data, 0644); err != nil {
		t.Fatal(err)
	}
	service := NewService(dataDir)
	if err := service.Init(); err != nil {
		t.Fatal(err)
	}
	installed, err := service.InstallDirectory(root, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetGrants(installed.Manifest.ID, installed.Manifest.Permissions); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Enable(installed.Manifest.ID); err != nil {
		t.Fatal(err)
	}
	document, err := service.GetWebviewHTML(installed.Manifest.ID, "test.webview-network.panel")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(document, "connect-src https://api.example.test") {
		t.Fatalf("granted origin missing from CSP: %s", document)
	}
	if strings.Contains(document, "connect-src *") || strings.Contains(document, "wails") {
		t.Fatalf("unsafe webview capability in document: %s", document)
	}
}

func TestServiceBuildsSandboxedWebviewDocument(t *testing.T) {
	if storage.DB() != nil {
		storage.Close()
	}
	dataDir := t.TempDir()
	if err := storage.Open(dataDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(storage.Close)
	root := filepath.Join(t.TempDir(), "webview")
	if _, err := Scaffold(root, ScaffoldOptions{ID: "test.webview", Name: "Webview", Publisher: "test", Template: "webview"}); err != nil {
		t.Fatal(err)
	}
	service := NewService(dataDir)
	if err := service.Init(); err != nil {
		t.Fatal(err)
	}
	installed, err := service.InstallDirectory(root, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetGrants(installed.Manifest.ID, []string{"notifications"}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Enable(installed.Manifest.ID); err != nil {
		t.Fatal(err)
	}
	document, err := service.GetWebviewHTML(installed.Manifest.ID, "test.webview.panel")
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"Content-Security-Policy", "default-src &#39;none&#39;", "connect-src &#39;none&#39;", "form-action &#39;none&#39;", "navigate-to &#39;none&#39;", "child-src &#39;none&#39;", "This UI is isolated"} {
		if !strings.Contains(document, expected) {
			t.Fatalf("webview document missing %q:\n%s", expected, document)
		}
	}
}
