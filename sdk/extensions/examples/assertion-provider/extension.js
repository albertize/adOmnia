export function activate(api) {
  api.context.subscriptions.add(api.assertions.registerProvider(
    'adomnia.assertion-provider-example.security-headers',
    ({ response }) => {
      const headers = response?.headers || {}
      const present = Object.keys(headers).some((name) => name.toLowerCase() === 'x-content-type-options')
      return {
        label: 'Response prevents MIME sniffing',
        passed: present,
        actual: present ? 'header present' : 'header missing',
        expected: 'x-content-type-options header',
      }
    },
  ))
}
