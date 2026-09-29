const VIEW_ID = 'adomnia.open-web-page-example.open'
const COMMAND_ID = 'adomnia.open-web-page-example.open'
let sequence = 0

function id(prefix) {
  sequence += 1
  return `${prefix}-${Date.now()}-${sequence}`
}

function blankRow() {
  return { id: id('row'), key: '', value: '', enabled: false }
}

function requestFor(url) {
  return {
    id: id('request'),
    name: `Web page: ${url.replace(/^https?:\/\//i, '').split(/[/?#]/, 1)[0]}`,
    description: 'Opened by the Open Web Page extension.',
    type: 'request',
    method: 'GET',
    url,
    params: [blankRow()],
    headers: [blankRow()],
    cookies: [blankRow()],
    bodies: [{ id: id('body'), name: 'Request', type: 'none', raw: '', lang: 'text', form: [blankRow()] }],
    activeBodyIdx: 0,
    auth: {
      type: 'none', token: '', username: '', password: '',
    },
    timeout: 0,
    followRedirects: true,
  }
}

function validateUrl(value) {
  const url = value.trim()
  const http = url.indexOf('http://') === 0
  const https = url.indexOf('https://') === 0
  const host = url.slice(http ? 7 : https ? 8 : url.length).split(/[/?#]/, 1)[0]
  if ((!http && !https) || !host || url.indexOf(' ') >= 0) throw new Error('Enter a complete URL, for example https://example.com')
  return url
}

export async function activate(api) {
  api.context.subscriptions.add(
    api.commands.registerCommand(COMMAND_ID, async (args = {}) => {
      const url = validateUrl(typeof args.url === 'string' ? args.url : '')
      await api.tabs.open(requestFor(url))
      api.logging.info('Opened web page URL in a new request tab', { url })
      return { opened: url, note: 'The page is open as a GET request tab. Press Send to fetch it.' }
    }),
  )

  await api.views.setState(VIEW_ID, {
    kind: 'form',
    title: 'Open Web Page',
    message: 'Enter an HTTP(S) URL. adOmnia opens it as a new GET request tab; press Send to load the page.',
    fields: [{
      id: 'url',
      label: 'Page URL',
      type: 'text',
      placeholder: 'https://example.com',
    }],
    actions: [{ id: 'open', title: 'Open request tab', command: COMMAND_ID }],
  })
}
