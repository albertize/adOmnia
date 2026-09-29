package extensions

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

func TestServiceReportsAndResetsOwnedStorage(t *testing.T) {
	if storage.DB() != nil {
		storage.Close()
	}
	dataDir := t.TempDir()
	if err := storage.Open(dataDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(storage.Close)
	service := NewService(dataDir)
	if err := service.Init(); err != nil {
		t.Fatal(err)
	}
	installed, err := service.InstallDirectory(writeValidExtension(t), true)
	if err != nil {
		t.Fatal(err)
	}
	ownedKeys := []string{
		"state/global/" + installed.Manifest.ID + "/one",
		"state/workspace/workspace-1/" + installed.Manifest.ID + "/two",
		"secret/" + installed.Manifest.ID + "/three",
	}
	for _, key := range ownedKeys {
		if err := storage.Put(extensionsBucket, key, []byte("value")); err != nil {
			t.Fatal(err)
		}
	}
	if err := storage.Put(extensionsBucket, "state/global/another.extension/keep", []byte("keep")); err != nil {
		t.Fatal(err)
	}
	usage, err := service.GetStorageUsage(installed.Manifest.ID)
	if err != nil || usage.Entries != len(ownedKeys) || usage.Bytes != int64(len(ownedKeys)*len("value")) {
		t.Fatalf("usage=%#v err=%v", usage, err)
	}
	if err := service.ResetStorage(installed.Manifest.ID); err != nil {
		t.Fatal(err)
	}
	usage, err = service.GetStorageUsage(installed.Manifest.ID)
	if err != nil || usage.Entries != 0 || usage.Bytes != 0 {
		t.Fatalf("reset usage=%#v err=%v", usage, err)
	}
	kept, err := storage.Get(extensionsBucket, "state/global/another.extension/keep")
	if err != nil || string(kept) != "keep" {
		t.Fatalf("unrelated state=%q err=%v", kept, err)
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
	manifest.Permissions = append(manifest.Permissions, "tabs.write", "requests.execute", "browserDebug.control", "mock.read", "mock.control", "proxy.read", "proxy.control", "flows.read", "flows.execute", "databases.read", "databases.execute", "brokers.read", "brokers.publish", "documents.read", "documents.readContents", "documents.write", "ai.execute")
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
	params, _ = json.Marshal(map[string]any{"extensionId": installed.Manifest.ID, "flowId": "flow-1", "options": map[string]any{}})
	started, err := service.handleHostCall(context.Background(), "flows.start", params)
	if err != nil {
		t.Fatal(err)
	}
	jobID, _ := started.(map[string]any)["jobId"].(string)
	if jobID == "" || received.Domain != "flows" || received.Action != "start" || received.Payload["jobId"] != jobID {
		t.Fatalf("flow start=%#v action=%#v", started, received)
	}
	params, _ = json.Marshal(map[string]any{"extensionId": installed.Manifest.ID, "jobId": jobID})
	if _, err := service.handleHostCall(context.Background(), "flows.cancel", params); err != nil || received.Action != "cancel" {
		t.Fatalf("flow cancel action=%#v err=%v", received, err)
	}
	if err := service.ReportFlowJobEvent(installed.Manifest.ID, jobID, "onFlowComplete", `{"success":false,"error":"cancelled"}`); err == nil {
		t.Fatal("completion without an active host unexpectedly succeeded")
	}
	if err := service.ReportFlowJobEvent(installed.Manifest.ID, "unknown", "onFlowComplete", `{}`); err == nil {
		t.Fatal("unknown flow job completion accepted")
	}
	params, _ = json.Marshal(map[string]any{"extensionId": installed.Manifest.ID, "flowId": "flow-1", "options": map[string]any{"vus": 1, "mode": "iterations", "iterations": 1}})
	stressStarted, err := service.handleHostCall(context.Background(), "flows.startStress", params)
	if err != nil || received.Action != "stress" {
		t.Fatalf("flow stress start=%#v action=%#v err=%v", stressStarted, received, err)
	}
	stressJobID := stressStarted.(map[string]any)["jobId"].(string)
	_ = service.ReportFlowJobEvent(installed.Manifest.ID, stressJobID, "onFlowComplete", `{"success":true}`)
	if err := storage.Put("pdfprojects", "meta:pdf-1", []byte(`{"id":"pdf-1","name":"Contract","pageCount":3,"updatedAt":42,"annotations":[{"secret":"not-exposed"}]}`)); err != nil {
		t.Fatal(err)
	}
	params, _ = json.Marshal(map[string]any{"extensionId": installed.Manifest.ID})
	documents, err := service.handleHostCall(context.Background(), "documents.listPdfProjects", params)
	if err != nil || len(documents.([]map[string]any)) != 1 || documents.([]map[string]any)[0]["annotations"] != nil {
		t.Fatalf("documents=%#v err=%v", documents, err)
	}
	requestStarted := make(chan struct{}, 1)
	requestServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestStarted <- struct{}{}
		<-r.Context().Done()
	}))
	defer requestServer.Close()
	params, _ = json.Marshal(map[string]any{"extensionId": installed.Manifest.ID, "request": map[string]any{"method": "GET", "url": requestServer.URL, "headers": map[string]string{}, "timeoutMs": 30000}})
	requestJob, err := service.handleHostCall(context.Background(), "requests.start", params)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-requestStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("extension request did not start")
	}
	requestJobID := requestJob.(map[string]any)["jobId"].(string)
	params, _ = json.Marshal(map[string]any{"extensionId": installed.Manifest.ID, "jobId": requestJobID})
	if _, err := service.handleHostCall(context.Background(), "requests.cancel", params); err != nil {
		t.Fatal(err)
	}
	params, _ = json.Marshal(map[string]any{"extensionId": installed.Manifest.ID, "systemPrompt": "Be concise", "userPrompt": "Summarize this request", "options": map[string]any{"maxTokens": 200}})
	aiStarted, err := service.handleHostCall(context.Background(), "ai.startComplete", params)
	if err != nil || received.Domain != "ai" || received.Action != "complete" {
		t.Fatalf("AI start=%#v action=%#v err=%v", aiStarted, received, err)
	}
	aiJobID := aiStarted.(map[string]any)["jobId"].(string)
	params, _ = json.Marshal(map[string]any{"extensionId": installed.Manifest.ID, "jobId": aiJobID})
	if _, err := service.handleHostCall(context.Background(), "ai.cancel", params); err != nil || received.Action != "cancel" {
		t.Fatalf("AI cancel action=%#v err=%v", received, err)
	}
	if err := service.ReportAIJobEvent("another.extension", aiJobID, `{}`); err == nil {
		t.Fatal("AI job ownership was not enforced")
	}
	if err := service.ReportAIJobEvent(installed.Manifest.ID, aiJobID, `{"success":false,"error":"cancelled"}`); err == nil {
		t.Fatal("AI completion without an active host unexpectedly succeeded")
	}
	for _, documentJob := range []struct{ method, action, event string }{{"documents.startReadText", "readText", "onDocumentReadComplete"}, {"documents.startExport", "export", "onDocumentWriteComplete"}} {
		params, _ = json.Marshal(map[string]any{"extensionId": installed.Manifest.ID, "projectId": "pdf-1", "options": map[string]any{}})
		started, startErr := service.handleHostCall(context.Background(), documentJob.method, params)
		if startErr != nil || received.Domain != "documents" || received.Action != documentJob.action {
			t.Fatalf("document start=%#v action=%#v err=%v", started, received, startErr)
		}
		documentJobID := started.(map[string]any)["jobId"].(string)
		params, _ = json.Marshal(map[string]any{"extensionId": installed.Manifest.ID, "jobId": documentJobID})
		if _, cancelErr := service.handleHostCall(context.Background(), "documents.cancel", params); cancelErr != nil || received.Action != "cancel" {
			t.Fatalf("document cancel action=%#v err=%v", received, cancelErr)
		}
		if reportErr := service.ReportDocumentJobEvent("another.extension", documentJobID, documentJob.event, `{}`); reportErr == nil {
			t.Fatal("document job ownership was not enforced")
		}
		if reportErr := service.ReportDocumentJobEvent(installed.Manifest.ID, documentJobID, documentJob.event, `{"success":false,"error":"cancelled"}`); reportErr == nil {
			t.Fatal("document completion without an active host unexpectedly succeeded")
		}
	}
	if err := storage.Put("database", "connections", []byte(`[{"id":"db-1","name":"Local","driver":"sqlite","password":"secret","dsn":"private"}]`)); err != nil {
		t.Fatal(err)
	}
	if err := storage.Put("broker_connections", "profiles-v2", []byte(`{"version":2,"profiles":[{"id":"broker-1","name":"Kafka","protocol":"kafka","config":{"password":"secret"}}]}`)); err != nil {
		t.Fatal(err)
	}
	params, _ = json.Marshal(map[string]any{"extensionId": installed.Manifest.ID, "connectionId": "db-1", "query": "SELECT 1", "options": map[string]any{"limit": 10}})
	databaseStarted, err := service.handleHostCall(context.Background(), "databases.startQuery", params)
	if err != nil || received.Domain != "databases" || received.Action != "query" {
		t.Fatalf("database start=%#v action=%#v err=%v", databaseStarted, received, err)
	}
	databaseJobID := databaseStarted.(map[string]any)["jobId"].(string)
	params, _ = json.Marshal(map[string]any{"extensionId": installed.Manifest.ID, "jobId": databaseJobID})
	if _, err := service.handleHostCall(context.Background(), "databases.cancel", params); err != nil || received.Action != "cancel" {
		t.Fatalf("database cancel action=%#v err=%v", received, err)
	}
	if err := service.ReportDatabaseJobEvent("another.extension", databaseJobID, `{}`); err == nil {
		t.Fatal("database job ownership was not enforced")
	}
	if err := service.ReportDatabaseJobEvent(installed.Manifest.ID, databaseJobID, `{"success":false,"error":"cancelled"}`); err == nil {
		t.Fatal("database completion without an active host unexpectedly succeeded")
	}
	params, _ = json.Marshal(map[string]any{"extensionId": installed.Manifest.ID})
	databaseConnections, err := service.handleHostCall(context.Background(), "databases.listConnections", params)
	if err != nil || len(databaseConnections.([]map[string]any)) != 1 || databaseConnections.([]map[string]any)[0]["password"] != nil {
		t.Fatalf("database connections=%#v err=%v", databaseConnections, err)
	}
	brokerConnections, err := service.handleHostCall(context.Background(), "brokers.listConnections", params)
	if err != nil || len(brokerConnections.([]map[string]any)) != 1 || brokerConnections.([]map[string]any)[0]["config"] != nil {
		t.Fatalf("broker connections=%#v err=%v", brokerConnections, err)
	}
	params, _ = json.Marshal(map[string]any{"extensionId": installed.Manifest.ID, "connectionId": "broker-1", "destination": "events", "message": "hello", "options": map[string]any{}})
	brokerStarted, err := service.handleHostCall(context.Background(), "brokers.startPublish", params)
	if err != nil || received.Domain != "brokers" || received.Action != "publish" {
		t.Fatalf("broker start=%#v action=%#v err=%v", brokerStarted, received, err)
	}
	brokerJobID := brokerStarted.(map[string]any)["jobId"].(string)
	params, _ = json.Marshal(map[string]any{"extensionId": installed.Manifest.ID, "jobId": brokerJobID})
	if _, err := service.handleHostCall(context.Background(), "brokers.cancel", params); err != nil || received.Action != "cancel" {
		t.Fatalf("broker cancel action=%#v err=%v", received, err)
	}
	if err := service.ReportBrokerJobEvent("another.extension", brokerJobID, `{}`); err == nil {
		t.Fatal("broker job ownership was not enforced")
	}
	if err := service.ReportBrokerJobEvent(installed.Manifest.ID, brokerJobID, `{"success":false,"error":"cancelled"}`); err == nil {
		t.Fatal("broker completion without an active host unexpectedly succeeded")
	}
	if err := storage.Put("flows", "all", []byte(`[{"id":"flow-1","name":"Health"}]`)); err != nil {
		t.Fatal(err)
	}
	params, _ = json.Marshal(map[string]any{"extensionId": installed.Manifest.ID})
	listed, err := service.handleHostCall(context.Background(), "flows.list", params)
	if err != nil || len(listed.([]map[string]any)) != 1 {
		t.Fatalf("flows.list=%#v err=%v", listed, err)
	}
	params, _ = json.Marshal(map[string]any{"extensionId": installed.Manifest.ID, "id": "flow-1"})
	flow, err := service.handleHostCall(context.Background(), "flows.get", params)
	if err != nil || flow.(map[string]any)["name"] != "Health" {
		t.Fatalf("flows.get=%#v err=%v", flow, err)
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
	var finalActions []ExtensionDomainAction
	ConfigureDomainActionNotifier(service, func(action ExtensionDomainAction) { finalActions = append(finalActions, action) })
	params, _ = json.Marshal(map[string]any{"extensionId": installed.Manifest.ID, "flowId": "flow-1", "options": map[string]any{}})
	secondJob, err := service.handleHostCall(context.Background(), "flows.start", params)
	if err != nil {
		t.Fatal(err)
	}
	secondJobID := secondJob.(map[string]any)["jobId"].(string)
	params, _ = json.Marshal(map[string]any{"extensionId": installed.Manifest.ID, "connectionId": "db-1", "query": "SELECT 1", "options": map[string]any{}})
	lastDatabaseJob, err := service.handleHostCall(context.Background(), "databases.startQuery", params)
	if err != nil {
		t.Fatal(err)
	}
	lastDatabaseJobID := lastDatabaseJob.(map[string]any)["jobId"].(string)
	params, _ = json.Marshal(map[string]any{"extensionId": installed.Manifest.ID, "connectionId": "broker-1", "destination": "events", "message": "hello", "options": map[string]any{}})
	lastBrokerJob, err := service.handleHostCall(context.Background(), "brokers.startPublish", params)
	if err != nil {
		t.Fatal(err)
	}
	lastBrokerJobID := lastBrokerJob.(map[string]any)["jobId"].(string)
	params, _ = json.Marshal(map[string]any{"extensionId": installed.Manifest.ID, "projectId": "pdf-1", "options": map[string]any{}})
	lastDocumentJob, err := service.handleHostCall(context.Background(), "documents.startExport", params)
	if err != nil {
		t.Fatal(err)
	}
	lastDocumentJobID := lastDocumentJob.(map[string]any)["jobId"].(string)
	params, _ = json.Marshal(map[string]any{"extensionId": installed.Manifest.ID, "systemPrompt": "", "userPrompt": "Summarize", "options": map[string]any{}})
	lastAIJob, err := service.handleHostCall(context.Background(), "ai.startComplete", params)
	if err != nil {
		t.Fatal(err)
	}
	lastAIJobID := lastAIJob.(map[string]any)["jobId"].(string)
	if _, err := service.Disable(installed.Manifest.ID); err != nil {
		t.Fatal(err)
	}
	cancelled := map[string]string{}
	for _, action := range finalActions {
		if action.Action == "cancel" {
			cancelled[action.Domain] = action.Payload["jobId"].(string)
		}
	}
	if cancelled["flows"] != secondJobID || cancelled["databases"] != lastDatabaseJobID || cancelled["brokers"] != lastBrokerJobID || cancelled["documents"] != lastDocumentJobID || cancelled["ai"] != lastAIJobID {
		t.Fatalf("disable did not cancel owned jobs: %#v", finalActions)
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
	activated, err := service.registry.Get(installed.Manifest.ID)
	if err != nil || activated.ActivationReason != "command:test.recover.hello" || activated.ActivatedAt == "" || activated.ActivationTimeMS < 0 {
		t.Fatalf("activation metadata=%#v err=%v", activated, err)
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
	manifest.ActivationEvents = []string{"onStartup", "onCommand:test.events.seen", "onWorkspaceOpen", "onWorkspaceClose", "onEnvChange", "onThemeChange", "onTabOpen", "onTabClose", "onSave", "onImport", "onExport", "onAssertions", "onVariables", "onBrowserNetwork", "onFlowProgress", "onFlowComplete", "onDatabaseComplete", "onBrokerPublishComplete", "onDocumentReadComplete", "onDocumentWriteComplete", "onAIComplete"}
	manifest.Permissions = []string{"workspace.read", "environments.read", "tabs.read", "assertions.provide", "variables.provide", "browserDebug.read", "flows.execute", "databases.execute", "brokers.publish", "documents.readContents", "documents.write", "ai.execute"}
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
  api.events.onFlowProgress(record('onFlowProgress'))
  api.events.onFlowComplete(record('onFlowComplete'))
  api.events.onDatabaseComplete(record('onDatabaseComplete'))
  api.events.onBrokerPublishComplete(record('onBrokerPublishComplete'))
  api.events.onDocumentReadComplete(record('onDocumentReadComplete'))
  api.events.onDocumentWriteComplete(record('onDocumentWriteComplete'))
  api.events.onAIComplete(record('onAIComplete'))
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
	var flowAction ExtensionDomainAction
	ConfigureDomainActionNotifier(service, func(action ExtensionDomainAction) { flowAction = action })
	params, _ := json.Marshal(map[string]any{"extensionId": installed.Manifest.ID, "flowId": "flow-1", "options": map[string]any{}})
	started, err := service.handleHostCall(context.Background(), "flows.start", params)
	if err != nil {
		t.Fatal(err)
	}
	jobID, _ := started.(map[string]any)["jobId"].(string)
	if flowAction.Action != "start" || flowAction.Payload["jobId"] != jobID {
		t.Fatalf("flow action=%#v", flowAction)
	}
	if err := service.ReportFlowJobEvent(installed.Manifest.ID, jobID, "onFlowProgress", `{"entriesCompleted":1}`); err != nil {
		t.Fatal(err)
	}
	if err := service.ReportFlowJobEvent(installed.Manifest.ID, jobID, "onFlowComplete", `{"success":true}`); err != nil {
		t.Fatal(err)
	}
	events = waitForExtensionEvents(t, service, installed.Manifest.ID, 17)
	if entry, ok := events[len(events)-1].(map[string]any); !ok || entry["event"] != "onFlowComplete" {
		t.Fatalf("flow completion events=%#v", events)
	}
	params, _ = json.Marshal(map[string]any{"extensionId": installed.Manifest.ID, "connectionId": "db-1", "query": "SELECT 1", "options": map[string]any{}})
	databaseStarted, err := service.handleHostCall(context.Background(), "databases.startQuery", params)
	if err != nil {
		t.Fatal(err)
	}
	databaseJobID := databaseStarted.(map[string]any)["jobId"].(string)
	if flowAction.Domain != "databases" || flowAction.Action != "query" {
		t.Fatalf("database action=%#v", flowAction)
	}
	if err := service.ReportDatabaseJobEvent(installed.Manifest.ID, databaseJobID, `{"success":true,"result":{"rows":[[1]]}}`); err != nil {
		t.Fatal(err)
	}
	events = waitForExtensionEvents(t, service, installed.Manifest.ID, 18)
	if entry, ok := events[len(events)-1].(map[string]any); !ok || entry["event"] != "onDatabaseComplete" {
		t.Fatalf("database completion events=%#v", events)
	}
	params, _ = json.Marshal(map[string]any{"extensionId": installed.Manifest.ID, "connectionId": "broker-1", "destination": "events", "message": "hello", "options": map[string]any{}})
	brokerStarted, err := service.handleHostCall(context.Background(), "brokers.startPublish", params)
	if err != nil {
		t.Fatal(err)
	}
	brokerJobID := brokerStarted.(map[string]any)["jobId"].(string)
	if flowAction.Domain != "brokers" || flowAction.Action != "publish" {
		t.Fatalf("broker action=%#v", flowAction)
	}
	if err := service.ReportBrokerJobEvent(installed.Manifest.ID, brokerJobID, `{"success":true,"result":{"ok":true}}`); err != nil {
		t.Fatal(err)
	}
	events = waitForExtensionEvents(t, service, installed.Manifest.ID, 19)
	if entry, ok := events[len(events)-1].(map[string]any); !ok || entry["event"] != "onBrokerPublishComplete" {
		t.Fatalf("broker completion events=%#v", events)
	}
	for index, documentJob := range []struct{ method, event string }{{"documents.startReadText", "onDocumentReadComplete"}, {"documents.startExport", "onDocumentWriteComplete"}} {
		params, _ = json.Marshal(map[string]any{"extensionId": installed.Manifest.ID, "projectId": "pdf-1", "options": map[string]any{}})
		documentStarted, startErr := service.handleHostCall(context.Background(), documentJob.method, params)
		if startErr != nil {
			t.Fatal(startErr)
		}
		documentJobID := documentStarted.(map[string]any)["jobId"].(string)
		if reportErr := service.ReportDocumentJobEvent(installed.Manifest.ID, documentJobID, documentJob.event, `{"success":true,"result":{}}`); reportErr != nil {
			t.Fatal(reportErr)
		}
		events = waitForExtensionEvents(t, service, installed.Manifest.ID, 20+index)
		if entry, ok := events[len(events)-1].(map[string]any); !ok || entry["event"] != documentJob.event {
			t.Fatalf("document completion events=%#v", events)
		}
	}
	params, _ = json.Marshal(map[string]any{"extensionId": installed.Manifest.ID, "systemPrompt": "", "userPrompt": "Summarize", "options": map[string]any{}})
	aiStarted, err := service.handleHostCall(context.Background(), "ai.startComplete", params)
	if err != nil {
		t.Fatal(err)
	}
	aiJobID := aiStarted.(map[string]any)["jobId"].(string)
	if reportErr := service.ReportAIJobEvent(installed.Manifest.ID, aiJobID, `{"success":true,"result":"summary"}`); reportErr != nil {
		t.Fatal(reportErr)
	}
	events = waitForExtensionEvents(t, service, installed.Manifest.ID, 22)
	if entry, ok := events[len(events)-1].(map[string]any); !ok || entry["event"] != "onAIComplete" {
		t.Fatalf("AI completion events=%#v", events)
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
