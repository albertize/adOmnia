package extensionsdk_test

import (
	"path/filepath"
	"testing"

	"adomnia/internal/extensions"
	extensionsdk "adomnia/sdk/extensions"
)

func TestBundledExtensionExamplesValidate(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "sdk")
	if _, err := extensionsdk.Export(destination); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"response-security", "variable-inspector", "form-actions"} {
		report := extensions.CheckDirectory(filepath.Join(destination, "examples", name))
		if !report.Valid {
			t.Errorf("example %s is invalid: %#v", name, report.Diagnostics)
		}
	}
}
