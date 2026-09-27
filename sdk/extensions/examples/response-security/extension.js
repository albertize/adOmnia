export function activate(api) {
  api.events.onResponse(async (response) => {
    const headers = response.headers || {}
    const checks = ['content-security-policy', 'strict-transport-security', 'x-content-type-options'].map((header) => ({
      id: header,
      title: header,
      badge: Object.keys(headers).some((name) => name.toLowerCase() === header) ? 'present' : 'missing',
    }))
    await api.views.setState('adomnia.response-security-example.results', { kind: 'list', title: 'Security headers', items: checks })
    await api.diagnostics.set(checks.filter((item) => item.badge === 'missing').map((item) => ({ severity: 'warning', message: `Missing ${item.title}`, resource: 'active-response' })))
  })
}
