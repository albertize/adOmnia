/** Version-matched contract for the isolated adOmnia Extension Platform v2 runtime. */
export interface Disposable {
  dispose(): void | Promise<void>
}

export interface SubscriptionStore {
  add(disposable: Disposable): void
}

export interface ExtensionContext {
  readonly id: string
  readonly version: string
  readonly subscriptions: SubscriptionStore
}

export interface CommandContext {
  readonly source: 'palette' | 'menu' | 'keybinding' | 'extension' | 'unknown'
}

export interface CommandsAPI {
  registerCommand(
    id: string,
    handler: (args: unknown, context: CommandContext) => unknown | Promise<unknown>,
  ): Disposable
  executeCommand<T = unknown>(id: string, args?: unknown): Promise<T>
}

export interface TransformResult<T extends Record<string, unknown>> {
  modified: boolean
  data: T
}

export type EventHandler<T extends Record<string, unknown> = Record<string, unknown>> =
  (payload: T) => void | TransformResult<T> | Promise<void | TransformResult<T>>

export interface EventsAPI {
  on<T extends Record<string, unknown> = Record<string, unknown>>(event: string, handler: EventHandler<T>): Disposable
  onRequest(handler: EventHandler): Disposable
  onSend(handler: EventHandler): Disposable
  onResponse(handler: EventHandler): Disposable
  onSave(handler: EventHandler): Disposable
  onImport(handler: EventHandler): Disposable
  onExport(handler: EventHandler): Disposable
  onThemeChange(handler: EventHandler): Disposable
  onEnvironmentChange(handler: EventHandler): Disposable
  onTabOpen(handler: EventHandler): Disposable
  onTabClose(handler: EventHandler): Disposable
  onWorkspaceOpen(handler: EventHandler): Disposable
  onWorkspaceClose(handler: EventHandler): Disposable
  onBrowserNetwork(handler: EventHandler): Disposable
  onMockHit(handler: EventHandler): Disposable
  onProxyTraffic(handler: EventHandler): Disposable
  onFlowProgress(handler: EventHandler): Disposable
  onFlowComplete(handler: EventHandler): Disposable
  onDatabaseComplete(handler: EventHandler): Disposable
  onBrokerPublishComplete(handler: EventHandler): Disposable
  onDocumentReadComplete(handler: EventHandler): Disposable
  onDocumentWriteComplete(handler: EventHandler): Disposable
}

export interface DeclarativeViewState {
  kind: 'empty' | 'list' | 'tree' | 'table' | 'form' | 'details' | 'markdown' | 'json'
  title?: string
  message?: string
  columns?: Array<{ key: string; title: string }>
  rows?: Array<Record<string, unknown>>
  items?: Array<{ id: string; title: string; description?: string; badge?: string; parentId?: string }>
  fields?: Array<{
    id: string
    label: string
    type: 'text' | 'number' | 'boolean' | 'select'
    value?: unknown
    placeholder?: string
    options?: string[]
  }>
  actions?: Array<{ id: string; title: string; command: string }>
  data?: unknown
}

export interface RequestsAPI {
  getActive<T extends Record<string, unknown> = Record<string, unknown>>(): Promise<T>
  execute<TResponse extends Record<string, unknown> = Record<string, unknown>>(request: Record<string, unknown>): Promise<TResponse>
}

export interface VariablesAPI {
  getAll(): Promise<Readonly<Record<string, string>>>
  resolve(value: string): Promise<string>
  registerProvider(
    id: string,
    handler: (context: Record<string, unknown>) => Readonly<Record<string, string>> | Promise<Readonly<Record<string, string>>>,
  ): Disposable
}

export interface AssertionProviderResult {
  label: string
  passed: boolean
  actual?: string
  expected?: string
  message?: string
}

export interface AssertionsAPI {
  registerProvider(
    id: string,
    handler: (payload: Record<string, unknown>) => AssertionProviderResult | readonly AssertionProviderResult[] | Promise<AssertionProviderResult | readonly AssertionProviderResult[]>,
  ): Disposable
}

export interface ResponsesAPI {
  getActive<T extends Record<string, unknown> = Record<string, unknown>>(): Promise<T>
}

export interface ReadDomainAPI<T = Record<string, unknown>> {
  getActive(): Promise<T | null>
  list(): Promise<readonly T[]>
  getSnapshot(): Promise<unknown>
}

export interface ExtensionDiagnostic {
  severity: 'info' | 'warning' | 'error'
  message: string
  resource?: string
  line?: number
  column?: number
}

export interface ProgressAPI {
  report(update: { title: string; message?: string; increment?: number; done?: boolean }): Promise<void>
}

export interface DiagnosticsAPI {
  set(entries: readonly ExtensionDiagnostic[]): Promise<void>
}

export interface CollectionsAPI<T = Record<string, unknown>> extends ReadDomainAPI<T> {
  importCollection(collection: Record<string, unknown>): Promise<void>
  addRequest(collectionId: string, parentId: string | null, request: Record<string, unknown>): Promise<void>
}

export interface EnvironmentsAPI<T = Record<string, unknown>> extends ReadDomainAPI<T> {
  setActive(id: string | null): Promise<void>
}

export interface TabsAPI<T = Record<string, unknown>> extends ReadDomainAPI<T> {
  open(request: Record<string, unknown>, collectionId?: string): Promise<void>
  close(id: string): Promise<void>
  setActive(id: string): Promise<void>
}

export interface MockAPI {
  getSnapshot<T = Record<string, unknown>>(): Promise<T>
  clearHits(): Promise<void>
  stop(): Promise<void>
}

export interface DocumentsAPI<T = Record<string, unknown>> {
  listPdfProjects(): Promise<readonly T[]>
  readPdfText(projectId: string, options?: { pages?: readonly number[] }): Promise<{ jobId: string }>
  exportPdf(projectId: string, options?: { flatten?: boolean; suggestedName?: string }): Promise<{ jobId: string }>
  cancel(jobId: string): Promise<void>
}

export interface ConnectionMetadataAPI<T = Record<string, unknown>> {
  listConnections(): Promise<readonly T[]>
}

export interface DatabasesAPI<T = Record<string, unknown>> extends ConnectionMetadataAPI<T> {
  execute(connectionId: string, query: string, options?: { limit?: number; timeoutMs?: number; explain?: boolean }): Promise<{ jobId: string }>
  cancel(jobId: string): Promise<void>
}

export interface BrokersAPI<T = Record<string, unknown>> extends ConnectionMetadataAPI<T> {
  publish(connectionId: string, destination: string, message: string, options?: { key?: string; headers?: Record<string, string>; qos?: 0 | 1 | 2; retained?: boolean; persistent?: boolean; contentType?: string; partition?: number }): Promise<{ jobId: string }>
  cancel(jobId: string): Promise<void>
}

export interface FlowsAPI<T = Record<string, unknown>> {
  list(): Promise<readonly T[]>
  get(id: string): Promise<T | null>
  execute(id: string, options?: { startNodeId?: string }): Promise<{ jobId: string }>
  executeStress(id: string, config: Record<string, unknown>): Promise<{ jobId: string }>
  cancel(jobId: string): Promise<void>
}

export interface ProxyAPI {
  getSnapshot<T = Record<string, unknown>>(): Promise<T>
  clearTraffic(): Promise<void>
  stop(): Promise<void>
}

export interface BrowserDebugAPI<T = Record<string, unknown>> extends ReadDomainAPI<T> {
  clear(): Promise<void>
  select(id: string | null): Promise<void>
}

export interface ViewsAPI {
  setState(viewId: string, state: DeclarativeViewState): Promise<void>
}

export interface ConfigurationAPI {
  get<T>(key: string, fallback?: T): Promise<T>
  onDidChange(handler: (keys: readonly string[]) => void | Promise<void>): Disposable
}

export interface KeyValueState {
  get<T>(key: string, fallback?: T): Promise<T>
  set(key: string, value: unknown): Promise<void>
  delete(key: string): Promise<void>
}

export interface SecretsAPI {
  get(key: string, fallback?: string): Promise<string | undefined>
  set(key: string, value: string): Promise<void>
  delete(key: string): Promise<void>
}

export interface LoggingAPI {
  debug(message: string, fields?: Record<string, unknown>): void
  info(message: string, fields?: Record<string, unknown>): void
  warn(message: string, fields?: Record<string, unknown>): void
  error(message: string, fields?: Record<string, unknown>): void
}

export interface WindowAPI {
  notify(message: string, type?: 'info' | 'success' | 'warning' | 'error'): Promise<void>
}

export interface ExtensionAPI {
  readonly context: ExtensionContext
  readonly commands: CommandsAPI
  readonly events: EventsAPI
  readonly requests: RequestsAPI
  readonly responses: ResponsesAPI
  readonly variables: VariablesAPI
  readonly assertions: AssertionsAPI
  readonly environments: EnvironmentsAPI
  readonly collections: CollectionsAPI
  readonly tabs: TabsAPI
  readonly workspace: ReadDomainAPI
  readonly browserDebug: BrowserDebugAPI
  readonly mock: MockAPI
  readonly proxy: ProxyAPI
  readonly flows: FlowsAPI
  readonly databases: DatabasesAPI
  readonly brokers: BrokersAPI
  readonly documents: DocumentsAPI
  readonly diagnostics: DiagnosticsAPI
  readonly progress: ProgressAPI
  readonly views: ViewsAPI
  readonly configuration: ConfigurationAPI
  readonly globalState: KeyValueState
  readonly workspaceState: KeyValueState
  readonly secrets: SecretsAPI
  readonly logging: LoggingAPI
  readonly window: WindowAPI
}

/** Called lazily after one of the manifest activation events fires. */
export function activate(api: ExtensionAPI): void | Promise<void>

/** Optional cleanup. Registered subscriptions are disposed even when this is absent. */
export function deactivate(): void | Promise<void>
