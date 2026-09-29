# Recipe: PDF project catalog

Request `documents.read` and list lightweight local PDF project metadata:

```js
const projects = await api.documents.listPdfProjects()
```

The API returns ID, name, page count, and update timestamp. PDF bytes, annotations, form values, arbitrary filesystem paths, and Markdown workspace files are not exposed. `documents.write` remains reserved.

See `../examples/document-catalog` for a native table view.
