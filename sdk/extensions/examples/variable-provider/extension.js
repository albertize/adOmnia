export function activate(api) {
  api.context.subscriptions.add(api.variables.registerProvider(
    'adomnia.variable-provider-example.workspace',
    ({ workspace, environment }) => ({
      extension_workspace_id: workspace?.id || '',
      extension_environment_id: environment?.id || '',
    }),
  ))
}
