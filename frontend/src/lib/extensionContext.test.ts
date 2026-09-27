import { describe, expect, it } from 'vitest'
import { evaluateWhen } from './extensionContext'

const context = {
  activeTool: 'collections',
  hasResponse: true,
  'response.status': 200,
  'extension.local.demo.enabled': true,
}

describe('evaluateWhen', () => {
  it('supports equality, boolean keys and conjunctions', () => {
    expect(evaluateWhen("activeTool == 'collections' && hasResponse", context)).toBe(true)
    expect(evaluateWhen("activeTool == 'proxy' && hasResponse", context)).toBe(false)
  })

  it('supports negation, inequality and disjunctions', () => {
    expect(evaluateWhen("!hasResponse || response.status != 500", context)).toBe(true)
    expect(evaluateWhen("!extension.local.demo.enabled", context)).toBe(false)
  })

  it('rejects oversized or control-character expressions', () => {
    expect(evaluateWhen('a'.repeat(1001), context)).toBe(false)
    expect(evaluateWhen("activeTool == 'collections'\n|| hasResponse", context)).toBe(false)
  })

  it('does not execute JavaScript expressions', () => {
    expect(evaluateWhen('globalThis.alert()', context)).toBe(false)
  })
})
