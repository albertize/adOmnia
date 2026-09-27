# State and settings

- `api.globalState`: extension-owned state shared across workspaces; requires `globalState`.
- `api.workspaceState`: extension-owned state scoped to the active workspace; requires `workspaceState`.
- `api.configuration`: reads values declared under `contributes.configuration` and edited by the user.

State values must be JSON serializable. Each value is limited to 1 MiB and each extension/scope to 10 MiB. Extensions never receive bbolt handles. Workspace changes update the broker context before later calls.

Use configuration for user choices, workspace state for project-specific durable data, and global state for machine-local preferences. Do not store credentials there. `secrets.own` and Vault-mediated references remain reserved until the v2 secrets namespace is connected.
