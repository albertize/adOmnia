# Recipe: native assertion provider

Use this when an extension should add bounded checks to adOmnia’s native response Assertions tab.

## Manifest

Declare lazy activation and the provider permission:

```json
{
  "activationEvents": ["onAssertions"],
  "permissions": ["assertions.provide"]
}
```

## Runtime

```js
export function activate(api) {
  api.context.subscriptions.add(api.assertions.registerProvider(
    'acme.security.content-type-options',
    ({ response }) => ({
      label: 'MIME sniffing is disabled',
      passed: Object.keys(response.headers || {}).some(
        (name) => name.toLowerCase() === 'x-content-type-options',
      ),
      expected: 'x-content-type-options header',
    }),
  ))
}
```

Provider IDs must belong to the extension namespace. A provider may return one result, an array, or no value. Results are limited and validated by the host. Providers run only when a response’s Assertions surface requests them; they cannot mutate the response.

Run `adomnia extension check`, `test`, and `pack` before delivery. Verify the result in the desktop response Assertions tab.
