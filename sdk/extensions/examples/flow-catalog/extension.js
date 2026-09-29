export async function activate(api) {
  const render = async () => {
    const flows = await api.flows.list()
    await api.views.setState('adomnia.flow-catalog-example.flows', {
      kind: 'table', title: 'Saved API flows',
      columns: [{ key: 'name', title: 'Name' }, { key: 'updatedAt', title: 'Updated' }, { key: 'version', title: 'Version' }],
      rows: flows,
    })
    return { count: flows.length }
  }
  api.commands.registerCommand('adomnia.flow-catalog-example.refresh', render)
  await render()
}
