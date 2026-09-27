# adOmnia Extension SDK 2.0 preview

This directory is the version-matched, local authoring contract for Extension Platform v2.

> Current milestone: local package registry, permission review, isolated subprocess activation, commands, state, HTTP request/response events, declarative views, sandboxed webviews, TypeScript bundling, SDK export, testing, and deterministic packaging are implemented. Consult [EVENTS.md](EVENTS.md) because several declared non-HTTP lifecycle producers remain pending.

## Read by task

| Task | Read |
|---|---|
| Create a first package | [QUICKSTART.md](QUICKSTART.md), [MANIFEST.md](MANIFEST.md) |
| Add permissions | [PERMISSIONS.md](PERMISSIONS.md), [SECURITY.md](SECURITY.md) |
| Add activation events | [EVENTS.md](EVENTS.md), [LIFECYCLE.md](LIFECYCLE.md) |
| Add native or custom UI | [DECLARATIVE-UI.md](DECLARATIVE-UI.md), [WEBVIEWS.md](WEBVIEWS.md) |
| Use state and settings | [STORAGE.md](STORAGE.md), [API.md](API.md) |
| Package and deliver | [PACKAGING.md](PACKAGING.md), [TESTING.md](TESTING.md) |
| Inspect complete packages | `../examples/response-security`, `../examples/variable-inspector`, `../examples/form-actions` |
| Follow a concrete recipe | [Command](recipes/COMMAND.md), [View](recipes/VIEW.md), [Request hook](recipes/REQUEST-HOOK.md), [Response analysis](recipes/RESPONSE-TAB.md), [Importer](recipes/IMPORTER.md), [Webview](recipes/WEBVIEW.md) |
| Use the programmatic API | `../api/index.d.ts` |
| Validate a manifest in an editor | `../schemas/manifest-v2.schema.json` |

The prose, declarations, schema, and templates are embedded in the same adOmnia executable. Always use the SDK exported by the target application version.
