# Troubleshooting

| Problem | Action |
|---|---|
| Manifest rejected | Run `adomnia extension check . --json`; use diagnostic code and path. |
| TypeScript main missing | Run `adomnia extension build . --json`; ensure `source` and `main` are declared. |
| Permission review required | Open Extensions, inspect every permission, save the grant decision, then enable. |
| Command not registered | Ensure it appears in `contributes.commands`, activation reaches `activate()`, and registration uses the exact ID. |
| Event handler rejected | Declare the event in `activationEvents` and its required permission. |
| Host timeout/crash | Inspect the extension error, fix it, reload source, and explicitly re-enable. Three failures quarantine it. |
| Webview is blank | Keep the HTML self-contained and inspect CSP errors; remote scripts and parent access are intentionally blocked. |
| Package differs unexpectedly | Remove generated nondeterministic files; packaging sorts entries and fixes timestamps. |

Run `adomnia extension doctor [folder] --json` to verify the schema, executable, SDK, and child-host handshake.
