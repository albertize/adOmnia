export async function activate(api) {
  let entries = []
  const render = () => api.views.setState('adomnia.browser-network-example.traffic', {
    kind: 'table',
    title: 'Recent browser traffic',
    columns: [{ key: 'method', title: 'Method' }, { key: 'status', title: 'Status' }, { key: 'url', title: 'URL' }],
    rows: entries.slice(-100),
  })
  api.events.onBrowserNetwork(async (entry) => {
    entries = [...entries, entry].slice(-100)
    await render()
  })
  api.commands.registerCommand('adomnia.browser-network-example.clear', async () => {
    await api.browserDebug.clear()
    entries = []
    await render()
  })
  const snapshot = await api.browserDebug.list()
  entries = [...snapshot].slice(-100)
  await render()
}
