# Recipe: request hook

Scaffold `request-hook`. Declare `onRequest` and `requests.read`. A transform must explicitly return `modified: true`:

```js
export function activate(api) {
  api.events.onRequest((request) => ({
    modified: true,
    data: { ...request, headers: [...(request.headers || []), { key: 'X-Local', value: '1', enabled: true }] },
  }))
}
```

Hooks run serially in deterministic extension-ID order and fail closed. Test with `--event onRequest --payload fixture.json`.
