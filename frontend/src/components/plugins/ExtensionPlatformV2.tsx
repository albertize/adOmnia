import { useEffect, useMemo, useState } from 'react'
import { AlertTriangle, Check, ChevronDown, ChevronRight, Code2, Download, FolderPlus, Loader2, PackageOpen, Play, RefreshCw, ShieldCheck, Trash2 } from 'lucide-react'
import { cn } from '@/lib/utils'
import { exportExtensionSDK, getExtensionDiagnostics, getExtensionLogs, selectExtensionArchive, selectExtensionDirectory, setExtensionSetting } from '@/lib/extensions-v2-api'
import { useExtensionsStore } from '@/stores/extensions'
import { ExtensionDeclarativeView } from './ExtensionDeclarativeView'
import { ExtensionWebview } from './ExtensionWebview'

export function ExtensionPlatformV2() {
  const { extensions, loading, error, load, installDirectory, installArchive, reloadSource, setGrants, enable, disable, uninstall, execute } = useExtensionsStore()
  const [expanded, setExpanded] = useState<string | null>(null)
  const [busy, setBusy] = useState<string | null>(null)
  const [result, setResult] = useState<Record<string, string>>({})
  const [logs, setLogs] = useState<Record<string, string>>({})
  const [diagnostics, setDiagnostics] = useState<Record<string, Array<{ severity: string; message: string; resource?: string; line?: number }>>>({})
  const [grantDraft, setGrantDraft] = useState<Record<string, string[]>>({})
  const [confirmDelete, setConfirmDelete] = useState<string | null>(null)
  const [sdkMessage, setSDKMessage] = useState<string | null>(null)

  useEffect(() => { void load() }, [load])
  useEffect(() => {
    setGrantDraft(Object.fromEntries(extensions.map((extension) => [extension.manifest.id, extension.grants])))
  }, [extensions])

  const activeCount = useMemo(() => extensions.filter((extension) => extension.active).length, [extensions])

  const run = async (key: string, operation: () => Promise<unknown>) => {
    setBusy(key)
    try { await operation() } finally { setBusy(null) }
  }

  const install = async (development: boolean) => {
    const path = await selectExtensionDirectory()
    if (!path) return
    await run('install', () => installDirectory(path, development))
  }

  const installPackage = async () => {
    const path = await selectExtensionArchive()
    if (!path) return
    await run('install', () => installArchive(path))
  }

  const exportSDK = async () => {
    const path = await selectExtensionDirectory()
    if (!path) return
    await run('sdk', async () => setSDKMessage(`SDK exported to ${await exportExtensionSDK(path)}`))
  }

  return (
    <section className="mb-6 overflow-hidden rounded-lg border border-accent/25 bg-surface-1">
      <header className="flex items-center justify-between gap-3 border-b border-border-1 bg-accent/[0.04] px-4 py-3">
        <div>
          <div className="flex items-center gap-2">
            <ShieldCheck size={15} className="text-accent" />
            <h2 className="text-sm font-semibold text-text-1">Extension Platform v2</h2>
            <span className="rounded bg-accent/10 px-1.5 py-0.5 text-[9px] font-medium uppercase tracking-wide text-accent">Preview</span>
          </div>
          <p className="mt-1 text-[11px] text-text-3">Isolated extension host, explicit permissions, commands and local packages.</p>
        </div>
        <div className="flex items-center gap-2">
          <span className="text-[10px] text-text-4">{extensions.length} installed · {activeCount} active</span>
          <button type="button" title="Reload extensions" onClick={() => void load()} className="rounded p-1.5 text-text-3 hover:bg-surface-2 hover:text-text-1"><RefreshCw size={13} /></button>
          <button type="button" title="Export version-matched authoring SDK" disabled={busy === 'sdk'} onClick={() => void exportSDK()} className="rounded p-1.5 text-text-3 hover:bg-surface-2 hover:text-text-1 disabled:opacity-40"><Download size={13} /></button>
          <button type="button" disabled={busy === 'install'} onClick={() => void installPackage()} className="flex items-center gap-1.5 rounded-md bg-accent px-2.5 py-1.5 text-[11px] font-medium text-white disabled:opacity-50"><PackageOpen size={12} /> Install package</button>
          <button type="button" disabled={busy === 'install'} onClick={() => void install(false)} className="flex items-center gap-1.5 rounded-md border border-border-2 bg-surface-1 px-2.5 py-1.5 text-[11px] text-text-2 hover:bg-surface-2 disabled:opacity-50"><FolderPlus size={12} /> Install folder</button>
          <button type="button" disabled={busy === 'install'} onClick={() => void install(true)} className="flex items-center gap-1.5 rounded-md border border-border-2 bg-surface-1 px-2.5 py-1.5 text-[11px] text-text-2 hover:bg-surface-2 disabled:opacity-50"><Code2 size={12} /> Link development</button>
        </div>
      </header>

      {error && <div className="flex items-center gap-2 border-b border-error/25 bg-error/10 px-4 py-2 text-[11px] text-error"><AlertTriangle size={12} /> {error}</div>}
      {sdkMessage && <div className="border-b border-success/25 bg-success/10 px-4 py-2 text-[11px] text-success">{sdkMessage}</div>}

      <div className="p-3">
        {loading && extensions.length === 0 ? (
          <div className="flex h-20 items-center justify-center gap-2 text-xs text-text-3"><Loader2 size={13} className="animate-spin" /> Loading extensions…</div>
        ) : extensions.length === 0 ? (
          <div className="flex h-24 flex-col items-center justify-center text-center">
            <ShieldCheck size={22} className="mb-2 text-text-4" />
            <p className="text-xs text-text-2">No v2 extensions installed.</p>
            <p className="mt-1 text-[10px] text-text-4">Install a validated folder generated with the local authoring SDK.</p>
          </div>
        ) : (
          <div className="space-y-2">
            {extensions.map((extension) => {
              const id = extension.manifest.id
              const open = expanded === id
              const requested = extension.manifest.permissions ?? []
              const draft = grantDraft[id] ?? []
              const reviewed = requested.every((permission) => extension.grants.includes(permission))
              return (
                <article key={id} className={cn('rounded-md border bg-surface-0', extension.error ? 'border-error/35' : 'border-border-1')}>
                  <div className="flex items-center gap-3 px-3 py-2.5">
                    <button type="button" onClick={() => setExpanded(open ? null : id)} className="rounded p-0.5 text-text-4 hover:bg-surface-2 hover:text-text-2">{open ? <ChevronDown size={13} /> : <ChevronRight size={13} />}</button>
                    <div className="grid h-8 w-8 place-items-center rounded bg-surface-2 text-[10px] font-bold text-accent">{extension.manifest.name.slice(0, 2).toUpperCase()}</div>
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center gap-2"><span className="truncate text-xs font-medium text-text-1">{extension.manifest.name}</span><span className="text-[9px] text-text-4">v{extension.manifest.version}</span></div>
                      <div className="truncate text-[10px] text-text-4">{id} · {extension.installKind}{extension.active ? ' · active' : extension.enabled ? ' · lazy' : ''}</div>
                    </div>
                    {!reviewed && <span className="rounded bg-warning/10 px-2 py-0.5 text-[9px] text-warning">Permission review</span>}
                    {extension.error && <span title={extension.error} className="max-w-40 truncate rounded bg-error/10 px-2 py-0.5 text-[9px] text-error">{extension.quarantined ? 'Quarantined' : 'Failed'}</span>}
                    <button type="button" title="Reload from source and disable for review" disabled={busy === `${id}:reload`} onClick={() => void run(`${id}:reload`, () => reloadSource(id))} className="rounded p-1 text-text-4 hover:bg-surface-2 hover:text-text-2 disabled:opacity-40"><RefreshCw size={11} /></button>
                    <button
                      type="button"
                      disabled={busy === id || (!extension.enabled && !reviewed)}
                      onClick={() => void run(id, () => extension.enabled ? disable(id) : enable(id))}
                      className={cn('relative h-4 w-8 rounded-full transition-colors disabled:opacity-40', extension.enabled ? 'bg-accent' : 'bg-surface-3')}
                      aria-label={`${extension.enabled ? 'Disable' : 'Enable'} ${extension.manifest.name}`}
                    ><span className={cn('absolute top-0.5 h-3 w-3 rounded-full bg-white transition-all', extension.enabled ? 'left-[18px]' : 'left-0.5')} /></button>
                  </div>

                  {open && (
                    <div className="space-y-4 border-t border-border-1 px-4 py-3">
                      {extension.manifest.description && <p className="text-[11px] text-text-3">{extension.manifest.description}</p>}
                      <div>
                        <h3 className="mb-2 text-[9px] font-semibold uppercase tracking-wider text-text-4">Permission review</h3>
                        {requested.length === 0 ? <p className="text-[10px] text-text-4">This extension requests no privileged capabilities.</p> : (
                          <div className="space-y-1.5">
                            {requested.map((permission) => (
                              <label key={permission} className="flex items-center gap-2 text-[10px] text-text-2">
                                <input
                                  type="checkbox"
                                  checked={draft.includes(permission)}
                                  onChange={(event) => setGrantDraft((current) => ({ ...current, [id]: event.target.checked ? [...draft, permission] : draft.filter((item) => item !== permission) }))}
                                />
                                <code className="rounded bg-surface-2 px-1.5 py-0.5 text-accent">{permission}</code>
                              </label>
                            ))}
                            <button type="button" onClick={() => void run(`${id}:grants`, () => setGrants(id, draft))} className="mt-1 flex items-center gap-1 rounded border border-border-2 px-2 py-1 text-[10px] text-text-2 hover:bg-surface-2"><Check size={10} /> Save permission decision</button>
                          </div>
                        )}
                      </div>

                      {Object.entries(extension.manifest.contributes?.configuration ?? {}).length > 0 && (
                        <div>
                          <h3 className="mb-2 text-[9px] font-semibold uppercase tracking-wider text-text-4">Settings</h3>
                          <div className="grid gap-2 md:grid-cols-2">
                            {Object.entries(extension.manifest.contributes?.configuration ?? {}).map(([key, property]) => property ? (
                              <label key={key} className="space-y-1 text-[10px] text-text-3">
                                <span>{key}</span>
                                {property.enum || property.type === 'boolean' ? (
                                  <select
                                    className="w-full rounded border border-border-1 bg-surface-1 px-2 py-1.5 text-text-1 outline-none focus:border-accent"
                                    value={String(extension.settings[key] ?? property.default ?? '')}
                                    onChange={(event) => {
                                      const raw = event.target.value
                                      const options = property.enum ?? [true, false]
                                      const value = options.find((option) => String(option) === raw) ?? raw
                                      void setExtensionSetting(id, key, value).then(() => load())
                                    }}
                                  >{(property.enum ?? [true, false]).map((option) => <option key={String(option)} value={String(option)}>{String(option)}</option>)}</select>
                                ) : (
                                  <input
                                    type={property.type === 'number' || property.type === 'integer' ? 'number' : 'text'}
                                    min={property.minimum ?? undefined}
                                    max={property.maximum ?? undefined}
                                    step={property.type === 'integer' ? 1 : undefined}
                                    className="w-full rounded border border-border-1 bg-surface-1 px-2 py-1.5 text-text-1 outline-none focus:border-accent"
                                    value={String(extension.settings[key] ?? property.default ?? '')}
                                    onChange={(event) => {
                                      const raw = event.target.value
                                      const value = property.type === 'number' || property.type === 'integer' ? Number(raw) : raw
                                      void setExtensionSetting(id, key, value).then(() => load())
                                    }}
                                  />
                                )}
                              </label>
                            ) : null)}
                          </div>
                        </div>
                      )}

                      {(extension.manifest.contributes?.views ?? []).filter((view) => view.renderer === 'declarative').length > 0 && (
                        <div>
                          <h3 className="mb-2 text-[9px] font-semibold uppercase tracking-wider text-text-4">Views</h3>
                          <div className="space-y-2">
                            {(extension.manifest.contributes?.views ?? []).filter((view) => view.renderer === 'declarative').map((view) => (
                              <ExtensionDeclarativeView key={view.id} extensionId={id} viewId={view.id} name={view.name} />
                            ))}
                          </div>
                        </div>
                      )}

                      {(extension.manifest.contributes?.views ?? []).filter((view) => view.renderer === 'webview').length > 0 && (
                        <div>
                          <h3 className="mb-2 text-[9px] font-semibold uppercase tracking-wider text-text-4">Isolated webviews</h3>
                          <div className="space-y-2">
                            {(extension.manifest.contributes?.views ?? []).filter((view) => view.renderer === 'webview').map((view) => (
                              <ExtensionWebview key={view.id} extensionId={id} viewId={view.id} name={view.name} />
                            ))}
                          </div>
                        </div>
                      )}

                      {(extension.manifest.contributes?.commands ?? []).length > 0 && (
                        <div>
                          <h3 className="mb-2 text-[9px] font-semibold uppercase tracking-wider text-text-4">Commands</h3>
                          <div className="space-y-1.5">
                            {(extension.manifest.contributes?.commands ?? []).map((command) => (
                              <div key={command.id} className="rounded border border-border-1 bg-surface-1 p-2">
                                <div className="flex items-center justify-between gap-2">
                                  <div><p className="text-[11px] text-text-1">{command.title}</p><code className="text-[9px] text-text-4">{command.id}</code></div>
                                  <button type="button" disabled={!extension.enabled || busy === command.id} onClick={() => void run(command.id, async () => {
                                    const response = await execute(id, command.id, 'extensionManager')
                                    setResult((current) => ({ ...current, [command.id]: JSON.stringify(response.data ?? { ok: true }, null, 2) }))
                                  })} className="flex items-center gap-1 rounded bg-accent px-2 py-1 text-[10px] text-white disabled:opacity-40">{busy === command.id ? <Loader2 size={10} className="animate-spin" /> : <Play size={10} />} Run</button>
                                </div>
                                {result[command.id] && <pre className="mt-2 max-h-32 overflow-auto rounded bg-surface-0 p-2 text-[9px] text-text-2">{result[command.id]}</pre>}
                              </div>
                            ))}
                          </div>
                        </div>
                      )}

                      <div>
                        <div className="mb-2 flex items-center justify-between">
                          <h3 className="text-[9px] font-semibold uppercase tracking-wider text-text-4">Output</h3>
                          <button type="button" onClick={() => void Promise.all([getExtensionLogs(id), getExtensionDiagnostics(id)]).then(([entries, problems]) => {
                            setLogs((current) => ({ ...current, [id]: entries.map((entry) => `${entry.timestamp} [${entry.level}] ${entry.message}${entry.fields ? ` ${JSON.stringify(entry.fields)}` : ''}`).join('\n') || 'No output yet.' }))
                            setDiagnostics((current) => ({ ...current, [id]: problems }))
                          })} className="text-[9px] text-accent hover:underline">Refresh output</button>
                        </div>
                        {diagnostics[id]?.length > 0 && <div className="mb-2 space-y-1">{diagnostics[id].map((entry, index) => <div key={`${entry.message}:${index}`} className={cn('rounded border px-2 py-1 text-[9px]', entry.severity === 'error' ? 'border-error/30 text-error' : entry.severity === 'warning' ? 'border-warning/30 text-warning' : 'border-border-1 text-text-3')}>{entry.message}{entry.resource ? ` · ${entry.resource}${entry.line ? `:${entry.line}` : ''}` : ''}</div>)}</div>}
                        {logs[id] && <pre className="max-h-36 overflow-auto rounded border border-border-1 bg-surface-1 p-2 text-[9px] text-text-3">{logs[id]}</pre>}
                      </div>

                      <div className="flex items-center justify-between border-t border-border-1 pt-3 text-[9px] text-text-4">
                        <span className="truncate" title={extension.installDir}>{extension.installDir}</span>
                        {confirmDelete === id ? (
                          <span className="flex items-center gap-2"><button type="button" onClick={() => setConfirmDelete(null)} className="text-text-3">Cancel</button><button type="button" onClick={() => void run(`${id}:delete`, () => uninstall(id))} className="text-error">Confirm uninstall</button></span>
                        ) : <button type="button" onClick={() => setConfirmDelete(id)} className="flex items-center gap-1 text-text-3 hover:text-error"><Trash2 size={10} /> Uninstall</button>}
                      </div>
                    </div>
                  )}
                </article>
              )
            })}
          </div>
        )}
      </div>
    </section>
  )
}
