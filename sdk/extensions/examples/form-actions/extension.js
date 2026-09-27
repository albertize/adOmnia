export async function activate(api) {
  api.commands.registerCommand('adomnia.form-actions-example.submit', async (values) => {
    api.logging.info('Form submitted', { values })
    return { accepted: true, values }
  })
  await api.views.setState('adomnia.form-actions-example.form', {
    kind: 'form',
    title: 'Request metadata',
    fields: [
      { id: 'name', label: 'Name', type: 'text', placeholder: 'Example' },
      { id: 'priority', label: 'Priority', type: 'select', value: 'normal', options: ['low', 'normal', 'high'] },
      { id: 'enabled', label: 'Enabled', type: 'boolean', value: true }
    ],
    actions: [{ id: 'submit', title: 'Submit', command: 'adomnia.form-actions-example.submit' }]
  })
}
