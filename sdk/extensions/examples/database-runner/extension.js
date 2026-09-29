export async function activate(api) {
  api.commands.registerCommand('adomnia.database-runner-example.query', async ({ connectionId, query } = {}) => {
    if (!connectionId || !query) throw new Error('connectionId and query are required')
    return api.databases.execute(connectionId, query, { limit: 200, timeoutMs: 30000 })
  })
  api.events.onDatabaseComplete(async ({ jobId, success, result, error }) => {
    await api.views.setState('adomnia.database-runner-example.results', {
      kind: 'details', title: `Database job ${jobId}`,
      data: success
        ? { status: 'completed', rows: result?.rows?.length || 0, durationMs: result?.durationMs || 0 }
        : { status: 'failed', error },
    })
  })
  await api.views.setState('adomnia.database-runner-example.results', {
    kind: 'empty', message: 'Run the database command with an existing connection ID and query.'
  })
}
