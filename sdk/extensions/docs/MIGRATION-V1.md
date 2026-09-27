# Migrating a v1 plugin

V1 plugins remain executable through `internal/plugins`; v2 is installed separately.

1. Run `adomnia extension migrate-v1 <plugin-folder> --publisher <name> --json` to obtain a non-destructive suggested manifest and manual-step report.
2. Scaffold the closest v2 template beside the original.
3. Change the ID to a namespaced `publisher.extension` ID.
4. Replace global `adomnia.*` calls with the injected `ExtensionAPI`.
5. Convert manifest actions to commands and action panels to declarative views.
6. Convert hooks to `api.events` handlers and declare the matching activation event and permission.
7. Move settings into `contributes.configuration` and persistent data into scoped state.
8. Run build, check, isolated test, and package commands.
9. Install disabled and review permissions before removing the v1 copy.

There is no destructive automatic rewrite. Existing v1 storage and enabled state are untouched.
