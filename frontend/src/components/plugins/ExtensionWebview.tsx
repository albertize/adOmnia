import { useEffect, useMemo, useRef, useState } from 'react'
import { AlertTriangle, Loader2 } from 'lucide-react'
import { executeExtensionCommand, getExtensionWebviewHTML } from '@/lib/extensions-v2-api'
import { useSettingsStore } from '@/stores/settings'

export const EXTENSION_WEBVIEW_SANDBOX = 'allow-scripts'
export const EXTENSION_WEBVIEW_PERMISSIONS = "clipboard-read 'none'; clipboard-write 'none'; camera 'none'; microphone 'none'; geolocation 'none'"
export const EXTENSION_WEBVIEW_MAX_MESSAGE_BYTES = 256 * 1024

interface Props {
  extensionId: string
  viewId: string
  name: string
}

export function ExtensionWebview({ extensionId, viewId, name }: Props) {
  const frameRef = useRef<HTMLIFrameElement>(null)
  const [html, setHTML] = useState('')
  const [error, setError] = useState('')
  const token = useMemo(() => crypto.randomUUID(), [extensionId, viewId])
  const appearance = useSettingsStore((state) => state.settings.appearance)

  useEffect(() => {
    let cancelled = false
    setHTML('')
    setError('')
    void getExtensionWebviewHTML(extensionId, viewId).then((extensionDocument) => {
      if (cancelled) return
      const tokenNames = ['--color-surface-0', '--color-surface-1', '--color-surface-2', '--color-text-1', '--color-text-2', '--color-text-3', '--color-border-1', '--color-border-2', '--color-accent', '--color-error', '--color-warning', '--color-success', '--font-ui', '--font-mono', '--radius-sm', '--radius-md']
      const computed = getComputedStyle(document.documentElement)
      const declarations = tokenNames.map((name) => `${name}:${computed.getPropertyValue(name).trim().replace(/</g, '')}`).join(';')
      const bootstrap = `<style>:root{${declarations};color-scheme:${appearance.theme === 'light' ? 'light' : 'dark'}}html,body{background:var(--color-surface-0);color:var(--color-text-1);font-family:var(--font-ui)}</style><script>(function(){const token=${JSON.stringify(token)};const pending=new Map();let seq=0;window.adomnia=Object.freeze({executeCommand(command,args={}){return new Promise((resolve,reject)=>{const requestId='wv-'+(++seq);pending.set(requestId,{resolve,reject});parent.postMessage({__adomniaExtension:true,token,requestId,type:'executeCommand',command,args},'*')})}});addEventListener('message',(event)=>{const value=event.data;if(!value||value.__adomniaHost!==true||value.token!==token)return;const waiter=pending.get(value.requestId);if(!waiter)return;pending.delete(value.requestId);value.error?waiter.reject(new Error(value.error)):waiter.resolve(value.result)})})();</script>`
      const body = /<body(?:\s[^>]*)?>/i
      setHTML(body.test(extensionDocument) ? extensionDocument.replace(body, (match) => match + bootstrap) : bootstrap + extensionDocument)
    }).catch((reason: unknown) => {
      if (!cancelled) setError(reason instanceof Error ? reason.message : String(reason))
    })
    return () => { cancelled = true }
  }, [appearance.theme, appearance.themeId, extensionId, token, viewId])

  useEffect(() => {
    const receive = (event: MessageEvent) => {
      if (event.source !== frameRef.current?.contentWindow) return
      const value = event.data as { __adomniaExtension?: boolean; token?: string; requestId?: string; type?: string; command?: string; args?: Record<string, unknown> }
      if (!value?.__adomniaExtension || value.token !== token || value.type !== 'executeCommand' || !value.requestId || value.requestId.length > 100 || !value.command || value.command.length > 200) return
      let messageSize = 0
      try { messageSize = JSON.stringify(value.args ?? {}).length } catch { return }
      if (messageSize > EXTENSION_WEBVIEW_MAX_MESSAGE_BYTES) return
      void executeExtensionCommand(extensionId, value.command, value.args ?? {}, 'webview').then((execution) => {
        frameRef.current?.contentWindow?.postMessage({ __adomniaHost: true, token, requestId: value.requestId, result: execution.data }, '*')
      }).catch((reason: unknown) => {
        frameRef.current?.contentWindow?.postMessage({ __adomniaHost: true, token, requestId: value.requestId, error: reason instanceof Error ? reason.message : String(reason) }, '*')
      })
    }
    window.addEventListener('message', receive)
    return () => window.removeEventListener('message', receive)
  }, [extensionId, token])

  if (error) return <div className="flex items-center gap-2 rounded border border-error/30 bg-error/10 px-3 py-2 text-[10px] text-error"><AlertTriangle size={11} /> {error}</div>
  if (!html) return <div className="flex h-24 items-center justify-center gap-2 rounded border border-border-1 bg-surface-0 text-[10px] text-text-4"><Loader2 size={11} className="animate-spin" /> Loading {name}…</div>
  return (
    <iframe
      ref={frameRef}
      title={`${name} extension webview`}
      srcDoc={html}
      sandbox={EXTENSION_WEBVIEW_SANDBOX}
      allow={EXTENSION_WEBVIEW_PERMISSIONS}
      referrerPolicy="no-referrer"
      loading="lazy"
      className="h-72 w-full rounded border border-border-1 bg-surface-0"
    />
  )
}
