package kratixutil

import (
	"reflect"
	"strings"
	"testing"
)

// newMockResource creates a MockResource from a nested map literal.
func newMockResource(data map[string]interface{}) *MockResource {
	return &MockResource{Data: data}
}

func TestGetStringValue(t *testing.T) {
	tests := []struct {
		name      string
		data      map[string]interface{}
		path      string
		expected  string
		wantErr   bool
		errSubstr string
	}{
		{
			name:     "success",
			data:     map[string]interface{}{"spec": map[string]interface{}{"name": "my-app"}},
			path:     "spec.name",
			expected: "my-app",
		},
		{
			name:    "not found",
			data:    map[string]interface{}{},
			path:    "spec.missing",
			wantErr: true,
		},
		{
			name:      "not a string",
			data:      map[string]interface{}{"spec": map[string]interface{}{"count": 42}},
			path:      "spec.count",
			wantErr:   true,
			errSubstr: "spec.count is not a string",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newMockResource(tt.data)
			val, err := GetStringValue(r, tt.path)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				if tt.errSubstr != "" && err.Error() != tt.errSubstr {
					t.Errorf("unexpected error message: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if val != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, val)
			}
		})
	}
}

func TestGetStringValueWithDefault(t *testing.T) {
	tests := []struct {
		name     string
		data     map[string]interface{}
		path     string
		defVal   string
		expected string
	}{
		{
			name:     "returns value",
			data:     map[string]interface{}{"spec": map[string]interface{}{"name": "found"}},
			path:     "spec.name",
			defVal:   "default",
			expected: "found",
		},
		{
			name:     "uses default on missing",
			data:     map[string]interface{}{},
			path:     "spec.missing",
			defVal:   "fallback",
			expected: "fallback",
		},
		{
			name:     "uses default on empty",
			data:     map[string]interface{}{"spec": map[string]interface{}{"name": ""}},
			path:     "spec.name",
			defVal:   "fallback",
			expected: "fallback",
		},
		{
			name:     "treats null as empty",
			data:     map[string]interface{}{"spec": map[string]interface{}{"name": "null"}},
			path:     "spec.name",
			defVal:   "fallback",
			expected: "fallback",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newMockResource(tt.data)
			val := GetStringValueWithDefault(r, tt.path, tt.defVal)
			if val != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, val)
			}
		})
	}
}

func TestGetIntValue(t *testing.T) {
	tests := []struct {
		name     string
		data     map[string]interface{}
		path     string
		expected int
		wantErr  bool
	}{
		{
			name:     "int",
			data:     map[string]interface{}{"spec": map[string]interface{}{"count": 5}},
			path:     "spec.count",
			expected: 5,
		},
		{
			name:     "float64",
			data:     map[string]interface{}{"spec": map[string]interface{}{"count": float64(42)}},
			path:     "spec.count",
			expected: 42,
		},
		{
			name:     "int64",
			data:     map[string]interface{}{"spec": map[string]interface{}{"count": int64(99)}},
			path:     "spec.count",
			expected: 99,
		},
		{
			name:     "string",
			data:     map[string]interface{}{"spec": map[string]interface{}{"count": "77"}},
			path:     "spec.count",
			expected: 77,
		},
		{
			name:    "invalid string",
			data:    map[string]interface{}{"spec": map[string]interface{}{"count": "not-a-number"}},
			path:    "spec.count",
			wantErr: true,
		},
		{
			name:    "unsupported type",
			data:    map[string]interface{}{"spec": map[string]interface{}{"count": true}},
			path:    "spec.count",
			wantErr: true,
		},
		{
			name:    "not found",
			data:    map[string]interface{}{},
			path:    "spec.missing",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newMockResource(tt.data)
			val, err := GetIntValue(r, tt.path)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if val != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, val)
			}
		})
	}
}

func TestGetIntValueWithDefault(t *testing.T) {
	tests := []struct {
		name     string
		data     map[string]interface{}
		path     string
		defVal   int
		expected int
	}{
		{
			name:     "returns value",
			data:     map[string]interface{}{"spec": map[string]interface{}{"replicas": 3}},
			path:     "spec.replicas",
			defVal:   1,
			expected: 3,
		},
		{
			name:     "uses default on missing",
			data:     map[string]interface{}{},
			path:     "spec.missing",
			defVal:   10,
			expected: 10,
		},
		{
			name:     "returns explicit zero",
			data:     map[string]interface{}{"spec": map[string]interface{}{"replicas": 0}},
			path:     "spec.replicas",
			defVal:   5,
			expected: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newMockResource(tt.data)
			val := GetIntValueWithDefault(r, tt.path, tt.defVal)
			if val != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, val)
			}
		})
	}
}

func TestGetBoolValue(t *testing.T) {
	tests := []struct {
		name     string
		data     map[string]interface{}
		path     string
		expected bool
		wantErr  bool
	}{
		{
			name:     "true",
			data:     map[string]interface{}{"spec": map[string]interface{}{"enabled": true}},
			path:     "spec.enabled",
			expected: true,
		},
		{
			name:     "false",
			data:     map[string]interface{}{"spec": map[string]interface{}{"enabled": false}},
			path:     "spec.enabled",
			expected: false,
		},
		{
			name:    "not a bool",
			data:    map[string]interface{}{"spec": map[string]interface{}{"enabled": "yes"}},
			path:    "spec.enabled",
			wantErr: true,
		},
		{
			name:    "not found",
			data:    map[string]interface{}{},
			path:    "spec.missing",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newMockResource(tt.data)
			val, err := GetBoolValue(r, tt.path)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if val != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, val)
			}
		})
	}
}

func TestGetBoolValueWithDefault(t *testing.T) {
	tests := []struct {
		name     string
		data     map[string]interface{}
		path     string
		defVal   bool
		expected bool
	}{
		{
			name:     "returns value",
			data:     map[string]interface{}{"spec": map[string]interface{}{"debug": true}},
			path:     "spec.debug",
			defVal:   false,
			expected: true,
		},
		{
			name:     "uses default on missing",
			data:     map[string]interface{}{},
			path:     "spec.missing",
			defVal:   true,
			expected: true,
		},
		{
			name:     "uses default on non-bool",
			data:     map[string]interface{}{"spec": map[string]interface{}{"flag": "yes"}},
			path:     "spec.flag",
			defVal:   true,
			expected: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newMockResource(tt.data)
			val := GetBoolValueWithDefault(r, tt.path, tt.defVal)
			if val != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, val)
			}
		})
	}
}

func TestGetOptionalStringValue(t *testing.T) {
	tests := []struct {
		name      string
		data      map[string]interface{}
		path      string
		expected  string
		wantErr   bool
		errSubstr string
	}{
		{
			name:     "present and valid",
			data:     map[string]interface{}{"spec": map[string]interface{}{"chart": "nginx"}},
			path:     "spec.chart",
			expected: "nginx",
		},
		{
			name:     "absent returns empty",
			data:     map[string]interface{}{},
			path:     "spec.chart",
			expected: "",
			wantErr:  false,
		},
		{
			name:      "wrong type returns error",
			data:      map[string]interface{}{"spec": map[string]interface{}{"chart": 42}},
			path:      "spec.chart",
			wantErr:   true,
			errSubstr: "expected string",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newMockResource(tt.data)
			val, err := GetOptionalStringValue(r, tt.path)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("expected error containing %q, got %v", tt.errSubstr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if val != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, val)
			}
		})
	}
}

func TestGetOptionalBoolValue(t *testing.T) {
	tests := []struct {
		name      string
		data      map[string]interface{}
		path      string
		expected  bool
		wantErr   bool
		errSubstr string
	}{
		{
			name:     "present and true",
			data:     map[string]interface{}{"spec": map[string]interface{}{"enabled": true}},
			path:     "spec.enabled",
			expected: true,
		},
		{
			name:     "present and false",
			data:     map[string]interface{}{"spec": map[string]interface{}{"enabled": false}},
			path:     "spec.enabled",
			expected: false,
		},
		{
			name:     "absent returns false",
			data:     map[string]interface{}{},
			path:     "spec.enabled",
			expected: false,
			wantErr:  false,
		},
		{
			name:      "wrong type returns error",
			data:      map[string]interface{}{"spec": map[string]interface{}{"enabled": "yes"}},
			path:      "spec.enabled",
			wantErr:   true,
			errSubstr: "expected bool",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newMockResource(tt.data)
			val, err := GetOptionalBoolValue(r, tt.path)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("expected error containing %q, got %v", tt.errSubstr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if val != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, val)
			}
		})
	}
}

func TestGetOptionalIntValue(t *testing.T) {
	tests := []struct {
		name      string
		data      map[string]interface{}
		path      string
		expected  int
		wantErr   bool
		errSubstr string
	}{
		{
			name:     "present int",
			data:     map[string]interface{}{"spec": map[string]interface{}{"count": 5}},
			path:     "spec.count",
			expected: 5,
		},
		{
			name:     "present float64",
			data:     map[string]interface{}{"spec": map[string]interface{}{"count": float64(42)}},
			path:     "spec.count",
			expected: 42,
		},
		{
			name:     "present int64",
			data:     map[string]interface{}{"spec": map[string]interface{}{"count": int64(99)}},
			path:     "spec.count",
			expected: 99,
		},
		{
			name:     "absent returns zero",
			data:     map[string]interface{}{},
			path:     "spec.count",
			expected: 0,
			wantErr:  false,
		},
		{
			name:      "wrong type returns error",
			data:      map[string]interface{}{"spec": map[string]interface{}{"count": "not-a-number"}},
			path:      "spec.count",
			wantErr:   true,
			errSubstr: "expected numeric",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newMockResource(tt.data)
			val, err := GetOptionalIntValue(r, tt.path)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("expected error containing %q, got %v", tt.errSubstr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if val != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, val)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tests for Resource-level typed extraction helpers
// ---------------------------------------------------------------------------

func TestExtractStringMapFromResource(t *testing.T) {
	tests := []struct {
		name    string
		spec    map[string]interface{}
		path    string
		want    map[string]string
		wantErr bool
	}{
		{"present and valid", map[string]interface{}{"spec": map[string]interface{}{"labels": map[string]interface{}{"a": "b"}}}, "spec.labels", map[string]string{"a": "b"}, false},
		{"absent returns nil", map[string]interface{}{}, "spec.labels", nil, false},
		{"wrong type returns error", map[string]interface{}{"spec": map[string]interface{}{"labels": "not-a-map"}}, "spec.labels", nil, true},
		{"empty map", map[string]interface{}{"spec": map[string]interface{}{"labels": map[string]interface{}{}}}, "spec.labels", map[string]string{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newMockResource(tt.spec)
			got, err := ExtractStringMapFromResource(r, tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr = %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractStringSliceFromResource(t *testing.T) {
	tests := []struct {
		name    string
		spec    map[string]interface{}
		path    string
		want    []string
		wantErr bool
	}{
		{"present and valid", map[string]interface{}{"spec": map[string]interface{}{"repos": []interface{}{"a", "b"}}}, "spec.repos", []string{"a", "b"}, false},
		{"absent returns nil", map[string]interface{}{}, "spec.repos", nil, false},
		{"wrong type returns error", map[string]interface{}{"spec": map[string]interface{}{"repos": "not-a-slice"}}, "spec.repos", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newMockResource(tt.spec)
			got, err := ExtractStringSliceFromResource(r, tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr = %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractObjectSliceFromResource(t *testing.T) {
	tests := []struct {
		name    string
		spec    map[string]interface{}
		path    string
		want    []map[string]interface{}
		wantErr bool
	}{
		{"present and valid", map[string]interface{}{"spec": map[string]interface{}{"items": []interface{}{map[string]interface{}{"k": "v"}}}}, "spec.items", []map[string]interface{}{{"k": "v"}}, false},
		{"absent returns nil", map[string]interface{}{}, "spec.items", nil, false},
		{"wrong type returns error", map[string]interface{}{"spec": map[string]interface{}{"items": "not-a-slice"}}, "spec.items", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newMockResource(tt.spec)
			got, err := ExtractObjectSliceFromResource(r, tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr = %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractSecretsFromResource(t *testing.T) {
	tests := []struct {
		name    string
		spec    map[string]interface{}
		path    string
		want    []SecretRef
		wantErr bool
	}{
		{
			"present and valid",
			map[string]interface{}{
				"spec": map[string]interface{}{
					"secrets": []interface{}{
						map[string]interface{}{
							"name":            "db",
							"onePasswordItem": "item",
						},
					},
				},
			},
			"spec.secrets",
			[]SecretRef{{Name: "db", OnePasswordItem: "item"}},
			false,
		},
		{"absent returns nil", map[string]interface{}{}, "spec.secrets", nil, false},
		{"wrong type returns error", map[string]interface{}{"spec": map[string]interface{}{"secrets": "not-an-array"}}, "spec.secrets", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newMockResource(tt.spec)
			got, err := ExtractSecretsFromResource(r, tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr = %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractMapFromResource(t *testing.T) {
	tests := []struct {
		name    string
		spec    map[string]interface{}
		path    string
		want    map[string]interface{}
		wantErr bool
	}{
		{"present and valid", map[string]interface{}{"spec": map[string]interface{}{"data": map[string]interface{}{"key": "val"}}}, "spec.data", map[string]interface{}{"key": "val"}, false},
		{"absent returns nil", map[string]interface{}{}, "spec.data", nil, false},
		{"wrong type returns error", map[string]interface{}{"spec": map[string]interface{}{"data": "not-a-map"}}, "spec.data", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newMockResource(tt.spec)
			got, err := ExtractMapFromResource(r, tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr = %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
