import { describe, expect, it } from 'vitest'
import { renderToString } from 'react-dom/server'
import { ExtensionDeclarativeView, type DeclarativeViewState } from './ExtensionDeclarativeView'

const table: DeclarativeViewState = {
  kind: 'table',
  title: '1,000 rows',
  columns: [
    { key: 'id', title: 'ID' },
    { key: 'status', title: 'Status' },
    { key: 'duration', title: 'Duration' },
  ],
  rows: Array.from({ length: 1000 }, (_, index) => ({ id: index, status: 'ok', duration: index % 100 })),
}

describe('extension declarative view render performance', () => {
  it('server-renders a 1,000-row table model', () => {
    const samples: number[] = []
    let html = ''
    for (let iteration = 0; iteration < 20; iteration++) {
      const started = performance.now()
      html = renderToString(<ExtensionDeclarativeView extensionId="benchmark.extension" viewId="benchmark.extension.table" name="Benchmark" initialState={table} />)
      samples.push(performance.now() - started)
    }
    samples.sort((left, right) => left - right)
    const p95 = samples[Math.ceil(samples.length * 0.95) - 1] ?? 0
    console.info(`[extension benchmark] declarative table SSR p95=${p95.toFixed(2)}ms samples=${samples.length}`)
    expect(html).toContain('1,000 rows')
    expect(html).toContain('<table')
  })
})
