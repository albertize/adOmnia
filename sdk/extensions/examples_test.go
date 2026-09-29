package extensionsdk_test

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"adomnia/internal/extensions"
	extensionsdk "adomnia/sdk/extensions"
)

var bundledExamples = []string{"response-security", "variable-inspector", "form-actions", "assertion-provider", "variable-provider", "browser-network", "mock-monitor", "proxy-monitor", "flow-catalog", "data-source-catalog", "document-catalog", "flow-runner", "database-runner", "broker-publisher", "document-worker", "ai-action"}

func TestBundledExtensionExamplesValidate(t *testing.T) {
	destination := exportSDK(t)
	for _, name := range bundledExamples {
		report := extensions.CheckDirectory(filepath.Join(destination, "examples", name))
		if !report.Valid {
			t.Errorf("example %s is invalid: %#v", name, report.Diagnostics)
		}
	}
}

func TestBundledExtensionExamplesActivateInIsolatedHost(t *testing.T) {
	destination := exportSDK(t)
	token := "0123456789abcdef0123456789abcdef"
	t.Setenv("ADOMNIA_EXTENSION_HOST_TOKEN", token)
	parentInput, childOutput := io.Pipe()
	childInput, parentOutput := io.Pipe()
	done := make(chan error, 1)
	go func() { done <- extensions.RunExtensionHost(childInput, childOutput) }()
	handler := func(_ context.Context, method string, _ json.RawMessage) (interface{}, error) {
		switch method {
		case "variables.getAll":
			return map[string]string{"example": "value"}, nil
		case "domains.get":
			return []map[string]any{}, nil
		case "mock.getSnapshot":
			return map[string]any{"running": false, "hits": []any{}}, nil
		case "proxy.getSnapshot":
			return map[string]any{"running": false, "entries": []any{}}, nil
		case "documents.listPdfProjects":
			return []map[string]any{}, nil
		case "databases.listConnections":
			return []map[string]any{}, nil
		case "brokers.listConnections":
			return []map[string]any{}, nil
		case "flows.list":
			return []map[string]any{}, nil
		case "flows.get":
			return nil, nil
		default:
			return map[string]bool{"ok": true}, nil
		}
	}
	client := extensions.NewHostClient(parentInput, parentOutput, token, handler)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var initialized extensions.InitializeHostResult
	if err := client.Request(ctx, "initialize", extensions.InitializeHostRequest{ProtocolVersion: extensions.HostProtocolVersion}, &initialized); err != nil {
		t.Fatal(err)
	}
	for _, name := range bundledExamples {
		root := filepath.Join(destination, "examples", name)
		manifest, err := extensions.LoadManifest(root)
		if err != nil {
			t.Fatal(err)
		}
		source, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(manifest.Main)))
		if err != nil {
			t.Fatal(err)
		}
		var result extensions.HostExecutionResult
		if err := client.Request(ctx, "activate", extensions.ActivateHostRequest{Manifest: manifest, Source: string(source)}, &result); err != nil || !result.Success {
			t.Errorf("example %s activation result=%#v err=%v", name, result, err)
		}
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("extension example host did not stop")
	}
}

func exportSDK(t *testing.T) string {
	t.Helper()
	destination := filepath.Join(t.TempDir(), "sdk")
	if _, err := extensionsdk.Export(destination); err != nil {
		t.Fatal(err)
	}
	return destination
}
