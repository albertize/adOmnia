# State and settings

- `api.globalState`: extension-owned state shared across workspaces; requires `globalState`.
- `api.workspaceState`: extension-owned state scoped to the active workspace; requires `workspaceState`.
- `api.configuration`: reads values declared under `contributes.configuration` and edited by the user.
- `api.secrets`: extension-owned encrypted string values; requires `secrets.own` and an unlocked adOmnia Vault.

State values must be JSON serializable. Each value is limited to 1 MiB and each extension/scope to 10 MiB. Extensions never receive bbolt handles. Workspace changes update the broker context before later calls.

Use configuration for user choices, workspace state for project-specific durable data, and global state for machine-local preferences. Do not store credentials there. Use `api.secrets` for credentials owned by the extension. Secret values are limited to 64 KiB, encrypted with the active Vault key before reaching bbolt, never listed, and unavailable while the Vault is locked. `vault.requestReference` remains reserved for user-mediated access to existing Vault entries.
