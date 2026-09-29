package extensions

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"adomnia/internal/storage"

	bolt "go.etcd.io/bbolt"
)

const (
	extensionsBucket = "extensions_v2"
	registryStateKey = "registry"
)

type InstallKind string

const (
	InstallManaged     InstallKind = "managed"
	InstallDevelopment InstallKind = "development"
)

type ExtensionInstance struct {
	Manifest         Manifest       `json:"manifest"`
	Enabled          bool           `json:"enabled"`
	Active           bool           `json:"active"`
	Grants           []string       `json:"grants"`
	Settings         map[string]any `json:"settings"`
	InstallDir       string         `json:"installDir"`
	InstallKind      InstallKind    `json:"installKind"`
	Source           string         `json:"source"`
	PackageHash      string         `json:"packageHash,omitempty"`
	InstalledAt      string         `json:"installedAt"`
	Error            string         `json:"error,omitempty"`
	FailureCount     int            `json:"failureCount,omitempty"`
	Quarantined      bool           `json:"quarantined,omitempty"`
	ActivationReason string         `json:"activationReason,omitempty"`
	ActivationTimeMS float64        `json:"activationTimeMs,omitempty"`
	ActivatedAt      string         `json:"activatedAt,omitempty"`
}

type Registry struct {
	mu       sync.RWMutex
	dataDir  string
	rootDir  string
	packages string
	items    map[string]*ExtensionInstance
}

func NewRegistry(dataDir string) *Registry {
	root := filepath.Join(dataDir, "extensions-v2")
	return &Registry{
		dataDir:  dataDir,
		rootDir:  root,
		packages: filepath.Join(root, "packages"),
		items:    map[string]*ExtensionInstance{},
	}
}

func (r *Registry) Init() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := os.MkdirAll(r.packages, 0700); err != nil {
		return fmt.Errorf("create extension registry: %w", err)
	}
	if storage.DB() == nil {
		return fmt.Errorf("extension registry requires initialized storage")
	}
	if err := storage.DB().Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(extensionsBucket))
		return err
	}); err != nil {
		return fmt.Errorf("create extension registry bucket: %w", err)
	}
	if err := r.loadLocked(); err != nil {
		return err
	}
	return r.reconcileLocked()
}

func (r *Registry) List() []ExtensionInstance {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]ExtensionInstance, 0, len(r.items))
	for _, item := range r.items {
		result = append(result, cloneInstance(item))
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Manifest.ID < result[j].Manifest.ID })
	return result
}

func (r *Registry) Get(id string) (ExtensionInstance, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	item, ok := r.items[id]
	if !ok {
		return ExtensionInstance{}, fmt.Errorf("extension not found: %s", id)
	}
	return cloneInstance(item), nil
}

func (r *Registry) InstallDirectory(source string, development bool) (ExtensionInstance, error) {
	absolute, err := filepath.Abs(source)
	if err != nil {
		return ExtensionInstance{}, fmt.Errorf("resolve extension source: %w", err)
	}
	if _, err := Build(absolute); err != nil {
		return ExtensionInstance{}, err
	}
	report := CheckDirectory(absolute)
	if !report.Valid {
		return ExtensionInstance{}, fmt.Errorf("extension validation failed: %s", formatFirstDiagnostic(report.Diagnostics))
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.items[report.Manifest.ID]; exists {
		return ExtensionInstance{}, fmt.Errorf("extension already installed: %s", report.Manifest.ID)
	}

	installDir := absolute
	kind := InstallDevelopment
	packageHash := ""
	if !development {
		kind = InstallManaged
		packageHash, err = hashDirectory(absolute)
		if err != nil {
			return ExtensionInstance{}, err
		}
		installDir = filepath.Join(r.packages, packageHash)
		if err := copyExtensionTree(absolute, installDir); err != nil {
			return ExtensionInstance{}, err
		}
	}
	instance := &ExtensionInstance{
		Manifest:    *report.Manifest,
		Enabled:     false,
		Active:      false,
		Grants:      []string{},
		Settings:    configurationDefaults(report.Manifest.Contributes.Configuration),
		InstallDir:  installDir,
		InstallKind: kind,
		Source:      absolute,
		PackageHash: packageHash,
		InstalledAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	r.items[instance.Manifest.ID] = instance
	if err := r.saveLocked(); err != nil {
		delete(r.items, instance.Manifest.ID)
		if kind == InstallManaged {
			_ = os.RemoveAll(installDir)
		}
		return ExtensionInstance{}, err
	}
	copy := cloneInstance(instance)
	return copy, nil
}

func (r *Registry) UpdateDirectory(source string, development bool) (ExtensionInstance, error) {
	absolute, err := filepath.Abs(source)
	if err != nil {
		return ExtensionInstance{}, err
	}
	if _, err := Build(absolute); err != nil {
		return ExtensionInstance{}, err
	}
	report := CheckDirectory(absolute)
	if !report.Valid {
		return ExtensionInstance{}, fmt.Errorf("extension validation failed: %s", formatFirstDiagnostic(report.Diagnostics))
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	current, exists := r.items[report.Manifest.ID]
	if !exists {
		return ExtensionInstance{}, fmt.Errorf("extension not found: %s", report.Manifest.ID)
	}
	installDir := absolute
	kind := InstallDevelopment
	packageHash := ""
	if !development {
		kind = InstallManaged
		packageHash, err = hashDirectory(absolute)
		if err != nil {
			return ExtensionInstance{}, err
		}
		installDir = filepath.Join(r.packages, packageHash)
		if installDir != current.InstallDir {
			if err := copyExtensionTree(absolute, installDir); err != nil {
				return ExtensionInstance{}, err
			}
		}
	}
	oldInstallDir, oldKind := current.InstallDir, current.InstallKind
	declared := map[string]struct{}{}
	for _, permission := range report.Manifest.Permissions {
		declared[permission] = struct{}{}
	}
	grants := make([]string, 0, len(current.Grants))
	for _, grant := range current.Grants {
		if _, ok := declared[grant]; ok {
			grants = append(grants, grant)
		}
	}
	current.Manifest = *report.Manifest
	current.Grants = grants
	current.InstallDir = installDir
	current.InstallKind = kind
	current.Source = absolute
	current.PackageHash = packageHash
	current.Active = false
	current.Enabled = false
	current.Error = ""
	for key, value := range configurationDefaults(current.Manifest.Contributes.Configuration) {
		if _, ok := current.Settings[key]; !ok {
			current.Settings[key] = value
		}
	}
	if !hasAllGrants(current) {
		current.Error = "permission review required after update"
	}
	if err := r.saveLocked(); err != nil {
		return ExtensionInstance{}, err
	}
	if oldKind == InstallManaged && oldInstallDir != installDir {
		_ = os.RemoveAll(oldInstallDir)
	}
	return cloneInstance(current), nil
}

func (r *Registry) InstallArchive(archivePath string) (ExtensionInstance, error) {
	absolute, err := filepath.Abs(archivePath)
	if err != nil {
		return ExtensionInstance{}, fmt.Errorf("resolve extension archive: %w", err)
	}
	tempRoot, err := os.MkdirTemp(r.rootDir, ".install-*")
	if err != nil {
		return ExtensionInstance{}, fmt.Errorf("create extension install staging: %w", err)
	}
	defer os.RemoveAll(tempRoot)
	if err := extractArchive(absolute, tempRoot); err != nil {
		return ExtensionInstance{}, err
	}
	instance, err := r.InstallDirectory(tempRoot, false)
	if err != nil {
		return ExtensionInstance{}, err
	}
	r.mu.Lock()
	if item := r.items[instance.Manifest.ID]; item != nil {
		item.Source = absolute
		instance = cloneInstance(item)
		err = r.saveLocked()
	}
	r.mu.Unlock()
	return instance, err
}

func (r *Registry) UpdateArchive(archivePath string) (ExtensionInstance, error) {
	absolute, err := filepath.Abs(archivePath)
	if err != nil {
		return ExtensionInstance{}, fmt.Errorf("resolve extension archive: %w", err)
	}
	tempRoot, err := os.MkdirTemp(r.rootDir, ".update-*")
	if err != nil {
		return ExtensionInstance{}, err
	}
	defer os.RemoveAll(tempRoot)
	if err := extractArchive(absolute, tempRoot); err != nil {
		return ExtensionInstance{}, err
	}
	instance, err := r.UpdateDirectory(tempRoot, false)
	if err != nil {
		return ExtensionInstance{}, err
	}
	r.mu.Lock()
	if item := r.items[instance.Manifest.ID]; item != nil {
		item.Source = absolute
		instance = cloneInstance(item)
		err = r.saveLocked()
	}
	r.mu.Unlock()
	return instance, err
}

func (r *Registry) SetGrants(id string, grants []string) (ExtensionInstance, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, ok := r.items[id]
	if !ok {
		return ExtensionInstance{}, fmt.Errorf("extension not found: %s", id)
	}
	declared := make(map[string]struct{}, len(item.Manifest.Permissions))
	for _, permission := range item.Manifest.Permissions {
		declared[permission] = struct{}{}
	}
	unique := map[string]struct{}{}
	for _, grant := range grants {
		if _, ok := declared[grant]; !ok {
			return ExtensionInstance{}, fmt.Errorf("permission is not declared by %s: %s", id, grant)
		}
		unique[grant] = struct{}{}
	}
	item.Grants = item.Grants[:0]
	for grant := range unique {
		item.Grants = append(item.Grants, grant)
	}
	sort.Strings(item.Grants)
	if !hasAllGrants(item) {
		item.Enabled = false
		item.Active = false
	}
	if err := r.saveLocked(); err != nil {
		return ExtensionInstance{}, err
	}
	return cloneInstance(item), nil
}

func (r *Registry) Enable(id string) (ExtensionInstance, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, ok := r.items[id]
	if !ok {
		return ExtensionInstance{}, fmt.Errorf("extension not found: %s", id)
	}
	report := CheckDirectory(item.InstallDir)
	if !report.Valid {
		item.Error = formatFirstDiagnostic(report.Diagnostics)
		_ = r.saveLocked()
		return ExtensionInstance{}, fmt.Errorf("installed extension is invalid: %s", item.Error)
	}
	item.Manifest = *report.Manifest
	if !hasAllGrants(item) {
		item.Enabled = false
		item.Active = false
		item.Error = "permission review required after manifest change"
		_ = r.saveLocked()
		return ExtensionInstance{}, fmt.Errorf("permission review required before enabling %s", id)
	}
	for key, value := range configurationDefaults(item.Manifest.Contributes.Configuration) {
		if _, exists := item.Settings[key]; !exists {
			item.Settings[key] = value
		}
	}
	item.Enabled = true
	item.Quarantined = false
	item.FailureCount = 0
	item.Error = ""
	if err := r.saveLocked(); err != nil {
		return ExtensionInstance{}, err
	}
	return cloneInstance(item), nil
}

func (r *Registry) Disable(id string) (ExtensionInstance, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, ok := r.items[id]
	if !ok {
		return ExtensionInstance{}, fmt.Errorf("extension not found: %s", id)
	}
	item.Enabled = false
	item.Active = false
	if err := r.saveLocked(); err != nil {
		return ExtensionInstance{}, err
	}
	return cloneInstance(item), nil
}

func (r *Registry) RecordRuntimeFailure(id string, failure error) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, ok := r.items[id]
	if !ok {
		return fmt.Errorf("extension not found: %s", id)
	}
	item.Active = false
	item.Error = failure.Error()
	item.FailureCount++
	if item.FailureCount >= 3 {
		item.Enabled = false
		item.Quarantined = true
		item.Error = "extension quarantined after repeated runtime failures: " + failure.Error()
	}
	return r.saveLocked()
}

func (r *Registry) SetActivationInfo(id, reason string, durationMS float64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, ok := r.items[id]
	if !ok {
		return fmt.Errorf("extension not found: %s", id)
	}
	item.ActivationReason = reason
	item.ActivationTimeMS = durationMS
	item.ActivatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return r.saveLocked()
}

func (r *Registry) SetActive(id string, active bool, errorMessage string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, ok := r.items[id]
	if !ok {
		return fmt.Errorf("extension not found: %s", id)
	}
	item.Active = active
	item.Error = errorMessage
	return r.saveLocked()
}

func (r *Registry) SetSetting(id, key string, value interface{}) (ExtensionInstance, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, ok := r.items[id]
	if !ok {
		return ExtensionInstance{}, fmt.Errorf("extension not found: %s", id)
	}
	property, ok := item.Manifest.Contributes.Configuration[key]
	if !ok {
		return ExtensionInstance{}, fmt.Errorf("unknown extension setting: %s", key)
	}
	normalized := normalizeJSONValue(value)
	if !configurationValueMatches(property.Type, normalized) {
		return ExtensionInstance{}, fmt.Errorf("setting %s does not match type %s", key, property.Type)
	}
	if len(property.Enum) > 0 {
		matched := false
		candidate, _ := json.Marshal(normalized)
		for _, allowed := range property.Enum {
			encoded, _ := json.Marshal(normalizeJSONValue(allowed))
			if string(encoded) == string(candidate) {
				matched = true
				break
			}
		}
		if !matched {
			return ExtensionInstance{}, fmt.Errorf("setting %s is not an allowed enum value", key)
		}
	}
	if number, ok := normalized.(float64); ok {
		if property.Minimum != nil && number < *property.Minimum {
			return ExtensionInstance{}, fmt.Errorf("setting %s is below minimum", key)
		}
		if property.Maximum != nil && number > *property.Maximum {
			return ExtensionInstance{}, fmt.Errorf("setting %s is above maximum", key)
		}
	}
	item.Settings[key] = normalized
	if err := r.saveLocked(); err != nil {
		return ExtensionInstance{}, err
	}
	return cloneInstance(item), nil
}

func (r *Registry) Uninstall(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, ok := r.items[id]
	if !ok {
		return fmt.Errorf("extension not found: %s", id)
	}
	if item.InstallKind == InstallManaged {
		absolutePackages, _ := filepath.Abs(r.packages)
		absoluteInstall, _ := filepath.Abs(item.InstallDir)
		relative, err := filepath.Rel(absolutePackages, absoluteInstall)
		if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return fmt.Errorf("refusing to remove extension outside managed package directory")
		}
		if err := os.RemoveAll(absoluteInstall); err != nil {
			return fmt.Errorf("remove managed extension: %w", err)
		}
	}
	delete(r.items, id)
	return r.saveLocked()
}

func (r *Registry) loadLocked() error {
	return storage.DB().View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(extensionsBucket))
		if bucket == nil {
			return fmt.Errorf("extension registry bucket not found")
		}
		data := bucket.Get([]byte(registryStateKey))
		if len(data) == 0 {
			return nil
		}
		var items map[string]*ExtensionInstance
		if err := json.Unmarshal(data, &items); err != nil {
			return fmt.Errorf("decode extension registry: %w", err)
		}
		if items != nil {
			r.items = items
		}
		return nil
	})
}

func (r *Registry) saveLocked() error {
	data, err := json.Marshal(r.items)
	if err != nil {
		return fmt.Errorf("encode extension registry: %w", err)
	}
	return storage.DB().Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(extensionsBucket))
		if bucket == nil {
			return fmt.Errorf("extension registry bucket not found")
		}
		return bucket.Put([]byte(registryStateKey), data)
	})
}

func (r *Registry) reconcileLocked() error {
	changed := false
	for id, item := range r.items {
		report := CheckDirectory(item.InstallDir)
		if !report.Valid {
			item.Enabled = false
			item.Active = false
			item.Error = formatFirstDiagnostic(report.Diagnostics)
			changed = true
			continue
		}
		if report.Manifest.ID != id {
			item.Enabled = false
			item.Active = false
			item.Error = "installed manifest id does not match registry id"
			changed = true
			continue
		}
		item.Manifest = *report.Manifest
		item.Active = false
		if !hasAllGrants(item) && item.Enabled {
			item.Enabled = false
			changed = true
		}
	}
	if changed {
		return r.saveLocked()
	}
	return nil
}

func cloneInstance(item *ExtensionInstance) ExtensionInstance {
	copy := *item
	copy.Grants = append([]string(nil), item.Grants...)
	copy.Settings = make(map[string]any, len(item.Settings))
	for key, value := range item.Settings {
		copy.Settings[key] = value
	}
	return copy
}

func hasAllGrants(item *ExtensionInstance) bool {
	grants := make(map[string]struct{}, len(item.Grants))
	for _, grant := range item.Grants {
		grants[grant] = struct{}{}
	}
	for _, permission := range item.Manifest.Permissions {
		if _, ok := grants[permission]; !ok {
			return false
		}
	}
	return true
}

func configurationDefaults(properties map[string]ConfigurationProperty) map[string]any {
	settings := make(map[string]any, len(properties))
	for key, property := range properties {
		if property.Default != nil {
			settings[key] = property.Default
		}
	}
	return settings
}

func normalizeJSONValue(value interface{}) interface{} {
	data, err := json.Marshal(value)
	if err != nil {
		return value
	}
	var normalized interface{}
	if json.Unmarshal(data, &normalized) != nil {
		return value
	}
	return normalized
}

func hashDirectory(root string) (string, error) {
	files, err := packageFiles(root, "")
	if err != nil {
		return "", err
	}
	hasher := sha256.New()
	for _, file := range files {
		_, _ = io.WriteString(hasher, file.relative)
		_, _ = io.WriteString(hasher, "\x00")
		content, err := os.Open(file.absolute)
		if err != nil {
			return "", err
		}
		_, copyErr := io.Copy(hasher, content)
		closeErr := content.Close()
		if copyErr != nil {
			return "", copyErr
		}
		if closeErr != nil {
			return "", closeErr
		}
		_, _ = io.WriteString(hasher, "\x00")
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func copyExtensionTree(source, destination string) error {
	if _, err := os.Stat(destination); err == nil {
		return fmt.Errorf("managed package already exists: %s", destination)
	} else if !os.IsNotExist(err) {
		return err
	}
	temporary, err := os.MkdirTemp(filepath.Dir(destination), ".copy-*")
	if err != nil {
		return fmt.Errorf("create extension staging directory: %w", err)
	}
	defer os.RemoveAll(temporary)
	files, err := packageFiles(source, "")
	if err != nil {
		return err
	}
	for _, file := range files {
		target := filepath.Join(temporary, filepath.FromSlash(file.relative))
		if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			return err
		}
		input, err := os.Open(file.absolute)
		if err != nil {
			return err
		}
		output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			_ = input.Close()
			return err
		}
		_, copyErr := io.Copy(output, io.LimitReader(input, MaxPackageBytes+1))
		inputErr := input.Close()
		outputErr := output.Close()
		if copyErr != nil {
			return copyErr
		}
		if inputErr != nil {
			return inputErr
		}
		if outputErr != nil {
			return outputErr
		}
	}
	if err := os.Rename(temporary, destination); err != nil {
		return fmt.Errorf("publish managed extension: %w", err)
	}
	return nil
}

func extractArchive(archivePath, destination string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("open extension package: %w", err)
	}
	defer reader.Close()
	if len(reader.File) > MaxPackageFiles {
		return fmt.Errorf("extension package contains too many files: %d", len(reader.File))
	}
	var total int64
	for _, entry := range reader.File {
		name := filepath.ToSlash(entry.Name)
		if name == "" || strings.HasPrefix(name, "/") || strings.Contains(name, "\\") || strings.Contains(name, ":") {
			return fmt.Errorf("unsafe extension package path: %s", entry.Name)
		}
		clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(name)))
		if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || clean != strings.TrimSuffix(name, "/") {
			return fmt.Errorf("unsafe extension package path: %s", entry.Name)
		}
		if entry.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("extension package contains symbolic link: %s", entry.Name)
		}
		if entry.FileInfo().IsDir() {
			continue
		}
		if !entry.Mode().IsRegular() {
			return fmt.Errorf("extension package contains non-regular file: %s", entry.Name)
		}
		total += int64(entry.UncompressedSize64)
		if total > MaxPackageBytes {
			return fmt.Errorf("extension package exceeds %d bytes", MaxPackageBytes)
		}
		target := filepath.Join(destination, filepath.FromSlash(clean))
		if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			return err
		}
		input, err := entry.Open()
		if err != nil {
			return err
		}
		output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			_ = input.Close()
			return err
		}
		written, copyErr := io.Copy(output, io.LimitReader(input, MaxPackageBytes-total+int64(entry.UncompressedSize64)+1))
		inputErr := input.Close()
		outputErr := output.Close()
		if copyErr != nil {
			return copyErr
		}
		if written != int64(entry.UncompressedSize64) {
			return fmt.Errorf("extension package size mismatch for %s", entry.Name)
		}
		if inputErr != nil {
			return inputErr
		}
		if outputErr != nil {
			return outputErr
		}
	}
	return nil
}
