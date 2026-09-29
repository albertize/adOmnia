# adOmnia Extension SDK 2.0 preview

This directory is the version-matched, local authoring contract for Extension Platform v2.

> Current milestone: local package registry, permission review, isolated subprocess activation, commands, state and encrypted extension secrets, HTTP and workbench events, assertion providers, declarative views, sandboxed webviews, TypeScript bundling, SDK export, testing, and deterministic packaging are implemented. Consult [EVENTS.md](EVENTS.md) for exact producer and permission semantics.

## Read by task

| Task | Read |
|---|---|
| Create a first package | [QUICKSTART.md](QUICKSTART.md), [MANIFEST.md](MANIFEST.md) |
| Add permissions | [PERMISSIONS.md](PERMISSIONS.md), [SECURITY.md](SECURITY.md) |
| Add activation events | [EVENTS.md](EVENTS.md), [LIFECYCLE.md](LIFECYCLE.md) |
| Add native or custom UI | [DECLARATIVE-UI.md](DECLARATIVE-UI.md), [WEBVIEWS.md](WEBVIEWS.md) |
| Use state and settings | [STORAGE.md](STORAGE.md), [API.md](API.md) |
| Package and deliver | [PACKAGING.md](PACKAGING.md), [TESTING.md](TESTING.md), [VERSIONING.md](VERSIONING.md), [CHANGELOG.md](CHANGELOG.md) |
| Inspect complete packages | `../examples/response-security`, `../examples/variable-inspector`, `../examples/form-actions`, `../examples/assertion-provider`, `../examples/variable-provider`, `../examples/browser-network`, `../examples/mock-monitor`, `../examples/proxy-monitor`, `../examples/flow-catalog`, `../examples/data-source-catalog`, `../examples/document-catalog`, `../examples/flow-runner`, `../examples/database-runner`, `../examples/broker-publisher`, `../examples/document-worker`, `../examples/ai-action` |
| Follow a concrete recipe | [Command](recipes/COMMAND.md), [View](recipes/VIEW.md), [Request hook](recipes/REQUEST-HOOK.md), [Response analysis](recipes/RESPONSE-TAB.md), [Importer](recipes/IMPORTER.md), [Variable provider](recipes/VARIABLE-PROVIDER.md), [Assertion provider](recipes/ASSERTION-PROVIDER.md), [Browser network](recipes/BROWSER-NETWORK.md), [Mock monitor](recipes/MOCK-MONITOR.md), [Proxy monitor](recipes/PROXY-MONITOR.md), [Flow catalog](recipes/FLOW-CATALOG.md), [Data sources](recipes/DATA-SOURCE-CATALOG.md), [Database execution](recipes/DATABASE-EXECUTION.md), [Broker publish](recipes/BROKER-PUBLISH.md), [Documents](recipes/DOCUMENT-CATALOG.md), [Document jobs](recipes/DOCUMENT-JOBS.md), [AI action](recipes/AI-ACTION.md), [Webview](recipes/WEBVIEW.md) |
| Use the programmatic API | `../api/index.d.ts` |
| Validate a manifest in an editor | `../schemas/manifest-v2.schema.json` |

The prose, declarations, schema, and templates are embedded in the same adOmnia executable. Always use the SDK exported by the target application version.
