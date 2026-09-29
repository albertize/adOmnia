export async function activate(api) {
  api.commands.registerCommand('adomnia.broker-publisher-example.publish', async ({ connectionId, destination, message } = {}) => {
    if (!connectionId || !destination || typeof message !== 'string') throw new Error('connectionId, destination and message are required')
    return api.brokers.publish(connectionId, destination, message, { contentType: 'application/json' })
  })
  api.events.onBrokerPublishComplete(async ({ jobId, success, result, error }) => {
    await api.views.setState('adomnia.broker-publisher-example.results', {
      kind: 'details', title: `Broker publish ${jobId}`,
      data: success ? { status: 'published', destination: result?.topic || result?.subject || result?.channel || result?.routingKey } : { status: 'failed', error },
    })
  })
  await api.views.setState('adomnia.broker-publisher-example.results', {
    kind: 'empty', message: 'Run the publish command with a saved connection ID, destination and message.'
  })
}
