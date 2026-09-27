# adOmnia Extension Platform v2 — Architecture and Delivery Plan

**Status:** active preview implementation plan; checked items are verified in the repository, while unchecked items remain release work.  
**Goal:** evolve the existing JavaScript plugin runtime into a complete, local-first extension platform comparable in breadth to VS Code, while preserving adOmnia's single-binary distribution and providing an agent-first Markdown SDK inspired by Pi.

**Implemented preview:** secure package registry, embedded esbuild, permission review, authenticated child host, commands/events/state/domain reads, contribution surfaces, declarative views, sandboxed webviews, authoring CLI/SDK, failure quarantine, and legacy migration report. Remaining gaps are tracked below. Deliberate preview deviations are recorded in [`ADR-EXTENSION-V2.md`](ADR-EXTENSION-V2.md).

## Implementation checkpoint — resume here

**Recorded:** 2026-09-27. **Estimated plan coverage:** approximately 87%. This checkpoint is the handoff source of truth for the next session; retain the detailed checked/unchecked items below.

### Last verified green state

The following passed after response-tab contributions, request/response toolbar actions, workbench event producers, diagnostics/progress, webview hardening, menu integration, and native declarative tree/form rendering were added:

```bash
gofmt -w internal/extensions/*.go
go test ./...
cd sdk/extensions && npm exec --yes --package typescript -- tsc --noEmit --strict --target ES2020 --module ESNext --moduleResolution bundler api/index.d.ts
cd frontend && npx tsc --noEmit
cd frontend && npm test -- --run       # 133 files / 686 tests
cd frontend && npm run build
```

An executable CLI `doctor/init/test/pack/install/inspect` smoke test also passed earlier in the same implementation pass. `wails3 task dev` has **not** received the required final manual product pass.

### Current completed edge

Declarative `tree` and `form` support is now implemented and verified across the SDK type contract, JSON Schema, Go broker validation, command-ownership checks, and native React renderer. Forms use native keyboard-accessible controls and can invoke only commands declared by their owning extension. Commands now reach the palette, keybindings, status bar, tab context menu, and request/response toolbars with parsed `when` clauses. The protocol supports cancellation and structured error codes; read/write domain slices cover variables, collections, environments, and tabs; v1 lifecycle/hook execution is routed through an explicit compatibility adapter; and security/recovery/fuzz tests cover host termination, slow JavaScript, state corruption, package inputs, protocol messages, `when` clauses, view models, and webview policy constants.

### Exact next steps

1. Continue with the unchecked items below, prioritizing rail containers, advanced mock/proxy/browser/flow/database/broker/document namespaces, assertion providers, performance benchmarks, accessibility, and cross-platform/manual desktop verification.
2. Regenerate Wails bindings only if a public Go binding/model changes.
3. Keep the full verification sequence above green after each vertical slice.
4. Run `wails3 task dev` and manually exercise install → permission review → enable → command/event/view/webview → reload/update → disable/uninstall before checking the final release gates.

### Important implementation notes

- V2 and legacy v1 remain separate; v1 behavior/data are intentionally preserved. `extension migrate-v1` emits a non-destructive report but is not a runtime adapter.
- Managed archives retain their original archive source so explicit reload/update works; all updates disable the extension and require review, and undeclared grants are removed.
- Frontend-owned collections/environments/tabs/workspace data reaches the child only through bounded broker snapshots and declared grants.
- HTTP request/response transformations run through the canonical Go execution path. Workbench-owned events are allowlisted and bounded.
- Webviews are self-contained `srcdoc` documents with CSP, opaque sandbox origin, denied clipboard/device permissions, tokenized messages, message-size limits, theme tokens, and same-extension command ownership. Local asset serving and OS-level process sandbox profiles remain deferred GA gates documented in the ADR.
- The host supports immediate/resolved promises but deliberately rejects pending asynchronous work that is not backed by a broker capability; full cancellation/job-queue semantics remain unchecked.
- `wails3` reports beta.5 while project dependencies reference a newer beta; generated bindings currently type-check, but keep verifying compatibility after regeneration.

---

## 1. Product outcome

A user should be able to ask an agent:

> Create an adOmnia extension that adds a response-security panel, a command to scan the active response, and a configurable rule set.

The agent should be able to discover the local authoring skill, read version-matched Markdown documentation, scaffold the package, implement it, validate it, run contract tests, and produce an installable local package without changing adOmnia core.

The user then reviews the source and requested permissions, installs it from a folder or package, and can enable, disable, reload, inspect, or remove it without a marketplace or account.

### Definition of success

Extension Platform v2 is complete when an extension can, through stable public APIs:

1. register commands, shortcuts, menus, settings, status items, views, panels, and contextual actions;
2. observe or transform supported application lifecycle and domain events;
3. read and modify explicitly granted adOmnia resources;
4. persist global, workspace, and secret state;
5. render host-native declarative UI and, when necessary, an isolated custom webview;
6. run asynchronously outside the desktop process and recover from extension failures;
7. be authored in JavaScript or TypeScript with local modules and bundled pure-JavaScript dependencies;
8. be created and maintained from bundled Markdown documentation by a coding agent;
9. be validated, tested, packaged, installed, and debugged entirely locally;
10. load existing v1 plugins through a compatibility adapter.

---

## 2. Principles and explicit non-goals

### Principles

- **Local first:** no account, registry, telemetry, or implicit network access.
- **Agent first, not agent only:** the SDK must be precise enough for an agent and readable enough for a human.
- **Contract first:** schemas, types, lifecycle, error behavior, and permission semantics are defined before implementation.
- **Declarative before arbitrary UI:** common contributions are rendered by adOmnia so extensions remain cohesive across themes and platforms.
- **Capability based:** extension code receives only declared and user-approved host capabilities.
- **Fault isolated:** extension failures must not crash or freeze the Wails application.
- **Lazy by default:** installing many inactive extensions must not materially slow startup.
- **Inspectability:** users can see activation state, permissions, logs, registrations, storage, and failures.
- **Backward compatible:** v1 plugins continue to work while migration is available.
- **Portable:** the production application remains a single executable with no Node.js runtime requirement.

### Non-goals for v2

- A public marketplace, ratings, accounts, payments, or cloud sync.
- Native binary extensions loaded into the adOmnia process.
- Unrestricted Node.js built-ins or direct operating-system access.
- Direct injection of third-party React components into the main React tree.
- Silent installation or silent updates.
- Automatically executing extension code embedded in an imported workspace.
- API compatibility with VS Code. The breadth is comparable; the API is designed for adOmnia.

---

## 3. What to retain and what to replace

### Retain from the current plugin system

- Local folder installation and secure path handling.
- Manifest metadata, settings, actions, enable/disable, and persisted state.
- Existing request/response hook behavior.
- The goja JavaScript engine.
- Per-extension storage, notifications, logs, and permission-aware host calls.
- Existing `request-advisor` as the first compatibility fixture.

### Replace or evolve

| Current behavior | v2 direction |
|---|---|
| Fresh VM for every call | Persistent VM per activated extension, disposed on reload/deactivation |
| Runtime executes inside desktop process | Dedicated extension-host subprocess |
| Regex-based partial ESM transform | Real TypeScript/ESM build step using embedded esbuild |
| Eight synchronous host functions | Versioned asynchronous `ExtensionAPI` namespaces |
| Fixed action-button plugin panel | Contribution registry plus declarative views and isolated webviews |
| Twelve named hooks, partially wired | Typed event catalog with explicit semantics and complete wiring |
| No module/dependency model | Local modules and install-time bundling of pure-JS dependencies |
| Manifest fields without compatibility enforcement | JSON Schema, API version, engine range, activation events, validation |
| Local folder only | Folder, development link, `.adomnia-extension` package, explicit Git source |
| No authoring SDK | Markdown docs, TypeScript declarations, schemas, examples, CLI, Agent Skill |

The old `WasmRuntime` public binding remains temporarily for compatibility, but new internals and UI use `ExtensionHost` and `ExtensionService`. Legacy WASM manifests remain readable and non-executable as they are today.

---

## 4. Target architecture

```text
React workbench
  ├─ contribution registry
  ├─ native declarative view renderers
  ├─ isolated extension webviews
  └─ extension manager / inspector / logs
                │ Wails bindings and app events
                ▼
Go ExtensionService (desktop process)
  ├─ package registry and manifest validation
  ├─ trust and permission broker
  ├─ command/event router
  ├─ state, secrets, and asset service
  ├─ contribution synchronization
  └─ extension-host supervisor
                │ authenticated JSON-RPC over stdio
                ▼
adomnia extension-host (child mode of the same executable)
  ├─ persistent goja VM per activated extension
  ├─ ESM bundle loader
  ├─ async job/promise bridge
  ├─ activation/deactivation and disposables
  └─ no direct bbolt, Wails, renderer, filesystem, environment, or network access
```

### Why a child mode of the same executable

`main.go` already branches into headless commands before Wails starts. Add an internal `extension-host` mode using the same pattern. This provides process-level crash and hang containment without shipping a second runtime or breaking portable single-binary distribution.

The desktop process is always authoritative. The child host cannot open adOmnia storage directly; it requests capabilities over the protocol. If it exits or exceeds limits, the supervisor can terminate and restart it while the main application stays available.

### Host protocol

Use a private, versioned JSON-RPC 2.0 protocol over stdin/stdout:

- random per-process handshake token;
- protocol and SDK version negotiation;
- request IDs, cancellation, deadlines, and structured errors;
- bounded message size and bounded queued events;
- notifications for registrations, logs, state changes, and webview messages;
- no sensitive values in protocol logs;
- parent-enforced permission checks for every privileged request.

Protocol types live in one Go package and have generated TypeScript fixtures. Contract tests replay the same messages against both sides.

### Runtime lifecycle

```text
discovered → validated → installed → disabled
                                  ↘ enabled → inactive
                                               ↓ activation event
                                            activating
                                               ↓
                                             active
                                      ↙ reload / disable ↘ failure
                               deactivating               failed
                                      ↓                     ↓
                                   inactive          restart or disable
```

Rules:

- only manifests are read during discovery;
- code loads on an activation event, not merely because it is installed;
- activation and deactivation have deadlines and cancellation;
- registrations return disposables and are removed automatically on unload;
- long-lived resources start during activation or on demand, never during discovery;
- host restart replays enabled manifests and lazy activation state;
- repeated crashes quarantine the responsible extension until the user re-enables it.

---

## 5. Extension package format

### Canonical directory

```text
security-inspector/
├── manifest.json
├── README.md
├── CHANGELOG.md
├── LICENSE
├── AGENTS.md
├── src/
│   ├── extension.ts
│   └── rules.ts
├── ui/                         # optional custom webview source
│   ├── index.ts
│   └── index.css
├── docs/                       # extension-specific user/agent docs
├── assets/
├── tests/
│   └── extension.test.ts
├── package.json                # optional authoring dependencies
└── dist/                       # deterministic packaged output
    ├── extension.js
    └── webviews/security.js
```

`README.md` is user-facing. `AGENTS.md` explains how to modify that specific extension and points to the version-matched SDK docs. Neither file is executed.

### Manifest v2 sketch

```json
{
  "$schema": "https://adomnia.local/schemas/extension-manifest-v2.json",
  "manifestVersion": 2,
  "id": "acme.security-inspector",
  "name": "Security Inspector",
  "version": "1.0.0",
  "publisher": "acme",
  "description": "Inspect response security headers locally.",
  "license": "MIT",
  "engines": { "adomnia": ">=1.0.0 <2" },
  "apiVersion": "2.0",
  "main": "dist/extension.js",
  "activationEvents": [
    "onCommand:acme.security.scan",
    "onView:acme.security.results",
    "onResponse"
  ],
  "permissions": [
    "responses.read",
    "workspaceState",
    "notifications"
  ],
  "contributes": {
    "commands": [
      { "id": "acme.security.scan", "title": "Scan Active Response", "category": "Security" }
    ],
    "views": [
      { "id": "acme.security.results", "container": "analysis", "name": "Security Results", "renderer": "declarative" }
    ],
    "configuration": {
      "acme.security.minimumScore": { "type": "number", "default": 80, "minimum": 0, "maximum": 100 }
    }
  }
}
```

The production package is a ZIP with the `.adomnia-extension` suffix. Installation must reject absolute paths, traversal, symlinks, duplicate normalized paths, excessive file counts, excessive expanded size, and zip bombs. The install directory is derived from a hash, never directly from the extension ID.

### Sources without a marketplace

Supported sources:

1. **Development folder:** linked and hot-reloaded; clearly marked as development code.
2. **Local package:** copied into the managed extension directory.
3. **`.adomnia-extension`:** validated and unpacked locally.
4. **Explicit Git URL/ref:** cloned only after a user action, pinned to a commit, reviewed, then built locally.

There is no background discovery, recommendation service, or automatic update. “Check source” and “Update” are explicit actions and always show permission and file-hash changes before activation.

### TypeScript and dependencies

- JavaScript works without a build file.
- TypeScript is compiled by embedded esbuild in development/install validation.
- Packaged extensions contain deterministic `dist` bundles.
- Local imports and pure-JavaScript dependencies can be bundled.
- Node built-ins, native addons, install scripts, and runtime package installation are rejected.
- The runtime exposes `@adomnia/api` as a virtual typed module at authoring time; the bundle receives the actual API through activation.
- Source maps are retained locally for diagnostics but stripped from ordinary user-facing errors.

This gives Pi-like direct authoring ergonomics while keeping the shipped application independent from Node.js.

---

## 6. Public Extension API

### Entry point

```ts
import type { ExtensionAPI } from '@adomnia/api'

export async function activate(api: ExtensionAPI): Promise<void> {
  api.subscriptions.add(
    api.commands.registerCommand('acme.security.scan', async () => {
      const response = await api.responses.getActive()
      if (!response) return
      await api.views.setState('acme.security.results', analyse(response))
    })
  )
}

export async function deactivate(): Promise<void> {
  // Optional. Registered disposables are always released by the host.
}
```

No privileged global object is exposed in v2. The API passed to `activate` is scoped to the extension and its granted permissions.

### Core namespaces

| Namespace | Initial responsibilities |
|---|---|
| `commands` | register and execute commands; command context and enablement |
| `events` | typed app, workspace, request, response, browser, proxy, mock, and lifecycle events |
| `window` | notifications, progress, dialogs, status items, output channels |
| `views` | trees, lists, tables, forms, details, empty states, view state, selection |
| `webviews` | create isolated panels, post typed messages, persist panel state |
| `workspace` | workspace identity, scoped files, collections, environments, change events |
| `requests` | inspect/build/execute requests through the canonical HTTP engine |
| `responses` | inspect active/history responses and register response tabs or renderers |
| `variables` | resolve variables and register scoped variable providers |
| `storage` | extension-local global and workspace key/value state |
| `secrets` | extension-owned encrypted values and explicit user-mediated secret references |
| `configuration` | read configuration, inspect source/default, receive changes |
| `clipboard` | explicit read/write access |
| `filesystem` | picker-granted files/directories and extension-owned storage only |
| `tasks` | cancellable background work and progress reporting |
| `logging` | named output channel with structured levels and redaction helpers |
| `extensions` | inspect compatible public metadata and optional exported APIs |
| `context` | extension ID, version, paths, mode, cancellation, subscriptions |

Domain namespaces are added in slices after core lifecycle is stable:

- `browserDebug`
- `proxy`
- `mock`
- `flows`
- `databases`
- `brokers`
- `documents`
- `importExport`
- `auth`
- `codegen`
- `assertions`
- `ai` only through an explicit user-selected provider/action, never implicitly

### Event contract

Each event documents:

- payload schema and version;
- whether it is notification, transform, veto, or request/response;
- ordering rules;
- timeout and cancellation behavior;
- whether handlers run sequentially or concurrently;
- error policy: fail-open, fail-closed, or user-configurable;
- which mutations are accepted;
- required permissions.

Transform events are sequential and deterministic. Every handler sees the previous handler's output. Notification events may run concurrently. Request execution remains fail-closed for enabled transform hooks, matching current behavior, but the error names the extension and offers “disable and retry”.

The existing event names become aliases in the v1 adapter. An event is not documented as available until there is a real producer and an integration test for that producer.

---

## 7. Contribution model

Static contributions are read from the manifest without activating code. Dynamic registrations are allowed after activation when state is genuinely runtime-dependent.

### Workbench contributions

- commands and command palette entries;
- default keybindings with `when` clauses;
- top-level and context menus;
- rail view containers;
- tree/list/table/form/detail views;
- editor and response tabs;
- toolbar and status items;
- settings schema and settings UI;
- welcome/empty-state content;
- icons and theme-aware assets;
- snippets and code-generation targets;
- importers/exporters and file associations;
- authentication methods;
- variable, assertion, and schema providers;
- templates, themes, and agent resources bundled as non-executable package resources.

### Context keys and `when` clauses

A central typed context-key service controls visibility and enablement, for example:

```text
activeTool == 'collections' && response.contentType == 'application/json' && extension.acme.security.enabled
```

Extensions may set only keys under their own namespace. Core keys are read-only. Expressions use a small parser, not arbitrary JavaScript.

### UI strategy

#### Level 1: host-native declarative UI

The default renderer accepts versioned JSON models for:

- action rows and forms;
- trees and lists;
- sortable/filterable tables;
- key/value and headers views;
- code/JSON/XML/Markdown viewers;
- tabs, badges, progress, empty states, and detail panes.

The React host renders these with adOmnia tokens, accessibility, keyboard behavior, virtualization, and theme support. Extensions provide state and actions, not CSS. This is the preferred route for agent-generated extensions.

#### Level 2: isolated webviews

Custom UI runs in an iframe served from an extension-specific local asset origin with:

- `sandbox="allow-scripts"` by default, without same-origin privileges;
- strict generated CSP;
- no direct Wails bindings or parent DOM access;
- a schema-validated `postMessage` bridge;
- network destinations derived from granted permissions;
- explicit clipboard, file, download, popup, and navigation grants;
- host-provided theme tokens and accessibility metadata;
- persisted webview state through the host API.

Arbitrary extension React code is allowed only inside this isolated webview bundle, never inside the main application tree.

---

## 8. Trust, permissions, and data boundaries

### Trust levels

| Source | Default behavior |
|---|---|
| Managed local install/package | Disabled until permission review succeeds |
| Development folder | Disabled until “Trust development extension” succeeds |
| Explicit Git source | Source/ref/hash shown; disabled until review succeeds |
| Workspace recommendation | Metadata only; never installs or executes automatically |
| Legacy v1 plugin | Marked legacy; existing enablement retained, new permissions reviewed on migration |

Workspace trust and extension trust are separate. Opening a `.adomnia` file cannot grant either.

### Permission design

Permissions are granular and, where appropriate, scoped:

```text
collections.read
collections.write
requests.read
requests.execute
responses.read
workspace.files.read:<picker-granted-root>
workspace.files.write:<picker-granted-root>
network:https://api.example.com
network.private
process.spawn:<declared-command>
clipboard.read
clipboard.write
browserDebug.read
browserDebug.control
proxy.read
proxy.control
mock.control
notifications
secrets.own
vault.requestReference
```

Rules:

- permissions are checked in the desktop broker, never only in JavaScript;
- an update adding permissions disables the extension pending review;
- temporary grants are supported for one action/session;
- broad wildcards require an additional warning;
- secrets are returned only when the user chooses a named reference and the operation requires it;
- the v2 API does not expose arbitrary environment-variable reads;
- all privileged operations are visible in the extension inspector;
- permission denials are structured errors, not empty values.

### State boundaries

- `globalState`: bbolt namespace by extension ID and major version.
- `workspaceState`: bbolt namespace by workspace ID and extension ID.
- `secrets`: Vault-backed, extension-owned namespace.
- `webviewState`: size-limited UI state, separate from business data.
- caches: disposable and quota-limited.

Extensions never receive direct bbolt handles. Per-extension and per-value quotas prevent accidental unbounded storage.

---

## 9. Agent-first Markdown SDK

The Pi pattern to adopt is: small discoverable instructions, detailed Markdown loaded only when needed, checked examples, and machine-readable contracts beside the prose.

### Canonical repository layout

```text
docs/extensions/
└── README.md                    # engineering status and link to the versioned SDK

sdk/extensions/
├── bundle.go                    # embeds and exports the SDK from the binary
├── package.json
├── api/index.d.ts
├── docs/
│   ├── README.md                # routing index and API/version matrix
│   ├── QUICKSTART.md
│   ├── MANIFEST.md
│   ├── LIFECYCLE.md
│   ├── API.md
│   ├── EVENTS.md
│   ├── CONTRIBUTION-POINTS.md
│   ├── DECLARATIVE-UI.md
│   ├── WEBVIEWS.md
│   ├── PERMISSIONS.md
│   ├── STORAGE.md
│   ├── PACKAGING.md
│   ├── TESTING.md
│   ├── MIGRATION-V1.md
│   ├── TROUBLESHOOTING.md
│   └── recipes/
├── schemas/manifest-v2.schema.json
├── schemas/declarative-view-v1.schema.json
├── schemas/events/*.schema.json
├── templates/{minimal,view,request-hook,webview}/
└── examples/

.agents/skills/adomnia-extension-authoring/
├── SKILL.md
└── references/
    ├── workflow.md
    ├── api-routing.md
    └── review-checklist.md
```

### Skill behavior

`SKILL.md` follows the Agent Skills format and advertises when it should be loaded. It must instruct an agent to:

1. query or read the installed SDK version;
2. read the routing index, then only the relevant API/recipe pages;
3. scaffold rather than invent package structure;
4. request the minimum permissions;
5. use declarative UI unless a webview is necessary;
6. write tests for every registered command/event transform;
7. run machine-readable validation and tests;
8. never edit adOmnia core to satisfy an extension request;
9. summarize permissions, generated files, checks, and manual verification.

The skill must not claim v2 functionality before the matching runtime milestone ships.

### SDK availability to external agents

The same versioned SDK is:

- present in the source repository;
- embedded in the production binary;
- exportable from **Extensions → Developer → Export Authoring SDK**;
- exportable for agents with `adomnia extension sdk <directory> --json`;
- included in every generated extension as a version pointer, not copied prose;
- available through a local read-only `GetExtensionSDKInfo` binding.

This lets an agent author an extension even when it is working outside the adOmnia repository.

### Drift prevention

- Type declarations and JSON Schemas are release artifacts with one `sdkVersion`.
- Documentation examples compile in CI.
- Manifest examples validate against the shipped schema.
- Every public API change requires types, schema, Markdown, changelog, compatibility tests, and SDK version review.
- Generated reference sections are marked and regenerated from canonical declarations.
- The embedded SDK hash is tested against the repository SDK hash.

---

## 10. Authoring and developer experience

### CLI

Extend the existing headless command router:

```bash
adomnia extension init my-extension --template view
adomnia extension check ./my-extension --json
adomnia extension test ./my-extension --json
adomnia extension dev ./my-extension
adomnia extension pack ./my-extension
adomnia extension install ./my-extension.adomnia-extension
adomnia extension inspect acme.my-extension --json
adomnia extension sdk ./adomnia-extension-sdk --json
adomnia extension doctor --json
```

Every command has stable exit codes and optional JSON output so coding agents do not need to parse decorated terminal text.

### Development mode

- watch source, manifest, docs, and assets;
- rebuild and reload only the changed extension;
- preserve state unless the manifest requests a reset;
- show activation time, registered contributions, event traffic, permissions, logs, and source-mapped errors;
- allow “Restart extension host” without restarting adOmnia;
- provide a mock context for views when no request/response is active;
- expose a copyable diagnostic bundle with secrets redacted.

### GUI

Replace the current plugin page incrementally with an Extensions workbench:

- Installed, Development, Disabled, Failed, and Legacy filters;
- source, version, engine compatibility, package hash, and trust state;
- permission review and permission-diff UI;
- activation reason and activation duration;
- contributions and commands list;
- output/log channel;
- storage usage and reset actions;
- reload, disable, uninstall, inspect, open folder, and package actions;
- “Create Extension” flow that exports/scaffolds the SDK and copies an agent-ready prompt.

No screen should imply an online catalog.

---

## 11. Backward compatibility

A `legacy/v1` adapter loads current plugins unchanged:

- current manifest fields are normalized into an internal v2 descriptor;
- current `actions` become generated commands and a declarative action view;
- current hooks keep sequential transform behavior;
- the global `adomnia` object remains only inside the legacy adapter;
- current settings and `plugin_storage` data remain readable;
- existing enabled/disabled state is preserved;
- the UI labels these packages **Legacy plugin** and offers a migration report;
- no automatic source rewrite occurs.

`adomnia extension migrate ./legacy-plugin` creates a side-by-side v2 candidate, reports unsupported behavior, and leaves the original untouched.

Removal of the v1 adapter requires a separately documented major release and real-world migration data.

---

## 12. Delivery plan

The work is intentionally vertical and gated. Do not build every API namespace before proving installation → activation → contribution → reload → recovery end to end.

### Phase 0 — Contracts and threat model

**Deliverables**

- [x] Approve this architecture or record deviations as ADRs.
- [ ] Inventory every current plugin call site and every proposed extension point.
- [x] Define manifest v2 JSON Schema and API/protocol versioning policy.
- [x] Define event semantics table and permission catalog.
- [x] Define subprocess threat model, webview threat model, and package extraction limits.
- [ ] Define compatibility fixtures from current v1 plugins.
- [x] Create documentation skeleton and authoring Agent Skill marked “preview/not yet available”.

**Exit gate:** schemas and lifecycle can describe the first vertical-slice extension without implementation-specific ambiguity.

### Phase 1 — Package registry, SDK, and CLI foundation

**Backend**

- [x] Add the `internal/extensions` manifest, package, registry, trust, permission, and state responsibilities.
- [x] Securely install folders and `.adomnia-extension` archives.
- [x] Store source, hash, enabled state, grants, failures, and compatibility metadata.
- [x] Enforce `engines.adomnia`, `apiVersion`, and package limits.
- [x] Add CLI `init`, `check`, `pack`, `install`, `inspect`, and `sdk-path`.
- [x] Add embedded esbuild compilation for development TypeScript and deterministic bundles.

**Frontend**

- [x] Show v2 packages beside legacy plugins without changing legacy execution.
- [x] Implement trust and permission review.
- [x] Show validation and engine compatibility errors.

**Docs/agent**

- [x] Ship Quickstart, Manifest, Permissions, Packaging, Testing, and minimal template.
- [ ] Compile and validate all snippets in CI.

**Exit gate:** an agent can generate and package a valid but not yet executable minimal extension using only exported local docs.

### Phase 2 — Out-of-process extension host

- [x] Add the private `extension-host` child command before Wails initialization.
- [x] Implement authenticated JSON-RPC, cancellation, limits, and structured errors.
- [ ] Add persistent goja VM lifecycle, ESM bundles, promises/job queue, and disposables.
- [x] Add supervisor restart, quarantine, activation deadlines, and crash diagnostics.
- [x] Implement `context`, `logging`, `storage`, `configuration`, `notifications`, and `commands` APIs.
- [x] Implement lazy activation events for startup, command, and configuration.
- [x] Route host logs into named output channels.

**Exit gate:** a generated extension registers a command, reads configuration, persists state, survives reload, and cannot crash the desktop app.

### Phase 3 — Workbench contribution infrastructure

- [x] Add a central frontend/backend contribution registry.
- [x] Add typed context keys and parsed `when` clauses.
- [x] Connect commands to command palette, keybindings, menus, toolbars, and context menus.
- [x] Add extension settings to the existing settings experience.
- [x] Add output channels, progress, status items, and diagnostics.
- [ ] Complete wiring and tests for core lifecycle, tab, environment, import, export, and workspace events.
- [x] Implement contribution cleanup on disable/reload/host restart.

**Exit gate:** one extension contributes a command, shortcut, menu action, setting, status item, and lifecycle listener without core-specific code.

### Phase 4 — Views, panels, and webviews

- [x] Implement declarative view schema and host-native React renderers.
- [ ] Add rail containers, trees, lists, tables, forms, detail panes, and response tabs.
- [x] Add extension-view state restoration and keyboard/focus contracts.
- [ ] Implement isolated webview asset serving, CSP generation, message bridge, theme tokens, and disposal.
- [x] Add webview permission tests for navigation, network, clipboard, downloads, and Wails isolation.
- [x] Add UI inspection and reload tooling.

**Exit gate:** an extension can add a cohesive sidebar view and response tab; a second extension can render a custom isolated webview without reaching Wails or the parent DOM.

### Phase 5 — adOmnia domain API breadth

Implement and stabilize namespaces in small vertical slices:

1. [ ] requests, responses, variables, assertions;
2. [ ] collections, environments, import/export, code generation;
3. [ ] mock, proxy, browser debugging;
4. [ ] flows and load/stress execution;
5. [ ] databases, brokers, and documents;
6. [ ] authentication providers and explicit AI actions.

For each namespace:

- define permission scopes and event semantics;
- expose cancellation and progress;
- add one realistic example extension;
- add backend contract and desktop integration tests;
- add a recipe and update the Agent Skill routing table;
- verify that extension state remains portable and local.

**Exit gate:** the documented contribution-point matrix is implemented, tested, and no event is advertised without a producer.

### Phase 6 — Compatibility, hardening, and general availability

- [x] Route v1 plugins through the compatibility adapter.
- [x] Ship and test the migration command/report.
- [x] Fuzz manifests, archives, protocol messages, `when` expressions, and view models.
- [x] Test malicious/slow extensions, host crashes, update permission changes, and corrupted state.
- [ ] Add startup, activation, event-dispatch, and view-render performance budgets.
- [ ] Test Windows, macOS, Linux, WebView2, WKWebView, and WebKitGTK.
- [ ] Complete accessibility and keyboard-only passes.
- [ ] Complete docs, examples, troubleshooting, and version policy.
- [ ] Run a manual agent-authoring benchmark from ten representative user requests.
- [ ] Remove “preview” only after all acceptance scenarios pass in production desktop builds.

**Exit gate:** all Definition of Done items below pass and the feature catalog can truthfully describe Extension Platform v2 as shipped.

---

## 13. First vertical slice

Build this before broad domain APIs:

**Response Policy extension**

- command palette command: “Evaluate Active Response Policy”;
- one setting for allowed status codes;
- `onResponse` listener;
- declarative response tab with a sortable findings table;
- status item with pass/fail count;
- workspace state for ignored findings;
- notification action;
- no custom webview;
- generated from the Agent Skill and minimal/view templates;
- v1 `request-advisor` continues to run simultaneously.

This slice crosses package validation, permissions, activation, command routing, event delivery, state, settings, UI contributions, diagnostics, reload, and compatibility. It is the primary architecture test.

---

## 14. Verification strategy

### Automated

- Go tests for registry, path safety, package extraction, grants, storage quotas, supervisor, and protocol.
- Protocol golden tests shared with TypeScript fixtures.
- Runtime tests for activation, promises, cancellation, timeout, disposal, module loading, and source maps.
- Frontend tests for contribution cleanup, context clauses, commands, settings, declarative views, permission review, and failure states.
- Security tests proving webviews cannot access parent DOM, Wails bindings, unauthorized network targets, or ungranted capabilities.
- Compatibility tests using unchanged copies of every bundled v1 example.
- CI compilation and execution of every SDK example and recipe.
- Fuzzing for archives, manifests, protocol messages, and declarative UI models.

### Manual product checks

- Ask an agent to create an extension from a natural-language request using only the exported SDK.
- Install from a folder, inspect permission review, activate, use, reload, disable, and uninstall.
- Crash and hang the extension host while an HTTP request tab remains usable.
- Upgrade an extension with and without new permissions.
- Exercise dark/light/custom themes and keyboard-only navigation.
- Verify Windows, macOS, and Linux custom webviews.
- Confirm no network request occurs unless initiated by the user or granted to an active extension.

### Performance budgets

Initial release gates on reference development hardware:

- discover and deserialize 100 installed but inactive extensions in **≤100 ms**;
- extension-host cold start and authenticated handshake in **≤750 ms p95**;
- warm lazy command activation in **≤200 ms p95** and cold activation in **≤1 s p95**;
- broker overhead for a synchronous transform event in **≤10 ms per extension**, excluding extension code;
- render/update a 1,000-row declarative table in **≤100 ms** without blocking keyboard input;
- idle extension-host RSS **≤64 MiB**, and no more than **5 MiB retained growth** after 50 activate/reload/deactivate cycles.

Benchmarks report separately from correctness tests; GA requires collecting them in production-mode builds on every supported platform. A budget regression blocks release unless this document records a measured, reviewed replacement.

---

## 15. Definition of Done

Extension Platform v2 is generally available only when:

- [ ] a fresh agent can create at least the command, view, request-hook, response-tab, importer, and webview examples from bundled Markdown docs;
- [x] validation catches unknown permissions, invalid contribution IDs, unsupported engines, missing activation targets, and unsafe package paths;
- [x] the desktop remains usable after extension timeout, panic, protocol corruption, and host termination;
- [x] permission checks occur in the Go broker and updates cannot retain undeclared grants;
- [x] inactive extensions do not execute code;
- [x] workspace files cannot install or activate code automatically;
- [ ] declarative views match application themes, density, keyboard, and accessibility behavior;
- [x] custom webviews are isolated from Wails and the parent renderer;
- [x] v1 plugins retain their current behavior and data;
- [ ] every public API has types, schema where applicable, Markdown, tests, and an example;
- [x] all authoring commands offer stable JSON output;
- [x] no marketplace, telemetry, account, or hidden update request has been introduced;
- [ ] full frontend/Go builds, tests, and manual Wails desktop checks pass on supported platforms.

---

## 16. Decisions required before Phase 1

These decisions must be recorded, not left implicit during implementation:

1. exact manifest ID and publisher naming rules;
2. semantic-version range library and compatibility policy;
3. maximum package, asset, RPC message, state, and log sizes;
4. default activation, event, and deactivation deadlines;
5. exact Git-source support and whether cloning ships in v2.0 or v2.1;
6. permission grant lifetime and workspace-trust UX;
7. local asset origin implementation for Wails on all three platforms;
8. whether pure-JS npm dependency bundling is accepted in v2.0 or initially limited to local modules;
9. public API deprecation window and support duration;
10. which domain namespaces are mandatory for the first stable release.

Until these are resolved, implementation should focus only on reversible discovery, schema, documentation, and test-harness work.
