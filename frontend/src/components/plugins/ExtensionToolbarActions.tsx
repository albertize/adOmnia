import { Braces } from 'lucide-react'
import { evaluateWhen, type ExtensionContextValues } from '@/lib/extensionContext'
import { executeExtensionCommand } from '@/lib/extensions-v2-api'
import { useExtensionsStore } from '@/stores/extensions'
import { useUiTranslation } from '@/lib/uiI18n'

interface Props {
  location: 'request/toolbar' | 'response/toolbar'
  context: ExtensionContextValues
  args?: Record<string, unknown>
}

export function ExtensionToolbarActions({ location, context, args = {} }: Props) {
  const tr = useUiTranslation()
  const extensions = useExtensionsStore((state) => state.extensions)
  const actions = extensions.flatMap((extension) => {
    if (!extension.enabled) return []
    const titles = new Map((extension.manifest.contributes?.commands ?? []).map((command) => [command.id, command.title]))
    return (extension.manifest.contributes?.menus?.[location] ?? [])
      .filter((item) => evaluateWhen(item.when, context))
      .map((item) => ({ extensionId: extension.manifest.id, command: item.command, title: titles.get(item.command) ?? item.command, group: item.group ?? '' }))
  }).sort((left, right) => left.group.localeCompare(right.group) || left.title.localeCompare(right.title))

  if (actions.length === 0) return null
  return <div className="flex min-w-0 items-center gap-1" role="group" aria-label={tr('Extension actions')}>
    {actions.map((action, index) => <button
      key={`${action.extensionId}:${action.command}:${index}`}
      type="button"
      title={action.title}
      aria-label={action.title}
      onClick={() => void executeExtensionCommand(action.extensionId, action.command, args, 'menu').catch((error: unknown) => {
        window.dispatchEvent(new CustomEvent('adomnia:extension-error', { detail: error instanceof Error ? error.message : String(error) }))
      })}
      className="flex h-[var(--ui-control-h)] max-w-40 items-center gap-1 rounded-md border border-border-1 bg-surface-2 px-2 text-[10px] text-text-2 hover:border-accent/40 hover:text-text-1"
    ><Braces size={11} className="shrink-0 text-accent" /><span className="truncate">{action.title}</span></button>)}
  </div>
}
