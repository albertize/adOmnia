# Recipe: command

Scaffold `minimal`, declare `publisher.tool.run` in `contributes.commands`, add `onCommand:publisher.tool.run`, then register it in `activate`:

```js
export function activate(api) {
  api.context.subscriptions.add(api.commands.registerCommand('publisher.tool.run', async (args) => {
    api.logging.info('run', { args })
    return { ok: true }
  }))
}
```

No permission is needed unless the handler calls a privileged namespace. Test with `adomnia extension test . --command publisher.tool.run --json`.
