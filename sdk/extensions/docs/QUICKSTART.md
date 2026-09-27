# Quickstart

Create a JavaScript extension without external dependencies:

```bash
adomnia extension init ./hello-adomnia \
  --id local.hello-adomnia \
  --name "Hello adOmnia" \
  --template minimal \
  --json

adomnia extension check ./hello-adomnia --json
adomnia extension pack ./hello-adomnia --json
```

Available templates are `minimal`, `view`, `request-hook`, `webview`, and `typescript`.

## Workflow for coding agents

1. Read this SDK's `README.md` routing table.
2. Scaffold with the CLI instead of inventing the directory shape.
3. Change `manifest.json` and `src/extension.js` locally.
4. Keep contribution IDs under the extension ID namespace.
5. Request only documented permissions required by the implementation.
6. Run `extension build --json` for TypeScript and `extension check --json` after every manifest change.
7. Exercise activation and behavior with `extension test --command <id> --json` and/or `--event <name>`.
8. Create the deterministic package with `extension pack --json`.
9. Report package path, SHA-256, permissions, files changed, and checks run.

The desktop installs v2 packages disabled. The user must review every requested permission before enabling them. Legacy plugins continue to work through the existing v1 runtime.
