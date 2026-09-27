package adomniacli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestExtensionCLIInitCheckAndPackJSON(t *testing.T) {
	root := filepath.Join(t.TempDir(), "response-policy")
	var stdout, stderr bytes.Buffer
	code := Run([]string{"extension", "init", root, "--id", "local.response-policy", "--name", "Response Policy", "--template", "request-hook", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("init code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	var initResult extensionResult
	if err := json.Unmarshal(stdout.Bytes(), &initResult); err != nil {
		t.Fatalf("decode init result: %v\n%s", err, stdout.String())
	}
	if !initResult.OK || initResult.Scaffold == nil || initResult.Scaffold.ID != "local.response-policy" {
		t.Fatalf("init result = %#v", initResult)
	}

	stdout.Reset()
	stderr.Reset()
	code = Run([]string{"extension", "check", root, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("check code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	var validation struct {
		Valid bool `json:"valid"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &validation); err != nil || !validation.Valid {
		t.Fatalf("validation=%#v err=%v output=%s", validation, err, stdout.String())
	}

	output := filepath.Join(t.TempDir(), "response-policy.adomnia-extension")
	stdout.Reset()
	stderr.Reset()
	code = Run([]string{"extension", "pack", root, "--out", output, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("pack code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	var packResult extensionResult
	if err := json.Unmarshal(stdout.Bytes(), &packResult); err != nil {
		t.Fatalf("decode pack result: %v\n%s", err, stdout.String())
	}
	if !packResult.OK || packResult.Package == nil || len(packResult.Package.SHA256) != 64 {
		t.Fatalf("pack result = %#v", packResult)
	}
	if _, err := os.Stat(output); err != nil {
		t.Fatalf("package missing: %v", err)
	}
}

func TestExtensionCLICheckReportsInvalidManifestAsJSON(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "manifest.json"), []byte(`{"manifestVersion":2}`), 0644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := Run([]string{"extension", "check", root, "--json"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("check code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	var report struct {
		Valid       bool `json:"valid"`
		Diagnostics []struct {
			Code string `json:"code"`
		} `json:"diagnostics"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode report: %v\n%s", err, stdout.String())
	}
	if report.Valid || len(report.Diagnostics) == 0 {
		t.Fatalf("invalid report = %#v", report)
	}
}

func TestExtensionCLIExportsEmbeddedSDK(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "sdk")
	var stdout, stderr bytes.Buffer
	code := Run([]string{"extension", "sdk", destination, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("sdk code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	for _, relative := range []string{"docs/README.md", "api/index.d.ts", "schemas/manifest-v2.schema.json", "templates/minimal/manifest.json.tmpl"} {
		if _, err := os.Stat(filepath.Join(destination, filepath.FromSlash(relative))); err != nil {
			t.Fatalf("exported SDK missing %s: %v", relative, err)
		}
	}
}
