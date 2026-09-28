# Recipe: proxy traffic monitor

Declare `onProxyTraffic` and `proxy.read`, then subscribe with `api.events.onProxyTraffic(handler)`. Request/response headers are redacted by the proxy before extension delivery, and bodies retain the user-configured truncation limits.

`api.proxy.getSnapshot()` returns canonical status and bounded traffic. Request `proxy.control` only for explicit `clearTraffic()` or `stop()` commands. Starting interception, changing rules, resuming breakpoints, and CA operations remain user-owned workbench actions.

See `../examples/proxy-monitor` for a complete native table view.
