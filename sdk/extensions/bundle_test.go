package extensionsdk

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestEmbeddedManifestSchemaIsValidJSON(t *testing.T) {
	data, err := Files.ReadFile("schemas/manifest-v2.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema map[string]interface{}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("decode embedded schema: %v", err)
	}
	if schema["$schema"] == nil || schema["$defs"] == nil {
		t.Fatalf("schema is missing required metadata: %#v", schema)
	}
}

func TestEmbeddedSDKContainsAgentRoutingDocumentsAndSchemas(t *testing.T) {
	for _, path := range []string{
		"docs/README.md", "docs/API.md", "docs/LIFECYCLE.md", "docs/DECLARATIVE-UI.md", "docs/WEBVIEWS.md",
		"docs/recipes/COMMAND.md", "docs/recipes/VIEW.md", "docs/recipes/REQUEST-HOOK.md", "docs/recipes/RESPONSE-TAB.md", "docs/recipes/IMPORTER.md", "docs/recipes/WEBVIEW.md",
		"schemas/declarative-view-v1.schema.json", "api/index.d.ts",
	} {
		if _, err := Files.ReadFile(path); err != nil {
			t.Errorf("missing embedded SDK file %s: %v", path, err)
		}
	}
	data, err := Files.ReadFile("schemas/declarative-view-v1.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema map[string]any
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("decode declarative view schema: %v", err)
	}
}

func TestExportRefusesNonEmptyDestination(t *testing.T) {
	destination := t.TempDir()
	if err := os.WriteFile(filepath.Join(destination, "keep.txt"), []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Export(destination); err == nil {
		t.Fatal("Export() accepted a non-empty destination")
	}
	data, err := os.ReadFile(filepath.Join(destination, "keep.txt"))
	if err != nil || string(data) != "keep" {
		t.Fatalf("existing destination content changed: data=%q err=%v", data, err)
	}
}
