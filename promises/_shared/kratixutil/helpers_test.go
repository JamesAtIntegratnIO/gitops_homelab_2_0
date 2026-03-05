package kratixutil

import (
	"testing"
)

// newMockResource creates a MockResource from a nested map literal.
func newMockResource(data map[string]interface{}) *MockResource {
	return &MockResource{Data: data}
}

// ============================================================================
// GetStringValue
// ============================================================================

func TestGetStringValue_Success(t *testing.T) {
	r := newMockResource(map[string]interface{}{
		"spec": map[string]interface{}{
			"name": "my-app",
		},
	})
	val, err := GetStringValue(r, "spec.name")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "my-app" {
		t.Errorf("expected %q, got %q", "my-app", val)
	}
}

func TestGetStringValue_NotFound(t *testing.T) {
	r := newMockResource(map[string]interface{}{})
	_, err := GetStringValue(r, "spec.missing")
	if err == nil {
		t.Fatal("expected error for missing path")
	}
}

func TestGetStringValue_NotAString(t *testing.T) {
	r := newMockResource(map[string]interface{}{
		"spec": map[string]interface{}{
			"count": 42,
		},
	})
	_, err := GetStringValue(r, "spec.count")
	if err == nil {
		t.Fatal("expected error for non-string value")
	}
	if err.Error() != "spec.count is not a string" {
		t.Errorf("unexpected error message: %v", err)
	}
}

// ============================================================================
// GetStringValueWithDefault
// ============================================================================

func TestGetStringValueWithDefault_ReturnsValue(t *testing.T) {
	r := newMockResource(map[string]interface{}{
		"spec": map[string]interface{}{
			"name": "found",
		},
	})
	val := GetStringValueWithDefault(r, "spec.name", "default")
	if val != "found" {
		t.Errorf("expected %q, got %q", "found", val)
	}
}

func TestGetStringValueWithDefault_UsesDefaultOnMissing(t *testing.T) {
	r := newMockResource(map[string]interface{}{})
	val := GetStringValueWithDefault(r, "spec.missing", "fallback")
	if val != "fallback" {
		t.Errorf("expected %q, got %q", "fallback", val)
	}
}

func TestGetStringValueWithDefault_UsesDefaultOnEmpty(t *testing.T) {
	r := newMockResource(map[string]interface{}{
		"spec": map[string]interface{}{
			"name": "",
		},
	})
	val := GetStringValueWithDefault(r, "spec.name", "fallback")
	if val != "fallback" {
		t.Errorf("expected %q, got %q", "fallback", val)
	}
}

func TestGetStringValueWithDefault_TreatsNullAsEmpty(t *testing.T) {
	r := newMockResource(map[string]interface{}{
		"spec": map[string]interface{}{
			"name": "null",
		},
	})
	val := GetStringValueWithDefault(r, "spec.name", "fallback")
	if val != "fallback" {
		t.Errorf("expected %q, got %q", "fallback", val)
	}
}

// ============================================================================
// GetIntValue
// ============================================================================

func TestGetIntValue_Int(t *testing.T) {
	r := newMockResource(map[string]interface{}{
		"spec": map[string]interface{}{"count": 5},
	})
	val, err := GetIntValue(r, "spec.count")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != 5 {
		t.Errorf("expected 5, got %d", val)
	}
}

func TestGetIntValue_Float64(t *testing.T) {
	r := newMockResource(map[string]interface{}{
		"spec": map[string]interface{}{"count": float64(42)},
	})
	val, err := GetIntValue(r, "spec.count")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != 42 {
		t.Errorf("expected 42, got %d", val)
	}
}

func TestGetIntValue_Int64(t *testing.T) {
	r := newMockResource(map[string]interface{}{
		"spec": map[string]interface{}{"count": int64(99)},
	})
	val, err := GetIntValue(r, "spec.count")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != 99 {
		t.Errorf("expected 99, got %d", val)
	}
}

func TestGetIntValue_String(t *testing.T) {
	r := newMockResource(map[string]interface{}{
		"spec": map[string]interface{}{"count": "77"},
	})
	val, err := GetIntValue(r, "spec.count")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != 77 {
		t.Errorf("expected 77, got %d", val)
	}
}

func TestGetIntValue_InvalidString(t *testing.T) {
	r := newMockResource(map[string]interface{}{
		"spec": map[string]interface{}{"count": "not-a-number"},
	})
	_, err := GetIntValue(r, "spec.count")
	if err == nil {
		t.Fatal("expected error for non-numeric string")
	}
}

func TestGetIntValue_UnsupportedType(t *testing.T) {
	r := newMockResource(map[string]interface{}{
		"spec": map[string]interface{}{"count": true},
	})
	_, err := GetIntValue(r, "spec.count")
	if err == nil {
		t.Fatal("expected error for unsupported type")
	}
}

func TestGetIntValue_NotFound(t *testing.T) {
	r := newMockResource(map[string]interface{}{})
	_, err := GetIntValue(r, "spec.missing")
	if err == nil {
		t.Fatal("expected error for missing path")
	}
}

// ============================================================================
// GetIntValueWithDefault
// ============================================================================

func TestGetIntValueWithDefault_ReturnsValue(t *testing.T) {
	r := newMockResource(map[string]interface{}{
		"spec": map[string]interface{}{"replicas": 3},
	})
	val := GetIntValueWithDefault(r, "spec.replicas", 1)
	if val != 3 {
		t.Errorf("expected 3, got %d", val)
	}
}

func TestGetIntValueWithDefault_UsesDefaultOnMissing(t *testing.T) {
	r := newMockResource(map[string]interface{}{})
	val := GetIntValueWithDefault(r, "spec.missing", 10)
	if val != 10 {
		t.Errorf("expected 10, got %d", val)
	}
}

func TestGetIntValueWithDefault_ReturnsExplicitZero(t *testing.T) {
	r := newMockResource(map[string]interface{}{
		"spec": map[string]interface{}{"replicas": 0},
	})
	val := GetIntValueWithDefault(r, "spec.replicas", 5)
	if val != 0 {
		t.Errorf("expected 0 (explicit zero preserved), got %d", val)
	}
}

// ============================================================================
// GetBoolValue
// ============================================================================

func TestGetBoolValue_True(t *testing.T) {
	r := newMockResource(map[string]interface{}{
		"spec": map[string]interface{}{"enabled": true},
	})
	val, err := GetBoolValue(r, "spec.enabled")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !val {
		t.Error("expected true")
	}
}

func TestGetBoolValue_False(t *testing.T) {
	r := newMockResource(map[string]interface{}{
		"spec": map[string]interface{}{"enabled": false},
	})
	val, err := GetBoolValue(r, "spec.enabled")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val {
		t.Error("expected false")
	}
}

func TestGetBoolValue_NotBool(t *testing.T) {
	r := newMockResource(map[string]interface{}{
		"spec": map[string]interface{}{"enabled": "yes"},
	})
	_, err := GetBoolValue(r, "spec.enabled")
	if err == nil {
		t.Fatal("expected error for non-bool")
	}
}

func TestGetBoolValue_NotFound(t *testing.T) {
	r := newMockResource(map[string]interface{}{})
	_, err := GetBoolValue(r, "spec.missing")
	if err == nil {
		t.Fatal("expected error for missing path")
	}
}

// ============================================================================
// GetBoolValueWithDefault
// ============================================================================

func TestGetBoolValueWithDefault_ReturnsValue(t *testing.T) {
	r := newMockResource(map[string]interface{}{
		"spec": map[string]interface{}{"debug": true},
	})
	val := GetBoolValueWithDefault(r, "spec.debug", false)
	if !val {
		t.Error("expected true")
	}
}

func TestGetBoolValueWithDefault_UsesDefaultOnMissing(t *testing.T) {
	r := newMockResource(map[string]interface{}{})
	val := GetBoolValueWithDefault(r, "spec.missing", true)
	if !val {
		t.Error("expected true (default)")
	}
}

func TestGetBoolValueWithDefault_UsesDefaultOnNonBool(t *testing.T) {
	r := newMockResource(map[string]interface{}{
		"spec": map[string]interface{}{"flag": "yes"},
	})
	val := GetBoolValueWithDefault(r, "spec.flag", true)
	if !val {
		t.Error("expected true (default for non-bool)")
	}
}

// ============================================================================
// ExtractStringMap
// ============================================================================

func TestExtractStringMap_Success(t *testing.T) {
	r := newMockResource(map[string]interface{}{
		"spec": map[string]interface{}{
			"labels": map[string]interface{}{
				"app":  "nginx",
				"tier": "frontend",
			},
		},
	})
	m := ExtractStringMap(r, "spec.labels")
	if m == nil {
		t.Fatal("expected non-nil map")
	}
	if m["app"] != "nginx" || m["tier"] != "frontend" {
		t.Errorf("unexpected map contents: %v", m)
	}
}

func TestExtractStringMap_MissingPath(t *testing.T) {
	r := newMockResource(map[string]interface{}{})
	m := ExtractStringMap(r, "spec.missing")
	if m != nil {
		t.Errorf("expected nil, got %v", m)
	}
}

func TestExtractStringMap_NotAMap(t *testing.T) {
	r := newMockResource(map[string]interface{}{
		"spec": map[string]interface{}{"labels": "not-a-map"},
	})
	m := ExtractStringMap(r, "spec.labels")
	if m != nil {
		t.Errorf("expected nil, got %v", m)
	}
}

func TestExtractStringMap_EmptyMap(t *testing.T) {
	r := newMockResource(map[string]interface{}{
		"spec": map[string]interface{}{
			"labels": map[string]interface{}{},
		},
	})
	m := ExtractStringMap(r, "spec.labels")
	if m != nil {
		t.Errorf("expected nil for empty map, got %v", m)
	}
}

func TestExtractStringMap_SkipsNonStringValues(t *testing.T) {
	r := newMockResource(map[string]interface{}{
		"spec": map[string]interface{}{
			"labels": map[string]interface{}{
				"good": "val",
				"bad":  42,
			},
		},
	})
	m := ExtractStringMap(r, "spec.labels")
	if len(m) != 1 || m["good"] != "val" {
		t.Errorf("expected only 'good' key, got %v", m)
	}
}

// ============================================================================
// ExtractStringSlice
// ============================================================================

func TestExtractStringSlice_Success(t *testing.T) {
	r := newMockResource(map[string]interface{}{
		"spec": map[string]interface{}{
			"repos": []interface{}{"repo1", "repo2"},
		},
	})
	s := ExtractStringSlice(r, "spec.repos")
	if len(s) != 2 || s[0] != "repo1" || s[1] != "repo2" {
		t.Errorf("unexpected slice: %v", s)
	}
}

func TestExtractStringSlice_MissingPath(t *testing.T) {
	r := newMockResource(map[string]interface{}{})
	s := ExtractStringSlice(r, "spec.missing")
	if s != nil {
		t.Errorf("expected nil, got %v", s)
	}
}

func TestExtractStringSlice_NotAnArray(t *testing.T) {
	r := newMockResource(map[string]interface{}{
		"spec": map[string]interface{}{"repos": "single"},
	})
	s := ExtractStringSlice(r, "spec.repos")
	if s != nil {
		t.Errorf("expected nil, got %v", s)
	}
}

func TestExtractStringSlice_FiltersNonStrings(t *testing.T) {
	r := newMockResource(map[string]interface{}{
		"spec": map[string]interface{}{
			"items": []interface{}{"a", 42, "b"},
		},
	})
	s := ExtractStringSlice(r, "spec.items")
	if len(s) != 2 || s[0] != "a" || s[1] != "b" {
		t.Errorf("expected [a b], got %v", s)
	}
}

// ============================================================================
// ExtractObjectSlice
// ============================================================================

func TestExtractObjectSlice_Success(t *testing.T) {
	r := newMockResource(map[string]interface{}{
		"spec": map[string]interface{}{
			"destinations": []interface{}{
				map[string]interface{}{"server": "*", "namespace": "default"},
				map[string]interface{}{"server": "*", "namespace": "kube-system"},
			},
		},
	})
	s := ExtractObjectSlice(r, "spec.destinations")
	if len(s) != 2 {
		t.Fatalf("expected 2 objects, got %d", len(s))
	}
	if s[0]["server"] != "*" || s[1]["namespace"] != "kube-system" {
		t.Errorf("unexpected contents: %v", s)
	}
}

func TestExtractObjectSlice_MissingPath(t *testing.T) {
	r := newMockResource(map[string]interface{}{})
	s := ExtractObjectSlice(r, "spec.missing")
	if s != nil {
		t.Errorf("expected nil, got %v", s)
	}
}

func TestExtractObjectSlice_NotAnArray(t *testing.T) {
	r := newMockResource(map[string]interface{}{
		"spec": map[string]interface{}{"items": "string"},
	})
	s := ExtractObjectSlice(r, "spec.items")
	if s != nil {
		t.Errorf("expected nil, got %v", s)
	}
}

func TestExtractObjectSlice_FiltersNonMaps(t *testing.T) {
	r := newMockResource(map[string]interface{}{
		"spec": map[string]interface{}{
			"items": []interface{}{
				map[string]interface{}{"key": "val"},
				"not-a-map",
				42,
			},
		},
	})
	s := ExtractObjectSlice(r, "spec.items")
	if len(s) != 1 {
		t.Errorf("expected 1 map, got %d", len(s))
	}
}

// ============================================================================
// ExtractSecrets
// ============================================================================

func TestExtractSecrets_FullParse(t *testing.T) {
	r := newMockResource(map[string]interface{}{
		"spec": map[string]interface{}{
			"secrets": []interface{}{
				map[string]interface{}{
					"name":            "db-creds",
					"onePasswordItem": "vault-item-1",
					"keys": []interface{}{
						map[string]interface{}{
							"secretKey": "password",
							"property":  "password",
						},
						map[string]interface{}{
							"secretKey": "username",
							"property":  "username",
						},
					},
				},
			},
		},
	})
	secrets := ExtractSecrets(r, "spec.secrets")
	if len(secrets) != 1 {
		t.Fatalf("expected 1 secret, got %d", len(secrets))
	}
	s := secrets[0]
	if s.Name != "db-creds" {
		t.Errorf("expected name 'db-creds', got %q", s.Name)
	}
	if s.OnePasswordItem != "vault-item-1" {
		t.Errorf("expected onePasswordItem 'vault-item-1', got %q", s.OnePasswordItem)
	}
	if len(s.Keys) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(s.Keys))
	}
	if s.Keys[0].SecretKey != "password" || s.Keys[0].Property != "password" {
		t.Errorf("unexpected key[0]: %+v", s.Keys[0])
	}
	if s.Keys[1].SecretKey != "username" || s.Keys[1].Property != "username" {
		t.Errorf("unexpected key[1]: %+v", s.Keys[1])
	}
}

func TestExtractSecrets_MissingPath(t *testing.T) {
	r := newMockResource(map[string]interface{}{})
	secrets := ExtractSecrets(r, "spec.missing")
	if secrets != nil {
		t.Errorf("expected nil, got %v", secrets)
	}
}

func TestExtractSecrets_NotAnArray(t *testing.T) {
	r := newMockResource(map[string]interface{}{
		"spec": map[string]interface{}{"secrets": "not-array"},
	})
	secrets := ExtractSecrets(r, "spec.secrets")
	if secrets != nil {
		t.Errorf("expected nil, got %v", secrets)
	}
}

func TestExtractSecrets_EmptyKeys(t *testing.T) {
	r := newMockResource(map[string]interface{}{
		"spec": map[string]interface{}{
			"secrets": []interface{}{
				map[string]interface{}{
					"name":            "minimal",
					"onePasswordItem": "item",
				},
			},
		},
	})
	secrets := ExtractSecrets(r, "spec.secrets")
	if len(secrets) != 1 {
		t.Fatalf("expected 1 secret, got %d", len(secrets))
	}
	if len(secrets[0].Keys) != 0 {
		t.Errorf("expected 0 keys, got %d", len(secrets[0].Keys))
	}
}

func TestExtractSecrets_SkipsNonMapItems(t *testing.T) {
	r := newMockResource(map[string]interface{}{
		"spec": map[string]interface{}{
			"secrets": []interface{}{
				"not-a-map",
				map[string]interface{}{"name": "good"},
			},
		},
	})
	secrets := ExtractSecrets(r, "spec.secrets")
	if len(secrets) != 1 {
		t.Fatalf("expected 1 secret (skip non-map), got %d", len(secrets))
	}
	if secrets[0].Name != "good" {
		t.Errorf("expected name 'good', got %q", secrets[0].Name)
	}
}

// ============================================================================
// DeepMerge
// ============================================================================

func TestDeepMerge_SimpleOverride(t *testing.T) {
	dst := map[string]interface{}{"a": "1", "b": "2"}
	src := map[string]interface{}{"b": "3", "c": "4"}
	result := DeepMerge(dst, src)
	if result["a"] != "1" || result["b"] != "3" || result["c"] != "4" {
		t.Errorf("unexpected merge: %v", result)
	}
}

func TestDeepMerge_NestedMaps(t *testing.T) {
	dst := map[string]interface{}{
		"top": map[string]interface{}{"a": "1", "b": "2"},
	}
	src := map[string]interface{}{
		"top": map[string]interface{}{"b": "3", "c": "4"},
	}
	result := DeepMerge(dst, src)
	top, ok := result["top"].(map[string]interface{})
	if !ok {
		t.Fatal("expected nested map")
	}
	if top["a"] != "1" || top["b"] != "3" || top["c"] != "4" {
		t.Errorf("unexpected nested merge: %v", top)
	}
}

func TestDeepMerge_NilDst(t *testing.T) {
	src := map[string]interface{}{"a": "1"}
	result := DeepMerge(nil, src)
	if result["a"] != "1" {
		t.Errorf("expected 'a'='1', got %v", result)
	}
}

func TestDeepMerge_NilSrc(t *testing.T) {
	dst := map[string]interface{}{"a": "1"}
	result := DeepMerge(dst, nil)
	if result["a"] != "1" {
		t.Errorf("expected unchanged dst, got %v", result)
	}
}

func TestDeepMerge_BothNil(t *testing.T) {
	result := DeepMerge(nil, nil)
	if result == nil {
		t.Error("expected non-nil empty map")
	}
	if len(result) != 0 {
		t.Errorf("expected empty map, got %v", result)
	}
}

func TestDeepMerge_SrcOverridesNonMap(t *testing.T) {
	dst := map[string]interface{}{
		"key": "string-value",
	}
	src := map[string]interface{}{
		"key": map[string]interface{}{"nested": "value"},
	}
	result := DeepMerge(dst, src)
	nested, ok := result["key"].(map[string]interface{})
	if !ok {
		t.Fatal("expected src map to override dst string")
	}
	if nested["nested"] != "value" {
		t.Errorf("unexpected value: %v", nested)
	}
}

func TestDeepMerge_DoesNotMutateDst(t *testing.T) {
	dst := map[string]interface{}{"a": "1"}
	src := map[string]interface{}{"b": "2"}
	_ = DeepMerge(dst, src)
	if _, exists := dst["b"]; exists {
		t.Error("DeepMerge should not mutate dst")
	}
}

// ============================================================================
// MergeStringMap
// ============================================================================

func TestMergeStringMap_MergesInto(t *testing.T) {
	dst := map[string]string{"a": "1"}
	src := map[string]string{"b": "2"}
	result := MergeStringMap(dst, src)
	if result["a"] != "1" || result["b"] != "2" {
		t.Errorf("unexpected merge: %v", result)
	}
}

func TestMergeStringMap_SrcOverrides(t *testing.T) {
	dst := map[string]string{"a": "old"}
	src := map[string]string{"a": "new"}
	result := MergeStringMap(dst, src)
	if result["a"] != "new" {
		t.Errorf("expected 'new', got %q", result["a"])
	}
}

func TestMergeStringMap_NilDst(t *testing.T) {
	src := map[string]string{"a": "1"}
	result := MergeStringMap(nil, src)
	if result["a"] != "1" {
		t.Errorf("expected 'a'='1', got %v", result)
	}
}

func TestMergeStringMap_NilSrc(t *testing.T) {
	dst := map[string]string{"a": "1"}
	result := MergeStringMap(dst, nil)
	if result["a"] != "1" {
		t.Errorf("expected unchanged, got %v", result)
	}
}
