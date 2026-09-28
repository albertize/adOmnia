# Extension Platform performance checks

The release budgets are defined in [`../EXTENSION-PLATFORM-PLAN.md`](../EXTENSION-PLATFORM-PLAN.md). Benchmarks are local and produce no telemetry.

## Go/runtime harness

```bash
go test ./internal/extensions -run '^$' \
  -bench 'Benchmark(ExtensionManifestDiscovery100|DeclarativeViewValidation1000Rows|HostEventDispatch)$' \
  -benchmem -count=5
```

This reports manifest decode/validation for 100 inactive packages, validation of a 1,000-row declarative table model, and authenticated in-process protocol/event overhead. It intentionally does not hide extension execution time inside the broker measurement.

Measure a production extension-host process start and authenticated handshake with:

```bash
ADOMNIA_EXTENSION_PERF_EXECUTABLE=/absolute/path/to/adomnia \
  go test ./internal/extensions -run '^$' -bench BenchmarkExtensionHostColdStart -count=5
```

Use a production binary from the target platform. Do not use `go test` binaries as the child executable.

## Required manual production measurements

Before GA, record results separately for Windows/WebView2, macOS/WKWebView, and Linux/WebKitGTK:

- warm and cold lazy activation p95;
- actual 1,000-row React table commit and keyboard responsiveness;
- idle extension-host RSS;
- retained RSS after 50 activate/reload/deactivate cycles.

A benchmark result is evidence, not an automatic relaxation of a budget. Regressions require review and an explicit plan update.
