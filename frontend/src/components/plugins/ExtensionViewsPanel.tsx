import { useEffect, useMemo, useRef } from 'react'
import { Boxes, Puzzle } from 'lucide-react'
import { ExtensionDeclarativeView } from './ExtensionDeclarativeView'
import { ExtensionWebview } from './ExtensionWebview'
import { evaluateWhen, type ExtensionContextValues } from '@/lib/extensionContext'
import type { ExtensionInstance } from '@/lib/extensions-v2-api'
import { useExtensionsStore } from '@/stores/extensions'
import { useAppStore } from '@/stores/app'
import { useTabsStore } from '@/stores/tabs'
import { useUiTranslation } from '@/lib/uiI18n'
import { cn } from '@/lib/utils'

interface ExtensionViewContainer {
  id: string
  extensionId: string
  title: string
  views: NonNullable<ExtensionInstance['manifest']['contributes']>['views']
}

export function collectExtensionViewContainers(extensions: ExtensionInstance[], context: ExtensionContextValues): ExtensionViewContainer[] {
  return extensions.flatMap((extension) => {
    if (!extension.enabled) return []
    const grouped = new Map<string, NonNullable<ExtensionViewContainer['views']>>()
    for (const view of extension.manifest.contributes?.views ?? []) {
      if (view.container === 'response' || !evaluateWhen(view.when, context)) continue
      grouped.set(view.container, [...(grouped.get(view.container) ?? []), view])
    }
    return [...grouped.entries()].map(([container, views]) => ({
      id: `${extension.manifest.id}:${container}`,
      extensionId: extension.manifest.id,
      title: container === 'extensions'
        ? extension.manifest.name
        : `${extension.manifest.name} · ${humanizeContainer(container)}`,
      views,
    }))
  }).sort((left, right) => left.title.localeCompare(right.title))
}

export function ExtensionViewsPanel() {
  const tr = useUiTranslation()
  const extensions = useExtensionsStore((state) => state.extensions)
  const activeContainer = useAppStore((state) => state.activeExtensionContainer)
  const setActiveContainer = useAppStore((state) => state.setActiveExtensionContainer)
  const tabs = useTabsStore((state) => state.tabs)
  const activeTabId = useTabsStore((state) => state.activeTabId)
  const tabRefs = useRef(new Map<string, HTMLButtonElement>())
  const activeResponse = tabs.find((tab) => tab.id === activeTabId)?.response

  const context = useMemo<ExtensionContextValues>(() => {
    const values: ExtensionContextValues = {
      activeTool: 'extensionviews',
      hasResponse: Boolean(activeResponse),
      'response.status': activeResponse?.status,
      'response.contentType': activeResponse?.contentType,
    }
    for (const extension of extensions) values[`extension.${extension.manifest.id}.enabled`] = extension.enabled
    return values
  }, [activeResponse, extensions])
  const containers = useMemo(() => collectExtensionViewContainers(extensions, context), [context, extensions])
  const selected = containers.find((container) => container.id === activeContainer) ?? containers[0]

  useEffect(() => {
    if (selected && selected.id !== activeContainer) setActiveContainer(selected.id)
  }, [activeContainer, selected, setActiveContainer])

  if (!selected) {
    return <div className="flex flex-1 items-center justify-center p-8">
      <div className="max-w-sm rounded-xl border border-dashed border-border-2 bg-surface-1/40 p-6 text-center">
        <Boxes size={24} className="mx-auto mb-3 text-text-4" />
        <h2 className="text-sm font-semibold text-text-1">{tr('No extension views')}</h2>
        <p className="mt-1 text-xs leading-5 text-text-3">{tr('Enable an extension that contributes a view to show it here.')}</p>
      </div>
    </div>
  }

  const selectAt = (index: number) => {
    const next = containers[(index + containers.length) % containers.length]
    if (!next) return
    setActiveContainer(next.id)
    requestAnimationFrame(() => tabRefs.current.get(next.id)?.focus())
  }

  return <div className="flex min-h-0 flex-1 flex-col">
    <div
      role="tablist"
      aria-label={tr('Extension view containers')}
      className="flex shrink-0 items-center gap-1 overflow-x-auto border-b border-border-1 bg-surface-1/50 px-3 py-2"
      onKeyDown={(event) => {
        const index = containers.findIndex((container) => container.id === selected.id)
        if (event.key === 'ArrowRight') { event.preventDefault(); selectAt(index + 1) }
        else if (event.key === 'ArrowLeft') { event.preventDefault(); selectAt(index - 1) }
        else if (event.key === 'Home') { event.preventDefault(); selectAt(0) }
        else if (event.key === 'End') { event.preventDefault(); selectAt(containers.length - 1) }
      }}
    >
      {containers.map((container) => <button
        key={container.id}
        ref={(element) => { if (element) tabRefs.current.set(container.id, element); else tabRefs.current.delete(container.id) }}
        type="button"
        role="tab"
        aria-selected={container.id === selected.id}
        tabIndex={container.id === selected.id ? 0 : -1}
        onClick={() => setActiveContainer(container.id)}
        className={cn(
          'flex h-7 shrink-0 items-center gap-1.5 rounded-md border px-2.5 text-[10px] font-medium transition-colors',
          container.id === selected.id
            ? 'border-accent/40 bg-accent/10 text-accent'
            : 'border-transparent text-text-3 hover:border-border-2 hover:bg-surface-2 hover:text-text-1',
        )}
      ><Puzzle size={11} />{container.title}</button>)}
    </div>
    <div role="tabpanel" aria-label={selected.title} className="min-h-0 flex-1 overflow-auto p-4">
      <div className="mx-auto max-w-5xl space-y-3">
        {selected.views?.map((view) => view.renderer === 'webview'
          ? <ExtensionWebview key={view.id} extensionId={selected.extensionId} viewId={view.id} name={view.name} />
          : <ExtensionDeclarativeView key={view.id} extensionId={selected.extensionId} viewId={view.id} name={view.name} />)}
      </div>
    </div>
  </div>
}

function humanizeContainer(value: string): string {
  const tail = value.split('.').pop() ?? value
  return tail.replace(/[-_]+/g, ' ').replace(/\b\w/g, (letter) => letter.toUpperCase())
}
