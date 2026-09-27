package adomniacli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestExtensionMigrateV1ProducesNonDestructiveReport(t *testing.T) {
	root := t.TempDir()
	legacy := `{"id":"request-advisor","name":"Request Advisor","version":"1.0.0","author":"Example","minAppVersion":"0.6.10","entryPoint":"main.js","permissions":["notifications"],"hooks":[{"event":"onRequest","handler":"prepare"}],"actions":[{"id":"inspect","name":"Inspect"}],"settings":[{"key":"enabled","type":"boolean","default":true}]}`
	if err := os.WriteFile(filepath.Join(root, "manifest.json"), []byte(legacy), 0644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := Extension([]string{"migrate-v1", root, "--publisher", "acme", "--json"}, &stdout, &stderr); code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	var report migrationReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.SuggestedManifest.ID != "acme.request-advisor" {
		t.Fatalf("id=%q", report.SuggestedManifest.ID)
	}
	if len(report.ManualSteps) == 0 || report.SuggestedManifest.Source != "main.js" {
		t.Fatalf("report=%#v", report)
	}
	data, err := os.ReadFile(filepath.Join(root, "manifest.json"))
	if err != nil || string(data) != legacy {
		t.Fatalf("v1 manifest was modified")
	}
}
