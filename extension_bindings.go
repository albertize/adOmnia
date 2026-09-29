package main

import (
	extensionRuntime "adomnia/internal/extensions"
	extensionsdk "adomnia/sdk/extensions"
)

// ExtensionService is the intentionally small Wails binding layer for
// Extension Platform v2. Host lifecycle and HTTP transform methods remain
// private; the renderer can emit only the bounded, allowlisted workbench events
// whose canonical stores live in the frontend.
type ExtensionService struct{ service *extensionRuntime.Service }

var globalExtensionService *ExtensionService

func NewExtensionService() *ExtensionService {
	service := extensionRuntime.NewService(dataDir())
	service.SetAppVersion(Version)
	return &ExtensionService{service: service}
}

func (e *ExtensionService) GetExtensions() []extensionRuntime.ExtensionInstance {
	return e.service.GetExtensions()
}

func (e *ExtensionService) GetLogs(extensionID string) []extensionRuntime.ExtensionLog {
	return e.service.GetLogs(extensionID)
}

func (e *ExtensionService) GetDiagnostics(extensionID string) []extensionRuntime.ExtensionDiagnostic {
	return e.service.GetDiagnostics(extensionID)
}

func (e *ExtensionService) GetStorageUsage(extensionID string) (extensionRuntime.ExtensionStorageUsage, error) {
	return e.service.GetStorageUsage(extensionID)
}

func (e *ExtensionService) ResetStorage(extensionID string) error {
	return e.service.ResetStorage(extensionID)
}

func (e *ExtensionService) ExportSDK(destination string) (string, error) {
	return extensionsdk.Export(destination)
}

func (e *ExtensionService) InstallDirectory(path string, development bool) (extensionRuntime.ExtensionInstance, error) {
	return e.service.InstallDirectory(path, development)
}

func (e *ExtensionService) InstallArchive(path string) (extensionRuntime.ExtensionInstance, error) {
	return e.service.InstallArchive(path)
}

func (e *ExtensionService) Reload(id string) (extensionRuntime.ExtensionInstance, error) {
	return e.service.Reload(id)
}

func (e *ExtensionService) SetGrants(id string, grants []string) (extensionRuntime.ExtensionInstance, error) {
	return e.service.SetGrants(id, grants)
}

func (e *ExtensionService) Enable(id string) (extensionRuntime.ExtensionInstance, error) {
	return e.service.Enable(id)
}

func (e *ExtensionService) Disable(id string) (extensionRuntime.ExtensionInstance, error) {
	return e.service.Disable(id)
}

func (e *ExtensionService) Uninstall(id string) error {
	return e.service.Uninstall(id)
}

func (e *ExtensionService) SetSetting(id, key, valueJSON string) (extensionRuntime.ExtensionInstance, error) {
	return e.service.SetSetting(id, key, valueJSON)
}

func (e *ExtensionService) ExecuteCommand(extensionID, commandID, argsJSON, source string) (extensionRuntime.HostExecutionResult, error) {
	return e.service.ExecuteCommand(extensionID, commandID, argsJSON, source)
}

func (e *ExtensionService) GetRuntimeStatus() extensionRuntime.RuntimeStatus {
	return e.service.GetRuntimeStatus()
}

func (e *ExtensionService) GetViewState(extensionID, viewID string) string {
	return e.service.GetViewState(extensionID, viewID)
}

func (e *ExtensionService) OpenView(extensionID, viewID string) (string, error) {
	return e.service.OpenView(extensionID, viewID)
}

func (e *ExtensionService) GetWebviewHTML(extensionID, viewID string) (string, error) {
	return e.service.GetWebviewHTML(extensionID, viewID)
}

func (e *ExtensionService) Contributions() []extensionRuntime.Manifest {
	return e.service.Contributions()
}

func (e *ExtensionService) SetWorkspaceContext(workspaceID string) {
	e.service.SetWorkspaceContext(workspaceID)
}

func (e *ExtensionService) SetDomainContext(contextJSON string) error {
	return e.service.SetDomainContext(contextJSON)
}

func (e *ExtensionService) NotifyWorkbenchEvent(event, payloadJSON string) error {
	return e.service.NotifyWorkbenchEvent(event, payloadJSON)
}

func (e *ExtensionService) ReportFlowJobEvent(extensionID, jobID, event, payloadJSON string) error {
	return e.service.ReportFlowJobEvent(extensionID, jobID, event, payloadJSON)
}

func (e *ExtensionService) ReportDatabaseJobEvent(extensionID, jobID, payloadJSON string) error {
	return e.service.ReportDatabaseJobEvent(extensionID, jobID, payloadJSON)
}

func (e *ExtensionService) ReportBrokerJobEvent(extensionID, jobID, payloadJSON string) error {
	return e.service.ReportBrokerJobEvent(extensionID, jobID, payloadJSON)
}

func (e *ExtensionService) ReportDocumentJobEvent(extensionID, jobID, event, payloadJSON string) error {
	return e.service.ReportDocumentJobEvent(extensionID, jobID, event, payloadJSON)
}

func (e *ExtensionService) ReportAIJobEvent(extensionID, jobID, payloadJSON string) error {
	return e.service.ReportAIJobEvent(extensionID, jobID, payloadJSON)
}

func (e *ExtensionService) EvaluateAssertions(payloadJSON string) ([]extensionRuntime.AssertionProviderResult, error) {
	return e.service.EvaluateAssertions(payloadJSON)
}

func (e *ExtensionService) EvaluateVariableProviders(contextJSON string) ([]extensionRuntime.VariableProviderResult, error) {
	return e.service.EvaluateVariableProviders(contextJSON)
}

func (e *ExtensionService) init() error  { return e.service.Init() }
func (e *ExtensionService) fireStartup() { e.service.FireStartup() }
func (e *ExtensionService) shutdown()    { e.service.Shutdown() }
func (e *ExtensionService) applyEventJSON(event, payload string) (string, error) {
	return e.service.ApplyEventJSON(event, payload)
}
