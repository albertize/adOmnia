package extensions

import "testing"

func TestValidateDeclarativeViewState(t *testing.T) {
	valid := map[string]any{
		"kind":    "table",
		"columns": []any{map[string]any{"key": "status", "title": "Status"}},
		"rows":    []any{map[string]any{"status": 200.0}},
	}
	if err := ValidateDeclarativeViewState(valid); err != nil {
		t.Fatalf("valid state rejected: %v", err)
	}
	form := map[string]any{
		"kind":    "form",
		"fields":  []any{map[string]any{"id": "severity", "label": "Severity", "type": "select", "options": []any{"low", "high"}}},
		"actions": []any{map[string]any{"id": "run", "title": "Run", "command": "test.extension.run"}},
	}
	if err := ValidateDeclarativeViewState(form); err != nil {
		t.Fatalf("valid form rejected: %v", err)
	}
	for _, invalid := range []any{
		map[string]any{},
		map[string]any{"kind": "html"},
		map[string]any{"kind": "list", "unknown": true},
		map[string]any{"kind": "list", "items": []any{map[string]any{"id": "one"}}},
		map[string]any{"kind": "table", "columns": []any{map[string]any{"key": "one", "title": "One", "script": "bad"}}},
		map[string]any{"kind": "form", "fields": []any{map[string]any{"id": "x", "label": "X", "type": "html"}}},
		map[string]any{"kind": "form", "actions": []any{map[string]any{"id": "run", "title": "Run"}}},
	} {
		if err := ValidateDeclarativeViewState(invalid); err == nil {
			t.Fatalf("invalid state accepted: %#v", invalid)
		}
	}
}

func FuzzValidateDeclarativeViewState(f *testing.F) {
	f.Add("table", "title")
	f.Add("html", "<script>")
	f.Fuzz(func(t *testing.T, kind, title string) {
		_ = ValidateDeclarativeViewState(map[string]any{"kind": kind, "title": title})
	})
}
