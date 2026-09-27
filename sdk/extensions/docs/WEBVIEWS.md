# Isolated webviews

Use a webview only when declarative UI cannot express the interaction. Declare `renderer: "webview"` and a self-contained packaged HTML `entry`.

The host loads it into an iframe with `sandbox="allow-scripts"`, no same-origin privilege, no referrer, no parent DOM access, and no Wails bindings. A generated CSP denies everything except inline package script/style, data/blob images, and explicitly granted `network:<origin>` connect targets. Forms, nested frames, objects, and base URL changes are denied.

The bridge exposes only:

```js
const result = await window.adomnia.executeCommand('publisher.extension.command', { value: 1 })
```

The parent verifies the sending window, a per-view channel token, extension command ownership, enablement, and host permissions. Keep HTML below 2 MiB and self-contained. Never use `allow-same-origin`, remote scripts, or broad network grants.
