# Extension review checklist

- The SDK comes from the target adOmnia version.
- `manifest.json` passes `adomnia extension check --json`.
- IDs are namespaced under the extension ID.
- Every permission maps to an actual operation.
- No credentials or response secrets are logged or persisted.
- The entry point is a local packaged JavaScript file.
- UI is declarative unless a webview is essential.
- Errors are readable and future asynchronous work is cancellable.
- README explains the user workflow.
- AGENTS.md points maintainers to the version-matched SDK.
- Package output and SHA-256 are included in the delivery summary.
