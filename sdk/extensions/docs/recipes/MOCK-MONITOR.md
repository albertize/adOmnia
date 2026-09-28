# Recipe: mock-server monitor

Declare `onMockHit` and `mock.read`, then subscribe with `api.events.onMockHit(handler)`. `api.mock.getSnapshot()` returns canonical runtime status, endpoint definitions, and at most 500 recent hits.

Use `mock.control` only for an explicit user command that calls `clearHits()` or `stop()`. Extensions cannot silently start or reconfigure the server; startup remains a user-owned workbench action.

See `../examples/mock-monitor` for a complete native table view.
