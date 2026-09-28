# Recipe: browser-network observer

Use this for local analysis of traffic captured by adOmnia Browser Debug.

```json
{
  "activationEvents": ["onBrowserNetwork"],
  "permissions": ["browserDebug.read"]
}
```

```js
export function activate(api) {
  api.events.onBrowserNetwork((entry) => {
    api.logging.info('Browser request captured', {
      method: entry.method,
      status: entry.status,
      url: entry.url,
    })
  })
}
```

`api.browserDebug.list()` returns at most the latest 500 workbench entries. `getActive()` returns the selected entry. Reading requires `browserDebug.read`.

Request `browserDebug.control` only when the extension genuinely needs `clear()` or `select(id)`. These controls operate on the canonical Browser Debug store; they do not grant page navigation, arbitrary CDP commands, DOM access, or browser launch control.

See `../examples/browser-network` for a complete native table view with an explicit clear command.
