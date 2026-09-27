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
	manifest.Permissions = append(manifest.Permissions, "tabs.write")
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
