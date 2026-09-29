import * as ExtensionBindings from '../../bindings/adomnia/extensionservice'
import * as AppBindings from '../../bindings/adomnia/app'
import type {
  AssertionProviderResult,
  ExtensionDiagnostic,
  ExtensionInstance,
  ExtensionLog,
  ExtensionStorageUsage,
  HostExecutionResult,
  Manifest,
  RuntimeStatus,
  VariableProviderResult,
} from '../../bindings/adomnia/internal/extensions/models'

export type { AssertionProviderResult, ExtensionDiagnostic, ExtensionInstance, ExtensionLog, ExtensionStorageUsage, HostExecutionResult, Manifest, RuntimeStatus, VariableProviderResult }

export const listExtensions = (): Promise<ExtensionInstance[]> => ExtensionBindings.GetExtensions()
export const getExtensionLogs = (extensionId: string): Promise<ExtensionLog[]> => ExtensionBindings.GetLogs(extensionId)
export const getExtensionDiagnostics = (extensionId: string): Promise<ExtensionDiagnostic[]> => ExtensionBindings.GetDiagnostics(extensionId)
export const getExtensionStorageUsage = (extensionId: string): Promise<ExtensionStorageUsage> => ExtensionBindings.GetStorageUsage(extensionId)
export const resetExtensionStorage = (extensionId: string): Promise<void> => ExtensionBindings.ResetStorage(extensionId)
export const exportExtensionSDK = (destination: string): Promise<string> => ExtensionBindings.ExportSDK(destination)
export const listExtensionContributions = (): Promise<Manifest[]> => ExtensionBindings.Contributions()
export const installExtensionDirectory = (path: string, development = false): Promise<ExtensionInstance> =>
  ExtensionBindings.InstallDirectory(path, development)
export const installExtensionArchive = (path: string): Promise<ExtensionInstance> =>
  ExtensionBindings.InstallArchive(path)
export const reloadExtension = (id: string): Promise<ExtensionInstance> => ExtensionBindings.Reload(id)
export const setExtensionGrants = (id: string, grants: string[]): Promise<ExtensionInstance> =>
  ExtensionBindings.SetGrants(id, grants)
export const enableExtension = (id: string): Promise<ExtensionInstance> => ExtensionBindings.Enable(id)
export const disableExtension = (id: string): Promise<ExtensionInstance> => ExtensionBindings.Disable(id)
export const uninstallExtension = (id: string): Promise<void> => ExtensionBindings.Uninstall(id)
export const setExtensionSetting = (id: string, key: string, value: unknown): Promise<ExtensionInstance> =>
  ExtensionBindings.SetSetting(id, key, JSON.stringify(value))
export const executeExtensionCommand = (
  extensionId: string,
  commandId: string,
  args: Record<string, unknown> = {},
  source = 'extension',
): Promise<HostExecutionResult> => ExtensionBindings.ExecuteCommand(extensionId, commandId, JSON.stringify(args), source)
export const getExtensionRuntimeStatus = (): Promise<RuntimeStatus> => ExtensionBindings.GetRuntimeStatus()
export const getExtensionViewState = (extensionId: string, viewId: string): Promise<string> => ExtensionBindings.GetViewState(extensionId, viewId)
export const openExtensionView = (extensionId: string, viewId: string): Promise<string> => ExtensionBindings.OpenView(extensionId, viewId)
export const getExtensionWebviewHTML = (extensionId: string, viewId: string): Promise<string> => ExtensionBindings.GetWebviewHTML(extensionId, viewId)
export const setExtensionWorkspaceContext = (workspaceId: string): Promise<void> => ExtensionBindings.SetWorkspaceContext(workspaceId)
export const setExtensionDomainContext = (context: Record<string, unknown>): Promise<void> => ExtensionBindings.SetDomainContext(JSON.stringify(context))
export const notifyExtensionWorkbenchEvent = (event: 'onSave' | 'onImport' | 'onExport', payload: Record<string, unknown>): Promise<void> => ExtensionBindings.NotifyWorkbenchEvent(event, JSON.stringify(payload))
export const reportExtensionAIJob = (extensionId: string, jobId: string, payload: Record<string, unknown>): Promise<void> => ExtensionBindings.ReportAIJobEvent(extensionId, jobId, JSON.stringify(payload))
export const reportExtensionDocumentJob = (extensionId: string, jobId: string, event: 'onDocumentReadComplete' | 'onDocumentWriteComplete', payload: Record<string, unknown>): Promise<void> => ExtensionBindings.ReportDocumentJobEvent(extensionId, jobId, event, JSON.stringify(payload))
export const reportExtensionBrokerJob = (extensionId: string, jobId: string, payload: Record<string, unknown>): Promise<void> => ExtensionBindings.ReportBrokerJobEvent(extensionId, jobId, JSON.stringify(payload))
export const reportExtensionDatabaseJob = (extensionId: string, jobId: string, payload: Record<string, unknown>): Promise<void> => ExtensionBindings.ReportDatabaseJobEvent(extensionId, jobId, JSON.stringify(payload))
export const reportExtensionFlowJob = (extensionId: string, jobId: string, event: 'onFlowProgress' | 'onFlowComplete', payload: Record<string, unknown>): Promise<void> => ExtensionBindings.ReportFlowJobEvent(extensionId, jobId, event, JSON.stringify(payload))
export const evaluateExtensionAssertions = (payload: Record<string, unknown>): Promise<AssertionProviderResult[]> => ExtensionBindings.EvaluateAssertions(JSON.stringify(payload))
export const evaluateExtensionVariableProviders = (context: Record<string, unknown>): Promise<VariableProviderResult[]> => ExtensionBindings.EvaluateVariableProviders(JSON.stringify(context))

export const selectExtensionDirectory = (): Promise<string> => AppBindings.SelectFolder('Select adOmnia extension folder')
export const selectExtensionArchive = (): Promise<string> => AppBindings.SelectExtensionArchive()
