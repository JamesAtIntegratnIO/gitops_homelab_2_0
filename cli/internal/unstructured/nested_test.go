package unstructured

import (
	"testing"
)

func TestNestedField(t *testing.T) {
	obj := map[string]interface{}{
		"status": map[string]interface{}{
			"sync": map[string]interface{}{
				"status": "Synced",
			},
		},
		"name": "not-a-map",
		"key":  "value",
	}

	t.Run("deep nested", func(t *testing.T) {
		val, found, err := NestedField(obj, "status", "sync", "status")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !found {
			t.Fatal("expected found")
		}
		if val != "Synced" {
			t.Errorf("got %v, want Synced", val)
		}
	})

	t.Run("missing field", func(t *testing.T) {
		_, found, err := NestedField(obj, "status", "missing")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if found {
			t.Error("expected not found")
		}
	})

	t.Run("empty path returns obj", func(t *testing.T) {
		val, found, err := NestedField(obj)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !found {
			t.Fatal("expected found for empty path")
		}
		m, ok := val.(map[string]interface{})
		if !ok {
			t.Fatalf("expected map, got %T", val)
		}
		if m["key"] != "value" {
			t.Error("expected original object")
		}
	})

	t.Run("type mismatch traversal", func(t *testing.T) {
		_, found, err := NestedField(obj, "name", "subfield")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if found {
			t.Error("expected not found when traversing through non-map")
		}
	})
}

func TestNestedString(t *testing.T) {
	obj := map[string]interface{}{
		"status": map[string]interface{}{
			"phase": "Running",
		},
		"count": 42,
	}

	t.Run("valid string", func(t *testing.T) {
		val, found, err := NestedString(obj, "status", "phase")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !found || val != "Running" {
			t.Errorf("got (%q, %v), want (Running, true)", val, found)
		}
	})

	t.Run("not a string", func(t *testing.T) {
		_, found, err := NestedString(obj, "count")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if found {
			t.Error("expected not found for non-string value")
		}
	})

	t.Run("missing", func(t *testing.T) {
		_, found, err := NestedString(obj, "missing")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if found {
			t.Error("expected not found")
		}
	})
}

func TestNestedSlice(t *testing.T) {
	obj := map[string]interface{}{
		"items": []interface{}{"a", "b", "c"},
	}

	val, found, err := NestedSlice(obj, "items")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Fatal("expected found")
	}
	if len(val) != 3 {
		t.Errorf("got %d items, want 3", len(val))
	}
}

func TestNestedInt64(t *testing.T) {
	obj := map[string]interface{}{
		"retryCount": int64(5),
		"floatVal":   float64(3.0),
		"intVal":     7,
		"strVal":     "not-a-number",
	}

	tests := []struct {
		name  string
		field string
		want  int64
		found bool
	}{
		{"int64", "retryCount", 5, true},
		{"float64", "floatVal", 3, true},
		{"int", "intVal", 7, true},
		{"wrong type", "strVal", 0, false},
		{"missing", "missing", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, found, err := NestedInt64(obj, tt.field)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if found != tt.found {
				t.Errorf("found = %v, want %v", found, tt.found)
			}
			if found && val != tt.want {
				t.Errorf("got %d, want %d", val, tt.want)
			}
		})
	}
}

func TestNestedBool(t *testing.T) {
	obj := map[string]interface{}{
		"spec": map[string]interface{}{
			"selfHeal": true,
		},
		"strVal": "not-a-bool",
	}

	t.Run("valid bool", func(t *testing.T) {
		val, found, err := NestedBool(obj, "spec", "selfHeal")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !found || !val {
			t.Errorf("got (%v, %v), want (true, true)", val, found)
		}
	})

	t.Run("wrong type", func(t *testing.T) {
		_, found, err := NestedBool(obj, "strVal")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if found {
			t.Error("expected not found for non-bool")
		}
	})
}

func TestMustString(t *testing.T) {
	obj := map[string]interface{}{
		"status": map[string]interface{}{
			"phase": "Running",
		},
		"count": 42,
	}

	t.Run("returns value when present", func(t *testing.T) {
		val := MustString(obj, "status", "phase")
		if val != "Running" {
			t.Errorf("got %q, want %q", val, "Running")
		}
	})

	t.Run("returns empty string for wrong type", func(t *testing.T) {
		val := MustString(obj, "count")
		if val != "" {
			t.Errorf("got %q, want empty string", val)
		}
	})

	t.Run("returns empty string for missing key", func(t *testing.T) {
		val := MustString(obj, "missing", "path")
		if val != "" {
			t.Errorf("got %q, want empty string", val)
		}
	})

	t.Run("returns empty string for nil obj", func(t *testing.T) {
		val := MustString(nil)
		if val != "" {
			t.Errorf("got %q, want empty string", val)
		}
	})
}

func TestMustSlice(t *testing.T) {
	obj := map[string]interface{}{
		"items":  []interface{}{"a", "b", "c"},
		"notArr": "string-value",
	}

	t.Run("returns slice when present", func(t *testing.T) {
		val := MustSlice(obj, "items")
		if len(val) != 3 {
			t.Errorf("got %d items, want 3", len(val))
		}
	})

	t.Run("returns nil for wrong type", func(t *testing.T) {
		val := MustSlice(obj, "notArr")
		if val != nil {
			t.Errorf("got %v, want nil", val)
		}
	})

	t.Run("returns nil for missing key", func(t *testing.T) {
		val := MustSlice(obj, "missing")
		if val != nil {
			t.Errorf("got %v, want nil", val)
		}
	})

	t.Run("returns nil for nil obj", func(t *testing.T) {
		val := MustSlice(nil)
		if val != nil {
			t.Errorf("got %v, want nil", val)
		}
	})
}
