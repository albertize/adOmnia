# ADR: Extension Platform v2 implementation deviations

Status: accepted for preview; revisit before GA.

## Single executable child host

The extension runtime is a private child mode of the adOmnia executable over authenticated stdio JSON-RPC. This preserves single-binary delivery and isolates panics/hangs from Wails. Capability isolation is enforced by the Go broker. OS-level process sandbox profiles remain a GA hardening gate.

## Bundled CommonJS execution format

Author source may use TypeScript and ESM syntax, but embedded esbuild emits one deterministic CommonJS bundle for goja. This avoids runtime package resolution and a second module loader while retaining ESM authoring. Node built-ins and native modules remain unsupported.

## Self-contained webviews

Preview webviews use one packaged HTML entry via `srcdoc`, CSP, opaque iframe origin, `sandbox="allow-scripts"`, denied device/clipboard capabilities, and a tokenized command bridge. Local asset serving is intentionally deferred: authors embed data URLs or bundle assets into HTML. This removes an asset URL/traversal surface. A future asset protocol requires equivalent CSP and traversal tests.

## Local sources only

V2 accepts local folders and `.adomnia-extension` archives. Git URL installation and silent updates are excluded from preview. Users may clone explicitly, inspect locally, then link/install. Updates are explicit and disable the extension for review.

## Domain snapshots

Frontend-owned workspace data is copied into a bounded desktop broker snapshot. The child host can retrieve a domain only after manifest declaration and persisted grant. Initial APIs are read-only; writes remain reserved until each canonical store exposes an atomic backend command.

## Legacy coexistence

V1 plugins retain their existing runtime and data path rather than being silently wrapped. `extension migrate-v1` emits a non-destructive report, and migration requires an explicit new v2 package. This avoids changing legacy behavior during preview.
