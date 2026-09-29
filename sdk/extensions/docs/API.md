# Extension API

The canonical declarations are `api/index.d.ts`. Runtime capabilities are passed to `activate(api)`; there is no privileged Node or browser global.

Implemented namespaces:

- `context`: extension identity and subscription collection;
- `commands`: manifest-owned command registration;
- `events`: typed helpers and sequential transforms;
- `configuration`: manifest setting values;
- `globalState` / `workspaceState`: brokered JSON state with quotas;
- `secrets`: extension-owned encrypted strings available only while the local Vault is unlocked;
- `logging`: named, bounded host logs;
- `window.notify`: permission-gated desktop toast;
- `requests`: active request and canonical HTTP execution;
- `responses`: active response snapshot;
- `variables`: permission-gated active variable map, `{{name}}` resolution, and lazy in-memory providers merged below explicit active-environment values;
- `assertions`: manifest-owned providers evaluated against the active response and merged into the native Assertions result surface;
- `environments`, `collections`, `tabs`, `workspace`: permission-gated workbench snapshots with `getActive`, `list`, and `getSnapshot`; narrow granted writes can select an environment, import/add collection data, or open/close/select tabs;
- `browserDebug`: bounded browser-network snapshots plus clear/select controls;
- `mock`: canonical status/endpoints/hits snapshot plus explicit clear/stop controls;
- `proxy`: canonical status/redacted traffic snapshot plus explicit clear/stop controls;
- `flows`: bounded saved definitions plus cancellable asynchronous jobs through the canonical frontend flow/stress runners and targeted progress/completion events;
- `databases`: redacted connection metadata plus cancellable query jobs through the canonical database workbench sidecar; credentials remain broker-owned, destructive operations require native confirmation, and completion is owner-targeted;
- `brokers`: redacted connection metadata plus cancellable, natively confirmed publish jobs through canonical Kafka, RabbitMQ, MQTT, Redis, or NATS sidecar endpoints; credentials remain broker-owned and completion is owner-targeted;
- `documents`: bounded PDF project metadata, cancellable bounded PDF text extraction, and flattened/original export through a native user-controlled Save dialog; raw bytes and selected filesystem paths are never returned to extension code;
- `ai`: cancellable, rate-limited completions through the user's configured provider after per-request native consent; provider credentials and configuration remain broker-owned;
- `views`: declarative view state.

Host functions are synchronous internally so goja can safely bridge them, but the TypeScript API returns promises where callers should `await` operations. Pure-JS dependencies can be bundled at build time. Node built-ins, native modules, dynamic runtime installation, `require` of external packages, and direct Wails calls are unsupported.

Write calls are accepted by the Go broker and delivered to canonical frontend stores through a bounded action event. Acceptance means the payload passed broker validation; the workbench may still reject stale IDs and reports that failure as an extension error notification.

Reserved domain namespaces from the platform plan are added only with a real producer, permission broker implementation, documentation, and integration test.
