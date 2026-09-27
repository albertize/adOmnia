# Testing and review

## Required foundation checks

```bash
adomnia extension build . --json       # when source is declared
adomnia extension check . --json
adomnia extension test . --command publisher.extension.command --json
adomnia extension pack . --json
```

A valid report has `valid: true` and no error diagnostics. Treat warnings as review items even when packaging remains possible.

## Tests to add with runtime APIs

- one test per registered command;
- transform-event success, no-change, cancellation, and error cases;
- permission denial behavior;
- state reconstruction and migration;
- activation and disposal cleanup;
- declarative view empty, loading, data, and error states;
- webview message-schema rejection when a webview is necessary.

## Human review checklist

- IDs are owned by the extension namespace.
- Requested permissions match actual API calls.
- No secret appears in logs, state, fixtures, or errors.
- Network destinations are scoped.
- UI is declarative unless custom rendering is essential.
- Errors are actionable and operations are cancellable.
- `README.md` explains the user workflow.
- `AGENTS.md` points maintainers to the version-matched SDK.

The test command starts the real isolated extension-host protocol and reports activation, command/event results, notifications, view state, and local test state as JSON. It does not replace a final Wails desktop interaction check for menus, keyboard focus, themes, or webviews.
