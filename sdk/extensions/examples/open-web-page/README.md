# Open Web Page

A minimal adOmnia Extension Platform v2 example using a native declarative form.

1. Open the **Open Web Page** view.
2. Enter an `http://` or `https://` URL.
3. Choose **Open request tab**.
4. Press **Send** in the new GET request tab.

The extension deliberately opens a normal adOmnia request tab rather than embedding arbitrary remote content or accessing Wails/browser APIs. It requests only `tabs.write`, validates the scheme locally, and never performs a network request itself.

```bash
adomnia extension check . --json
adomnia extension test . --command adomnia.open-web-page-example.open --payload '{"url":"https://example.com"}' --json
adomnia extension pack . --json
```
