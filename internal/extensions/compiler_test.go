package extensions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildBundlesTypeScriptWithoutNodeRuntime(t *testing.T) {
	root := t.TempDir()
	manifest := `{
	  "manifestVersion": 2,
	  "id": "test.typescript",
	  "name": "TypeScript",
	  "version": "1.0.0",
	  "publisher": "test",
	  "engines": {"adomnia": ">=1.0.0 <2"},
	  "apiVersion": "2.0",
	  "main": "dist/extension.js",
	  "source": "src/extension.ts",
	  "activationEvents": ["onCommand:test.typescript.run"],
	  "permissions": [],
	  "contributes": {"commands": [{"id":"test.typescript.run","title":"Run"}]}
	}`
	if err := os.MkdirAll(filepath.Join(root, "src"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "manifest.json"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
	source := `type Result = { ok: boolean }; export function activate(api: any) { api.commands.registerCommand("test.typescript.run", (): Result => ({ ok: true })) }`
	if err := os.WriteFile(filepath.Join(root, "src", "extension.ts"), []byte(source), 0644); err != nil {
		t.Fatal(err)
	}
	result, err := Build(root)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if !result.Built || result.Output != "dist/extension.js" {
		t.Fatalf("Build() = %#v", result)
	}
	bundle, err := os.ReadFile(filepath.Join(root, "dist", "extension.js"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(bundle), "type Result") || !strings.Contains(string(bundle), "activate") {
		t.Fatalf("unexpected bundle:\n%s", bundle)
	}
	if report := CheckDirectory(root); !report.Valid {
		t.Fatalf("built extension invalid: %#v", report.Diagnostics)
	}
}
