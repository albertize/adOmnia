package extensions

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"adomnia/internal/storage"
)

func newTestRegistry(t *testing.T) *Registry {
	t.Helper()
	if storage.DB() != nil {
		storage.Close()
	}
	dataDir := t.TempDir()
	if err := storage.Open(dataDir); err != nil {
		t.Fatalf("storage.Open() error = %v", err)
	}
	t.Cleanup(storage.Close)
	registry := NewRegistry(dataDir)
	if err := registry.Init(); err != nil {
		t.Fatalf("Registry.Init() error = %v", err)
	}
	return registry
}

func TestRegistryRequiresPermissionReviewAndPersistsState(t *testing.T) {
	registry := newTestRegistry(t)
	root := writeValidExtension(t)
	installed, err := registry.InstallDirectory(root, true)
	if err != nil {
		t.Fatalf("InstallDirectory() error = %v", err)
	}
	if installed.Enabled || installed.InstallKind != InstallDevelopment {
		t.Fatalf("installed extension = %#v", installed)
	}
	if _, err := registry.Enable(installed.Manifest.ID); err == nil {
		t.Fatal("Enable() succeeded without permission grants")
	}
	if _, err := registry.SetGrants(installed.Manifest.ID, []string{"notifications"}); err != nil {
		t.Fatalf("SetGrants() error = %v", err)
	}
	enabled, err := registry.Enable(installed.Manifest.ID)
	if err != nil {
		t.Fatalf("Enable() error = %v", err)
	}
	if !enabled.Enabled {
		t.Fatal("extension was not enabled")
	}

	reloaded := NewRegistry(registry.dataDir)
	if err := reloaded.Init(); err != nil {
		t.Fatalf("reloaded Init() error = %v", err)
	}
	item, err := reloaded.Get(installed.Manifest.ID)
	if err != nil || !item.Enabled || item.Active {
		t.Fatalf("reloaded extension = %#v err=%v", item, err)
	}
}

func TestRegistryDevelopmentManifestPermissionChangeForcesReview(t *testing.T) {
	registry := newTestRegistry(t)
	root := writeValidExtension(t)
	installed, err := registry.InstallDirectory(root, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := registry.SetGrants(installed.Manifest.ID, []string{"notifications"}); err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Enable(installed.Manifest.ID); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(root, ManifestFileName)
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	updated := strings.Replace(string(data), `"notifications"`, `"notifications", "globalState"`, 1)
	if err := os.WriteFile(manifestPath, []byte(updated), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Disable(installed.Manifest.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Enable(installed.Manifest.ID); err == nil {
		t.Fatal("Enable() accepted a newly requested permission without review")
	}
	item, err := registry.Get(installed.Manifest.ID)
	if err != nil {
		t.Fatal(err)
	}
	if item.Enabled || item.Error == "" || len(item.Manifest.Permissions) != 2 {
		t.Fatalf("updated extension = %#v", item)
	}
}

func TestRegistryQuarantinesRepeatedRuntimeFailures(t *testing.T) {
	registry := newTestRegistry(t)
	root := writeValidExtension(t)
	installed, err := registry.InstallDirectory(root, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := registry.SetGrants(installed.Manifest.ID, []string{"notifications"}); err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Enable(installed.Manifest.ID); err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 3; index++ {
		if err := registry.RecordRuntimeFailure(installed.Manifest.ID, fmt.Errorf("failure %d", index)); err != nil {
			t.Fatal(err)
		}
	}
	item, err := registry.Get(installed.Manifest.ID)
	if err != nil {
		t.Fatal(err)
	}
	if item.Enabled || !item.Quarantined || item.FailureCount != 3 {
		t.Fatalf("quarantined extension = %#v", item)
	}
}

func TestRegistryInstallsManagedArchiveAndUninstallsFiles(t *testing.T) {
	registry := newTestRegistry(t)
	root := writeValidExtension(t)
	archivePath := filepath.Join(t.TempDir(), "test.adomnia-extension")
	if _, report, err := Pack(root, archivePath); err != nil || !report.Valid {
		t.Fatalf("Pack() report=%#v err=%v", report, err)
	}
	installed, err := registry.InstallArchive(archivePath)
	if err != nil {
		t.Fatalf("InstallArchive() error = %v", err)
	}
	if installed.InstallKind != InstallManaged || installed.InstallDir == root || installed.Source != archivePath {
		t.Fatalf("managed extension = %#v", installed)
	}
	manifest, err := LoadManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	manifest.Version = "1.1.0"
	manifestData, _ := json.MarshalIndent(manifest, "", "  ")
	if err := os.WriteFile(filepath.Join(root, ManifestFileName), manifestData, 0644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Pack(root, archivePath); err != nil {
		t.Fatal(err)
	}
	updated, err := registry.UpdateArchive(archivePath)
	if err != nil || updated.Manifest.Version != "1.1.0" || updated.Source != archivePath {
		t.Fatalf("UpdateArchive() = %#v, %v", updated, err)
	}
	installed = updated
	if _, err := os.Stat(filepath.Join(installed.InstallDir, "manifest.json")); err != nil {
		t.Fatalf("managed manifest missing: %v", err)
	}
	if err := registry.Uninstall(installed.Manifest.ID); err != nil {
		t.Fatalf("Uninstall() error = %v", err)
	}
	if _, err := os.Stat(installed.InstallDir); !os.IsNotExist(err) {
		t.Fatalf("managed install directory still exists: %v", err)
	}
}

func TestRegistryRejectsArchiveTraversal(t *testing.T) {
	registry := newTestRegistry(t)
	path := filepath.Join(t.TempDir(), "unsafe.adomnia-extension")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	entry, err := writer.Create("../outside.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte("unsafe")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := registry.InstallArchive(path); err == nil {
		t.Fatal("InstallArchive() accepted path traversal")
	}
}
