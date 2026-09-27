# Contribution points

The manifest currently validates and the workbench consumes:

- `commands`: command palette, manager, keybindings, status items, and webviews;
- `keybindings`: keyboard shortcuts with parsed `when` expressions;
- `menus`: validated command references at `commandPalette`, `tab/context`, `request/toolbar`, and `response/toolbar`; request/response toolbar and tab context placements are rendered, while all declared commands remain discoverable from the palette;
- `statusBar`: sorted workbench status actions;
- `configuration`: permission-adjacent settings rendered in the Extensions workbench;
- `views`: host-native declarative views or isolated webviews.

All IDs must live below the extension ID. Static contributions are visible without activation; selecting one activates code lazily. Toolbar commands receive `{ tabId, requestId }`; tab-context commands receive `{ tabId }`. Treat arguments as optional because a contribution may be invoked from another supported surface. `when` supports `&&`, `||`, `!`, `==`, and `!=` over typed core context keys. It never evaluates JavaScript.

Current context keys include `activeTool`, `hasResponse`, `response.status`, `response.contentType`, and `extension.<id>.enabled`. Unknown keys are false.
