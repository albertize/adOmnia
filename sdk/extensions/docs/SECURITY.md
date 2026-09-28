# Extension security model

## Boundaries

The runtime executes extension JavaScript in an `extension-host` child mode of the same adOmnia executable. The host receives source over authenticated, bounded JSON-RPC rather than opening extension files itself. It has no exposed Wails, renderer, bbolt, process, environment, or unrestricted filesystem/network API. Privileged operations cross the desktop permission broker.

Custom UI runs in sandboxed webviews without parent-DOM or Wails access. Host-native declarative UI remains the preferred option.

## Package threats handled by the foundation

Validation and packaging reject:

- paths outside the extension root;
- non-normalized or absolute entry points;
- symbolic links and non-regular files;
- unknown permissions and manifest fields;
- excessive package file counts and expanded bytes;
- contribution references that do not resolve.

Packaging excludes `.git`, `node_modules`, temporary files, and existing `.adomnia-extension` outputs.

## Trust rules

- Reviewing a manifest is not equivalent to trusting its code.
- Workspace imports must never install or activate extensions.
- Development folders are trusted separately from managed packages.
- Added permissions require renewed approval.
- Network and secret access must be explicit and user-visible.
- Extension-owned secrets require `secrets.own`, are isolated by extension ID, and are encrypted before persistence with the currently unlocked local Vault key. They are inaccessible while the Vault is locked.

The subprocess permission broker and isolated webview are implemented. Process-level operating-system sandboxing is not yet applied: the capability API is the primary boundary, and extension code should still be treated as trusted local code. Repeated runtime failures quarantine an extension after three failures.
