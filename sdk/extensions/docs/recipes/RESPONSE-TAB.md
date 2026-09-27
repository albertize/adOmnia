# Recipe: response analysis view

Declare `onResponse`, `responses.read`, a command, and a declarative view. Observe the response, derive bounded data, then set native view state:

```js
export function activate(api) {
  api.events.onResponse(async (response) => {
    await api.views.setState('publisher.analyzer.results', {
      kind: 'details', title: `HTTP ${response.status}`,
      data: { contentType: response.contentType, duration: response.duration },
    })
  })
}
```

Set the view contribution's `container` to `response`. The workbench renders it as a response tab when its `when` clause matches. The same view remains inspectable from the Extensions manager.
