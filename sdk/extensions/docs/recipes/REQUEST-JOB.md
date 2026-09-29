# Execute a cancellable HTTP request

Use the canonical Go HTTP executor for long-running extension requests.

```json
{
  "activationEvents": ["onRequestComplete"],
  "permissions": ["requests.execute"]
}
```

```js
export async function activate(api) {
  api.events.onRequestComplete(({ jobId, success, result, error }) => {
    // Render the bounded response or cancellation.
  })

  const { jobId } = await api.requests.executeJob({
    method: 'GET',
    url: 'https://api.example.test/health',
    headers: {},
    timeoutMs: 30000,
    followRedirects: true
  })
  // await api.requests.cancel(jobId)
}
```

The broker replaces the low-level request ID with an extension-owned job ID, applies the canonical executor's timeout/TLS/redirect policy, supports cancellation through `httpexec.Cancel`, and caps the result at 5 MiB before dispatching `onRequestComplete`. Pending jobs are cancelled on disable or reload. The older synchronous `api.requests.execute` remains available for preview compatibility, but new long-running work should use `executeJob`.
