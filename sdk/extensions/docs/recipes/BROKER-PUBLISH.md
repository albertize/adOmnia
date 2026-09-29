# Publish a broker message

Publish through an existing Broker Studio connection without receiving its credentials or raw executable configuration.

```json
{
  "activationEvents": ["onCommand:publisher.extension.publish", "onBrokerPublishComplete"],
  "permissions": ["brokers.publish"]
}
```

```js
export function activate(api) {
  api.commands.registerCommand('publisher.extension.publish', ({ connectionId, destination, message }) => {
    return api.brokers.publish(connectionId, destination, message, {
      contentType: 'application/json'
    })
  })

  api.events.onBrokerPublishComplete(({ jobId, success, result, error }) => {
    // Render the bounded acknowledgement or error in a native view.
  })
}
```

The returned `{ jobId }` identifies an asynchronous publish. Cancel it with `api.brokers.cancel(jobId)`.

Every publish opens a native confirmation showing the owning extension, saved connection, protocol, destination, and a bounded message preview. After approval, adOmnia resolves Vault references only in the broker request and uses the canonical Kafka, RabbitMQ, MQTT, Redis, or NATS sidecar endpoint. Results are limited to 1 MiB, ownership is enforced, and pending jobs are cancelled when the extension is disabled.
