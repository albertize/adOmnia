# Recipe: database and broker catalog

`databases.read` and `brokers.read` expose profile metadata only:

```js
const databases = await api.databases.listConnections()
const brokers = await api.brokers.listConnections()
```

The broker strips passwords, DSNs, broker configuration, and managed/Vault secret references before crossing the host boundary. Query execution and broker publishing require separate future APIs and are not implied by these read grants.

See `../examples/data-source-catalog` for a native combined list.
