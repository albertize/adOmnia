import { useEffect, useState } from 'react'
import { Braces, List, ListTree, Send, Table2 } from 'lucide-react'
import { executeExtensionCommand, openExtensionView } from '@/lib/extensions-v2-api'

interface DeclarativeViewState {
  kind: 'empty' | 'list' | 'tree' | 'table' | 'form' | 'details' | 'markdown' | 'json'
  title?: string
  message?: string
  columns?: Array<{ key: string; title: string }>
  rows?: Array<Record<string, unknown>>
  items?: Array<{ id: string; title: string; description?: string; badge?: string; parentId?: string }>
  fields?: Array<{ id: string; label: string; type: 'text' | 'number' | 'boolean' | 'select'; value?: unknown; placeholder?: string; options?: string[] }>
  actions?: Array<{ id: string; title: string; command: string }>
  data?: unknown
}

interface Props {
  extensionId: string
  viewId: string
  name: string
}

function parseState(value: string): DeclarativeViewState | null {
  if (!value) return null
  try { return JSON.parse(value) as DeclarativeViewState } catch { return null }
}

export function ExtensionDeclarativeView({ extensionId, viewId, name }: Props) {
  const [state, setState] = useState<DeclarativeViewState | null>(null)
  const [formValues, setFormValues] = useState<Record<string, unknown>>({})
  const [actionResult, setActionResult] = useState('')
  const [busyAction, setBusyAction] = useState<string | null>(null)
  const [treeFocusId, setTreeFocusId] = useState<string | null>(null)

  useEffect(() => {
    let unsubscribe: (() => void) | undefined
    void openExtensionView(extensionId, viewId).then((value) => setState(parseState(value))).catch(() => setState(null))
    void import('@/wailsjs/runtime/runtime').then(({ EventsOn }) => {
      unsubscribe = EventsOn('extension:view-state', (value) => {
        const update = value as { extensionId: string; viewId: string; state: DeclarativeViewState }
        if (update.extensionId === extensionId && update.viewId === viewId) setState(update.state)
      })
    })
    return () => unsubscribe?.()
  }, [extensionId, viewId])

  useEffect(() => {
    if (state?.kind === 'tree') {
      const items = flattenTree(state.items ?? [])
      if (!items.some(({ item }) => item.id === treeFocusId)) setTreeFocusId(items[0]?.item.id ?? null)
    }
  }, [state, treeFocusId])

  useEffect(() => {
    if (state?.kind !== 'form') return
    setFormValues(Object.fromEntries((state.fields ?? []).map((field) => [field.id, field.value ?? (field.type === 'boolean' ? false : '')])))
    setActionResult('')
  }, [state])

  if (!state || state.kind === 'empty') {
    return <div className="rounded border border-dashed border-border-2 bg-surface-0 px-3 py-6 text-center text-[10px] text-text-4">{state?.message || `${name} has no data yet. Run one of its commands to populate this view.`}</div>
  }

  const title = state.title || name
  if (state.kind === 'table') {
    const columns = state.columns ?? []
    return (
      <div className="overflow-hidden rounded border border-border-1 bg-surface-0">
        <div className="flex items-center gap-2 border-b border-border-1 px-3 py-2 text-[10px] font-medium text-text-2"><Table2 size={11} className="text-accent" /> {title}</div>
        <div className="max-h-64 overflow-auto">
          <table aria-label={title} className="w-full border-collapse text-left text-[10px]">
            <thead className="sticky top-0 bg-surface-2 text-text-3"><tr>{columns.map((column) => <th key={column.key} scope="col" className="border-b border-border-1 px-2 py-1.5 font-medium">{column.title}</th>)}</tr></thead>
            <tbody>{(state.rows ?? []).map((row, index) => <tr key={index} className="border-b border-border-1/60 last:border-0">{columns.map((column) => <td key={column.key} className="max-w-64 truncate px-2 py-1.5 text-text-2">{formatValue(row[column.key])}</td>)}</tr>)}</tbody>
          </table>
        </div>
      </div>
    )
  }
  if (state.kind === 'tree') {
    const items = flattenTree(state.items ?? [])
    return (
      <div className="overflow-hidden rounded border border-border-1 bg-surface-0">
        <div className="flex items-center gap-2 border-b border-border-1 px-3 py-2 text-[10px] font-medium text-text-2"><ListTree size={11} className="text-accent" /> {title}</div>
        <div role="tree" aria-label={title} className="max-h-64 overflow-auto py-1" onKeyDown={(event) => {
          const current = items.findIndex(({ item }) => item.id === treeFocusId)
          let next = current
          if (event.key === 'ArrowDown') next = Math.min(items.length - 1, current + 1)
          else if (event.key === 'ArrowUp') next = Math.max(0, current - 1)
          else if (event.key === 'Home') next = 0
          else if (event.key === 'End') next = items.length - 1
          else return
          event.preventDefault()
          const id = items[next]?.item.id
          if (!id) return
          setTreeFocusId(id)
          requestAnimationFrame(() => document.getElementById(treeDOMId(viewId, id))?.focus())
        }}>{items.map(({ item, depth }) => (
          <div key={item.id} id={treeDOMId(viewId, item.id)} role="treeitem" aria-level={depth + 1} tabIndex={item.id === treeFocusId ? 0 : -1} onFocus={() => setTreeFocusId(item.id)} className="flex items-start gap-2 px-3 py-1.5 outline-none focus:bg-accent/10" style={{ paddingLeft: `${12 + depth * 16}px` }}>
            <span aria-hidden className="mt-1 text-[8px] text-text-4">{depth > 0 ? '└' : '•'}</span>
            <div className="min-w-0 flex-1"><p className="text-[10px] text-text-1">{item.title}</p>{item.description && <p className="text-[9px] text-text-4">{item.description}</p>}</div>
            {item.badge && <span className="rounded bg-accent/10 px-1.5 py-0.5 text-[8px] text-accent">{item.badge}</span>}
          </div>
        ))}</div>
      </div>
    )
  }
  if (state.kind === 'form') {
    return (
      <form className="overflow-hidden rounded border border-border-1 bg-surface-0" onSubmit={(event) => event.preventDefault()}>
        <div className="flex items-center gap-2 border-b border-border-1 px-3 py-2 text-[10px] font-medium text-text-2"><Braces size={11} className="text-accent" /> {title}</div>
        {state.message && <p className="px-3 pt-3 text-[10px] text-text-3">{state.message}</p>}
        <div className="grid gap-3 p-3 md:grid-cols-2">{(state.fields ?? []).map((field) => (
          <label key={field.id} className="space-y-1 text-[10px] text-text-3">
            <span>{field.label}</span>
            {field.type === 'boolean' ? (
              <input type="checkbox" checked={Boolean(formValues[field.id])} onChange={(event) => setFormValues((current) => ({ ...current, [field.id]: event.target.checked }))} className="ml-2 align-middle" />
            ) : field.type === 'select' ? (
              <select value={String(formValues[field.id] ?? '')} onChange={(event) => setFormValues((current) => ({ ...current, [field.id]: event.target.value }))} className="block w-full rounded border border-border-1 bg-surface-1 px-2 py-1.5 text-text-1 outline-none focus:border-accent">
                {(field.options ?? []).map((option) => <option key={option} value={option}>{option}</option>)}
              </select>
            ) : (
              <input type={field.type === 'number' ? 'number' : 'text'} value={String(formValues[field.id] ?? '')} placeholder={field.placeholder} onChange={(event) => setFormValues((current) => ({ ...current, [field.id]: field.type === 'number' ? Number(event.target.value) : event.target.value }))} className="block w-full rounded border border-border-1 bg-surface-1 px-2 py-1.5 text-text-1 outline-none focus:border-accent" />
            )}
          </label>
        ))}</div>
        {(state.actions ?? []).length > 0 && <div className="flex flex-wrap items-center gap-2 border-t border-border-1 px-3 py-2">{(state.actions ?? []).map((action) => (
          <button key={action.id} type="button" disabled={busyAction !== null} onClick={() => {
            setBusyAction(action.id); setActionResult('')
            void executeExtensionCommand(extensionId, action.command, formValues, 'view').then((result) => setActionResult(JSON.stringify(result.data ?? { ok: true }, null, 2))).catch((error: unknown) => setActionResult(error instanceof Error ? error.message : String(error))).finally(() => setBusyAction(null))
          }} className="flex items-center gap-1 rounded bg-accent px-2 py-1 text-[10px] text-white disabled:opacity-40"><Send size={10} /> {busyAction === action.id ? 'Running…' : action.title}</button>
        ))}</div>}
        {actionResult && <pre role="status" className="max-h-32 overflow-auto border-t border-border-1 p-3 text-[9px] text-text-2">{actionResult}</pre>}
      </form>
    )
  }
  if (state.kind === 'list') {
    return (
      <div className="overflow-hidden rounded border border-border-1 bg-surface-0">
        <div className="flex items-center gap-2 border-b border-border-1 px-3 py-2 text-[10px] font-medium text-text-2"><List size={11} className="text-accent" /> {title}</div>
        <div role="list" aria-label={title} className="divide-y divide-border-1">{(state.items ?? []).map((item) => <div key={item.id} role="listitem" className="flex items-start gap-2 px-3 py-2"><div className="min-w-0 flex-1"><p className="text-[10px] text-text-1">{item.title}</p>{item.description && <p className="mt-0.5 text-[9px] text-text-4">{item.description}</p>}</div>{item.badge && <span className="rounded bg-accent/10 px-1.5 py-0.5 text-[8px] text-accent">{item.badge}</span>}</div>)}</div>
      </div>
    )
  }
  if (state.kind === 'details') {
    const entries = state.data && typeof state.data === 'object' && !Array.isArray(state.data) ? Object.entries(state.data as Record<string, unknown>) : []
    return (
      <div className="overflow-hidden rounded border border-border-1 bg-surface-0">
        <div className="flex items-center gap-2 border-b border-border-1 px-3 py-2 text-[10px] font-medium text-text-2"><Braces size={11} className="text-accent" /> {title}</div>
        <dl className="divide-y divide-border-1">{entries.map(([key, value]) => <div key={key} className="grid grid-cols-[minmax(8rem,0.35fr)_1fr] gap-3 px-3 py-2 text-[10px]"><dt className="font-medium text-text-3">{key}</dt><dd className="min-w-0 break-words text-text-1">{formatValue(value)}</dd></div>)}</dl>
        {entries.length === 0 && <p className="px-3 py-4 text-[10px] text-text-4">{state.message ?? ''}</p>}
      </div>
    )
  }
  return (
    <div className="overflow-hidden rounded border border-border-1 bg-surface-0">
      <div className="flex items-center gap-2 border-b border-border-1 px-3 py-2 text-[10px] font-medium text-text-2"><Braces size={11} className="text-accent" /> {title}</div>
      {state.kind === 'markdown'
        ? <div className="whitespace-pre-wrap px-3 py-2 text-[10px] leading-5 text-text-2">{String(state.data ?? state.message ?? '')}</div>
        : <pre className="max-h-64 overflow-auto p-3 text-[9px] text-text-2">{JSON.stringify(state.data, null, 2)}</pre>}
    </div>
  )
}

function flattenTree(items: NonNullable<DeclarativeViewState['items']>): Array<{ item: NonNullable<DeclarativeViewState['items']>[number]; depth: number }> {
  const byParent = new Map<string, typeof items>()
  const known = new Set(items.map((item) => item.id))
  for (const item of items) {
    const parent = item.parentId && known.has(item.parentId) && item.parentId !== item.id ? item.parentId : ''
    byParent.set(parent, [...(byParent.get(parent) ?? []), item])
  }
  const result: Array<{ item: typeof items[number]; depth: number }> = []
  const visited = new Set<string>()
  const visit = (item: typeof items[number], depth: number) => {
    if (visited.has(item.id)) return
    visited.add(item.id)
    result.push({ item, depth: Math.min(depth, 20) })
    for (const child of byParent.get(item.id) ?? []) visit(child, depth + 1)
  }
  for (const item of byParent.get('') ?? []) visit(item, 0)
  for (const item of items) if (!visited.has(item.id)) visit(item, 0)
  return result
}

function treeDOMId(viewId: string, itemId: string): string {
  const safe = (value: string) => value.replace(/[^A-Za-z0-9_-]/g, '-')
  return `extension-tree-${safe(viewId)}-${safe(itemId)}`
}

function formatValue(value: unknown): string {
  if (value === null) return 'null'
  if (typeof value === 'object') return JSON.stringify(value)
  return String(value ?? '')
}
