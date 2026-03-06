package kratixutil

import (
	"strings"
	"testing"
)

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

// ---------------------------------------------------------------------------
// ParseSyncPolicyE tests
// ---------------------------------------------------------------------------

func TestParseSyncPolicyE(t *testing.T) {
	tests := []struct {
		name      string
		raw       interface{}
		wantNil   bool
		wantErr   bool
		errSubstr string
	}{
		{
			name:    "nil returns nil",
			raw:     nil,
			wantNil: true,
		},
		{
			name: "valid full policy",
			raw: map[string]interface{}{
				"automated": map[string]interface{}{
					"selfHeal": true,
					"prune":    true,
				},
				"syncOptions": []interface{}{"CreateNamespace=true"},
			},
		},
		{
			name:    "empty map",
			raw:     map[string]interface{}{},
			wantNil: false,
		},
		{
			name:      "wrong type string",
			raw:       "not-a-map",
			wantErr:   true,
			errSubstr: "syncPolicy",
		},
		{
			name:      "wrong type int",
			raw:       42,
			wantErr:   true,
			errSubstr: "syncPolicy",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSyncPolicyE(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("error %q should contain %q", err, tt.errSubstr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantNil {
				if got != nil {
					t.Errorf("expected nil, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected non-nil SyncPolicy")
			}
		})
	}
}

func TestParseSyncPolicyE_Values(t *testing.T) {
	raw := map[string]interface{}{
		"automated": map[string]interface{}{
			"selfHeal": true,
			"prune":    false,
		},
		"syncOptions": []interface{}{"CreateNamespace=true", "PruneLast=true"},
	}
	sp, err := ParseSyncPolicyE(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sp.Automated == nil {
		t.Fatal("expected Automated to be set")
	}
	if !sp.Automated.SelfHeal {
		t.Error("expected SelfHeal=true")
	}
	if sp.Automated.Prune {
		t.Error("expected Prune=false")
	}
	if len(sp.SyncOptions) != 2 {
		t.Fatalf("expected 2 sync options, got %d", len(sp.SyncOptions))
	}
	if sp.SyncOptions[0] != "CreateNamespace=true" {
		t.Errorf("unexpected syncOption[0]: %q", sp.SyncOptions[0])
	}
}

func TestParseSyncPolicyE_WrongTypeInnerFields(t *testing.T) {
	tests := []struct {
		name         string
		input        map[string]interface{}
		wantContains string
	}{
		{
			name:         "automated wrong type",
			input:        map[string]interface{}{"automated": "not-a-map"},
			wantContains: "syncPolicy",
		},
		{
			name:         "selfHeal wrong type",
			input:        map[string]interface{}{"automated": map[string]interface{}{"selfHeal": "not-bool"}},
			wantContains: "syncPolicy",
		},
		{
			name:         "prune wrong type",
			input:        map[string]interface{}{"automated": map[string]interface{}{"prune": 42}},
			wantContains: "syncPolicy",
		},
		{
			name:         "syncOptions wrong type",
			input:        map[string]interface{}{"syncOptions": "not-a-slice"},
			wantContains: "syncPolicy",
		},
		{
			name:         "syncOptions element wrong type",
			input:        map[string]interface{}{"syncOptions": []interface{}{123}},
			wantContains: "syncPolicy",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseSyncPolicyE(tt.input)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantContains) {
				t.Errorf("error %q should contain %q", err.Error(), tt.wantContains)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ToMap tests
// ---------------------------------------------------------------------------

func TestToMap(t *testing.T) {
	tests := []struct {
		name    string
		input   interface{}
		wantErr bool
		check   func(t *testing.T, m map[string]interface{})
	}{
		{
			name: "simple struct",
			input: struct {
				Name string `json:"name"`
				Age  int    `json:"age"`
			}{Name: "test", Age: 42},
			check: func(t *testing.T, m map[string]interface{}) {
				if m["name"] != "test" {
					t.Errorf("name = %v, want test", m["name"])
				}
				// JSON numbers are float64
				if m["age"] != float64(42) {
					t.Errorf("age = %v, want 42", m["age"])
				}
			},
		},
		{
			name: "nested struct",
			input: struct {
				Outer string `json:"outer"`
				Inner struct {
					Val string `json:"val"`
				} `json:"inner"`
			}{Outer: "a", Inner: struct {
				Val string `json:"val"`
			}{Val: "b"}},
			check: func(t *testing.T, m map[string]interface{}) {
				inner, ok := m["inner"].(map[string]interface{})
				if !ok {
					t.Fatal("inner should be a map")
				}
				if inner["val"] != "b" {
					t.Errorf("inner.val = %v, want b", inner["val"])
				}
			},
		},
		{
			name:    "channel is not JSON-marshalable",
			input:   make(chan int),
			wantErr: true,
		},
		{
			name:  "nil input returns nil map without error",
			input: nil,
			check: func(t *testing.T, m map[string]interface{}) {
				if m != nil {
					t.Errorf("expected nil map, got %v", m)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := ToMap(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.check != nil {
				tt.check(t, m)
			}
		})
	}
}
