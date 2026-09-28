export async function activate(api) {
  let hits = []
  const render = () => api.views.setState('adomnia.mock-monitor-example.hits', {
    kind: 'table', title: 'Recent mock hits',
    columns: [{ key: 'method', title: 'Method' }, { key: 'status', title: 'Status' }, { key: 'path', title: 'Path' }, { key: 'matched', title: 'Matched' }],
    rows: hits.slice(-100),
  })
  api.events.onMockHit(async (hit) => { hits = [...hits, hit].slice(-100); await render() })
  const snapshot = await api.mock.getSnapshot()
  hits = snapshot.hits || []
  await render()
}
