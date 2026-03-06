package kratixutil

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Extract*E tests
// ---------------------------------------------------------------------------

func TestExtractStringE(t *testing.T) {
	tests := []struct {
		name      string
		data      map[string]interface{}
		key       string
		want      string
		wantErr   bool
		errSubstr string
	}{
		{name: "present string", data: map[string]interface{}{"k": "hello"}, key: "k", want: "hello"},
		{name: "missing key", data: map[string]interface{}{}, key: "k", want: ""},
		{name: "nil value", data: map[string]interface{}{"k": nil}, key: "k", want: ""},
		{name: "wrong type int", data: map[string]interface{}{"k": 42}, key: "k", wantErr: true, errSubstr: "expected string"},
		{name: "wrong type bool", data: map[string]interface{}{"k": true}, key: "k", wantErr: true, errSubstr: "expected string"},
		{name: "empty string", data: map[string]interface{}{"k": ""}, key: "k", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractStringE(tt.data, tt.key)
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
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractIntE(t *testing.T) {
	tests := []struct {
		name      string
		data      map[string]interface{}
		key       string
		want      int
		wantErr   bool
		errSubstr string
	}{
		{name: "int value", data: map[string]interface{}{"k": 7}, key: "k", want: 7},
		{name: "float64 value", data: map[string]interface{}{"k": float64(42)}, key: "k", want: 42},
		{name: "int64 value", data: map[string]interface{}{"k": int64(99)}, key: "k", want: 99},
		{name: "missing key", data: map[string]interface{}{}, key: "k", want: 0},
		{name: "nil value", data: map[string]interface{}{"k": nil}, key: "k", want: 0},
		{name: "wrong type string", data: map[string]interface{}{"k": "nope"}, key: "k", wantErr: true, errSubstr: "expected int"},
		{name: "wrong type bool", data: map[string]interface{}{"k": true}, key: "k", wantErr: true, errSubstr: "expected int"},
		{name: "zero int", data: map[string]interface{}{"k": 0}, key: "k", want: 0},
		{name: "zero float64", data: map[string]interface{}{"k": float64(0)}, key: "k", want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractIntE(tt.data, tt.key)
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
			if got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestExtractBoolE(t *testing.T) {
	tests := []struct {
		name      string
		data      map[string]interface{}
		key       string
		want      bool
		wantErr   bool
		errSubstr string
	}{
		{name: "true", data: map[string]interface{}{"k": true}, key: "k", want: true},
		{name: "false", data: map[string]interface{}{"k": false}, key: "k", want: false},
		{name: "missing key", data: map[string]interface{}{}, key: "k", want: false},
		{name: "nil value", data: map[string]interface{}{"k": nil}, key: "k", want: false},
		{name: "wrong type string", data: map[string]interface{}{"k": "yes"}, key: "k", wantErr: true, errSubstr: "expected bool"},
		{name: "wrong type int", data: map[string]interface{}{"k": 1}, key: "k", wantErr: true, errSubstr: "expected bool"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractBoolE(tt.data, tt.key)
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
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractMapE(t *testing.T) {
	nested := map[string]interface{}{"a": "1"}
	tests := []struct {
		name      string
		data      map[string]interface{}
		key       string
		wantNil   bool
		wantErr   bool
		errSubstr string
	}{
		{name: "present map", data: map[string]interface{}{"k": nested}, key: "k"},
		{name: "missing key", data: map[string]interface{}{}, key: "k", wantNil: true},
		{name: "nil value", data: map[string]interface{}{"k": nil}, key: "k", wantNil: true},
		{name: "wrong type string", data: map[string]interface{}{"k": "oops"}, key: "k", wantErr: true, errSubstr: "expected map[string]interface{}"},
		{name: "wrong type int", data: map[string]interface{}{"k": 42}, key: "k", wantErr: true, errSubstr: "expected map[string]interface{}"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractMapE(tt.data, tt.key)
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
					t.Errorf("expected nil, got %v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected non-nil map")
			}
			if got["a"] != "1" {
				t.Errorf("expected map with a=1, got %v", got)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ExtractStringMapE tests
// ---------------------------------------------------------------------------

func TestExtractStringMapE(t *testing.T) {
	tests := []struct {
		name       string
		data       map[string]interface{}
		key        string
		wantNil    bool
		wantLen    int
		wantErr    bool
		errSubstr  string
		warnSubstr string
	}{
		{
			name:    "present map",
			data:    map[string]interface{}{"k": map[string]interface{}{"a": "1", "b": "2"}},
			key:     "k",
			wantLen: 2,
		},
		{name: "missing key", data: map[string]interface{}{}, key: "k", wantNil: true},
		{name: "nil value", data: map[string]interface{}{"k": nil}, key: "k", wantNil: true},
		{name: "wrong type string", data: map[string]interface{}{"k": "oops"}, key: "k", wantErr: true, errSubstr: "expected map[string]interface{}"},
		{name: "wrong type int", data: map[string]interface{}{"k": 42}, key: "k", wantErr: true, errSubstr: "expected map[string]interface{}"},
		{
			name:      "rejects non-string values",
			data:      map[string]interface{}{"k": map[string]interface{}{"good": "val", "bad": 42}},
			key:       "k",
			wantErr:   true,
			errSubstr: "expected string",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractStringMapE(tt.data, tt.key)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("error %q should contain %q", err, tt.errSubstr)
				}
				return
			}
			if tt.warnSubstr != "" {
				if err == nil {
					t.Fatal("expected warning error, got nil")
				}
				if !strings.Contains(err.Error(), tt.warnSubstr) {
					t.Errorf("warning error %q should contain %q", err, tt.warnSubstr)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantNil {
				if got != nil {
					t.Errorf("expected nil, got %v", got)
				}
				return
			}
			if len(got) != tt.wantLen {
				t.Errorf("expected len %d, got %d (%v)", tt.wantLen, len(got), got)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ExtractStringSliceE tests
// ---------------------------------------------------------------------------

func TestExtractStringSliceE(t *testing.T) {
	tests := []struct {
		name       string
		data       map[string]interface{}
		key        string
		wantNil    bool
		wantLen    int
		wantErr    bool
		errSubstr  string
		warnSubstr string
	}{
		{
			name:    "present slice",
			data:    map[string]interface{}{"k": []interface{}{"a", "b"}},
			key:     "k",
			wantLen: 2,
		},
		{name: "missing key", data: map[string]interface{}{}, key: "k", wantNil: true},
		{name: "nil value", data: map[string]interface{}{"k": nil}, key: "k", wantNil: true},
		{name: "wrong type string", data: map[string]interface{}{"k": "oops"}, key: "k", wantErr: true, errSubstr: "expected []interface{}"},
		{name: "wrong type int", data: map[string]interface{}{"k": 42}, key: "k", wantErr: true, errSubstr: "expected []interface{}"},
		{
			name:      "rejects non-strings",
			data:      map[string]interface{}{"k": []interface{}{"a", 42, "b"}},
			key:       "k",
			wantErr:   true,
			errSubstr: "expected string",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractStringSliceE(tt.data, tt.key)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("error %q should contain %q", err, tt.errSubstr)
				}
				return
			}
			if tt.warnSubstr != "" {
				if err == nil {
					t.Fatal("expected warning error, got nil")
				}
				if !strings.Contains(err.Error(), tt.warnSubstr) {
					t.Errorf("warning error %q should contain %q", err, tt.warnSubstr)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantNil {
				if got != nil {
					t.Errorf("expected nil, got %v", got)
				}
				return
			}
			if len(got) != tt.wantLen {
				t.Errorf("expected len %d, got %d (%v)", tt.wantLen, len(got), got)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ExtractObjectSliceE tests
// ---------------------------------------------------------------------------

func TestExtractObjectSliceE(t *testing.T) {
	tests := []struct {
		name       string
		data       map[string]interface{}
		key        string
		wantNil    bool
		wantLen    int
		wantErr    bool
		errSubstr  string
		warnSubstr string
	}{
		{
			name: "present slice",
			data: map[string]interface{}{"k": []interface{}{
				map[string]interface{}{"a": "1"},
				map[string]interface{}{"b": "2"},
			}},
			key:     "k",
			wantLen: 2,
		},
		{name: "missing key", data: map[string]interface{}{}, key: "k", wantNil: true},
		{name: "nil value", data: map[string]interface{}{"k": nil}, key: "k", wantNil: true},
		{name: "wrong type string", data: map[string]interface{}{"k": "oops"}, key: "k", wantErr: true, errSubstr: "expected []interface{}"},
		{name: "wrong type int", data: map[string]interface{}{"k": 42}, key: "k", wantErr: true, errSubstr: "expected []interface{}"},
		{
			name: "rejects non-map items",
			data: map[string]interface{}{"k": []interface{}{
				map[string]interface{}{"a": "1"},
				"not-a-map",
				42,
			}},
			key:       "k",
			wantErr:   true,
			errSubstr: "expected map",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractObjectSliceE(tt.data, tt.key)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("error %q should contain %q", err, tt.errSubstr)
				}
				return
			}
			if tt.warnSubstr != "" {
				if err == nil {
					t.Fatal("expected warning error, got nil")
				}
				if !strings.Contains(err.Error(), tt.warnSubstr) {
					t.Errorf("warning error %q should contain %q", err, tt.warnSubstr)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantNil {
				if got != nil {
					t.Errorf("expected nil, got %v", got)
				}
				return
			}
			if len(got) != tt.wantLen {
				t.Errorf("expected len %d, got %d (%v)", tt.wantLen, len(got), got)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ExtractSecretsE tests
// ---------------------------------------------------------------------------

func TestExtractSecretsE(t *testing.T) {
	tests := []struct {
		name       string
		data       map[string]interface{}
		key        string
		wantNil    bool
		wantLen    int
		wantErr    bool
		errSubstr  string
		warnSubstr string
	}{
		{
			name: "full parse",
			data: map[string]interface{}{"k": []interface{}{
				map[string]interface{}{
					"name":            "db-creds",
					"onePasswordItem": "vault-item",
					"keys": []interface{}{
						map[string]interface{}{"secretKey": "password", "property": "password"},
					},
				},
			}},
			key:     "k",
			wantLen: 1,
		},
		{name: "missing key", data: map[string]interface{}{}, key: "k", wantNil: true},
		{name: "nil value", data: map[string]interface{}{"k": nil}, key: "k", wantNil: true},
		{name: "wrong type string", data: map[string]interface{}{"k": "oops"}, key: "k", wantErr: true, errSubstr: "expected []interface{}"},
		{name: "wrong type int", data: map[string]interface{}{"k": 42}, key: "k", wantErr: true, errSubstr: "expected []interface{}"},
		{
			name: "rejects non-map items",
			data: map[string]interface{}{"k": []interface{}{
				"not-a-map",
				map[string]interface{}{"name": "good"},
			}},
			key:       "k",
			wantErr:   true,
			errSubstr: "expected map",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractSecretsE(tt.data, tt.key)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("error %q should contain %q", err, tt.errSubstr)
				}
				return
			}
			if tt.warnSubstr != "" {
				if err == nil {
					t.Fatal("expected warning error, got nil")
				}
				if !strings.Contains(err.Error(), tt.warnSubstr) {
					t.Errorf("warning error %q should contain %q", err, tt.warnSubstr)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantNil {
				if got != nil {
					t.Errorf("expected nil, got %v", got)
				}
				return
			}
			if len(got) != tt.wantLen {
				t.Errorf("expected len %d, got %d", tt.wantLen, len(got))
			}
		})
	}
}

func TestExtractSecretsE_FullParse(t *testing.T) {
	data := map[string]interface{}{
		"secrets": []interface{}{
			map[string]interface{}{
				"name":            "db-creds",
				"onePasswordItem": "vault-item-1",
				"keys": []interface{}{
					map[string]interface{}{"secretKey": "password", "property": "password"},
					map[string]interface{}{"secretKey": "username", "property": "username"},
				},
			},
		},
	}
	secrets, err := ExtractSecretsE(data, "secrets")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
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
}

// ---------------------------------------------------------------------------
// FromMapSliceE tests
// ---------------------------------------------------------------------------

type testDest struct {
	Namespace string `json:"namespace"`
	Server    string `json:"server"`
}

func TestFromMapSliceE_NilReturnsNil(t *testing.T) {
	got, err := FromMapSliceE[testDest](nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestFromMapSliceE_EmptySlice(t *testing.T) {
	got, err := FromMapSliceE[testDest]([]map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty slice, got %d items", len(got))
	}
}

func TestFromMapSliceE_ValidEntries(t *testing.T) {
	input := []map[string]interface{}{
		{"namespace": "default", "server": "https://k8s.local"},
		{"namespace": "prod", "server": "https://prod.example.com"},
	}
	got, err := FromMapSliceE[testDest](input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 items, got %d", len(got))
	}
	if got[0].Namespace != "default" || got[0].Server != "https://k8s.local" {
		t.Errorf("item 0 mismatch: %+v", got[0])
	}
	if got[1].Namespace != "prod" || got[1].Server != "https://prod.example.com" {
		t.Errorf("item 1 mismatch: %+v", got[1])
	}
}

func TestFromMapSliceE_MissingKeysZeroValue(t *testing.T) {
	input := []map[string]interface{}{
		{"server": "https://k8s.local"},
	}
	got, err := FromMapSliceE[testDest](input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got[0].Namespace != "" {
		t.Errorf("expected empty namespace, got %q", got[0].Namespace)
	}
}

func TestFromMapSliceE_WrongTypeReturnsError(t *testing.T) {
	input := []map[string]interface{}{
		{"namespace": 42, "server": "https://k8s.local"},
	}
	_, err := FromMapSliceE[testDest](input)
	if err == nil {
		t.Fatal("expected error for wrong type, got nil")
	}
	if !strings.Contains(err.Error(), "unmarshal") {
		t.Errorf("expected unmarshal error, got: %v", err)
	}
}
