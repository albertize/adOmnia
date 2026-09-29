import { useEffect, useMemo } from 'react'
import { useAppStore } from '@/stores/app'
import { useExtensionsStore } from '@/stores/extensions'
import { useTabsStore } from '@/stores/tabs'
import { useCollectionsStore } from '@/stores/collections'
import { useEnvironmentsStore } from '@/stores/environments'
import { useSettingsStore } from '@/stores/settings'
import { useBrowserDebugStore } from '@/stores/browser-debug'
import { evaluateExtensionVariableProviders, reportExtensionBrokerJob, reportExtensionDatabaseJob, reportExtensionDocumentJob, reportExtensionFlowJob, setExtensionDomainContext } from '@/lib/extensions-v2-api'
import { evaluateWhen, type ExtensionContextValues } from '@/lib/extensionContext'
import type { Collection, RequestItem } from '@/lib/types'
import { loadFlowDefinitions } from '@/lib/flowStorage'
import { runApiFlow } from '@/lib/flowRunner'
import { runFlowStress, type FlowStressConfig } from '@/lib/flowStress'
import { safeStorageGet } from '@/lib/wailsStorage'
import { hydrateDatabaseConnections, resolveDatabaseConnection } from '@/components/database/dbSecrets'
import { isDangerous, isDangerousMongo, normalizeConnection, substituteVars, validateConnection, type DbConnection } from '@/components/database/dbShared'
import { confirm } from '@/lib/confirmDialog'
import { listAllBrokerConnectionProfiles, resolveBrokerPayload, type BrokerConnectionProfile } from '@/lib/brokerConnections'
import { serverUrl, sidecarFetch, useServerPort } from '@/lib/useServerPort'
import { bytesToBase64, loadProject } from '@/lib/pdf/pdfProjects'
import * as AppBindings from '../../../bindings/adomnia/app'

const extensionFlowJobs = new Map<string, AbortController>()
const extensionDatabaseJobs = new Map<string, AbortController>()
const extensionBrokerJobs = new Map<string, AbortController>()
const extensionDocumentJobs = new Map<string, AbortController>()

type ExtensionBrokerPublishOptions = { key?: unknown; headers?: unknown; qos?: unknown; retained?: unknown; persistent?: unknown; contentType?: unknown; partition?: unknown }

function stringHeaders(value: unknown): Record<string, string> {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return {}
  return Object.fromEntries(Object.entries(value).filter((entry): entry is [string, string] => typeof entry[1] === 'string').slice(0, 100))
}

export function extensionBrokerPublishRequest(profile: BrokerConnectionProfile, destination: string, message: string, options: ExtensionBrokerPublishOptions = {}): { path: string; body: Record<string, unknown> } {
  const config = profile.config as Record<string, unknown>
  const headers = stringHeaders(options.headers)
  switch (profile.protocol) {
    case 'kafka': {
      const brokers = typeof config.brokers === 'string' ? config.brokers.split(',').map((item) => item.trim()).filter(Boolean) : config.brokers
      const sasl = config.saslEnabled === true ? { enabled: true, mechanism: config.saslMechanism, username: config.saslUsername, password: config.saslPassword } : config.sasl
      return { path: '/kafka/produce', body: { config: { ...config, brokers, topic: destination, sasl }, key: typeof options.key === 'string' ? options.key : '', value: message, headers, partition: typeof options.partition === 'number' && Number.isInteger(options.partition) ? options.partition : undefined } }
    }
    case 'rabbitmq':
      return { path: '/broker/rabbitmq/publish', body: { config, routingKey: destination, body: message, contentType: typeof options.contentType === 'string' ? options.contentType : 'application/json', headers, persistent: options.persistent === true } }
    case 'mqtt':
      return { path: '/broker/mqtt/publish', body: { config, topic: destination, payload: message, qos: options.qos === 1 || options.qos === 2 ? options.qos : 0, retained: options.retained === true } }
    case 'redis':
      return { path: '/broker/redis/publish', body: { config, channel: destination, message } }
    case 'nats':
      return { path: '/broker/nats/publish', body: { config, subject: destination, payload: message, headers } }
  }
}

export function databaseOperationNeedsConfirmation(connection: DbConnection, query: string): boolean {
  if (connection.driver === 'mongodb') {
    const operation = (JSON.parse(query) as { operation?: unknown }).operation
    const readOnly = new Set(['find', 'findone', 'aggregate', 'countdocuments', 'distinct', 'listcollections'])
    return typeof operation !== 'string' || !readOnly.has(operation.toLowerCase()) || isDangerousMongo(query)
  }
  const firstKeyword = query.trim().match(/^([a-z]+)/i)?.[1]?.toUpperCase()
  return isDangerous(query) || !firstKeyword || !new Set(['SELECT', 'EXPLAIN', 'SHOW', 'DESCRIBE', 'DESC', 'PRAGMA']).has(firstKeyword)
}

export function ExtensionContributionHost() {
  const sidecarPort = useServerPort()
  const activeRail = useAppStore((state) => state.activeRail)
  const extensions = useExtensionsStore((state) => state.extensions)
  const execute = useExtensionsStore((state) => state.execute)
  const tabs = useTabsStore((state) => state.tabs)
  const activeTabId = useTabsStore((state) => state.activeTabId)
  const collections = useCollectionsStore((state) => state.collections)
  const workspaces = useCollectionsStore((state) => state.workspaces)
  const activeWorkspaceId = useCollectionsStore((state) => state.activeWorkspaceId)
  const environments = useEnvironmentsStore((state) => state.environments)
  const activeEnvId = useEnvironmentsStore((state) => state.activeEnvId)
  const appearance = useSettingsStore((state) => state.settings.appearance)
  const browserConnected = useBrowserDebugStore((state) => state.connected)
  const browserEntries = useBrowserDebugStore((state) => state.entries)
  const selectedBrowserEntry = useBrowserDebugStore((state) => state.selectedEntry)
  const activeTab = tabs.find((tab) => tab.id === activeTabId)
  const activeResponse = activeTab?.response

  const context = useMemo<ExtensionContextValues>(() => {
    const values: ExtensionContextValues = {
      activeTool: activeRail,
      hasResponse: Boolean(activeResponse),
      'response.status': activeResponse?.status,
      'response.contentType': activeResponse?.contentType,
    }
    for (const extension of extensions) values[`extension.${extension.manifest.id}.enabled`] = extension.enabled
    return values
  }, [activeRail, activeResponse, extensions])

  useEffect(() => {
    let cancelled = false
    const timer = window.setTimeout(() => {
      const activeEnvironment = environments.find((environment) => environment.id === activeEnvId) ?? null
      const activeWorkspace = workspaces.find((workspace) => workspace.id === activeWorkspaceId) ?? null
      const workspaceTabs = tabs.filter((tab) => (tab.workspaceId ?? activeWorkspaceId) === activeWorkspaceId)
      void setExtensionDomainContext({
        environments: { active: activeEnvironment, items: environments },
        collections: { active: null, items: collections },
        tabs: { active: activeTab ?? null, items: workspaceTabs },
        workspace: { active: activeWorkspace, items: activeWorkspace ? [activeWorkspace] : [] },
        request: activeTab?.request ?? null,
        response: activeResponse ?? null,
        theme: { id: appearance.themeId, mode: appearance.theme },
        browserDebug: { active: selectedBrowserEntry, items: browserEntries.slice(-500), connected: browserConnected },
      }).then(() => evaluateExtensionVariableProviders({
        environment: activeEnvironment ? { id: activeEnvironment.id, name: activeEnvironment.name } : null,
        workspace: activeWorkspace ? { id: activeWorkspace.id, name: activeWorkspace.name } : null,
      })).then((providers) => {
        if (cancelled) return
        const values: Record<string, string> = {}
        for (const provider of providers.sort((left, right) => `${left.extensionId}:${left.providerId}`.localeCompare(`${right.extensionId}:${right.providerId}`))) {
          Object.assign(values, provider.values)
        }
        useEnvironmentsStore.getState().setExtensionVariables(values)
      }).catch((error: unknown) => {
        if (cancelled) return
        useEnvironmentsStore.getState().setExtensionVariables({})
        window.dispatchEvent(new CustomEvent('adomnia:extension-error', { detail: error instanceof Error ? error.message : String(error) }))
      })
    }, 100)
    return () => { cancelled = true; window.clearTimeout(timer) }
  }, [activeEnvId, activeResponse, activeTab, activeWorkspaceId, appearance, browserConnected, browserEntries, collections, environments, extensions, selectedBrowserEntry, tabs, workspaces])

  useEffect(() => {
    let unsubscribe: (() => void) | undefined
    void import('@/wailsjs/runtime/runtime').then(({ EventsOn }) => {
      unsubscribe = EventsOn('extension:domain-action', (raw) => {
        const message = raw as { extensionId?: string; domain?: string; action?: string; payload?: Record<string, unknown> }
        const extensionId = message.extensionId
        if (typeof extensionId !== 'string') return
        const installed = useExtensionsStore.getState().extensions.find((extension) => extension.manifest.id === extensionId)
        if (!installed?.enabled || !message.payload) return
        try {
          if (message.domain === 'collections' && message.action === 'import') {
            const collection = message.payload.collection as Partial<Collection> | undefined
            if (!collection || typeof collection.id !== 'string' || typeof collection.name !== 'string' || !Array.isArray(collection.children)) throw new Error('invalid collection payload')
            if (useCollectionsStore.getState().collections.some((item) => item.id === collection.id)) throw new Error('collection ID already exists')
            useCollectionsStore.getState().importCollection(collection as Collection)
          } else if (message.domain === 'collections' && message.action === 'addRequest') {
            const collectionId = message.payload.collectionId
            const request = message.payload.request as Partial<RequestItem> | undefined
            const parentId = typeof message.payload.parentId === 'string' ? message.payload.parentId : null
            if (typeof collectionId !== 'string' || !request || typeof request.id !== 'string' || typeof request.method !== 'string' || typeof request.url !== 'string') throw new Error('invalid request payload')
            if (!useCollectionsStore.getState().collections.some((item) => item.id === collectionId)) throw new Error('collection not found')
            useCollectionsStore.getState().addRequest(collectionId, parentId, request as RequestItem)
          } else if (message.domain === 'environments' && message.action === 'setActive') {
            const id = message.payload.id
            if (id !== null && (typeof id !== 'string' || !useEnvironmentsStore.getState().environments.some((item) => item.id === id))) throw new Error('environment not found')
            useEnvironmentsStore.getState().setActiveEnv(id as string | null)
          } else if (message.domain === 'tabs' && message.action === 'open') {
            const request = message.payload.request as Partial<RequestItem> | undefined
            if (!request || typeof request.id !== 'string' || typeof request.method !== 'string' || typeof request.url !== 'string') throw new Error('invalid tab request payload')
            const collectionId = typeof message.payload.collectionId === 'string' ? message.payload.collectionId : undefined
            useTabsStore.getState().openTab(request as RequestItem, collectionId)
          } else if (message.domain === 'tabs' && (message.action === 'close' || message.action === 'setActive')) {
            const id = message.payload.id
            if (typeof id !== 'string' || !useTabsStore.getState().tabs.some((item) => item.id === id)) throw new Error('tab not found')
            if (message.action === 'close') useTabsStore.getState().closeTab(id)
            else useTabsStore.getState().setActiveTab(id)
          } else if (message.domain === 'browserDebug' && message.action === 'clear') {
            useBrowserDebugStore.getState().clearEntries()
          } else if (message.domain === 'browserDebug' && message.action === 'select') {
            const id = message.payload.id
            if (id === null) useBrowserDebugStore.getState().setSelectedEntry(null)
            else {
              if (typeof id !== 'string') throw new Error('invalid browser network entry ID')
              const entry = useBrowserDebugStore.getState().entries.find((item) => item.id === id)
              if (!entry) throw new Error('browser network entry not found')
              useBrowserDebugStore.getState().setSelectedEntry(entry)
            }
          } else if (message.domain === 'documents' && message.action === 'cancel') {
            const jobId = message.payload.jobId
            if (typeof jobId !== 'string') throw new Error('invalid document job ID')
            extensionDocumentJobs.get(`${extensionId}:${jobId}`)?.abort()
          } else if (message.domain === 'documents' && (message.action === 'readText' || message.action === 'export')) {
            const jobId = message.payload.jobId
            const projectId = message.payload.projectId
            const options = message.payload.options as { pages?: unknown; flatten?: unknown; suggestedName?: unknown } | undefined
            if (typeof jobId !== 'string' || typeof projectId !== 'string') throw new Error('invalid document job payload')
            const key = `${extensionId}:${jobId}`
            if (extensionDocumentJobs.has(key)) throw new Error('document job already exists')
            const controller = new AbortController()
            extensionDocumentJobs.set(key, controller)
            const event = message.action === 'readText' ? 'onDocumentReadComplete' : 'onDocumentWriteComplete'
            void loadProject(projectId).then(async (project) => {
              if (!project) throw new Error('PDF project not found')
              if (controller.signal.aborted) throw new DOMException('Document job cancelled', 'AbortError')
              if (message.action === 'readText') {
                const { extractPageText, loadPdfDocument } = await import('@/lib/pdf/pdfDocument')
                const loaded = await loadPdfDocument(project.pdfBytes)
                try {
                  const requested = Array.isArray(options?.pages) ? options.pages.filter((page): page is number => Number.isInteger(page) && page >= 1 && page <= loaded.pageCount).slice(0, 500) : []
                  const pages = requested.length ? [...new Set(requested)] : Array.from({ length: Math.min(loaded.pageCount, 500) }, (_, index) => index + 1)
                  const result: Array<{ page: number; text: string }> = []
                  let remaining = 750_000
                  for (const page of pages) {
                    if (controller.signal.aborted) throw new DOMException('Document job cancelled', 'AbortError')
                    const text = (await extractPageText(loaded.doc, page)).slice(0, remaining)
                    result.push({ page, text })
                    remaining -= text.length
                    if (remaining <= 0) break
                  }
                  await reportExtensionDocumentJob(extensionId, jobId, event, { success: true, result: { pages: result, truncated: remaining <= 0 || pages.length < loaded.pageCount } })
                } finally {
                  loaded.doc.cleanup()
                }
              } else {
                const flatten = options?.flatten !== false
                const output = flatten ? await (await import('@/lib/pdf/pdfExport')).exportPdf(project.pdfBytes, project.annotations, project.formValues) : project.pdfBytes
                if (controller.signal.aborted) throw new DOMException('Document job cancelled', 'AbortError')
                const requestedName = typeof options?.suggestedName === 'string' ? options.suggestedName.trim().replace(/[\\/]/g, '-').slice(0, 180) : ''
                const suggestedName = requestedName || `${project.name.replace(/\.pdf$/i, '')}-extension-export.pdf`
                const savedPath = await AppBindings.SaveBinaryFileBase64(suggestedName, bytesToBase64(output))
                await reportExtensionDocumentJob(extensionId, jobId, event, savedPath ? { success: true, result: { saved: true } } : { success: false, error: 'export cancelled by user' })
              }
            }).catch((error: unknown) => reportExtensionDocumentJob(extensionId, jobId, event, { success: false, error: error instanceof Error ? error.message : String(error) })).catch(() => undefined).finally(() => extensionDocumentJobs.delete(key))
          } else if (message.domain === 'brokers' && message.action === 'cancel') {
            const jobId = message.payload.jobId
            if (typeof jobId !== 'string') throw new Error('invalid broker job ID')
            extensionBrokerJobs.get(`${extensionId}:${jobId}`)?.abort()
          } else if (message.domain === 'brokers' && message.action === 'publish') {
            const jobId = message.payload.jobId
            const connectionId = message.payload.connectionId
            const destination = message.payload.destination
            const content = message.payload.message
            const options = message.payload.options as ExtensionBrokerPublishOptions | undefined
            if (typeof jobId !== 'string' || typeof connectionId !== 'string' || typeof destination !== 'string' || typeof content !== 'string') throw new Error('invalid broker publish payload')
            const key = `${extensionId}:${jobId}`
            if (extensionBrokerJobs.has(key)) throw new Error('broker publish job already exists')
            const controller = new AbortController()
            extensionBrokerJobs.set(key, controller)
            void listAllBrokerConnectionProfiles().then(async (profiles) => {
              const profile = profiles.find((item) => item.id === connectionId)
              if (!profile) throw new Error('broker connection not found')
              const approved = await confirm({
                title: 'Extension broker publish',
                message: `${installed.manifest.name} wants to publish to ${profile.name} (${profile.protocol}) at “${destination}”.\n\n${content.slice(0, 500)}`,
                confirmLabel: 'Publish message', variant: 'danger',
              })
              if (!approved) throw new Error('broker publish cancelled by user')
              const request = extensionBrokerPublishRequest(profile, destination, content, options)
              const url = serverUrl(sidecarPort, request.path)
              if (!url) throw new Error('broker backend is not ready')
              const response = await sidecarFetch(url, {
                method: 'POST', signal: controller.signal,
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(await resolveBrokerPayload(request.body)),
              })
              const text = await response.text()
              if (!response.ok) throw new Error(text.trim() || response.statusText)
              await reportExtensionBrokerJob(extensionId, jobId, { success: true, result: text ? JSON.parse(text) : {} }).catch(async () => {
                await reportExtensionBrokerJob(extensionId, jobId, { success: false, error: 'Broker result exceeded the extension result limit.' })
              })
            }).catch((error: unknown) => reportExtensionBrokerJob(extensionId, jobId, { success: false, error: error instanceof Error ? error.message : String(error) })).catch(() => undefined).finally(() => extensionBrokerJobs.delete(key))
          } else if (message.domain === 'databases' && message.action === 'cancel') {
            const jobId = message.payload.jobId
            if (typeof jobId !== 'string') throw new Error('invalid database job ID')
            extensionDatabaseJobs.get(`${extensionId}:${jobId}`)?.abort()
          } else if (message.domain === 'databases' && message.action === 'query') {
            const jobId = message.payload.jobId
            const connectionId = message.payload.connectionId
            const query = message.payload.query
            const options = message.payload.options as { limit?: unknown; timeoutMs?: unknown; explain?: unknown } | undefined
            if (typeof jobId !== 'string' || typeof connectionId !== 'string' || typeof query !== 'string') throw new Error('invalid database job payload')
            const key = `${extensionId}:${jobId}`
            if (extensionDatabaseJobs.has(key)) throw new Error('database job already exists')
            const controller = new AbortController()
            extensionDatabaseJobs.set(key, controller)
            void safeStorageGet('database', 'connections').then(async (raw) => {
              const parsed = raw ? JSON.parse(raw) as Partial<DbConnection>[] : []
              const connection = hydrateDatabaseConnections(parsed.map(normalizeConnection)).find((item) => item.id === connectionId)
              if (!connection) throw new Error('database connection not found')
              const connectionError = validateConnection(connection)
              if (connectionError) throw new Error(connectionError)
              const renderedQuery = substituteVars(query, useEnvironmentsStore.getState().getResolvedVars())
              if (!renderedQuery.trim()) throw new Error('database query is empty')
              if (connection.driver === 'mongodb') JSON.parse(renderedQuery)
              const dangerous = databaseOperationNeedsConfirmation(connection, renderedQuery)
              if (dangerous) {
                const approved = await confirm({ title: 'Extension database write', message: `${installed.manifest.name} requested a potentially destructive database operation.\n\n${renderedQuery.slice(0, 500)}`, confirmLabel: 'Run query', variant: 'danger' })
                if (!approved) throw new Error('database operation cancelled by user')
              }
              const url = serverUrl(sidecarPort, '/database/query')
              if (!url) throw new Error('database backend is not ready')
              const response = await sidecarFetch(url, {
                method: 'POST', signal: controller.signal,
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                  connection: await resolveDatabaseConnection(connection), query: renderedQuery,
                  limit: typeof options?.limit === 'number' && Number.isFinite(options.limit) ? Math.max(1, Math.min(10_000, options.limit)) : 200,
                  timeoutMs: typeof options?.timeoutMs === 'number' && Number.isFinite(options.timeoutMs) ? Math.max(100, Math.min(120_000, options.timeoutMs)) : 30_000,
                  explain: options?.explain === true, confirm: dangerous,
                }),
              })
              const text = await response.text()
              if (!response.ok) throw new Error(text.trim() || response.statusText)
              await reportExtensionDatabaseJob(extensionId, jobId, { success: true, result: text ? JSON.parse(text) : {} }).catch(async () => {
                await reportExtensionDatabaseJob(extensionId, jobId, { success: false, error: 'Database result exceeded the extension result limit.' })
              })
            }).catch((error: unknown) => reportExtensionDatabaseJob(extensionId, jobId, { success: false, error: error instanceof Error ? error.message : String(error) })).catch(() => undefined).finally(() => extensionDatabaseJobs.delete(key))
          } else if (message.domain === 'flows' && message.action === 'cancel') {
            const jobId = message.payload.jobId
            if (typeof jobId !== 'string') throw new Error('invalid flow job ID')
            extensionFlowJobs.get(`${extensionId}:${jobId}`)?.abort()
          } else if (message.domain === 'flows' && (message.action === 'start' || message.action === 'stress')) {
            const jobId = message.payload.jobId
            const flowId = message.payload.flowId
            const options = message.payload.options as { startNodeId?: unknown } | undefined
            if (typeof jobId !== 'string' || typeof flowId !== 'string') throw new Error('invalid flow job payload')
            const key = `${extensionId}:${jobId}`
            if (extensionFlowJobs.has(key)) throw new Error('flow job already exists')
            const controller = new AbortController()
            extensionFlowJobs.set(key, controller)
            void loadFlowDefinitions().then(async (flows) => {
              const flow = flows.find((item) => item.id === flowId)
              if (!flow) throw new Error('flow not found')
              const initialVars = useEnvironmentsStore.getState().getResolvedVars()
              let boundedResult: unknown
              if (message.action === 'stress') {
                const result = await runFlowStress(flow.graph, options as FlowStressConfig, {
                  initialVars,
                  signal: controller.signal,
                  onProgress: (progress) => { void reportExtensionFlowJob(extensionId, jobId, 'onFlowProgress', { kind: 'stress', progress }).catch(() => undefined) },
                })
                boundedResult = { status: result.status, stats: result.stats, truncated: result.truncated, startedAt: result.startedAt, finishedAt: result.finishedAt }
              } else {
                boundedResult = await runApiFlow(flow.graph, {
                  initialVars,
                  startNodeId: typeof options?.startNodeId === 'string' ? options.startNodeId : undefined,
                  signal: controller.signal,
                  onEntry: (entries) => { void reportExtensionFlowJob(extensionId, jobId, 'onFlowProgress', { kind: 'flow', entriesCompleted: entries.length, lastEntry: entries[entries.length - 1] }).catch(() => undefined) },
                })
              }
              await reportExtensionFlowJob(extensionId, jobId, 'onFlowComplete', { success: true, kind: message.action, result: boundedResult }).catch(async () => {
                await reportExtensionFlowJob(extensionId, jobId, 'onFlowComplete', { success: false, error: 'Flow result exceeded the extension result limit.' })
              })
            }).catch((error: unknown) => reportExtensionFlowJob(extensionId, jobId, 'onFlowComplete', { success: false, error: error instanceof Error ? error.message : String(error) })).catch(() => undefined).finally(() => extensionFlowJobs.delete(key))
          }
        } catch (error) {
          window.dispatchEvent(new CustomEvent('adomnia:extension-error', { detail: `${message.extensionId}: ${error instanceof Error ? error.message : String(error)}` }))
        }
      })
    })
    return () => unsubscribe?.()
  }, [sidecarPort])

  useEffect(() => {
    const keybindings = extensions.flatMap((extension) => extension.enabled
      ? (extension.manifest.contributes?.keybindings ?? []).map((binding) => ({ extensionId: extension.manifest.id, binding }))
      : [])
    if (keybindings.length === 0) return
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.defaultPrevented || event.repeat) return
      const target = event.target as HTMLElement | null
      const editing = target?.matches('input, textarea, select, [contenteditable="true"]') ?? false
      for (const item of keybindings) {
        if (!matchesShortcut(event, item.binding.key) || !evaluateWhen(item.binding.when, context)) continue
        if (editing && !event.ctrlKey && !event.metaKey && !event.altKey) continue
        event.preventDefault()
        void execute(item.extensionId, item.binding.command, 'keybinding').catch((error: unknown) => {
          window.dispatchEvent(new CustomEvent('adomnia:extension-error', { detail: error instanceof Error ? error.message : String(error) }))
        })
        return
      }
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [context, execute, extensions])

  return null
}

function matchesShortcut(event: KeyboardEvent, shortcut: string): boolean {
  const parts = shortcut.toLowerCase().split('+').map((part) => part.trim()).filter(Boolean)
  const key = parts[parts.length - 1]
  const wantsCtrl = parts.includes('ctrl') || parts.includes('cmdorctrl')
  const wantsMeta = parts.includes('cmd') || parts.includes('meta')
  const platformModifier = parts.includes('cmdorctrl') ? event.ctrlKey || event.metaKey : true
  if (!platformModifier) return false
  if (!parts.includes('cmdorctrl') && event.ctrlKey !== wantsCtrl) return false
  if (!parts.includes('cmdorctrl') && event.metaKey !== wantsMeta) return false
  if (event.altKey !== parts.includes('alt')) return false
  if (event.shiftKey !== parts.includes('shift')) return false
  return event.key.toLowerCase() === key
}
