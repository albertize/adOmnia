# Permissions

Permissions declare intent and the Go desktop broker enforces grants again for every privileged host call. Packages install disabled. An update adding permissions disables the extension until renewed review; undeclared old grants are removed.

## Implemented broker permissions

| Permission | Capability |
|---|---|
| `notifications` | Show bounded desktop notifications. |
| `workspaceState`, `globalState` | Read/write extension-owned scoped JSON state. |
| `secrets.own` | Read/write encrypted strings in the extension-owned namespace while the local Vault is unlocked. |
| `requests.read`, `responses.read` | Read the active canonical HTTP context and receive corresponding events. |
| `requests.execute` | Execute through the canonical Go HTTP engine. |
| `environments.read`, `collections.read`, `tabs.read`, `workspace.read` | Read broker snapshots synchronized by the workbench. Environment variable values are redacted without `variables.read`. |
| `variables.read` | Read enabled active-environment variables and resolve `{{name}}` placeholders. |
| `variables.provide` | Add bounded, in-memory string values to normal request variable resolution. |
| `assertions.provide` | Register bounded assertion providers shown in the native response Assertions tab. |
| `collections.write` | Import a collection or add a request through the canonical collection store. |
| `environments.write` | Select an existing environment through the canonical environment store. |
| `tabs.write` | Open, close, or select a workbench tab. |
| `browserDebug.read` | Read the bounded browser-network snapshot and receive new-entry events. |
| `browserDebug.control` | Clear captured browser traffic or select an existing captured entry. |
| `mock.read`, `proxy.read` | Read bounded canonical mock/proxy runtime snapshots and receive hit/traffic events. |
| `mock.control`, `proxy.control` | Clear captured history or stop the corresponding local runtime. |
| `network:<origin>` | Allow a webview `connect-src` to one exact HTTP(S) or WebSocket origin. It does not expose fetch to extension-host code. |

## Reserved permissions

The manifest catalog also reserves narrow future permissions for environment/collection/tab/cookie writes, clipboard, advanced proxy/mock configuration, flows, databases, brokers, documents, Vault references, private network, workspace files, and declared process spawning. Declaration and grant do **not** imply an API exists: consult `API.md`.

Scoped forms are `network:<origin>`, `workspace.files.read:<scope>`, `workspace.files.write:<scope>`, and `process.spawn:<declared-command>`. The validator rejects malformed origins, wildcards, credentials, query/fragment components, and traversal-like scopes.

## Rules for agents

- Never request a wildcard as a convenience.
- Do not request write when read is enough.
- Do not request reserved capabilities before their API is shipped.
- Never persist or log resolved credentials.
- Prefer extension-owned `api.secrets` storage for credentials created by the extension; use Vault references for existing user secrets once that mediated API ships.
- Explain every permission in the delivery summary.
