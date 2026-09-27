# Recipe: native view

Scaffold `view`. Declare a `declarative` view and matching `onView:<id>` activation event. Populate it from a command or activation:

```js
await api.views.setState('publisher.tool.results', {
  kind: 'list', title: 'Checks',
  items: [{ id: 'tls', title: 'TLS', badge: 'pass' }],
})
```

Use only the fields in `declarative-view-v1.schema.json`. Never pass HTML. The native renderer inherits theme and accessibility behavior.
