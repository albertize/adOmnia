package extensions

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScaffoldCreatesValidTemplates(t *testing.T) {
	for _, template := range []string{"minimal", "view", "request-hook", "webview", "typescript"} {
		t.Run(template, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), template)
			result, err := Scaffold(root, ScaffoldOptions{ID: "test." + strings.ReplaceAll(template, "-", ""), Name: "Test " + template, Publisher: "test", Template: template})
			if err != nil {
				t.Fatalf("Scaffold() error = %v", err)
			}
			expectedFiles := 4
			if template == "webview" {
				expectedFiles = 5
			} else if template == "typescript" {
				expectedFiles = 6
			}
			if result.Template != template || len(result.Files) != expectedFiles {
				t.Fatalf("Scaffold() result = %#v", result)
			}
			report := CheckDirectory(root)
			if !report.Valid {
				t.Fatalf("generated extension invalid: %#v", report.Diagnostics)
			}
		})
	}
}

func TestDecodeManifestRejectsUnknownFields(t *testing.T) {
	_, err := DecodeManifest([]byte(`{
		"manifestVersion":2,
		"id":"test.unknown",
		"name":"Unknown",
		"version":"1.0.0",
		"publisher":"test",
		"engines":{"adomnia":">=1.0.0 <2"},
		"apiVersion":"2.0",
		"main":"main.js",
		"imaginary":true
	}`))
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("DecodeManifest() error = %v, want unknown field", err)
	}
}

func TestDecodeManifestRejectsTrailingJSON(t *testing.T) {
	_, err := DecodeManifest([]byte(`{"manifestVersion":2} {"manifestVersion":2}`))
	if err == nil || !strings.Contains(err.Error(), "multiple JSON values") {
		t.Fatalf("DecodeManifest() error = %v, want multiple JSON values", err)
	}
}

func TestValidateManifestRejectsUnownedReferencesAndUnknownPermissions(t *testing.T) {
	manifest := validTestManifest()
	manifest.Main = "../outside.js"
	manifest.Permissions = []string{"responses.read", "everything"}
	manifest.ActivationEvents = []string{"onCommand:other.command", "onView:test.extension.missing"}
	manifest.Contributes.Commands = []CommandContribution{{ID: "other.command", Title: "Other"}}
	manifest.Contributes.Configuration = map[string]ConfigurationProperty{
		"test.extension.limit": {Type: "integer", Default: "not-a-number"},
	}
	diagnostics := ValidateManifest(manifest)
	codes := map[string]bool{}
	for _, diagnostic := range diagnostics {
		codes[diagnostic.Code] = true
	}
	for _, expected := range []string{"manifest.main", "permission.unknown", "contribution.owner", "activation.view", "contribution.configurationDefault"} {
		if !codes[expected] {
			t.Fatalf("missing diagnostic %s in %#v", expected, diagnostics)
		}
	}
}

func TestCheckDirectoryRejectsSymlinks(t *testing.T) {
	root := writeValidExtension(t)
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("outside"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "linked.txt")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	report := CheckDirectory(root)
	if report.Valid || !hasDiagnostic(report.Diagnostics, "package.symlink") {
		t.Fatalf("CheckDirectory() = %#v, want symlink failure", report)
	}
}

func TestPackIsDeterministicAndExcludesDevelopmentDirectories(t *testing.T) {
	root := writeValidExtension(t)
	if err := os.MkdirAll(filepath.Join(root, "node_modules", "ignored"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "node_modules", "ignored", "file.js"), []byte("ignored"), 0644); err != nil {
		t.Fatal(err)
	}
	firstPath := filepath.Join(t.TempDir(), "first.adomnia-extension")
	secondPath := filepath.Join(t.TempDir(), "second.adomnia-extension")
	first, report, err := Pack(root, firstPath)
	if err != nil || !report.Valid {
		t.Fatalf("first Pack() result=%#v report=%#v err=%v", first, report, err)
	}
	second, report, err := Pack(root, secondPath)
	if err != nil || !report.Valid {
		t.Fatalf("second Pack() result=%#v report=%#v err=%v", second, report, err)
	}
	if first.SHA256 != second.SHA256 {
		t.Fatalf("package hashes differ: %s != %s", first.SHA256, second.SHA256)
	}
	replaced, report, err := Pack(root, firstPath)
	if err != nil || !report.Valid || replaced.SHA256 != first.SHA256 {
		t.Fatalf("replacement Pack() result=%#v report=%#v err=%v", replaced, report, err)
	}
	archive, err := zip.OpenReader(first.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	for _, file := range archive.File {
		if strings.HasPrefix(file.Name, "node_modules/") {
			t.Fatalf("archive contains excluded path %s", file.Name)
		}
	}
}

func TestValidateManifestConfigurationActivationAndMenuLocations(t *testing.T) {
	manifest := validTestManifest()
	manifest.Permissions = append(manifest.Permissions, "environments.read", "tabs.read", "workspace.read")
	manifest.Contributes.Configuration = map[string]ConfigurationProperty{"test.extension.level": {Type: "integer", Default: float64(1)}}
	manifest.ActivationEvents = append(manifest.ActivationEvents, "onConfiguration:test.extension.level")
	manifest.Contributes.Menus = map[string][]MenuContribution{"tab/context": {{Command: "test.extension.run"}}}
	if diagnostics := ValidateManifest(manifest); hasErrors(diagnostics) {
		t.Fatalf("valid domain manifest diagnostics = %#v", diagnostics)
	}
	manifest.ActivationEvents = append(manifest.ActivationEvents, "onConfiguration:test.extension.missing")
	manifest.Contributes.Menus["made/up"] = []MenuContribution{{Command: "test.extension.run"}}
	diagnostics := ValidateManifest(manifest)
	if !hasDiagnostic(diagnostics, "activation.configuration") || !hasDiagnostic(diagnostics, "contribution.menuLocation") {
		t.Fatalf("missing diagnostics: %#v", diagnostics)
	}
}

func TestValidateManifestAssertionProviderActivationRequiresPermission(t *testing.T) {
	manifest := validTestManifest()
	manifest.ActivationEvents = append(manifest.ActivationEvents, "onAssertions")
	if diagnostics := ValidateManifest(manifest); !hasDiagnostic(diagnostics, "activation.permission") {
		t.Fatalf("missing assertion permission diagnostic: %#v", diagnostics)
	}
	manifest.Permissions = append(manifest.Permissions, "assertions.provide")
	if diagnostics := ValidateManifest(manifest); hasErrors(diagnostics) {
		t.Fatalf("valid assertion provider diagnostics: %#v", diagnostics)
	}
}

func TestValidateManifestVariableProviderActivationRequiresPermission(t *testing.T) {
	manifest := validTestManifest()
	manifest.ActivationEvents = append(manifest.ActivationEvents, "onVariables")
	if diagnostics := ValidateManifest(manifest); !hasDiagnostic(diagnostics, "activation.permission") {
		t.Fatalf("missing variable provider permission diagnostic: %#v", diagnostics)
	}
	manifest.Permissions = append(manifest.Permissions, "variables.provide")
	if diagnostics := ValidateManifest(manifest); hasErrors(diagnostics) {
		t.Fatalf("valid variable provider diagnostics: %#v", diagnostics)
	}
}

func TestValidateManifestBrowserEventRequiresReadPermission(t *testing.T) {
	manifest := validTestManifest()
	manifest.ActivationEvents = append(manifest.ActivationEvents, "onBrowserNetwork")
	if diagnostics := ValidateManifest(manifest); !hasDiagnostic(diagnostics, "activation.permission") {
		t.Fatalf("missing browser permission diagnostic: %#v", diagnostics)
	}
	manifest.Permissions = append(manifest.Permissions, "browserDebug.read")
	if diagnostics := ValidateManifest(manifest); hasErrors(diagnostics) {
		t.Fatalf("valid browser event diagnostics: %#v", diagnostics)
	}
}

func TestValidateManifestMockAndProxyEventsRequireReadPermissions(t *testing.T) {
	manifest := validTestManifest()
	manifest.ActivationEvents = append(manifest.ActivationEvents, "onMockHit", "onProxyTraffic")
	diagnostics := ValidateManifest(manifest)
	if !hasDiagnostic(diagnostics, "activation.permission") {
		t.Fatalf("missing runtime read permission diagnostics: %#v", diagnostics)
	}
	manifest.Permissions = append(manifest.Permissions, "mock.read", "proxy.read")
	if diagnostics := ValidateManifest(manifest); hasErrors(diagnostics) {
		t.Fatalf("valid runtime event diagnostics: %#v", diagnostics)
	}
}

func TestValidateManifestFlowCompletionRequiresExecutePermission(t *testing.T) {
	manifest := validTestManifest()
	manifest.ActivationEvents = append(manifest.ActivationEvents, "onFlowProgress", "onFlowComplete")
	if diagnostics := ValidateManifest(manifest); !hasDiagnostic(diagnostics, "activation.permission") {
		t.Fatalf("missing flow execute permission diagnostic: %#v", diagnostics)
	}
	manifest.Permissions = append(manifest.Permissions, "flows.execute")
	if diagnostics := ValidateManifest(manifest); hasErrors(diagnostics) {
		t.Fatalf("valid flow completion diagnostics: %#v", diagnostics)
	}
}

func TestValidateManifestDatabaseCompletionRequiresExecutePermission(t *testing.T) {
	manifest := validTestManifest()
	manifest.ActivationEvents = append(manifest.ActivationEvents, "onDatabaseComplete")
	if diagnostics := ValidateManifest(manifest); !hasDiagnostic(diagnostics, "activation.permission") {
		t.Fatalf("missing database execute permission diagnostic: %#v", diagnostics)
	}
	manifest.Permissions = append(manifest.Permissions, "databases.execute")
	if diagnostics := ValidateManifest(manifest); hasErrors(diagnostics) {
		t.Fatalf("valid database completion diagnostics: %#v", diagnostics)
	}
}

func TestValidateManifestBrokerCompletionRequiresPublishPermission(t *testing.T) {
	manifest := validTestManifest()
	manifest.ActivationEvents = append(manifest.ActivationEvents, "onBrokerPublishComplete")
	if diagnostics := ValidateManifest(manifest); !hasDiagnostic(diagnostics, "activation.permission") {
		t.Fatalf("missing broker publish permission diagnostic: %#v", diagnostics)
	}
	manifest.Permissions = append(manifest.Permissions, "brokers.publish")
	if diagnostics := ValidateManifest(manifest); hasErrors(diagnostics) {
		t.Fatalf("valid broker completion diagnostics: %#v", diagnostics)
	}
}

func TestValidateManifestDocumentCompletionRequiresScopedPermissions(t *testing.T) {
	for _, test := range []struct{ event, permission string }{{"onDocumentReadComplete", "documents.readContents"}, {"onDocumentWriteComplete", "documents.write"}} {
		manifest := validTestManifest()
		manifest.ActivationEvents = append(manifest.ActivationEvents, test.event)
		if diagnostics := ValidateManifest(manifest); !hasDiagnostic(diagnostics, "activation.permission") {
			t.Fatalf("missing %s permission diagnostic: %#v", test.permission, diagnostics)
		}
		manifest.Permissions = append(manifest.Permissions, test.permission)
		if diagnostics := ValidateManifest(manifest); hasErrors(diagnostics) {
			t.Fatalf("valid %s diagnostics: %#v", test.event, diagnostics)
		}
	}
}

func TestValidateManifestAICompletionRequiresExecutePermission(t *testing.T) {
	manifest := validTestManifest()
	manifest.ActivationEvents = append(manifest.ActivationEvents, "onAIComplete")
	if diagnostics := ValidateManifest(manifest); !hasDiagnostic(diagnostics, "activation.permission") {
		t.Fatalf("missing AI execute permission diagnostic: %#v", diagnostics)
	}
	manifest.Permissions = append(manifest.Permissions, "ai.execute")
	if diagnostics := ValidateManifest(manifest); hasErrors(diagnostics) {
		t.Fatalf("valid AI completion diagnostics: %#v", diagnostics)
	}
}

func validTestManifest() Manifest {
	return Manifest{
		ManifestVersion:  SupportedManifestVersion,
		ID:               "test.extension",
		Name:             "Test Extension",
		Version:          "1.0.0",
		Publisher:        "test",
		Engines:          Engines{Adomnia: ">=1.0.0 <2"},
		APIVersion:       SupportedAPIVersion,
		Main:             "src/extension.js",
		ActivationEvents: []string{"onCommand:test.extension.run"},
		Permissions:      []string{"notifications"},
		Contributes:      Contributions{Commands: []CommandContribution{{ID: "test.extension.run", Title: "Run"}}},
	}
}

func writeValidExtension(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	manifest := `{
	  "manifestVersion": 2,
	  "id": "test.extension",
	  "name": "Test Extension",
	  "version": "1.0.0",
	  "publisher": "test",
	  "engines": {"adomnia": ">=1.0.0 <2"},
	  "apiVersion": "2.0",
	  "main": "src/extension.js",
	  "activationEvents": ["onCommand:test.extension.run"],
	  "permissions": ["notifications"],
	  "contributes": {"commands": [{"id":"test.extension.run","title":"Run"}]}
	}`
	if err := os.MkdirAll(filepath.Join(root, "src"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ManifestFileName), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "extension.js"), []byte("export function activate() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	return root
}

func hasDiagnostic(diagnostics []Diagnostic, code string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			return true
		}
	}
	return false
}
