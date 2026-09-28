export async function activate(api) {
  let entries = []
  const render = () => api.views.setState('adomnia.proxy-monitor-example.traffic', {
    kind: 'table', title: 'Recent proxy traffic',
    columns: [{ key: 'method', title: 'Method' }, { key: 'status', title: 'Status' }, { key: 'durationMs', title: 'ms' }, { key: 'url', title: 'URL' }],
    rows: entries.slice(-100),
  })
  api.events.onProxyTraffic(async (entry) => { entries = [...entries, entry].slice(-100); await render() })
  const snapshot = await api.proxy.getSnapshot()
  entries = snapshot.entries || []
  await render()
}
