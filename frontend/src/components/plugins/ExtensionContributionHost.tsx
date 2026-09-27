import { useEffect, useMemo } from 'react'
import { useAppStore } from '@/stores/app'
import { useExtensionsStore } from '@/stores/extensions'
import { useTabsStore } from '@/stores/tabs'
import { useCollectionsStore } from '@/stores/collections'
import { useEnvironmentsStore } from '@/stores/environments'
import { useSettingsStore } from '@/stores/settings'
import { setExtensionDomainContext } from '@/lib/extensions-v2-api'
import { evaluateWhen, type ExtensionContextValues } from '@/lib/extensionContext'
import type { Collection, RequestItem } from '@/lib/types'

export function ExtensionContributionHost() {
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
      }).catch(() => undefined)
    }, 100)
    return () => window.clearTimeout(timer)
  }, [activeEnvId, activeResponse, activeTab, activeWorkspaceId, appearance, collections, environments, tabs, workspaces])

  useEffect(() => {
    let unsubscribe: (() => void) | undefined
    void import('@/wailsjs/runtime/runtime').then(({ EventsOn }) => {
      unsubscribe = EventsOn('extension:domain-action', (raw) => {
        const message = raw as { extensionId?: string; domain?: string; action?: string; payload?: Record<string, unknown> }
        const installed = useExtensionsStore.getState().extensions.find((extension) => extension.manifest.id === message.extensionId)
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
          }
        } catch (error) {
          window.dispatchEvent(new CustomEvent('adomnia:extension-error', { detail: `${message.extensionId}: ${error instanceof Error ? error.message : String(error)}` }))
        }
      })
    })
    return () => unsubscribe?.()
  }, [])

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
