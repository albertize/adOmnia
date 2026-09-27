# Recipe: isolated webview

Scaffold `webview`. Keep `ui/index.html` self-contained. From the frame:

```js
const output = await window.adomnia.executeCommand('publisher.tool.calculate', { input: 2 })
document.querySelector('#result').textContent = JSON.stringify(output)
```

The command must belong to the same extension. Do not fetch remote script/style assets, navigate the frame, access the parent, or request broad network origins. See `WEBVIEWS.md`.
