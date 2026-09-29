# SDK versioning and compatibility

Three versions are intentionally separate:

- `manifestVersion` identifies the package document shape. Preview v2 requires `2`.
- `apiVersion` identifies the JavaScript/TypeScript contract. Preview requires `2.0`.
- `engines.adomnia` is the SemVer range of desktop versions the package accepts.

The exported SDK package has its own SemVer version (`2.0.0-preview.1` at this checkpoint) and is always matched to the exporting executable. Authors should export from the oldest adOmnia version they intend to support and validate against every supported desktop version.

## Compatibility policy

During preview, additive APIs may ship in a new preview SDK without changing `apiVersion`; removals, semantic changes, permission expansion, or payload incompatibilities require an API-version review and may require a new major API version. Preview packages must not assume undocumented capabilities.

After GA:

- additive optional fields and methods are minor SDK changes;
- documentation or type corrections that do not alter runtime behavior are patches;
- removals and incompatible behavior require a new major API version;
- a deprecated public API remains documented for at least one stable adOmnia major release before removal;
- manifests with an unsupported `apiVersion` or incompatible engine range fail before code execution.

Every public API change must update declarations, permission/event contracts, Markdown, the changelog, examples or recipes, host/service tests, and the embedded SDK hash test.

Updates never silently grant new permissions. Adding a permission disables the extension until explicit review regardless of version numbering.
