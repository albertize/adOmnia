export async function activate(api) {
  api.commands.registerCommand('adomnia.flow-runner-example.run', async ({ flowId, startNodeId } = {}) => {
    if (!flowId) throw new Error('flowId is required')
    return api.flows.execute(flowId, { startNodeId })
  })
  api.commands.registerCommand('adomnia.flow-runner-example.stress', async ({ flowId } = {}) => {
    if (!flowId) throw new Error('flowId is required')
    return api.flows.executeStress(flowId, { vus: 1, rampUpS: 0, mode: 'iterations', iterations: 10, durationS: 1, thinkTimeMs: 0 })
  })
  api.events.onFlowProgress(async ({ progress, entriesCompleted }) => {
    await api.views.setState('adomnia.flow-runner-example.results', {
      kind: 'details', title: 'Flow job running',
      data: { entriesCompleted: entriesCompleted || progress?.iterationsDone || 0, activeUsers: progress?.activeVus || 0 },
    })
  })
  api.events.onFlowComplete(async ({ jobId, success, result, error }) => {
    await api.views.setState('adomnia.flow-runner-example.results', {
      kind: 'details', title: `Flow job ${jobId}`,
      data: success ? { status: 'completed', steps: result?.entries?.length || 0 } : { status: 'failed', error },
    })
  })
  await api.views.setState('adomnia.flow-runner-example.results', { kind: 'empty', message: 'Run a saved flow command to populate this view.' })
}
