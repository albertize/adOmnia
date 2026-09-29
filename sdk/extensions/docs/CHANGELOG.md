# Extension SDK changelog

## 2.0.0-preview.1

- Added manifest v2, isolated goja extension host, authenticated protocol, cancellation, deadlines, quarantine, and deterministic packages.
- Added commands, events, configuration, global/workspace state, Vault-encrypted extension secrets, logs, notifications, progress, and diagnostics.
- Added request/response APIs, variable and assertion providers, collection/environment/tab writes, and bounded workbench snapshots.
- Added native rail/response views, declarative list/tree/table/form/details renderers, and sandboxed webviews.
- Added bounded Browser Debug, mock, proxy, flow, database, broker, and document read slices with narrow controls where documented.
- Added cancellable asynchronous API-flow and load/stress jobs executed by canonical workbench runners with owner-targeted progress/completion events.
- Added cancellable database query jobs through saved connections and the canonical sidecar, with Vault credential isolation, bounded results, ownership checks, and native confirmation for destructive operations.
- Added cancellable, natively confirmed broker publish jobs through saved Kafka, RabbitMQ, MQTT, Redis, and NATS connections without exposing credentials.
- Added bounded PDF text jobs and native-dialog PDF export jobs without exposing raw document bytes or selected filesystem paths.
- Added cancellable AI completion jobs with per-request native consent and no provider credential exposure.
- Added local CLI, embedded Markdown SDK, schemas, templates, recipes, executable examples, fuzz targets, and performance harnesses.

Preview limitations and deferred GA gates remain authoritative in the repository `docs/ADR-EXTENSION-V2.md` and `docs/EXTENSION-PLATFORM-PLAN.md`.
