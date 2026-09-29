# Activation events

An activation event says when extension code may be loaded. It is not an event subscription by itself.

## Accepted by the preview validator

- `onStartup`
- `onWorkspaceOpen`, `onWorkspaceClose`
- `onRequest`, `onResponse`, `onSend`
- `onSave`, `onImport`, `onExport`
- `onThemeChange`, `onEnvChange`
- `onTabOpen`, `onTabClose`
- `onAssertions`
- `onVariables`
- `onBrowserNetwork`
- `onMockHit`, `onProxyTraffic`
- `onFlowProgress`, `onFlowComplete`, `onDatabaseComplete`, `onBrokerPublishComplete`, `onDocumentReadComplete`, `onDocumentWriteComplete`
- `onCommand:<contributed-command-id>`
- `onView:<contributed-view-id>`

Command and view activation references are checked against static contributions.

## Runtime status

| Event | Producer | Semantics |
|---|---|---|
| `onCommand:<id>` | Command palette, keybinding, manager, or webview bridge | Lazy activation followed by the registered command. |
| `onView:<id>` | Opening/rendering an extension view | Lazy activation contract; manager command activation currently populates views. |
| `onStartup` | Desktop backend startup | Activates the extension after storage and registry initialization. |
| `onConfiguration:<key>` | Extension settings editor | Recreates the VM with current settings and emits changed keys. |
| `onTabOpen`, `onTabClose` | Workbench tab snapshot | Notification event; requires `tabs.read`. |
| `onEnvChange` | Active environment snapshot | Notification event with previous/current values; requires `environments.read`. |
| `onWorkspaceOpen`, `onWorkspaceClose` | Active workspace snapshot | Notification event; requires `workspace.read`. |
| `onThemeChange` | Active appearance snapshot | Notification event containing theme ID and light/dark mode. |
| `onSave` | Canonical request save actions | Notification event with tab, collection, and request IDs. |
| `onImport`, `onExport` | Collection import/export and global drop pipelines | Notification event with bounded operation metadata. |
| `onSend`, `onRequest` | Canonical Go HTTP send path | Sequential, deterministic transform; failures are fail-closed. Requires `requests.read`. |
| `onResponse` | Canonical Go HTTP response path | Sequential transform; failures are fail-closed. Requires `responses.read`. |
| `onAssertions` | Response Assertions tab | Lazily activates providers registered through `api.assertions`; requires `assertions.provide`. |
| `onVariables` | Workbench environment/workspace context synchronization | Lazily activates providers registered through `api.variables`; requires `variables.provide`. |
| `onBrowserNetwork` | Browser Debug capture store | Notification for each newly captured bounded network entry; requires `browserDebug.read`. |
| `onMockHit` | Canonical local mock runtime | Notification for each bounded match/miss hit; requires `mock.read`. |
| `onProxyTraffic` | Canonical local proxy runtime | Notification containing redacted headers and truncated bodies; requires `proxy.read`. |
| `onFlowProgress` | Canonical frontend flow/stress runner | Bounded owner-targeted progress notification; requires `flows.execute`. |
| `onFlowComplete` | Canonical frontend flow/stress runner | Targeted completion/failure notification for a job owned by the extension; requires `flows.execute`. |
| `onDatabaseComplete` | Canonical database workbench sidecar | Targeted completion/failure notification for a query job owned by the extension; requires `databases.execute`. |
| `onBrokerPublishComplete` | Canonical Broker Studio sidecar | Targeted acknowledgement/failure for a confirmed publish owned by the extension; requires `brokers.publish`. |
| `onDocumentReadComplete` | Canonical PDF Editor loader/text extractor | Targeted bounded text result/failure; requires `documents.readContents`. |
| `onDocumentWriteComplete` | Canonical PDF Editor exporter and native Save dialog | Targeted save/cancellation result without disclosing the selected path; requires `documents.write`. |


Register handlers through `api.events.onResponse(handler)` or `api.events.on(name, handler)`. A transform returns `{ modified: true, data: payload }`; returning nothing observes without modification. Handlers execute serially inside the extension VM with a deadline. Notification-style concurrency will be introduced only after the remaining producers have typed payloads.
