package extensions

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	ManifestFileName               = "manifest.json"
	SupportedManifestVersion       = 2
	SupportedAPIVersion            = "2.0"
	MaxPackageFiles                = 4096
	MaxPackageBytes          int64 = 64 << 20
	MaxEntrypointBytes       int64 = 4 << 20
	MaxWebviewHTMLBytes      int64 = 2 << 20
)

var (
	extensionIDPattern    = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*(\.[a-z0-9][a-z0-9-]*)+$`)
	contributionIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*(\.[a-z0-9][a-z0-9-]*)+$`)
	semverPattern         = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$`)
	engineTokenPattern    = regexp.MustCompile(`^(?:\^|~|>=|<=|>|<|=)?(?:0|[1-9][0-9]*)(?:\.(?:0|[1-9][0-9]*|[xX*])){0,2}(?:-[0-9A-Za-z.-]+)?$`)
)

// Manifest is the contract read without activating extension code.
type Manifest struct {
	Schema           string        `json:"$schema,omitempty"`
	ManifestVersion  int           `json:"manifestVersion"`
	ID               string        `json:"id"`
	Name             string        `json:"name"`
	Version          string        `json:"version"`
	Publisher        string        `json:"publisher"`
	Description      string        `json:"description,omitempty"`
	License          string        `json:"license,omitempty"`
	Homepage         string        `json:"homepage,omitempty"`
	Engines          Engines       `json:"engines"`
	APIVersion       string        `json:"apiVersion"`
	Main             string        `json:"main"`
	Source           string        `json:"source,omitempty"`
	ActivationEvents []string      `json:"activationEvents,omitempty"`
	Permissions      []string      `json:"permissions,omitempty"`
	Contributes      Contributions `json:"contributes,omitempty"`
}

type Engines struct {
	Adomnia string `json:"adomnia"`
}

type Contributions struct {
	Commands      []CommandContribution            `json:"commands,omitempty"`
	Views         []ViewContribution               `json:"views,omitempty"`
	Configuration map[string]ConfigurationProperty `json:"configuration,omitempty"`
	Keybindings   []KeybindingContribution         `json:"keybindings,omitempty"`
	Menus         map[string][]MenuContribution    `json:"menus,omitempty"`
	StatusBar     []StatusBarContribution          `json:"statusBar,omitempty"`
}

type CommandContribution struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Category string `json:"category,omitempty"`
	Icon     string `json:"icon,omitempty"`
}

type ViewContribution struct {
	ID        string `json:"id"`
	Container string `json:"container"`
	Name      string `json:"name"`
	Renderer  string `json:"renderer"`
	Entry     string `json:"entry,omitempty"`
	When      string `json:"when,omitempty"`
}

type ConfigurationProperty struct {
	Type        string        `json:"type"`
	Default     interface{}   `json:"default,omitempty"`
	Description string        `json:"description,omitempty"`
	Enum        []interface{} `json:"enum,omitempty"`
	Minimum     *float64      `json:"minimum,omitempty"`
	Maximum     *float64      `json:"maximum,omitempty"`
}

type KeybindingContribution struct {
	Command string `json:"command"`
	Key     string `json:"key"`
	When    string `json:"when,omitempty"`
}

type MenuContribution struct {
	Command string `json:"command"`
	When    string `json:"when,omitempty"`
	Group   string `json:"group,omitempty"`
}

type StatusBarContribution struct {
	ID       string `json:"id"`
	Text     string `json:"text"`
	Command  string `json:"command,omitempty"`
	Priority int    `json:"priority,omitempty"`
	When     string `json:"when,omitempty"`
}

type Diagnostic struct {
	Level   string `json:"level"`
	Code    string `json:"code"`
	Path    string `json:"path,omitempty"`
	Message string `json:"message"`
}

type ValidationReport struct {
	Valid       bool         `json:"valid"`
	Root        string       `json:"root,omitempty"`
	Manifest    *Manifest    `json:"manifest,omitempty"`
	Files       int          `json:"files"`
	Bytes       int64        `json:"bytes"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

var exactPermissions = map[string]struct{}{
	"notifications": {}, "workspaceState": {}, "globalState": {}, "secrets.own": {},
	"collections.read": {}, "collections.write": {}, "environments.read": {}, "environments.write": {},
	"tabs.read": {}, "tabs.write": {}, "workspace.read": {}, "cookies.read": {}, "cookies.write": {},
	"requests.read": {}, "requests.execute": {},
	"responses.read": {}, "variables.read": {}, "variables.provide": {}, "assertions.provide": {},
	"clipboard.read": {}, "clipboard.write": {}, "browserDebug.read": {}, "browserDebug.control": {},
	"proxy.read": {}, "proxy.control": {}, "mock.read": {}, "mock.control": {}, "flows.read": {}, "flows.execute": {},
	"databases.read": {}, "databases.execute": {}, "brokers.read": {}, "brokers.publish": {},
	"documents.read": {}, "documents.readContents": {}, "documents.write": {}, "vault.requestReference": {}, "ai.execute": {}, "network.private": {},
}

var scopedPermissionPrefixes = []string{"network:", "workspace.files.read:", "workspace.files.write:", "process.spawn:"}

var knownMenuLocations = map[string]struct{}{
	"commandPalette": {}, "tab/context": {}, "request/toolbar": {}, "response/toolbar": {},
}

var knownActivationEvents = map[string]struct{}{
	"onStartup": {}, "onWorkspaceOpen": {}, "onWorkspaceClose": {}, "onRequest": {}, "onResponse": {},
	"onSend": {}, "onSave": {}, "onImport": {}, "onExport": {}, "onThemeChange": {}, "onEnvChange": {},
	"onTabOpen": {}, "onTabClose": {}, "onAssertions": {}, "onVariables": {}, "onBrowserNetwork": {}, "onMockHit": {}, "onProxyTraffic": {}, "onFlowProgress": {}, "onFlowComplete": {}, "onDatabaseComplete": {}, "onBrokerPublishComplete": {}, "onDocumentReadComplete": {}, "onDocumentWriteComplete": {}, "onAIComplete": {},
}

func DecodeManifest(data []byte) (Manifest, error) {
	var manifest Manifest
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode manifest: %w", err)
	}
	var trailing interface{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return Manifest{}, fmt.Errorf("decode manifest: multiple JSON values")
		}
		return Manifest{}, fmt.Errorf("decode manifest: trailing content: %w", err)
	}
	return manifest, nil
}

func LoadManifest(root string) (Manifest, error) {
	data, err := os.ReadFile(filepath.Join(root, ManifestFileName))
	if err != nil {
		return Manifest{}, fmt.Errorf("read %s: %w", ManifestFileName, err)
	}
	return DecodeManifest(data)
}

func ValidateManifest(manifest Manifest) []Diagnostic {
	diagnostics := make([]Diagnostic, 0)
	add := func(code, path, message string) {
		diagnostics = append(diagnostics, Diagnostic{Level: "error", Code: code, Path: path, Message: message})
	}

	if manifest.ManifestVersion != SupportedManifestVersion {
		add("manifest.version", "manifestVersion", fmt.Sprintf("must be %d", SupportedManifestVersion))
	}
	if len(manifest.ID) > 128 || !extensionIDPattern.MatchString(manifest.ID) {
		add("manifest.id", "id", "must be a lowercase namespaced ID such as publisher.extension")
	}
	if strings.TrimSpace(manifest.Name) == "" {
		add("manifest.name", "name", "is required")
	}
	if !semverPattern.MatchString(manifest.Version) {
		add("manifest.semver", "version", "must be a semantic version such as 1.0.0")
	}
	if strings.TrimSpace(manifest.Publisher) == "" || strings.Contains(manifest.Publisher, ".") {
		add("manifest.publisher", "publisher", "must be the first namespace segment of id")
	} else if !strings.HasPrefix(manifest.ID, manifest.Publisher+".") {
		add("manifest.publisherMismatch", "publisher", "must match the first namespace segment of id")
	}
	if !validEngineRange(manifest.Engines.Adomnia) {
		add("manifest.engine", "engines.adomnia", "must be a non-empty semver range such as >=1.0.0 <2")
	}
	if manifest.APIVersion != SupportedAPIVersion {
		add("manifest.apiVersion", "apiVersion", fmt.Sprintf("unsupported API version %q; expected %s", manifest.APIVersion, SupportedAPIVersion))
	}
	if err := validateRelativePath(manifest.Main); err != nil {
		add("manifest.main", "main", err.Error())
	} else {
		ext := strings.ToLower(filepath.Ext(manifest.Main))
		if ext != ".js" && ext != ".mjs" && ext != ".cjs" {
			add("manifest.mainType", "main", "must point to a packaged JavaScript entry point")
		}
	}
	if manifest.Source != "" {
		if err := validateRelativePath(manifest.Source); err != nil {
			add("manifest.source", "source", err.Error())
		} else {
			ext := strings.ToLower(filepath.Ext(manifest.Source))
			if ext != ".ts" && ext != ".tsx" && ext != ".js" && ext != ".mjs" {
				add("manifest.sourceType", "source", "must point to a JavaScript or TypeScript source file")
			}
		}
	}

	commands := make(map[string]struct{}, len(manifest.Contributes.Commands))
	for i, command := range manifest.Contributes.Commands {
		path := fmt.Sprintf("contributes.commands[%d]", i)
		validateOwnedID(manifest.ID, command.ID, path+".id", add)
		if _, exists := commands[command.ID]; exists {
			add("contribution.duplicateCommand", path+".id", "command id is duplicated")
		}
		commands[command.ID] = struct{}{}
		if strings.TrimSpace(command.Title) == "" {
			add("contribution.commandTitle", path+".title", "is required")
		}
	}

	views := make(map[string]struct{}, len(manifest.Contributes.Views))
	for i, view := range manifest.Contributes.Views {
		path := fmt.Sprintf("contributes.views[%d]", i)
		validateOwnedID(manifest.ID, view.ID, path+".id", add)
		if _, exists := views[view.ID]; exists {
			add("contribution.duplicateView", path+".id", "view id is duplicated")
		}
		views[view.ID] = struct{}{}
		if strings.TrimSpace(view.Container) == "" {
			add("contribution.viewContainer", path+".container", "is required")
		}
		if strings.TrimSpace(view.Name) == "" {
			add("contribution.viewName", path+".name", "is required")
		}
		if err := validateWhenClause(view.When); err != nil {
			add("contribution.when", path+".when", err.Error())
		}
		if view.Renderer != "declarative" && view.Renderer != "webview" {
			add("contribution.viewRenderer", path+".renderer", "must be declarative or webview")
		}
		if view.Renderer == "webview" {
			if err := validateRelativePath(view.Entry); err != nil {
				add("contribution.webviewEntry", path+".entry", err.Error())
			} else if strings.ToLower(filepath.Ext(view.Entry)) != ".html" {
				add("contribution.webviewEntryType", path+".entry", "must point to a packaged HTML file")
			}
		} else if view.Entry != "" {
			add("contribution.declarativeEntry", path+".entry", "declarative views cannot declare an HTML entry")
		}
	}

	for key, property := range manifest.Contributes.Configuration {
		path := "contributes.configuration." + key
		validateOwnedID(manifest.ID, key, path, add)
		switch property.Type {
		case "string", "number", "integer", "boolean", "array", "object":
		default:
			add("contribution.configurationType", path+".type", "must be string, number, integer, boolean, array, or object")
		}
		if property.Minimum != nil && property.Maximum != nil && *property.Minimum > *property.Maximum {
			add("contribution.configurationRange", path, "minimum cannot be greater than maximum")
		}
		if property.Default != nil && !configurationValueMatches(property.Type, property.Default) {
			add("contribution.configurationDefault", path+".default", "does not match the declared type")
		}
		for i, enumValue := range property.Enum {
			if !configurationValueMatches(property.Type, enumValue) {
				add("contribution.configurationEnum", fmt.Sprintf("%s.enum[%d]", path, i), "does not match the declared type")
			}
		}
	}

	for i, binding := range manifest.Contributes.Keybindings {
		path := fmt.Sprintf("contributes.keybindings[%d]", i)
		validateCommandReference(commands, binding.Command, path+".command", add)
		if strings.TrimSpace(binding.Key) == "" {
			add("contribution.keybindingKey", path+".key", "is required")
		}
		if err := validateWhenClause(binding.When); err != nil {
			add("contribution.when", path+".when", err.Error())
		}
	}
	for location, entries := range manifest.Contributes.Menus {
		if _, known := knownMenuLocations[location]; !known {
			add("contribution.menuLocation", "contributes.menus", fmt.Sprintf("unknown menu location %q", location))
		}
		for i, entry := range entries {
			path := fmt.Sprintf("contributes.menus.%s[%d]", location, i)
			validateCommandReference(commands, entry.Command, path+".command", add)
			if err := validateWhenClause(entry.When); err != nil {
				add("contribution.when", path+".when", err.Error())
			}
		}
	}
	statusIDs := map[string]struct{}{}
	for i, item := range manifest.Contributes.StatusBar {
		path := fmt.Sprintf("contributes.statusBar[%d]", i)
		validateOwnedID(manifest.ID, item.ID, path+".id", add)
		if _, exists := statusIDs[item.ID]; exists {
			add("contribution.duplicateStatus", path+".id", "status item id is duplicated")
		}
		statusIDs[item.ID] = struct{}{}
		if strings.TrimSpace(item.Text) == "" {
			add("contribution.statusText", path+".text", "is required")
		}
		if item.Command != "" {
			validateCommandReference(commands, item.Command, path+".command", add)
		}
		if err := validateWhenClause(item.When); err != nil {
			add("contribution.when", path+".when", err.Error())
		}
	}

	seenPermissions := map[string]struct{}{}
	for i, permission := range manifest.Permissions {
		path := fmt.Sprintf("permissions[%d]", i)
		if !knownPermission(permission) {
			add("permission.unknown", path, fmt.Sprintf("unknown permission %q", permission))
		}
		if _, exists := seenPermissions[permission]; exists {
			add("permission.duplicate", path, "permission is duplicated")
		}
		seenPermissions[permission] = struct{}{}
	}

	if len(manifest.ActivationEvents) == 0 {
		add("activation.required", "activationEvents", "must declare at least one lazy activation event")
	}
	seenEvents := map[string]struct{}{}
	for i, event := range manifest.ActivationEvents {
		path := fmt.Sprintf("activationEvents[%d]", i)
		if _, exists := seenEvents[event]; exists {
			add("activation.duplicate", path, "activation event is duplicated")
		}
		seenEvents[event] = struct{}{}
		switch {
		case strings.HasPrefix(event, "onCommand:"):
			id := strings.TrimPrefix(event, "onCommand:")
			if _, exists := commands[id]; !exists {
				add("activation.command", path, "references a command not declared by this extension")
			}
		case strings.HasPrefix(event, "onView:"):
			id := strings.TrimPrefix(event, "onView:")
			if _, exists := views[id]; !exists {
				add("activation.view", path, "references a view not declared by this extension")
			}
		case strings.HasPrefix(event, "onConfiguration:"):
			key := strings.TrimPrefix(event, "onConfiguration:")
			if _, exists := manifest.Contributes.Configuration[key]; !exists {
				add("activation.configuration", path, "references a setting not declared by this extension")
			}
		default:
			if _, exists := knownActivationEvents[event]; !exists {
				add("activation.unknown", path, fmt.Sprintf("unknown activation event %q", event))
			}
		}
		if permission := eventPermission(event); permission != "" {
			if _, declared := seenPermissions[permission]; !declared {
				add("activation.permission", path, fmt.Sprintf("requires declared permission %q", permission))
			}
		}
	}

	sort.SliceStable(diagnostics, func(i, j int) bool {
		if diagnostics[i].Path == diagnostics[j].Path {
			return diagnostics[i].Code < diagnostics[j].Code
		}
		return diagnostics[i].Path < diagnostics[j].Path
	})
	return diagnostics
}

func CheckDirectory(root string) ValidationReport {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return invalidReport(root, "package.root", "", err.Error())
	}
	rootInfo, err := os.Lstat(absRoot)
	if err != nil {
		return invalidReport(absRoot, "package.root", "", err.Error())
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 {
		return invalidReport(absRoot, "package.rootSymlink", "", "extension root cannot be a symbolic link")
	}
	if !rootInfo.IsDir() {
		return invalidReport(absRoot, "package.root", "", "extension root must be a directory")
	}

	manifest, err := LoadManifest(absRoot)
	if err != nil {
		return invalidReport(absRoot, "manifest.decode", ManifestFileName, err.Error())
	}
	diagnostics := ValidateManifest(manifest)
	files, bytesUsed, walkDiagnostics := inspectPackageFiles(absRoot)
	diagnostics = append(diagnostics, walkDiagnostics...)

	if validateRelativePath(manifest.Main) == nil {
		entrypoint := filepath.Join(absRoot, filepath.FromSlash(manifest.Main))
		entryInfo, entryErr := os.Lstat(entrypoint)
		switch {
		case entryErr != nil:
			diagnostics = append(diagnostics, Diagnostic{Level: "error", Code: "package.mainMissing", Path: "main", Message: "entry point does not exist"})
		case entryInfo.Mode()&os.ModeSymlink != 0:
			diagnostics = append(diagnostics, Diagnostic{Level: "error", Code: "package.mainSymlink", Path: "main", Message: "entry point cannot be a symbolic link"})
		case !entryInfo.Mode().IsRegular():
			diagnostics = append(diagnostics, Diagnostic{Level: "error", Code: "package.mainType", Path: "main", Message: "entry point must be a regular file"})
		case entryInfo.Size() > MaxEntrypointBytes:
			diagnostics = append(diagnostics, Diagnostic{Level: "error", Code: "package.mainSize", Path: "main", Message: fmt.Sprintf("entry point exceeds %d bytes", MaxEntrypointBytes)})
		}
	}
	if manifest.Source != "" && validateRelativePath(manifest.Source) == nil {
		sourcePath := filepath.Join(absRoot, filepath.FromSlash(manifest.Source))
		sourceInfo, sourceErr := os.Lstat(sourcePath)
		if sourceErr != nil {
			diagnostics = append(diagnostics, Diagnostic{Level: "error", Code: "package.sourceMissing", Path: "source", Message: "source entry does not exist"})
		} else if sourceInfo.Mode()&os.ModeSymlink != 0 || !sourceInfo.Mode().IsRegular() {
			diagnostics = append(diagnostics, Diagnostic{Level: "error", Code: "package.sourceType", Path: "source", Message: "source entry must be a regular file"})
		}
	}
	for i, view := range manifest.Contributes.Views {
		if view.Renderer != "webview" || validateRelativePath(view.Entry) != nil {
			continue
		}
		entryPath := filepath.Join(absRoot, filepath.FromSlash(view.Entry))
		entryInfo, entryErr := os.Lstat(entryPath)
		path := fmt.Sprintf("contributes.views[%d].entry", i)
		if entryErr != nil {
			diagnostics = append(diagnostics, Diagnostic{Level: "error", Code: "package.webviewMissing", Path: path, Message: "webview entry does not exist"})
		} else if entryInfo.Mode()&os.ModeSymlink != 0 || !entryInfo.Mode().IsRegular() {
			diagnostics = append(diagnostics, Diagnostic{Level: "error", Code: "package.webviewType", Path: path, Message: "webview entry must be a regular file"})
		} else if entryInfo.Size() > MaxWebviewHTMLBytes {
			diagnostics = append(diagnostics, Diagnostic{Level: "error", Code: "package.webviewSize", Path: path, Message: fmt.Sprintf("webview HTML exceeds %d bytes", MaxWebviewHTMLBytes)})
		}
	}

	sort.SliceStable(diagnostics, func(i, j int) bool {
		if diagnostics[i].Path == diagnostics[j].Path {
			return diagnostics[i].Code < diagnostics[j].Code
		}
		return diagnostics[i].Path < diagnostics[j].Path
	})
	return ValidationReport{Valid: !hasErrors(diagnostics), Root: absRoot, Manifest: &manifest, Files: files, Bytes: bytesUsed, Diagnostics: diagnostics}
}

func inspectPackageFiles(root string) (int, int64, []Diagnostic) {
	files := 0
	var bytesUsed int64
	diagnostics := []Diagnostic{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if shouldExcludePath(rel, entry.IsDir()) {
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
			diagnostics = append(diagnostics, Diagnostic{Level: "error", Code: "package.symlink", Path: filepath.ToSlash(rel), Message: "symbolic links are not allowed"})
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() {
			diagnostics = append(diagnostics, Diagnostic{Level: "error", Code: "package.fileType", Path: filepath.ToSlash(rel), Message: "only regular files are allowed"})
			return nil
		}
		portablePath := filepath.ToSlash(rel)
		if strings.Contains(portablePath, ":") || strings.ContainsRune(portablePath, '\x00') {
			diagnostics = append(diagnostics, Diagnostic{Level: "error", Code: "package.nonPortablePath", Path: portablePath, Message: "path is not portable across supported platforms"})
		}
		files++
		bytesUsed += info.Size()
		return nil
	})
	if err != nil {
		diagnostics = append(diagnostics, Diagnostic{Level: "error", Code: "package.walk", Message: err.Error()})
	}
	if files > MaxPackageFiles {
		diagnostics = append(diagnostics, Diagnostic{Level: "error", Code: "package.fileLimit", Message: fmt.Sprintf("package contains %d files; maximum is %d", files, MaxPackageFiles)})
	}
	if bytesUsed > MaxPackageBytes {
		diagnostics = append(diagnostics, Diagnostic{Level: "error", Code: "package.sizeLimit", Message: fmt.Sprintf("package contains %d bytes; maximum is %d", bytesUsed, MaxPackageBytes)})
	}
	return files, bytesUsed, diagnostics
}

func validateRelativePath(value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("is required")
	}
	if strings.Contains(value, "\\") {
		return fmt.Errorf("must use forward slashes")
	}
	if strings.Contains(value, ":") || strings.ContainsRune(value, '\x00') {
		return fmt.Errorf("contains characters that are not portable across supported platforms")
	}
	if filepath.IsAbs(value) || strings.HasPrefix(value, "/") {
		return fmt.Errorf("must be relative to the extension root")
	}
	cleaned := filepath.ToSlash(filepath.Clean(filepath.FromSlash(value)))
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") || cleaned != value {
		return fmt.Errorf("must be a normalized path inside the extension root")
	}
	return nil
}

func validateOwnedID(owner, value, path string, add func(string, string, string)) {
	if len(value) > 160 || !contributionIDPattern.MatchString(value) {
		add("contribution.id", path, "must contain lowercase letters, numbers, dots, or hyphens")
		return
	}
	if !strings.HasPrefix(value, owner+".") {
		add("contribution.owner", path, "must be namespaced under the extension id")
	}
}

func validateCommandReference(commands map[string]struct{}, command, path string, add func(string, string, string)) {
	if _, exists := commands[command]; !exists {
		add("contribution.commandReference", path, "references a command not declared by this extension")
	}
}

func configurationValueMatches(kind string, value interface{}) bool {
	switch kind {
	case "string":
		_, ok := value.(string)
		return ok
	case "number":
		_, ok := value.(float64)
		return ok
	case "integer":
		number, ok := value.(float64)
		return ok && math.Trunc(number) == number
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "array":
		_, ok := value.([]interface{})
		return ok
	case "object":
		_, ok := value.(map[string]interface{})
		return ok
	default:
		return false
	}
}

func validateWhenClause(value string) error {
	if value == "" {
		return nil
	}
	if len(value) > 1000 {
		return fmt.Errorf("when clause exceeds 1000 bytes")
	}
	if strings.ContainsAny(value, "\r\n\x00;{}[]()`)") {
		return fmt.Errorf("when clause contains unsupported characters")
	}
	quote := rune(0)
	escaped := false
	terms := 1
	for _, character := range value {
		if escaped {
			escaped = false
			continue
		}
		if character == '\\' && quote != 0 {
			escaped = true
			continue
		}
		if character == '\'' || character == '"' {
			if quote == character {
				quote = 0
			} else if quote == 0 {
				quote = character
			}
		}
		if quote == 0 && (character == '&' || character == '|') {
			terms++
		}
	}
	if quote != 0 {
		return fmt.Errorf("when clause has an unterminated quote")
	}
	if terms > 128 {
		return fmt.Errorf("when clause has too many terms")
	}
	return nil
}

func validEngineRange(value string) bool {
	parts := strings.Fields(value)
	if len(parts) == 0 || len(parts) > 4 {
		return false
	}
	for _, part := range parts {
		if part == "||" || !engineTokenPattern.MatchString(part) {
			return false
		}
	}
	return true
}

func knownPermission(permission string) bool {
	if _, exists := exactPermissions[permission]; exists {
		return true
	}
	if strings.HasPrefix(permission, "network:") {
		rawOrigin := strings.TrimSpace(strings.TrimPrefix(permission, "network:"))
		parsed, err := url.Parse(rawOrigin)
		if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
			return false
		}
		switch parsed.Scheme {
		case "http", "https", "ws", "wss":
			return true
		default:
			return false
		}
	}
	for _, prefix := range scopedPermissionPrefixes[1:] {
		if strings.HasPrefix(permission, prefix) && strings.TrimSpace(strings.TrimPrefix(permission, prefix)) != "" {
			return true
		}
	}
	return false
}

func shouldExcludePath(relative string, isDir bool) bool {
	parts := strings.Split(filepath.ToSlash(relative), "/")
	if len(parts) == 0 {
		return false
	}
	if parts[0] == ".git" || parts[0] == "node_modules" {
		return true
	}
	if !isDir && (strings.HasSuffix(relative, ".adomnia-extension") || strings.HasSuffix(relative, ".tmp")) {
		return true
	}
	return false
}

func hasErrors(diagnostics []Diagnostic) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Level == "error" {
			return true
		}
	}
	return false
}

func invalidReport(root, code, path, message string) ValidationReport {
	return ValidationReport{Valid: false, Root: root, Diagnostics: []Diagnostic{{Level: "error", Code: code, Path: path, Message: message}}}
}
