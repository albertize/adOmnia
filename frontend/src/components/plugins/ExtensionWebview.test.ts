import { describe, expect, it } from 'vitest'
import { EXTENSION_WEBVIEW_MAX_MESSAGE_BYTES, EXTENSION_WEBVIEW_PERMISSIONS, EXTENSION_WEBVIEW_SANDBOX } from './ExtensionWebview'

describe('extension webview security policy', () => {
  it('allows scripts without origin, navigation, form, popup, or download privileges', () => {
    expect(EXTENSION_WEBVIEW_SANDBOX).toBe('allow-scripts')
    for (const capability of ['allow-same-origin', 'allow-top-navigation', 'allow-forms', 'allow-popups', 'allow-downloads']) {
      expect(EXTENSION_WEBVIEW_SANDBOX).not.toContain(capability)
    }
  })

  it('denies clipboard and device capabilities and bounds bridge messages', () => {
    for (const capability of ['clipboard-read', 'clipboard-write', 'camera', 'microphone', 'geolocation']) {
      expect(EXTENSION_WEBVIEW_PERMISSIONS).toContain(`${capability} 'none'`)
    }
    expect(EXTENSION_WEBVIEW_MAX_MESSAGE_BYTES).toBe(262144)
  })
})
