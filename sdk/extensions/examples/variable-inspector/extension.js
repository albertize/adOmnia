export function activate(api) {
  const render = async () => {
    const variables = await api.variables.getAll()
    await api.views.setState('adomnia.variable-inspector-example.variables', {
      kind: 'table',
      title: 'Active variables',
      columns: [{ key: 'key', title: 'Name' }, { key: 'value', title: 'Value' }],
      rows: Object.entries(variables).map(([key, value]) => ({ key, value })),
    })
    return { count: Object.keys(variables).length }
  }
  api.commands.registerCommand('adomnia.variable-inspector-example.inspect', render)
  return render()
}
