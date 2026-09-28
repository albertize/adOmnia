package extensions

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"adomnia/internal/httpexec"
	mockRuntime "adomnia/internal/mock"
	proxyRuntime "adomnia/internal/proxy"
	"adomnia/internal/storage"
	"adomnia/internal/vault"

	bolt "go.etcd.io/bbolt"
)

const (
	maxStateValueBytes     = 1 << 20
	maxSecretValueBytes    = 64 << 10
	maxExtensionStateBytes = 10 << 20
	defaultHostCallTimeout = 12 * time.Second
)

type ExtensionNotification struct {
	ExtensionID string `json:"extensionId"`
	Message     string `json:"message"`
	Type        string `json:"type"`
}

type ExtensionViewUpdate struct {
	ExtensionID string      `json:"extensionId"`
	ViewID      string      `json:"viewId"`
	State       interface{} `json:"state"`
}

type ExtensionLog struct {
	Timestamp   string         `json:"timestamp"`
	ExtensionID string         `json:"extensionId"`
	Level       string         `json:"level"`
	Message     string         `json:"message"`
	Fields      map[string]any `json:"fields,omitempty"`
}

type ExtensionDomainAction struct {
	ExtensionID string         `json:"extensionId"`
	Domain      string         `json:"domain"`
	Action      string         `json:"action"`
	Payload     map[string]any `json:"payload"`
}

type ExtensionDiagnostic struct {
	ExtensionID string `json:"extensionId"`
	Severity    string `json:"severity"`
	Message     string `json:"message"`
	Resource    string `json:"resource,omitempty"`
	Line        int    `json:"line,omitempty"`
	Column      int    `json:"column,omitempty"`
}

type RuntimeStatus struct {
	Running         bool   `json:"running"`
	ProtocolVersion string `json:"protocolVersion"`
	ActiveCount     int    `json:"activeCount"`
}

type Service struct {
	registry *Registry

	mu             sync.Mutex
	host           *HostClient
	executable     string
	appVersion     string
	workspaceID    string
	notifier       func(ExtensionNotification)
	viewNotifier   func(ExtensionViewUpdate)
	actionNotifier func(ExtensionDomainAction)
	viewStates     map[string]json.RawMessage
	activeRequest  map[string]any
	activeResponse map[string]any
	domainContext  map[string]any
	logs           map[string][]ExtensionLog
	diagnostics    map[string][]ExtensionDiagnostic
	rateWindows    map[string][]time.Time
}

func NewService(dataDir string) *Service {
	return &Service{registry: NewRegistry(dataDir), workspaceID: "default", viewStates: map[string]json.RawMessage{}, activeRequest: map[string]any{}, activeResponse: map[string]any{}, domainContext: map[string]any{}, logs: map[string][]ExtensionLog{}, diagnostics: map[string][]ExtensionDiagnostic{}, rateWindows: map[string][]time.Time{}}
}

func (s *Service) Init() error {
	if err := s.registry.Init(); err != nil {
		return err
	}
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve extension host executable: %w", err)
	}
	s.executable = executable
	mockRuntime.ConfigureExtensionObserver(func(payload map[string]any) { go s.emitObservedEvent("onMockHit", payload) })
	proxyRuntime.ConfigureExtensionObserver(func(payload map[string]any) { go s.emitObservedEvent("onProxyTraffic", payload) })
	return nil
}

// ConfigureNotifier connects extension notifications to the desktop event bus
// without exposing a function-valued method through Wails bindings.
func ConfigureNotifier(service *Service, notifier func(ExtensionNotification)) {
	service.mu.Lock()
	service.notifier = notifier
	service.mu.Unlock()
}

func ConfigureViewNotifier(service *Service, notifier func(ExtensionViewUpdate)) {
	service.mu.Lock()
	service.viewNotifier = notifier
	service.mu.Unlock()
}

func ConfigureDomainActionNotifier(service *Service, notifier func(ExtensionDomainAction)) {
	service.mu.Lock()
	service.actionNotifier = notifier
	service.mu.Unlock()
}

func (s *Service) SetAppVersion(version string) {
	s.mu.Lock()
	s.appVersion = version
	s.mu.Unlock()
}

func (s *Service) SetWorkspaceContext(workspaceID string) {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		workspaceID = "default"
	}
	s.mu.Lock()
	s.workspaceID = workspaceID
	s.mu.Unlock()
}

func (s *Service) GetExtensions() []ExtensionInstance { return s.registry.List() }

func (s *Service) GetLogs(extensionID string) []ExtensionLog {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries := s.logs[extensionID]
	result := make([]ExtensionLog, len(entries))
	copy(result, entries)
	return result
}

func (s *Service) GetDiagnostics(extensionID string) []ExtensionDiagnostic {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries := s.diagnostics[extensionID]
	result := make([]ExtensionDiagnostic, len(entries))
	copy(result, entries)
	return result
}

func (s *Service) SetDomainContext(contextJSON string) error {
	if len(contextJSON) > 5*1024*1024 {
		return fmt.Errorf("extension domain context exceeds 5 MiB")
	}
	contextValues := map[string]any{}
	if strings.TrimSpace(contextJSON) != "" {
		if err := json.Unmarshal([]byte(contextJSON), &contextValues); err != nil {
			return fmt.Errorf("decode extension domain context: %w", err)
		}
	}
	s.mu.Lock()
	previous := cloneAny(s.domainContext)
	s.domainContext = contextValues
	s.mu.Unlock()
	go s.emitDomainContextEvents(previous, cloneAny(contextValues))
	return nil
}

func (s *Service) emitObservedEvent(event string, payload map[string]any) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	if _, err := s.ApplyEventJSON(event, string(data)); err != nil {
		log.Printf("[extensions-v2] %s event failed: %v", event, err)
	}
}

func (s *Service) emitDomainContextEvents(previousValue, currentValue any) {
	previous, _ := previousValue.(map[string]any)
	current, _ := currentValue.(map[string]any)
	emit := func(event string, payload map[string]any) {
		data, err := json.Marshal(payload)
		if err != nil {
			return
		}
		if _, err := s.ApplyEventJSON(event, string(data)); err != nil {
			log.Printf("[extensions-v2] %s event failed: %v", event, err)
		}
	}
	previousWorkspace, currentWorkspace := domainActive(previous, "workspace"), domainActive(current, "workspace")
	if objectID(previousWorkspace) != objectID(currentWorkspace) {
		if previousWorkspace != nil {
			emit("onWorkspaceClose", previousWorkspace)
		}
		if currentWorkspace != nil {
			emit("onWorkspaceOpen", currentWorkspace)
		}
	}
	previousEnvironment, currentEnvironment := domainActive(previous, "environments"), domainActive(current, "environments")
	if objectID(previousEnvironment) != objectID(currentEnvironment) {
		emit("onEnvChange", map[string]any{"previous": previousEnvironment, "current": currentEnvironment})
	}
	previousTheme, _ := previous["theme"].(map[string]any)
	currentTheme, _ := current["theme"].(map[string]any)
	if objectID(previousTheme) != objectID(currentTheme) && currentTheme != nil {
		emit("onThemeChange", currentTheme)
	}
	previousTabs, currentTabs := domainItems(previous, "tabs"), domainItems(current, "tabs")
	for id, tab := range currentTabs {
		if _, exists := previousTabs[id]; !exists {
			emit("onTabOpen", tab)
		}
	}
	for id, tab := range previousTabs {
		if _, exists := currentTabs[id]; !exists {
			emit("onTabClose", tab)
		}
	}
	previousBrowserEntries, currentBrowserEntries := domainItems(previous, "browserDebug"), domainItems(current, "browserDebug")
	for id, entry := range currentBrowserEntries {
		if _, exists := previousBrowserEntries[id]; !exists {
			emit("onBrowserNetwork", entry)
		}
	}
}

func domainActive(contextValues map[string]any, domain string) map[string]any {
	container, _ := contextValues[domain].(map[string]any)
	active, _ := container["active"].(map[string]any)
	return active
}

func domainItems(contextValues map[string]any, domain string) map[string]map[string]any {
	result := map[string]map[string]any{}
	container, _ := contextValues[domain].(map[string]any)
	items, _ := container["items"].([]any)
	for _, value := range items {
		if item, ok := value.(map[string]any); ok {
			if id := objectID(item); id != "" {
				result[id] = item
			}
		}
	}
	return result
}

func objectID(value map[string]any) string {
	if value == nil {
		return ""
	}
	id, _ := value["id"].(string)
	return id
}

func (s *Service) GetViewState(extensionID, viewID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return string(s.viewStates[extensionID+"\x00"+viewID])
}

func (s *Service) OpenView(extensionID, viewID string) (string, error) {
	instance, err := s.registry.Get(extensionID)
	if err != nil {
		return "", err
	}
	declared := false
	for _, view := range instance.Manifest.Contributes.Views {
		if view.ID == viewID {
			declared = true
			break
		}
	}
	if !declared {
		return "", fmt.Errorf("view is not declared by %s: %s", extensionID, viewID)
	}
	if manifestHasActivation(instance.Manifest, "onView:"+viewID) {
		if err := s.activate(extensionID); err != nil {
			return "", err
		}
	} else if !instance.Enabled {
		return "", fmt.Errorf("extension is disabled: %s", extensionID)
	}
	return s.GetViewState(extensionID, viewID), nil
}

func (s *Service) GetWebviewHTML(extensionID, viewID string) (string, error) {
	instance, err := s.registry.Get(extensionID)
	if err != nil {
		return "", err
	}
	if !instance.Enabled {
		return "", fmt.Errorf("extension is disabled: %s", extensionID)
	}
	entry := ""
	for _, view := range instance.Manifest.Contributes.Views {
		if view.ID == viewID && view.Renderer == "webview" {
			entry = view.Entry
			break
		}
	}
	if err := validateRelativePath(entry); err != nil {
		return "", fmt.Errorf("invalid webview entry: %w", err)
	}
	content, err := os.ReadFile(filepath.Join(instance.InstallDir, filepath.FromSlash(entry)))
	if err != nil {
		return "", fmt.Errorf("read webview entry: %w", err)
	}
	if int64(len(content)) > MaxWebviewHTMLBytes {
		return "", fmt.Errorf("webview HTML exceeds %d bytes", MaxWebviewHTMLBytes)
	}
	connectSources := []string{"'none'"}
	for _, grant := range instance.Grants {
		if strings.HasPrefix(grant, "network:") {
			if len(connectSources) == 1 && connectSources[0] == "'none'" {
				connectSources = connectSources[:0]
			}
			connectSources = append(connectSources, strings.TrimPrefix(grant, "network:"))
		}
	}
	csp := "default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; img-src data: blob:; font-src data:; connect-src " + strings.Join(connectSources, " ") + "; form-action 'none'; frame-src 'none'; child-src 'none'; object-src 'none'; base-uri 'none'; navigate-to 'none'"
	meta := `<meta http-equiv="Content-Security-Policy" content="` + html.EscapeString(csp) + `">`
	document := string(content)
	lower := strings.ToLower(document)
	if index := strings.Index(lower, "<head>"); index >= 0 {
		insertAt := index + len("<head>")
		document = document[:insertAt] + meta + document[insertAt:]
	} else {
		document = meta + document
	}
	return document, nil
}

func (s *Service) InstallDirectory(path string, development bool) (ExtensionInstance, error) {
	instance, err := s.registry.InstallDirectory(path, development)
	if err != nil {
		return ExtensionInstance{}, err
	}
	if err := s.checkCompatibility(instance.Manifest); err != nil {
		_ = s.registry.Uninstall(instance.Manifest.ID)
		return ExtensionInstance{}, err
	}
	return instance, nil
}

func (s *Service) InstallArchive(path string) (ExtensionInstance, error) {
	instance, err := s.registry.InstallArchive(path)
	if err != nil {
		return ExtensionInstance{}, err
	}
	if err := s.checkCompatibility(instance.Manifest); err != nil {
		_ = s.registry.Uninstall(instance.Manifest.ID)
		return ExtensionInstance{}, err
	}
	return instance, nil
}

func (s *Service) Reload(id string) (ExtensionInstance, error) {
	instance, err := s.registry.Get(id)
	if err != nil {
		return ExtensionInstance{}, err
	}
	_ = s.deactivate(id)
	if strings.HasSuffix(strings.ToLower(instance.Source), ".adomnia-extension") {
		return s.registry.UpdateArchive(instance.Source)
	}
	return s.registry.UpdateDirectory(instance.Source, instance.InstallKind == InstallDevelopment)
}

func (s *Service) SetGrants(id string, grants []string) (ExtensionInstance, error) {
	return s.registry.SetGrants(id, grants)
}

func (s *Service) Enable(id string) (ExtensionInstance, error) {
	installed, err := s.registry.Get(id)
	if err != nil {
		return ExtensionInstance{}, err
	}
	if err := s.checkCompatibility(installed.Manifest); err != nil {
		return ExtensionInstance{}, err
	}
	instance, err := s.registry.Enable(id)
	if err != nil {
		return ExtensionInstance{}, err
	}
	if manifestHasActivation(instance.Manifest, "onStartup") {
		if err := s.activate(id); err != nil {
			return ExtensionInstance{}, err
		}
		return s.registry.Get(id)
	}
	return instance, nil
}

func (s *Service) Disable(id string) (ExtensionInstance, error) {
	_ = s.deactivate(id)
	return s.registry.Disable(id)
}

func (s *Service) Uninstall(id string) error {
	_ = s.deactivate(id)
	return s.registry.Uninstall(id)
}

func (s *Service) SetSetting(id, key, valueJSON string) (ExtensionInstance, error) {
	var value interface{}
	if err := json.Unmarshal([]byte(valueJSON), &value); err != nil {
		return ExtensionInstance{}, fmt.Errorf("decode setting value: %w", err)
	}
	instance, err := s.registry.SetSetting(id, key, value)
	if err != nil || !instance.Enabled || !manifestHasActivation(instance.Manifest, "onConfiguration:"+key) {
		return instance, err
	}
	// Recreate the VM so configuration.get observes the newly persisted value.
	_ = s.deactivate(id)
	if err := s.activate(id); err != nil {
		return instance, err
	}
	host, err := s.ensureHost()
	if err != nil {
		return instance, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultHostCallTimeout)
	defer cancel()
	var result HostExecutionResult
	err = host.Request(ctx, "dispatchEvent", DispatchEventRequest{ExtensionID: id, Event: "onConfiguration", Payload: map[string]any{"keys": []string{key}}}, &result)
	if err != nil {
		s.markRuntimeFailure(id, err)
		return instance, err
	}
	if !result.Success {
		return instance, fmt.Errorf("configuration event failed: %s", result.Error)
	}
	return s.registry.Get(id)
}

func (s *Service) ExecuteCommand(extensionID, commandID, argsJSON, source string) (HostExecutionResult, error) {
	instance, err := s.registry.Get(extensionID)
	if err != nil {
		return HostExecutionResult{}, err
	}
	if !instance.Enabled {
		return HostExecutionResult{}, fmt.Errorf("extension is disabled: %s", extensionID)
	}
	if err := s.checkCompatibility(instance.Manifest); err != nil {
		return HostExecutionResult{}, err
	}
	declared := false
	for _, command := range instance.Manifest.Contributes.Commands {
		if command.ID == commandID {
			declared = true
			break
		}
	}
	if !declared {
		return HostExecutionResult{}, fmt.Errorf("command is not declared by %s: %s", extensionID, commandID)
	}
	if err := s.activate(extensionID); err != nil {
		return HostExecutionResult{}, err
	}
	args := map[string]any{}
	if strings.TrimSpace(argsJSON) != "" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return HostExecutionResult{}, fmt.Errorf("decode command arguments: %w", err)
		}
	}
	host, err := s.ensureHost()
	if err != nil {
		return HostExecutionResult{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultHostCallTimeout)
	defer cancel()
	var result HostExecutionResult
	if err := host.Request(ctx, "executeCommand", ExecuteCommandRequest{ExtensionID: extensionID, CommandID: commandID, Args: args, Source: source}, &result); err != nil {
		s.markRuntimeFailure(extensionID, err)
		return HostExecutionResult{}, err
	}
	if !result.Success {
		return result, fmt.Errorf("extension command failed: %s", result.Error)
	}
	return result, nil
}

func (s *Service) EvaluateVariableProviders(contextJSON string) ([]VariableProviderResult, error) {
	if len(contextJSON) > maxStateValueBytes {
		return nil, fmt.Errorf("variable provider context exceeds %d bytes", maxStateValueBytes)
	}
	contextValues := map[string]any{}
	if strings.TrimSpace(contextJSON) != "" {
		if err := json.Unmarshal([]byte(contextJSON), &contextValues); err != nil {
			return nil, fmt.Errorf("decode variable provider context: %w", err)
		}
	}
	results := make([]VariableProviderResult, 0)
	for _, instance := range s.registry.List() {
		if !instance.Enabled || !manifestHasActivation(instance.Manifest, "onVariables") || requireGrant(instance, "variables.provide") != nil {
			continue
		}
		if err := s.activate(instance.Manifest.ID); err != nil {
			return nil, err
		}
		host, err := s.ensureHost()
		if err != nil {
			return nil, err
		}
		ctx, cancel := context.WithTimeout(context.Background(), defaultHostCallTimeout)
		var execution HostExecutionResult
		err = host.Request(ctx, "evaluateVariableProviders", EvaluateVariableProvidersRequest{ExtensionID: instance.Manifest.ID, Context: contextValues}, &execution)
		cancel()
		if err != nil {
			s.markRuntimeFailure(instance.Manifest.ID, err)
			return nil, fmt.Errorf("extension %s variable providers failed: %w", instance.Manifest.ID, err)
		}
		if !execution.Success {
			return nil, fmt.Errorf("extension %s variable providers failed: %s", instance.Manifest.ID, execution.Error)
		}
		encoded, err := json.Marshal(execution.Data)
		if err != nil {
			return nil, err
		}
		var extensionResults []VariableProviderResult
		if err := json.Unmarshal(encoded, &extensionResults); err != nil {
			return nil, fmt.Errorf("decode extension variable provider results: %w", err)
		}
		for index := range extensionResults {
			item := &extensionResults[index]
			item.ExtensionID = instance.Manifest.ID
			if !strings.HasPrefix(item.ProviderID, instance.Manifest.ID+".") || len(item.Values) > 200 {
				return nil, fmt.Errorf("extension %s returned an invalid variable provider result", instance.Manifest.ID)
			}
			for key, value := range item.Values {
				if strings.TrimSpace(key) == "" || len(key) > 256 || len(value) > maxSecretValueBytes {
					return nil, fmt.Errorf("extension %s returned an invalid variable", instance.Manifest.ID)
				}
			}
		}
		results = append(results, extensionResults...)
		if len(results) > 100 {
			return nil, fmt.Errorf("extension variable providers exceed 100 entries")
		}
	}
	return results, nil
}

func (s *Service) EvaluateAssertions(payloadJSON string) ([]AssertionProviderResult, error) {
	if len(payloadJSON) > 5*1024*1024 {
		return nil, fmt.Errorf("assertion payload exceeds 5 MiB")
	}
	payload := map[string]any{}
	if err := json.Unmarshal([]byte(payloadJSON), &payload); err != nil {
		return nil, fmt.Errorf("decode assertion payload: %w", err)
	}
	results := make([]AssertionProviderResult, 0)
	for _, instance := range s.registry.List() {
		if !instance.Enabled || !manifestHasActivation(instance.Manifest, "onAssertions") || requireGrant(instance, "assertions.provide") != nil {
			continue
		}
		if err := s.activate(instance.Manifest.ID); err != nil {
			return nil, err
		}
		host, err := s.ensureHost()
		if err != nil {
			return nil, err
		}
		ctx, cancel := context.WithTimeout(context.Background(), defaultHostCallTimeout)
		var execution HostExecutionResult
		err = host.Request(ctx, "evaluateAssertions", EvaluateAssertionsRequest{ExtensionID: instance.Manifest.ID, Payload: payload}, &execution)
		cancel()
		if err != nil {
			s.markRuntimeFailure(instance.Manifest.ID, err)
			return nil, fmt.Errorf("extension %s assertion providers failed: %w", instance.Manifest.ID, err)
		}
		if !execution.Success {
			return nil, fmt.Errorf("extension %s assertion providers failed: %s", instance.Manifest.ID, execution.Error)
		}
		encoded, err := json.Marshal(execution.Data)
		if err != nil {
			return nil, err
		}
		var extensionResults []AssertionProviderResult
		if err := json.Unmarshal(encoded, &extensionResults); err != nil {
			return nil, fmt.Errorf("decode extension assertion results: %w", err)
		}
		for index := range extensionResults {
			item := &extensionResults[index]
			item.ExtensionID = instance.Manifest.ID
			if strings.TrimSpace(item.Label) == "" || len(item.Label) > 500 || len(item.Actual) > 4096 || len(item.Expected) > 4096 || len(item.Message) > 4096 {
				return nil, fmt.Errorf("extension %s returned an invalid assertion result", instance.Manifest.ID)
			}
		}
		results = append(results, extensionResults...)
		if len(results) > 500 {
			return nil, fmt.Errorf("extension assertion results exceed 500 entries")
		}
	}
	return results, nil
}

func (s *Service) NotifyWorkbenchEvent(event, payloadJSON string) error {
	if event != "onSave" && event != "onImport" && event != "onExport" {
		return fmt.Errorf("unsupported workbench extension event: %s", event)
	}
	if len(payloadJSON) > maxStateValueBytes {
		return fmt.Errorf("workbench event payload exceeds %d bytes", maxStateValueBytes)
	}
	_, err := s.ApplyEventJSON(event, payloadJSON)
	return err
}

func (s *Service) ApplyEventJSON(event, payloadJSON string) (string, error) {
	var payload map[string]any
	if err := json.Unmarshal([]byte(payloadJSON), &payload); err != nil {
		return "", fmt.Errorf("decode extension event payload: %w", err)
	}
	s.mu.Lock()
	if event == "onRequest" || event == "onSend" {
		s.activeRequest = cloneMap(payload)
	} else if event == "onResponse" {
		s.activeResponse = cloneMap(payload)
	}
	s.mu.Unlock()
	for _, instance := range s.registry.List() {
		if !instance.Enabled || !manifestHasActivation(instance.Manifest, event) {
			continue
		}
		if err := s.activate(instance.Manifest.ID); err != nil {
			return "", err
		}
		host, err := s.ensureHost()
		if err != nil {
			return "", err
		}
		ctx, cancel := context.WithTimeout(context.Background(), defaultHostCallTimeout)
		var result HostExecutionResult
		err = host.Request(ctx, "dispatchEvent", DispatchEventRequest{ExtensionID: instance.Manifest.ID, Event: event, Payload: payload}, &result)
		cancel()
		if err != nil {
			s.markRuntimeFailure(instance.Manifest.ID, err)
			return "", fmt.Errorf("extension %s event %s failed: %w", instance.Manifest.ID, event, err)
		}
		if !result.Success {
			return "", fmt.Errorf("extension %s event %s failed: %s", instance.Manifest.ID, event, result.Error)
		}
		if result.Modified {
			next, ok := result.Data.(map[string]interface{})
			if !ok {
				return "", fmt.Errorf("extension %s returned invalid transformed payload", instance.Manifest.ID)
			}
			payload = next
		}
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s *Service) FireStartup() {
	for _, instance := range s.registry.List() {
		if instance.Enabled && manifestHasActivation(instance.Manifest, "onStartup") {
			if err := s.activate(instance.Manifest.ID); err != nil {
				log.Printf("[extensions-v2] startup activation failed for %s: %v", instance.Manifest.ID, err)
			}
		}
	}
}

func (s *Service) GetRuntimeStatus() RuntimeStatus {
	s.mu.Lock()
	running := s.host != nil
	s.mu.Unlock()
	active := 0
	for _, item := range s.registry.List() {
		if item.Active {
			active++
		}
	}
	return RuntimeStatus{Running: running, ProtocolVersion: HostProtocolVersion, ActiveCount: active}
}

func (s *Service) Shutdown() {
	for _, item := range s.registry.List() {
		if item.Active {
			_ = s.deactivate(item.Manifest.ID)
		}
	}
	s.mu.Lock()
	host := s.host
	s.host = nil
	s.notifier = nil
	s.viewNotifier = nil
	s.actionNotifier = nil
	s.mu.Unlock()
	mockRuntime.ConfigureExtensionObserver(nil)
	proxyRuntime.ConfigureExtensionObserver(nil)
	if host != nil {
		_ = host.Close()
	}
}

func (s *Service) activate(id string) error {
	instance, err := s.registry.Get(id)
	if err != nil {
		return err
	}
	if !instance.Enabled {
		return fmt.Errorf("extension is disabled: %s", id)
	}
	if err := s.checkCompatibility(instance.Manifest); err != nil {
		return err
	}
	if instance.Active {
		s.mu.Lock()
		host := s.host
		hostRunning := host != nil
		if hostRunning {
			select {
			case <-host.closed:
				hostRunning = false
			default:
			}
		}
		s.mu.Unlock()
		if hostRunning {
			return nil
		}
		_ = s.registry.SetActive(id, false, "extension host stopped unexpectedly; activation will be retried")
	}
	entrypoint := filepath.Join(instance.InstallDir, filepath.FromSlash(instance.Manifest.Main))
	source, err := os.ReadFile(entrypoint)
	if err != nil {
		return fmt.Errorf("read extension entry point: %w", err)
	}
	host, err := s.ensureHost()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultHostCallTimeout)
	defer cancel()
	var result HostExecutionResult
	if err := host.Request(ctx, "activate", ActivateHostRequest{Manifest: instance.Manifest, Settings: instance.Settings, Source: string(source)}, &result); err != nil {
		s.markRuntimeFailure(id, err)
		return err
	}
	if !result.Success {
		_ = s.registry.RecordRuntimeFailure(id, fmt.Errorf("%s", result.Error))
		return fmt.Errorf("extension activation failed: %s", result.Error)
	}
	return s.registry.SetActive(id, true, "")
}

func (s *Service) deactivate(id string) error {
	instance, err := s.registry.Get(id)
	if err != nil {
		return err
	}
	if !instance.Active {
		s.clearDynamicContributions(id)
		return nil
	}
	s.mu.Lock()
	host := s.host
	s.mu.Unlock()
	if host != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
		err = host.Request(ctx, "deactivate", DeactivateHostRequest{ExtensionID: id}, nil)
		cancel()
	}
	_ = s.registry.SetActive(id, false, "")
	s.clearDynamicContributions(id)
	return err
}

func (s *Service) clearDynamicContributions(id string) {
	s.mu.Lock()
	delete(s.diagnostics, id)
	prefix := id + "\x00"
	for key := range s.viewStates {
		if strings.HasPrefix(key, prefix) {
			delete(s.viewStates, key)
		}
	}
	for key := range s.rateWindows {
		if strings.HasPrefix(key, id+":") {
			delete(s.rateWindows, key)
		}
	}
	s.mu.Unlock()
}

func (s *Service) ensureHost() (*HostClient, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.host != nil {
		select {
		case <-s.host.closed:
			s.host = nil
			for _, item := range s.registry.List() {
				if item.Active {
					_ = s.registry.SetActive(item.Manifest.ID, false, "extension host stopped unexpectedly")
				}
			}
		default:
			return s.host, nil
		}
	}
	host, err := StartHostProcess(s.executable, s.handleHostCall)
	if err != nil {
		return nil, err
	}
	s.host = host
	return host, nil
}

func (s *Service) markRuntimeFailure(extensionID string, failure error) {
	_ = s.registry.RecordRuntimeFailure(extensionID, failure)
	s.mu.Lock()
	host := s.host
	s.host = nil
	s.mu.Unlock()
	if host != nil {
		_ = host.Close()
	}
	for _, item := range s.registry.List() {
		if item.Manifest.ID != extensionID && item.Active {
			_ = s.registry.SetActive(item.Manifest.ID, false, "extension host restarted after another extension failed")
		}
	}
}

func (s *Service) handleHostCall(_ context.Context, method string, raw json.RawMessage) (interface{}, error) {
	var params map[string]interface{}
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, fmt.Errorf("decode host API request: %w", err)
	}
	extensionID, _ := params["extensionId"].(string)
	instance, err := s.registry.Get(extensionID)
	if err != nil {
		return nil, err
	}
	if !instance.Enabled {
		return nil, fmt.Errorf("extension is disabled: %s", extensionID)
	}
	switch method {
	case "log.write":
		if err := s.checkRateLimit(extensionID+":log", 200, time.Minute); err != nil {
			return nil, err
		}
		level, _ := params["level"].(string)
		message, _ := params["message"].(string)
		message = strings.ReplaceAll(strings.ReplaceAll(message, "\n", " "), "\r", " ")
		if len(message) > 4096 {
			message = message[:4096] + "…"
		}
		fields, _ := params["fields"].(map[string]any)
		entry := ExtensionLog{Timestamp: time.Now().UTC().Format(time.RFC3339Nano), ExtensionID: extensionID, Level: level, Message: message, Fields: fields}
		s.mu.Lock()
		s.logs[extensionID] = append(s.logs[extensionID], entry)
		if len(s.logs[extensionID]) > 500 {
			s.logs[extensionID] = s.logs[extensionID][len(s.logs[extensionID])-500:]
		}
		s.mu.Unlock()
		log.Printf("[extension:%s:%s] %s", extensionID, level, message)
		return map[string]bool{"ok": true}, nil
	case "window.notify":
		if err := s.checkRateLimit(extensionID+":notify", 5, 10*time.Second); err != nil {
			return nil, err
		}
		if err := requireGrant(instance, "notifications"); err != nil {
			return nil, err
		}
		message, _ := params["message"].(string)
		notificationType, _ := params["type"].(string)
		if notificationType == "" {
			notificationType = "info"
		}
		s.mu.Lock()
		notifier := s.notifier
		s.mu.Unlock()
		if notifier != nil {
			notifier(ExtensionNotification{ExtensionID: extensionID, Message: message, Type: notificationType})
		}
		return map[string]bool{"ok": true}, nil
	case "progress.report":
		if err := s.checkRateLimit(extensionID+":progress", 20, time.Second); err != nil {
			return nil, err
		}
		progress, ok := params["progress"].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("progress report must be an object")
		}
		title, _ := progress["title"].(string)
		message, _ := progress["message"].(string)
		if strings.TrimSpace(title) == "" || len(title) > 200 || len(message) > 1000 {
			return nil, fmt.Errorf("invalid progress report")
		}
		s.mu.Lock()
		notifier := s.notifier
		s.mu.Unlock()
		if notifier != nil {
			notifier(ExtensionNotification{ExtensionID: extensionID, Message: strings.TrimSpace(title + " " + message), Type: "progress"})
		}
		return map[string]bool{"ok": true}, nil
	case "diagnostics.set":
		if err := s.checkRateLimit(extensionID+":diagnostics", 10, time.Second); err != nil {
			return nil, err
		}
		encoded, err := json.Marshal(params["entries"])
		if err != nil {
			return nil, err
		}
		var entries []ExtensionDiagnostic
		if err := json.Unmarshal(encoded, &entries); err != nil {
			return nil, fmt.Errorf("decode extension diagnostics: %w", err)
		}
		if len(entries) > 200 {
			return nil, fmt.Errorf("extension diagnostics exceed 200 entries")
		}
		for index := range entries {
			entry := &entries[index]
			if entry.Severity != "info" && entry.Severity != "warning" && entry.Severity != "error" {
				return nil, fmt.Errorf("invalid diagnostic severity")
			}
			if strings.TrimSpace(entry.Message) == "" || len(entry.Message) > 4096 || len(entry.Resource) > 1024 || entry.Line < 0 || entry.Column < 0 {
				return nil, fmt.Errorf("invalid extension diagnostic")
			}
			entry.ExtensionID = extensionID
		}
		s.mu.Lock()
		s.diagnostics[extensionID] = entries
		s.mu.Unlock()
		return map[string]bool{"ok": true}, nil
	case "state.get", "state.set", "state.delete":
		return s.handleStateCall(instance, method, params)
	case "secrets.get", "secrets.set", "secrets.delete":
		return s.handleSecretCall(instance, method, params)
	case "requests.getActive":
		if err := requireGrant(instance, "requests.read"); err != nil {
			return nil, err
		}
		s.mu.Lock()
		value := cloneMap(s.activeRequest)
		if len(value) == 0 {
			value = cloneMapValue(s.domainContext["request"])
		}
		s.mu.Unlock()
		return value, nil
	case "responses.getActive":
		if err := requireGrant(instance, "responses.read"); err != nil {
			return nil, err
		}
		s.mu.Lock()
		value := cloneMap(s.activeResponse)
		if len(value) == 0 {
			value = cloneMapValue(s.domainContext["response"])
		}
		s.mu.Unlock()
		return value, nil
	case "mock.getSnapshot":
		if err := requireGrant(instance, "mock.read"); err != nil {
			return nil, err
		}
		return mockRuntime.ExtensionSnapshot(), nil
	case "mock.clearHits", "mock.stop":
		if err := requireGrant(instance, "mock.control"); err != nil {
			return nil, err
		}
		if err := s.checkRateLimit(extensionID+":mock-control", 10, time.Minute); err != nil {
			return nil, err
		}
		if method == "mock.clearHits" {
			mockRuntime.ExtensionClearHits()
			return map[string]bool{"ok": true}, nil
		}
		return map[string]bool{"ok": true}, mockRuntime.ExtensionStop()
	case "proxy.getSnapshot":
		if err := requireGrant(instance, "proxy.read"); err != nil {
			return nil, err
		}
		return proxyRuntime.ExtensionSnapshot(), nil
	case "proxy.clearTraffic", "proxy.stop":
		if err := requireGrant(instance, "proxy.control"); err != nil {
			return nil, err
		}
		if err := s.checkRateLimit(extensionID+":proxy-control", 10, time.Minute); err != nil {
			return nil, err
		}
		if method == "proxy.clearTraffic" {
			proxyRuntime.ExtensionClearTraffic()
			return map[string]bool{"ok": true}, nil
		}
		return map[string]bool{"ok": true}, proxyRuntime.ExtensionStop()
	case "variables.getAll", "variables.resolve":
		if err := requireGrant(instance, "variables.read"); err != nil {
			return nil, err
		}
		variables := s.activeEnvironmentVariables()
		if method == "variables.getAll" {
			return variables, nil
		}
		value, _ := params["value"].(string)
		if len(value) > maxStateValueBytes {
			return nil, fmt.Errorf("variable input exceeds %d bytes", maxStateValueBytes)
		}
		keys := make([]string, 0, len(variables))
		for key := range variables {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			value = strings.ReplaceAll(value, "{{"+key+"}}", variables[key])
		}
		return value, nil
	case "domains.action":
		if err := s.checkRateLimit(extensionID+":domain-action", 30, time.Second); err != nil {
			return nil, err
		}
		domain, _ := params["domain"].(string)
		action, _ := params["action"].(string)
		permission := ""
		allowed := false
		switch domain {
		case "collections":
			permission = "collections.write"
			allowed = action == "import" || action == "addRequest"
		case "environments":
			permission = "environments.write"
			allowed = action == "setActive"
		case "tabs":
			permission = "tabs.write"
			allowed = action == "open" || action == "close" || action == "setActive"
		case "browserDebug":
			permission = "browserDebug.control"
			allowed = action == "clear" || action == "select"
		}
		if !allowed {
			return nil, fmt.Errorf("unsupported extension domain action: %s.%s", domain, action)
		}
		if err := requireGrant(instance, permission); err != nil {
			return nil, err
		}
		payload, ok := params["payload"].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("domain action payload must be an object")
		}
		encoded, err := json.Marshal(payload)
		if err != nil || len(encoded) > maxStateValueBytes {
			return nil, fmt.Errorf("domain action payload is invalid or too large")
		}
		s.mu.Lock()
		notifier := s.actionNotifier
		s.mu.Unlock()
		if notifier == nil {
			return nil, fmt.Errorf("domain action workbench is unavailable")
		}
		notifier(ExtensionDomainAction{ExtensionID: extensionID, Domain: domain, Action: action, Payload: payload})
		return map[string]bool{"accepted": true}, nil
	case "domains.get":
		domain, _ := params["domain"].(string)
		permission := map[string]string{
			"environments": "environments.read",
			"collections":  "collections.read",
			"tabs":         "tabs.read",
			"workspace":    "workspace.read",
			"browserDebug": "browserDebug.read",
		}[domain]
		if permission == "" {
			return nil, fmt.Errorf("unknown extension domain: %s", domain)
		}
		if err := requireGrant(instance, permission); err != nil {
			return nil, err
		}
		selector, _ := params["selector"].(string)
		s.mu.Lock()
		value := cloneAny(s.domainContext[domain])
		s.mu.Unlock()
		if domain == "environments" && requireGrant(instance, "variables.read") != nil {
			redactEnvironmentVariables(value)
		}
		if values, ok := value.(map[string]any); ok {
			if selector == "active" {
				return values["active"], nil
			}
			if selector == "list" {
				return values["items"], nil
			}
		}
		return value, nil
	case "requests.execute":
		if err := requireGrant(instance, "requests.execute"); err != nil {
			return nil, err
		}
		request, ok := params["request"].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("requests.execute requires a request object")
		}
		requestJSON, err := json.Marshal(request)
		if err != nil {
			return nil, err
		}
		responseJSON := httpexec.Execute(string(requestJSON))
		var response interface{}
		if err := json.Unmarshal([]byte(responseJSON), &response); err != nil {
			return nil, err
		}
		return response, nil
	case "views.setState":
		if err := s.checkRateLimit(extensionID+":views", 30, time.Second); err != nil {
			return nil, err
		}
		viewID, _ := params["viewId"].(string)
		declared := false
		for _, view := range instance.Manifest.Contributes.Views {
			if view.ID == viewID && view.Renderer == "declarative" {
				declared = true
				break
			}
		}
		if !declared {
			return nil, fmt.Errorf("declarative view is not declared: %s", viewID)
		}
		stateData, err := json.Marshal(params["state"])
		if err != nil {
			return nil, err
		}
		if len(stateData) > maxStateValueBytes {
			return nil, fmt.Errorf("extension view state exceeds %d bytes", maxStateValueBytes)
		}
		var state interface{}
		if err := json.Unmarshal(stateData, &state); err != nil {
			return nil, err
		}
		if err := ValidateDeclarativeViewState(state); err != nil {
			return nil, err
		}
		if stateObject, ok := state.(map[string]any); ok {
			if actions, ok := stateObject["actions"].([]any); ok {
				for _, rawAction := range actions {
					action, _ := rawAction.(map[string]any)
					commandID, _ := action["command"].(string)
					declaredCommand := false
					for _, command := range instance.Manifest.Contributes.Commands {
						if command.ID == commandID {
							declaredCommand = true
							break
						}
					}
					if !declaredCommand {
						return nil, fmt.Errorf("view action command is not declared: %s", commandID)
					}
				}
			}
		}
		s.mu.Lock()
		s.viewStates[extensionID+"\x00"+viewID] = append(json.RawMessage(nil), stateData...)
		notifier := s.viewNotifier
		s.mu.Unlock()
		if notifier != nil {
			notifier(ExtensionViewUpdate{ExtensionID: extensionID, ViewID: viewID, State: state})
		}
		return map[string]bool{"ok": true}, nil
	default:
		return nil, fmt.Errorf("unknown host API method: %s", method)
	}
}

func (s *Service) activeEnvironmentVariables() map[string]string {
	s.mu.Lock()
	active := cloneAny(s.domainContext["environments"])
	s.mu.Unlock()
	container, _ := active.(map[string]any)
	environment, _ := container["active"].(map[string]any)
	rows, _ := environment["variables"].([]any)
	result := map[string]string{}
	for _, raw := range rows {
		row, _ := raw.(map[string]any)
		enabled, exists := row["enabled"].(bool)
		if exists && !enabled {
			continue
		}
		key, _ := row["key"].(string)
		value, _ := row["value"].(string)
		if strings.TrimSpace(key) != "" {
			result[key] = value
		}
	}
	return result
}

func redactEnvironmentVariables(value any) {
	container, _ := value.(map[string]any)
	redact := func(environment map[string]any) {
		rows, _ := environment["variables"].([]any)
		for _, raw := range rows {
			if row, ok := raw.(map[string]any); ok {
				row["value"] = ""
			}
		}
	}
	if active, ok := container["active"].(map[string]any); ok {
		redact(active)
	}
	if items, ok := container["items"].([]any); ok {
		for _, raw := range items {
			if environment, ok := raw.(map[string]any); ok {
				redact(environment)
			}
		}
	}
}

func (s *Service) handleStateCall(instance ExtensionInstance, method string, params map[string]interface{}) (interface{}, error) {
	scope, _ := params["scope"].(string)
	permission := scope + "State"
	if err := requireGrant(instance, permission); err != nil {
		return nil, err
	}
	key, _ := params["key"].(string)
	if strings.TrimSpace(key) == "" || len(key) > 256 || strings.ContainsRune(key, '\x00') {
		return nil, fmt.Errorf("invalid extension state key")
	}
	s.mu.Lock()
	workspaceID := s.workspaceID
	s.mu.Unlock()
	prefix := "state/" + scope + "/" + instance.Manifest.ID + "/"
	if scope == "workspace" {
		prefix = "state/workspace/" + workspaceID + "/" + instance.Manifest.ID + "/"
	}
	storageKey := prefix + key
	switch method {
	case "state.get":
		data, err := storage.Get(extensionsBucket, storageKey)
		if err != nil {
			return nil, err
		}
		if len(data) == 0 {
			return params["fallback"], nil
		}
		var value interface{}
		if err := json.Unmarshal(data, &value); err != nil {
			return nil, err
		}
		return value, nil
	case "state.delete":
		return map[string]bool{"ok": true}, storage.Delete(extensionsBucket, storageKey)
	case "state.set":
		data, err := json.Marshal(params["value"])
		if err != nil {
			return nil, err
		}
		if len(data) > maxStateValueBytes {
			return nil, fmt.Errorf("extension state value exceeds %d bytes", maxStateValueBytes)
		}
		if err := ensureStateQuota(prefix, storageKey, int64(len(data))); err != nil {
			return nil, err
		}
		return map[string]bool{"ok": true}, storage.Put(extensionsBucket, storageKey, data)
	default:
		return nil, fmt.Errorf("unsupported state operation")
	}
}

func (s *Service) handleSecretCall(instance ExtensionInstance, method string, params map[string]interface{}) (interface{}, error) {
	if err := requireGrant(instance, "secrets.own"); err != nil {
		return nil, err
	}
	key, _ := params["key"].(string)
	if strings.TrimSpace(key) == "" || len(key) > 256 || strings.ContainsRune(key, '\x00') {
		return nil, fmt.Errorf("invalid extension secret key")
	}
	prefix := "secret/" + instance.Manifest.ID + "/"
	storageKey := prefix + key
	switch method {
	case "secrets.get":
		ciphertext, err := storage.Get(extensionsBucket, storageKey)
		if err != nil {
			return nil, err
		}
		if len(ciphertext) == 0 {
			return params["fallback"], nil
		}
		plaintext, err := vault.Open(string(ciphertext))
		if err != nil {
			return nil, fmt.Errorf("open extension secret: %w", err)
		}
		return plaintext, nil
	case "secrets.delete":
		return map[string]bool{"ok": true}, storage.Delete(extensionsBucket, storageKey)
	case "secrets.set":
		value, ok := params["value"].(string)
		if !ok {
			return nil, fmt.Errorf("extension secret value must be a string")
		}
		if len(value) > maxSecretValueBytes {
			return nil, fmt.Errorf("extension secret exceeds %d bytes", maxSecretValueBytes)
		}
		ciphertext, err := vault.Seal(value)
		if err != nil {
			return nil, fmt.Errorf("seal extension secret: %w", err)
		}
		if err := ensureStateQuota(prefix, storageKey, int64(len(ciphertext))); err != nil {
			return nil, err
		}
		return map[string]bool{"ok": true}, storage.Put(extensionsBucket, storageKey, []byte(ciphertext))
	default:
		return nil, fmt.Errorf("unsupported secret operation")
	}
}

func ensureStateQuota(prefix, replacingKey string, replacementBytes int64) error {
	var total int64
	err := storage.DB().View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(extensionsBucket))
		cursor := bucket.Cursor()
		for key, value := cursor.Seek([]byte(prefix)); key != nil && strings.HasPrefix(string(key), prefix); key, value = cursor.Next() {
			if string(key) == replacingKey {
				continue
			}
			total += int64(len(value))
		}
		return nil
	})
	if err != nil {
		return err
	}
	if total+replacementBytes > maxExtensionStateBytes {
		return fmt.Errorf("extension state exceeds %d bytes", maxExtensionStateBytes)
	}
	return nil
}

func (s *Service) checkRateLimit(key string, limit int, window time.Duration) error {
	now := time.Now()
	cutoff := now.Add(-window)
	s.mu.Lock()
	defer s.mu.Unlock()
	entries := s.rateWindows[key]
	kept := entries[:0]
	for _, entry := range entries {
		if entry.After(cutoff) {
			kept = append(kept, entry)
		}
	}
	if len(kept) >= limit {
		s.rateWindows[key] = kept
		return fmt.Errorf("extension API rate limit exceeded")
	}
	s.rateWindows[key] = append(kept, now)
	return nil
}

func requireGrant(instance ExtensionInstance, permission string) error {
	declared := false
	for _, item := range instance.Manifest.Permissions {
		if item == permission {
			declared = true
			break
		}
	}
	if !declared {
		return fmt.Errorf("permission not declared: %s", permission)
	}
	for _, grant := range instance.Grants {
		if grant == permission {
			return nil
		}
	}
	return fmt.Errorf("permission not granted: %s", permission)
}

func (s *Service) checkCompatibility(manifest Manifest) error {
	s.mu.Lock()
	version := s.appVersion
	s.mu.Unlock()
	return CheckEngineCompatibility(version, manifest.Engines.Adomnia)
}

func cloneMap(source map[string]any) map[string]any {
	if len(source) == 0 {
		return map[string]any{}
	}
	data, err := json.Marshal(source)
	if err != nil {
		return map[string]any{}
	}
	var result map[string]any
	if json.Unmarshal(data, &result) != nil || result == nil {
		return map[string]any{}
	}
	return result
}

func cloneMapValue(source any) map[string]any {
	value, _ := cloneAny(source).(map[string]any)
	if value == nil {
		return map[string]any{}
	}
	return value
}

func cloneAny(source any) any {
	if source == nil {
		return nil
	}
	data, err := json.Marshal(source)
	if err != nil {
		return nil
	}
	var result any
	if json.Unmarshal(data, &result) != nil {
		return nil
	}
	return result
}

func (s *Service) Contributions() []Manifest {
	items := s.registry.List()
	result := make([]Manifest, 0, len(items))
	for _, item := range items {
		if item.Enabled {
			result = append(result, item.Manifest)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}
