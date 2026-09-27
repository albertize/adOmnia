# Extension Platform v2 engineering index

Extension Platform v2 is being implemented from [the architecture and delivery plan](../EXTENSION-PLATFORM-PLAN.md).

## Current implementation status

The first foundation slice is available in source:

- manifest v2 contract, JSON Schema, engine compatibility, secure registry and package handling;
- embedded esbuild for JavaScript/TypeScript, deterministic `.adomnia-extension` creation;
- JSON-capable `init`, `build`, `check`, `test`, `pack`, `install`, `list`, `inspect`, `doctor`, and `sdk` CLI;
- authenticated out-of-process goja host with lazy activation, commands, events, state, timeout and quarantine;
- permission review, command palette, keybindings, status items, settings, declarative views and sandboxed webviews;
- embedded Markdown SDK, declarations, schemas, templates and Agent Skill.

The v1 plugin runtime remains available through a separate compatibility path. HTTP request/response hooks are connected in v2; consult the SDK event table for reserved events that still lack a core producer.

## Authoring documentation

The versioned SDK is canonical under [`sdk/extensions/`](../../sdk/extensions/):

- [`docs/README.md`](../../sdk/extensions/docs/README.md) — task routing and milestone status;
- [`schemas/manifest-v2.schema.json`](../../sdk/extensions/schemas/manifest-v2.schema.json) — manifest schema;
- [`api/index.d.ts`](../../sdk/extensions/api/index.d.ts) — preview API declarations;
- [`templates/`](../../sdk/extensions/templates/) — checked scaffolds.

Export the same bundle from a built executable:

```bash
adomnia extension sdk ./adomnia-extension-sdk --json
```

All docs deliberately distinguish validated preview contracts from executable runtime features.
