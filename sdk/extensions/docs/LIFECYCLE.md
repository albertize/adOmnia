# Lifecycle

Extensions move through discovered → installed disabled → enabled inactive → active. Installation never executes code. Enabling requires every declared permission to be granted; adding a permission to a development source disables it for renewed review.

Code activates lazily for commands, views, HTTP events, or `onStartup`. One persistent goja VM is retained per active extension inside the child host. Disable, reload, uninstall, shutdown, timeout, or host failure disposes it. `deactivate()` is optional; registered commands/events are dropped regardless.

The desktop remains authoritative. If the host exits, active flags are reset. Repeated failures increment a persisted counter; after three failures the extension is disabled and marked quarantined. Explicit re-enable clears the quarantine.

Do not start watchers or background work at module evaluation time. Start during `activate()` and return/register disposables. Operations have deadlines and pending asynchronous work not backed by a host capability is rejected.
