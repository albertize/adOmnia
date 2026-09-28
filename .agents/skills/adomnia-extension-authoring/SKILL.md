---
name: adomnia-extension-authoring
description: Create, validate, test, package, or modify local adOmnia Extension Platform v2 packages. Use when a user asks for an adOmnia plugin or extension, extension manifest, command, view, request hook, response integration, or authoring SDK workflow.
license: MIT
compatibility: Requires the version-matched adOmnia Extension SDK and extension CLI.
---

# adOmnia extension authoring

1. In the adOmnia source repository, read `sdk/extensions/docs/README.md`. Outside it, ask for or export the SDK with `adomnia extension sdk <directory>` and read that bundle's `docs/README.md`.
2. Check the SDK milestone and event-status tables before promising behavior. The v2 isolated runtime, commands, HTTP/workbench events, state, encrypted extension secrets, assertion providers, declarative views, and sandboxed webviews are available; consult the version-matched API docs before using reserved advanced domain namespaces.
3. Scaffold with `adomnia extension init` and the closest template. Do not invent a package layout.
4. Read only the routed documents needed for the requested contribution.
5. Keep all contribution IDs under the extension ID namespace.
6. Request the minimum documented permissions and explain each one.
7. Prefer host-native declarative UI. Use an isolated webview only when declarative UI cannot express the workflow.
8. Never modify adOmnia core to satisfy an extension-only request unless the user explicitly asks to add a missing public extension point.
9. Run `adomnia extension build <path> --json` when TypeScript is used, `check`, then the real isolated `test` command with relevant command/event arguments.
10. Run `adomnia extension pack <path> --json` and report changed files, permissions, validation/runtime results, package path, and SHA-256. Reserve desktop/UI claims for a real Wails interaction check.
