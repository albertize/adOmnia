# Packaging

Run validation before packaging:

```bash
adomnia extension check . --json
adomnia extension pack . --out ../my-extension.adomnia-extension --json
```

The output is a deterministic ZIP archive with the `.adomnia-extension` suffix. Entries are sorted and use a fixed timestamp so unchanged source produces the same SHA-256.

Excluded paths:

- `.git/`
- `node_modules/`
- existing `.adomnia-extension` files
- temporary `.tmp` files

Limits are currently 4,096 files and 64 MiB of source content. Symbolic links and non-regular files are rejected.

The pack command returns the path, SHA-256, file count, and uncompressed byte count. Include these values in an agent's delivery summary.
