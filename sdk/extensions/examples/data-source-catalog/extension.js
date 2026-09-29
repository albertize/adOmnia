export async function activate(api) {
  const databases = await api.databases.listConnections()
  const brokers = await api.brokers.listConnections()
  await api.views.setState('adomnia.data-source-catalog-example.sources', {
    kind: 'list', title: 'Local data sources',
    items: [
      ...databases.map((item) => ({ id: `db:${item.id}`, title: item.name, description: item.driver, badge: 'database' })),
      ...brokers.map((item) => ({ id: `broker:${item.id}`, title: item.name, description: item.protocol, badge: 'broker' })),
    ],
  })
}
