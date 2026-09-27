# Declarative UI

Prefer declarative views for cohesive, accessible UI. Declare a view with `renderer: "declarative"`, activate on that view or a command, then call:

```ts
await api.views.setState('publisher.extension.results', {
  kind: 'table',
  title: 'Findings',
  columns: [{ key: 'rule', title: 'Rule' }, { key: 'result', title: 'Result' }],
  rows: [{ rule: 'TLS', result: 'pass' }],
})
```

Schema: `schemas/declarative-view-v1.schema.json`.

Supported kinds are `empty`, `list`, `table`, `details`, `markdown`, and `json`. adOmnia owns rendering, tokens, scrolling, density, and keyboard behavior. State is JSON, limited to 1 MiB per update, and is session UI state rather than durable extension storage.
