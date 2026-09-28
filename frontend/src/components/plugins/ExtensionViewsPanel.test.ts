import { describe, expect, it } from 'vitest'
import { collectExtensionViewContainers } from './ExtensionViewsPanel'
import type { ExtensionInstance } from '@/lib/extensions-v2-api'

function installed(id: string, enabled: boolean, views: Array<Record<string, string>>): ExtensionInstance {
  return {
    enabled,
    manifest: { id, name: id, contributes: { views } },
  } as unknown as ExtensionInstance
}

describe('collectExtensionViewContainers', () => {
  it('groups enabled visible non-response views without merging extensions', () => {
    const containers = collectExtensionViewContainers([
      installed('acme.one', true, [
        { id: 'acme.one.main', name: 'Main', container: 'extensions', renderer: 'declarative' },
        { id: 'acme.one.hidden', name: 'Hidden', container: 'analysis', renderer: 'declarative', when: 'hasResponse' },
        { id: 'acme.one.response', name: 'Response', container: 'response', renderer: 'declarative' },
      ]),
      installed('acme.two', true, [{ id: 'acme.two.main', name: 'Main', container: 'extensions', renderer: 'webview' }]),
      installed('acme.disabled', false, [{ id: 'acme.disabled.main', name: 'Main', container: 'extensions', renderer: 'declarative' }]),
    ], { hasResponse: false })

    expect(containers.map((container) => container.id)).toEqual(['acme.one:extensions', 'acme.two:extensions'])
    expect(containers[0]?.views).toHaveLength(1)
  })
})
