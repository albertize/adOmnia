# Recipe: variable provider

Use this to add local computed values to adOmnia’s normal `{{name}}` substitution.

```json
{
  "activationEvents": ["onVariables"],
  "permissions": ["variables.provide"]
}
```

```js
export function activate(api) {
  api.context.subscriptions.add(api.variables.registerProvider(
    'acme.workspace-variables.active',
    ({ workspace }) => ({ workspace_id: workspace?.id || '' }),
  ))
}
```

Provider IDs must belong to the extension namespace. Providers receive bounded workspace/environment identity, not environment values. Return at most 200 string values. Values remain in memory and are never persisted by adOmnia. Active-environment variables override provider values with the same name, preserving explicit user control.

Request only `variables.read` as an additional permission when the extension itself must inspect or resolve active values.
