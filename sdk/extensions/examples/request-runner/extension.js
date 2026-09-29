export async function activate(api) {
  api.commands.registerCommand('adomnia.request-runner-example.run', ({ method = 'GET', url, headers = {}, body = '' } = {}) => {
    if (!url) throw new Error('url is required')
    return api.requests.executeJob({ method, url, headers, body, timeoutMs: 30000, followRedirects: true })
  })
  api.events.onRequestComplete(async ({ success, result, error }) => {
    await api.views.setState('adomnia.request-runner-example.result', {
      kind: 'details', title: 'HTTP request job',
      data: success ? { status: result?.status, durationMs: result?.durationMs, size: result?.size } : { status: 'failed', error },
    })
  })
  await api.views.setState('adomnia.request-runner-example.result', { kind: 'empty', message: 'Run the request command with a URL.' })
}
