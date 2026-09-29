export async function activate(api) {
  api.commands.registerCommand('adomnia.document-worker-example.read', ({ projectId, pages } = {}) => {
    if (!projectId) throw new Error('projectId is required')
    return api.documents.readPdfText(projectId, { pages })
  })
  api.commands.registerCommand('adomnia.document-worker-example.export', ({ projectId, suggestedName } = {}) => {
    if (!projectId) throw new Error('projectId is required')
    return api.documents.exportPdf(projectId, { flatten: true, suggestedName })
  })
  api.events.onDocumentReadComplete(async ({ success, result, error }) => {
    await api.views.setState('adomnia.document-worker-example.results', {
      kind: 'details', title: 'PDF text job',
      data: success ? { pages: result?.pages?.length || 0, truncated: result?.truncated || false } : { status: 'failed', error },
    })
  })
  api.events.onDocumentWriteComplete(async ({ success, result, error }) => {
    await api.views.setState('adomnia.document-worker-example.results', {
      kind: 'details', title: 'PDF export job', data: success ? { saved: result?.saved === true } : { status: 'cancelled or failed', error },
    })
  })
  await api.views.setState('adomnia.document-worker-example.results', { kind: 'empty', message: 'Run a document command with a saved PDF project ID.' })
}
