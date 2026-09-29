# Execute a database query

Use an existing adOmnia database connection instead of opening sockets or reading credentials directly.

Manifest requirements:

```json
{
  "activationEvents": ["onCommand:publisher.extension.query", "onDatabaseComplete"],
  "permissions": ["databases.execute"]
}
```

```js
export function activate(api) {
  api.commands.registerCommand('publisher.extension.query', ({ connectionId, query }) => {
    return api.databases.execute(connectionId, query, {
      limit: 200,
      timeoutMs: 30000,
    })
  })

  api.events.onDatabaseComplete(({ jobId, success, result, error }) => {
    // Update a native declarative view with the bounded result or error.
  })
}
```

The returned `{ jobId }` identifies an asynchronous job. Cancel it explicitly with `api.databases.cancel(jobId)`.

Execution uses the canonical database sidecar, the selected saved connection, active environment variable substitution, and Vault-backed credential resolution. The extension never receives connection secrets. Potentially destructive SQL or MongoDB operations always require an explicit native user confirmation showing the query. Results are limited to 5 MiB before entering the extension host and jobs are cancelled when the extension is disabled.
