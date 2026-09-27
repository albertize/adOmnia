# Manifest v2

Every extension root contains `manifest.json`. The canonical machine-readable contract is `schemas/manifest-v2.schema.json`.

## Required fields

| Field | Contract |
|---|---|
| `manifestVersion` | Must be `2`. |
| `id` | Lowercase namespaced ID, for example `acme.security-inspector`. |
| `name` | User-visible name. |
| `version` | Strict semantic version such as `1.0.0`. |
| `publisher` | First segment of `id`. |
| `engines.adomnia` | Non-empty range such as `>=1.0.0 <2`. |
| `apiVersion` | Currently `2.0`. |
| `main` | Normalized relative path to packaged `.js`, `.mjs`, or `.cjs`. |
| `source` | Optional JavaScript/TypeScript source bundled into `main` by embedded esbuild. |
| `activationEvents` | At least one documented lazy activation event. |

Unknown manifest fields are rejected in the preview contract. This catches misspellings and prevents agents from relying on imaginary APIs.

## Static contributions currently accepted by validation

- `commands`
- `views`
- `configuration`
- `keybindings`
- `menus`
- `statusBar`

Contribution IDs must start with `<extension-id>.`. Command references in keybindings, menus, status items, and activation events must resolve to a command declared by the same extension.

A webview contribution uses `renderer: "webview"` and must provide a packaged `.html` `entry`; declarative views must not provide one. Validation also resolves command references, checks engine ranges, verifies entry files, and rejects non-portable paths. Consult the event status before relying on reserved event producers.
