export async function activate(api) {
  const projects = await api.documents.listPdfProjects()
  await api.views.setState('adomnia.document-catalog-example.projects', {
    kind: 'table', title: 'Local PDF projects',
    columns: [{ key: 'name', title: 'Name' }, { key: 'pageCount', title: 'Pages' }, { key: 'updatedAt', title: 'Updated' }],
    rows: projects,
  })
}
