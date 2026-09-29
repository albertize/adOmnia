# Recipe: saved flow catalog

Request `flows.read` to inspect definitions saved in the active local adOmnia database:

```js
export async function activate(api) {
  const flows = await api.flows.list()
  const selected = await api.flows.get(flows[0]?.id)
  api.logging.info('Loaded local flow', { id: selected?.id })
}
```

Snapshots are capped at 5 MiB. Runtime results are not persisted with definitions.

## Cancellable execution jobs

Request `flows.execute`, declare `onFlowComplete`, and start a canonical runner job:

```js
const { jobId } = await api.flows.execute(flowId, { startNodeId })
api.events.onFlowProgress(({ jobId, progress, entriesCompleted }) => {
  // Render bounded progress without polling.
})
api.events.onFlowComplete(({ jobId, success, result, error }) => {
  // Update a native view; large results are capped by the broker.
})
// Optional explicit cancellation:
await api.flows.cancel(jobId)

// Stress jobs use the same ownership/cancellation/completion model and the
// canonical workbench stress engine:
const stress = await api.flows.executeStress(flowId, {
  vus: 1, rampUpS: 0, mode: 'iterations', iterations: 10,
  durationS: 1, thinkTimeMs: 0,
})
```

API-flow and stress execution happen in the same frontend runners used by the workbench and inherits active resolved variables. The host call returns immediately; completion is targeted only to the owning extension. Disabling the extension invalidates its outstanding jobs.

See `../examples/flow-catalog` for a read-only table and `../examples/flow-runner` for job execution.
