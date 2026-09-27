// Package extensionsdk embeds the version-matched authoring SDK in the adOmnia binary.
package extensionsdk

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const Version = "2.0.0-preview.1"

// Files is the complete local authoring kit exported by the extension CLI.
//
//go:embed api docs schemas templates examples package.json
var Files embed.FS

// Export writes the SDK to a new or empty destination directory.
func Export(destination string) (string, error) {
	absolute, err := filepath.Abs(destination)
	if err != nil {
		return "", fmt.Errorf("resolve SDK destination: %w", err)
	}
	if entries, readErr := os.ReadDir(absolute); readErr == nil && len(entries) > 0 {
		return "", fmt.Errorf("SDK destination is not empty: %s", absolute)
	} else if readErr != nil && !os.IsNotExist(readErr) {
		return "", fmt.Errorf("inspect SDK destination: %w", readErr)
	}
	if err := os.MkdirAll(absolute, 0755); err != nil {
		return "", fmt.Errorf("create SDK destination: %w", err)
	}
	err = fs.WalkDir(Files, ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == "." {
			return nil
		}
		target := filepath.Join(absolute, filepath.FromSlash(path))
		if !strings.HasPrefix(target, absolute+string(filepath.Separator)) {
			return fmt.Errorf("embedded SDK path escapes destination: %s", path)
		}
		if entry.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		data, err := Files.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0644)
	})
	if err != nil {
		return "", fmt.Errorf("export SDK: %w", err)
	}
	return absolute, nil
}
