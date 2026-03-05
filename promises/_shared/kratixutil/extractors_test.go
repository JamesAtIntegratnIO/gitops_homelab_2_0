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
		name      string
		data      map[string]interface{}
		key       string
		wantNil   bool
		wantLen   int
		wantErr   bool
		errSubstr string
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
			name:    "skips non-string values",
			data:    map[string]interface{}{"k": map[string]interface{}{"good": "val", "bad": 42}},
			key:     "k",
			wantLen: 1,
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
			if err != nil {
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
		name      string
		data      map[string]interface{}
		key       string
		wantNil   bool
		wantLen   int
		wantErr   bool
		errSubstr string
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
			name:    "filters non-strings",
			data:    map[string]interface{}{"k": []interface{}{"a", 42, "b"}},
			key:     "k",
			wantLen: 2,
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
			if err != nil {
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
		name      string
		data      map[string]interface{}
		key       string
		wantNil   bool
		wantLen   int
		wantErr   bool
		errSubstr string
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
			name: "filters non-maps",
			data: map[string]interface{}{"k": []interface{}{
				map[string]interface{}{"a": "1"},
				"not-a-map",
				42,
			}},
			key:     "k",
			wantLen: 1,
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
			if err != nil {
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
		name      string
		data      map[string]interface{}
		key       string
		wantNil   bool
		wantLen   int
		wantErr   bool
		errSubstr string
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
			name: "skips non-map items",
			data: map[string]interface{}{"k": []interface{}{
				"not-a-map",
				map[string]interface{}{"name": "good"},
			}},
			key:     "k",
			wantLen: 1,
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
			if err != nil {
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
