package extensions

import "fmt"

const (
	maxDeclarativeItems = 5000
	maxDeclarativeText  = 256 * 1024
)

func ValidateDeclarativeViewState(value any) error {
	state, ok := value.(map[string]any)
	if !ok {
		return fmt.Errorf("declarative view state must be an object")
	}
	allowed := map[string]bool{"kind": true, "title": true, "message": true, "columns": true, "rows": true, "items": true, "fields": true, "actions": true, "data": true}
	for key := range state {
		if !allowed[key] {
			return fmt.Errorf("unknown declarative view property %q", key)
		}
	}
	kind, ok := state["kind"].(string)
	if !ok {
		return fmt.Errorf("declarative view kind is required")
	}
	if !map[string]bool{"empty": true, "list": true, "tree": true, "table": true, "form": true, "details": true, "markdown": true, "json": true}[kind] {
		return fmt.Errorf("unsupported declarative view kind %q", kind)
	}
	for _, key := range []string{"title", "message"} {
		if raw, exists := state[key]; exists {
			text, ok := raw.(string)
			if !ok {
				return fmt.Errorf("declarative view %s must be a string", key)
			}
			if len(text) > maxDeclarativeText {
				return fmt.Errorf("declarative view %s is too large", key)
			}
		}
	}
	if raw, exists := state["columns"]; exists {
		columns, ok := raw.([]any)
		if !ok || len(columns) > 100 {
			return fmt.Errorf("declarative view columns must be an array of at most 100 entries")
		}
		for index, rawColumn := range columns {
			column, ok := rawColumn.(map[string]any)
			if !ok {
				return fmt.Errorf("declarative view column %d must be an object", index)
			}
			for key := range column {
				if key != "key" && key != "title" {
					return fmt.Errorf("unknown column property %q", key)
				}
			}
			if _, ok := column["key"].(string); !ok {
				return fmt.Errorf("declarative view column %d requires string key", index)
			}
			if _, ok := column["title"].(string); !ok {
				return fmt.Errorf("declarative view column %d requires string title", index)
			}
		}
	}
	if raw, exists := state["rows"]; exists {
		rows, ok := raw.([]any)
		if !ok || len(rows) > maxDeclarativeItems {
			return fmt.Errorf("declarative view rows must be an array of at most %d entries", maxDeclarativeItems)
		}
		for index, row := range rows {
			if _, ok := row.(map[string]any); !ok {
				return fmt.Errorf("declarative view row %d must be an object", index)
			}
		}
	}
	if raw, exists := state["items"]; exists {
		items, ok := raw.([]any)
		if !ok || len(items) > maxDeclarativeItems {
			return fmt.Errorf("declarative view items must be an array of at most %d entries", maxDeclarativeItems)
		}
		for index, rawItem := range items {
			item, ok := rawItem.(map[string]any)
			if !ok {
				return fmt.Errorf("declarative view item %d must be an object", index)
			}
			for key, value := range item {
				if !map[string]bool{"id": true, "title": true, "description": true, "badge": true, "parentId": true}[key] {
					return fmt.Errorf("unknown item property %q", key)
				}
				if _, ok := value.(string); !ok {
					return fmt.Errorf("declarative view item %d property %s must be a string", index, key)
				}
			}
			if _, ok := item["id"].(string); !ok {
				return fmt.Errorf("declarative view item %d requires id", index)
			}
			if _, ok := item["title"].(string); !ok {
				return fmt.Errorf("declarative view item %d requires title", index)
			}
		}
	}
	if raw, exists := state["fields"]; exists {
		fields, ok := raw.([]any)
		if !ok || len(fields) > 100 {
			return fmt.Errorf("declarative form fields must be an array of at most 100 entries")
		}
		for index, rawField := range fields {
			field, ok := rawField.(map[string]any)
			if !ok {
				return fmt.Errorf("declarative form field %d must be an object", index)
			}
			for key := range field {
				if !map[string]bool{"id": true, "label": true, "type": true, "value": true, "placeholder": true, "options": true}[key] {
					return fmt.Errorf("unknown form field property %q", key)
				}
			}
			for _, key := range []string{"id", "label", "type"} {
				if _, ok := field[key].(string); !ok {
					return fmt.Errorf("declarative form field %d requires string %s", index, key)
				}
			}
			fieldType, _ := field["type"].(string)
			if !map[string]bool{"text": true, "number": true, "boolean": true, "select": true}[fieldType] {
				return fmt.Errorf("unsupported form field type %q", fieldType)
			}
			if options, exists := field["options"]; exists {
				if values, ok := options.([]any); !ok || len(values) > 100 {
					return fmt.Errorf("invalid form field options")
				} else {
					for _, value := range values {
						if _, ok := value.(string); !ok {
							return fmt.Errorf("form field options must be strings")
						}
					}
				}
			}
		}
	}
	if raw, exists := state["actions"]; exists {
		actions, ok := raw.([]any)
		if !ok || len(actions) > 20 {
			return fmt.Errorf("declarative form actions must be an array of at most 20 entries")
		}
		for index, rawAction := range actions {
			action, ok := rawAction.(map[string]any)
			if !ok {
				return fmt.Errorf("declarative form action %d must be an object", index)
			}
			for key := range action {
				if key != "id" && key != "title" && key != "command" {
					return fmt.Errorf("unknown form action property %q", key)
				}
			}
			for _, key := range []string{"id", "title", "command"} {
				if _, ok := action[key].(string); !ok {
					return fmt.Errorf("declarative form action %d requires string %s", index, key)
				}
			}
		}
	}
	return nil
}
