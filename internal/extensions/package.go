package extensions

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type PackageResult struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Files  int    `json:"files"`
	Bytes  int64  `json:"bytes"`
}

// Pack creates a deterministic .adomnia-extension ZIP after validating the source tree.
func Pack(root, destination string) (PackageResult, ValidationReport, error) {
	if _, err := Build(root); err != nil {
		return PackageResult{}, invalidReport(root, "package.build", "source", err.Error()), err
	}
	report := CheckDirectory(root)
	if !report.Valid {
		return PackageResult{}, report, nil
	}
	if destination == "" {
		parent := filepath.Dir(report.Root)
		filename := fmt.Sprintf("%s-%s.adomnia-extension", report.Manifest.ID, report.Manifest.Version)
		destination = filepath.Join(parent, filename)
	}
	absDestination, err := filepath.Abs(destination)
	if err != nil {
		return PackageResult{}, report, fmt.Errorf("resolve package destination: %w", err)
	}
	if !strings.HasSuffix(strings.ToLower(absDestination), ".adomnia-extension") {
		absDestination += ".adomnia-extension"
	}
	if err := os.MkdirAll(filepath.Dir(absDestination), 0755); err != nil {
		return PackageResult{}, report, fmt.Errorf("create package directory: %w", err)
	}

	files, err := packageFiles(report.Root, absDestination)
	if err != nil {
		return PackageResult{}, report, err
	}
	temporary, err := os.CreateTemp(filepath.Dir(absDestination), ".adomnia-extension-*.tmp")
	if err != nil {
		return PackageResult{}, report, fmt.Errorf("create temporary package: %w", err)
	}
	temporaryPath := temporary.Name()
	removeTemporary := true
	defer func() {
		_ = temporary.Close()
		if removeTemporary {
			_ = os.Remove(temporaryPath)
		}
	}()

	hasher := sha256.New()
	writer := zip.NewWriter(io.MultiWriter(temporary, hasher))
	fixedTime := time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)
	var packageBytes int64
	for _, item := range files {
		info, err := os.Stat(item.absolute)
		if err != nil {
			_ = writer.Close()
			return PackageResult{}, report, fmt.Errorf("inspect %s: %w", item.relative, err)
		}
		header := &zip.FileHeader{Name: item.relative, Method: zip.Deflate}
		header.SetModTime(fixedTime)
		header.SetMode(info.Mode().Perm())
		entryWriter, err := writer.CreateHeader(header)
		if err != nil {
			_ = writer.Close()
			return PackageResult{}, report, fmt.Errorf("create archive entry %s: %w", item.relative, err)
		}
		source, err := os.Open(item.absolute)
		if err != nil {
			_ = writer.Close()
			return PackageResult{}, report, fmt.Errorf("open %s: %w", item.relative, err)
		}
		written, copyErr := io.Copy(entryWriter, source)
		closeErr := source.Close()
		if copyErr != nil {
			_ = writer.Close()
			return PackageResult{}, report, fmt.Errorf("archive %s: %w", item.relative, copyErr)
		}
		if closeErr != nil {
			_ = writer.Close()
			return PackageResult{}, report, fmt.Errorf("close %s: %w", item.relative, closeErr)
		}
		packageBytes += written
	}
	if err := writer.Close(); err != nil {
		return PackageResult{}, report, fmt.Errorf("finalize package: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return PackageResult{}, report, fmt.Errorf("sync package: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return PackageResult{}, report, fmt.Errorf("close package: %w", err)
	}
	if err := publishPackage(temporaryPath, absDestination); err != nil {
		return PackageResult{}, report, err
	}
	removeTemporary = false
	return PackageResult{Path: absDestination, SHA256: hex.EncodeToString(hasher.Sum(nil)), Files: len(files), Bytes: packageBytes}, report, nil
}

type packageFile struct {
	relative string
	absolute string
}

func publishPackage(temporaryPath, destination string) error {
	info, err := os.Lstat(destination)
	if os.IsNotExist(err) {
		if err := os.Rename(temporaryPath, destination); err != nil {
			return fmt.Errorf("publish package: %w", err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect existing package: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("refusing to replace non-regular package destination: %s", destination)
	}
	backupFile, err := os.CreateTemp(filepath.Dir(destination), ".adomnia-extension-backup-*.tmp")
	if err != nil {
		return fmt.Errorf("prepare package replacement: %w", err)
	}
	backupPath := backupFile.Name()
	if err := backupFile.Close(); err != nil {
		_ = os.Remove(backupPath)
		return fmt.Errorf("prepare package replacement: %w", err)
	}
	if err := os.Remove(backupPath); err != nil {
		return fmt.Errorf("prepare package replacement: %w", err)
	}
	if err := os.Rename(destination, backupPath); err != nil {
		return fmt.Errorf("backup existing package: %w", err)
	}
	if err := os.Rename(temporaryPath, destination); err != nil {
		restoreErr := os.Rename(backupPath, destination)
		if restoreErr != nil {
			return fmt.Errorf("publish package: %w (also failed to restore previous package: %v)", err, restoreErr)
		}
		return fmt.Errorf("publish package: %w", err)
	}
	_ = os.Remove(backupPath)
	return nil
}

func packageFiles(root, destination string) ([]packageFile, error) {
	files := []packageFile{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}
		if shouldExcludePath(relative, entry.IsDir()) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to package symbolic link %s", relative)
		}
		if entry.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("refusing to package non-regular file %s", relative)
		}
		absolute, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		if absolute == destination {
			return nil
		}
		files = append(files, packageFile{relative: filepath.ToSlash(relative), absolute: absolute})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("inspect package files: %w", err)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].relative < files[j].relative })
	return files, nil
}
