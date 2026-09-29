export async function activate(api) {
  api.commands.registerCommand('adomnia.ai-action-example.summarize', ({ text } = {}) => {
    if (!text) throw new Error('text is required')
    return api.ai.complete('Summarize accurately. Do not invent facts.', text, { maxTokens: 600 })
  })
  api.events.onAIComplete(async ({ success, result, error }) => {
    await api.views.setState('adomnia.ai-action-example.result', success
      ? { kind: 'markdown', content: result }
      : { kind: 'details', title: 'AI request failed or was cancelled', data: { error } })
  })
  await api.views.setState('adomnia.ai-action-example.result', { kind: 'empty', message: 'Run the summarize command. adOmnia will ask before contacting the configured provider.' })
}
