# Extension point inventory

This inventory is the Phase 0 map for Extension Platform v2. It distinguishes executable producers from reserved names so documentation does not advertise a hook without a real path.

## Legacy v1 call sites

| Core path | Legacy entry | Behavior |
|---|---|---|
| `main.go` | `plugins.NewPluginManager`, `extensions.NewLegacyAdapter` | Creates the unchanged v1 manager and explicit compatibility adapter. |
| `app.go` startup/shutdown | `LegacyAdapter.Init`, `FireStartup`, `Shutdown` | Preserves persisted enablement and lifecycle delivery. |
| `app.go` canonical HTTP path | `LegacyAdapter.ApplyEventJSON` | Runs sequential v1 request/send/response transforms before/alongside v2. |
| `internal/plugins/manager.go` | install, enable, settings, actions, hooks, storage | Owns all legacy package state and execution. |
| `frontend/src/components/plugins/PluginManager.tsx` | generated `PluginManager` binding | Legacy management, settings, actions, and DevTools UI. |
| `workspaces/plugins/request-advisor` | unchanged fixture | Executable compatibility fixture covered directly by `internal/plugins/manager_test.go`. |

The adapter is the only new runtime path allowed to execute v1 hooks. Legacy Wails management APIs continue to use the original manager so existing data and UI remain compatible.

## Executable v2 extension points

| Area | Public surface | Producer/consumer |
|---|---|---|
| Lifecycle | `onStartup`, `onConfiguration:<key>`, command/view activation | `Service.FireStartup`, settings broker, command/view routers |
| HTTP | `onRequest`, `onSend`, `onResponse`; `requests`, `responses` | canonical Go HTTP execution in `app.go` / `internal/httpexec` |
| Workbench | workspace, environment, theme, tab open/close | bounded snapshots from `ExtensionContributionHost` into `Service.SetDomainContext` |
| File/workflow | `onSave`, `onImport`, `onExport` | request workspace, collection tree, and global file-drop pipeline |
| Commands | commands, keybindings, palette, tab context, request/response toolbar, status | static manifest registry plus lazy command activation |
| Views | non-response rail containers, response tabs, declarative state, isolated webviews | React contribution surfaces and Go view/webview broker |
| State | configuration, global/workspace state, encrypted extension secrets | registry/bbolt broker and unlocked local Vault |
| Variables | read/resolve and `onVariables` providers | active environment snapshot and in-memory provider merge below explicit environment values |
| Assertions | `onAssertions` providers | native response Assertions tab |
| Domains | cancellable canonical HTTP request jobs, collection import/add request, environment selection, tab open/close/select, browser-network read/clear/select, mock hit/status/clear/stop, proxy traffic/status/clear/stop, saved flow and load/stress execution jobs, redacted database/broker profile reads, cancellable database query jobs with destructive-operation confirmation, confirmed broker publish jobs over canonical protocol endpoints, PDF project metadata/text reads, and native-dialog PDF exports | permission-checked Go broker into canonical stores and local runtimes |
| Observability | logs, notifications, progress, diagnostics | bounded service buffers and desktop event bridge |

Integration coverage for lifecycle/workbench producers lives in `internal/extensions/service_test.go`; host API/protocol coverage lives in `host_test.go` and `host_process_integration_test.go`.

## Reserved extension points not yet executable

The manifest permission catalog reserves these names, but the SDK must describe them as unavailable until a broker, real producer, integration test, recipe, and example ship:

- cookie read/write and user-mediated Vault references;
- clipboard and picker-scoped filesystem operations;
- advanced mock/proxy startup, configuration, rules, breakpoints, map-local/map-remote, throttling, and CA operations beyond the shipped bounded read/clear/stop slices;
- browser debugging navigation, page control, DOM, console, storage, and device emulation beyond the shipped bounded network read/clear/select slice;
- advanced load/stress orchestration beyond the shipped canonical flow stress runner;
- database schema mutation/configuration beyond confirmed query jobs over saved connections;
- broker consuming/subscriptions and administrative operations beyond confirmed one-message publishing;
- arbitrary document formats, raw PDF bytes, Markdown/filesystem integration, and unattended writes beyond bounded PDF text and native-dialog export;
- authentication providers and unattended/implicit AI actions;
- code-generation targets and richer import/export providers;
- process spawning and private-network host APIs.

OS-level extension-host sandbox profiles, Git-source installation, and local webview asset origins are separately deferred in `ADR-EXTENSION-V2.md`; they are not implicit extension points.

## Change rule

When adding an extension point, update this inventory, the manifest permission/event contract, `api/index.d.ts`, relevant Markdown, an example or recipe, and producer integration tests in the same change. Remove an item from the reserved list only when its end-to-end desktop path exists.
