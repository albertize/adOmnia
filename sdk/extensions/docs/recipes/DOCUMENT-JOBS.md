# Read PDF text and export a document

Document jobs operate on saved PDF Editor projects without giving extension code filesystem access.

```json
{
  "activationEvents": ["onDocumentReadComplete", "onDocumentWriteComplete"],
  "permissions": ["documents.readContents", "documents.write"]
}
```

```js
export async function activate(api) {
  api.events.onDocumentReadComplete(({ success, result, error }) => {
    // result.pages contains bounded { page, text } entries.
  })
  api.events.onDocumentWriteComplete(({ success, result, error }) => {
    // result.saved is true after the user selected a destination.
  })

  const readJob = await api.documents.readPdfText(projectId, { pages: [1, 2] })
  const exportJob = await api.documents.exportPdf(projectId, {
    flatten: true,
    suggestedName: 'reviewed.pdf'
  })
}
```

Cancel either job with `api.documents.cancel(jobId)`.

`documents.readContents` extracts at most 500 pages and returns at most 750,000 text characters through the owner-targeted event. It does not return raw PDF bytes. `documents.write` always uses the native Save dialog; the extension receives only `{ saved: true }`, never the selected filesystem path. Flattening uses the canonical PDF Editor exporter. Pending jobs are cancelled when the extension is disabled.
