# Explicit AI action

Extensions can request a completion from the AI provider already configured by the user. They cannot inspect provider credentials or bypass consent.

```json
{
  "activationEvents": ["onCommand:publisher.extension.summarize", "onAIComplete"],
  "permissions": ["ai.execute"]
}
```

```js
export function activate(api) {
  api.commands.registerCommand('publisher.extension.summarize', ({ text }) =>
    api.ai.complete('Summarize accurately.', text, { maxTokens: 600 }))

  api.events.onAIComplete(({ success, result, error }) => {
    // Render the bounded completion or cancellation in a native view.
  })
}
```

`api.ai.complete` returns `{ jobId }`; cancel it with `api.ai.cancel(jobId)`.

Every call displays a native confirmation naming the extension and showing a bounded prompt preview. Only after approval does adOmnia resolve its own configured provider credentials and invoke the canonical AI engine. Credentials, provider configuration, and automatic-mode fallback remain outside the extension host. Results are capped at 1 MiB, rate-limited, owner-targeted, cancellable, and never sent as telemetry by adOmnia.
