# Extension authoring instructions

Read `sdk/extensions/docs/README.md` when working in the adOmnia repository, or use the SDK exported with `adomnia extension sdk <directory>`.
Prefer host-native declarative UI over a webview. Keep IDs under the `adomnia.open-web-page-example` namespace and request only required permissions.

Run `adomnia extension check . --json` and `adomnia extension pack . --json` before delivery.
