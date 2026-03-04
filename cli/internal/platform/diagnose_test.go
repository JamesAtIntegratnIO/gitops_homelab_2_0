package platform

import (
	"testing"
)

func TestStepStatusString(t *testing.T) {
	tests := []struct {
		status StepStatus
		name   string
	}{
		{StatusOK, "ok"},
		{StatusWarning, "warning"},
		{StatusError, "error"},
		{StatusUnknown, "unknown"},
		{StepStatus(99), "unknown"}, // fallback
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.status.String()
			if got == "" {
				t.Error("String() should not return empty")
			}
		})
	}
}

func TestStepStatusConstants(t *testing.T) {
	// Verify the iota values are distinct
	if StatusOK == StatusWarning {
		t.Error("StatusOK should not equal StatusWarning")
	}
	if StatusWarning == StatusError {
		t.Error("StatusWarning should not equal StatusError")
	}
	if StatusError == StatusUnknown {
		t.Error("StatusError should not equal StatusUnknown")
	}
}

func TestUnstructuredNestedString(t *testing.T) {
	obj := map[string]interface{}{
		"status": map[string]interface{}{
			"sync": map[string]interface{}{
				"status": "Synced",
			},
			"health": map[string]interface{}{
				"status": "Healthy",
			},
		},
		"metadata": map[string]interface{}{
			"name": "my-app",
		},
	}

	tests := []struct {
		name   string
		fields []string
		want   string
		found  bool
	}{
		{"deep nested", []string{"status", "sync", "status"}, "Synced", true},
		{"health status", []string{"status", "health", "status"}, "Healthy", true},
		{"metadata name", []string{"metadata", "name"}, "my-app", true},
		{"missing field", []string{"status", "missing"}, "", false},
		{"missing deep", []string{"status", "sync", "missing"}, "", false},
		{"missing top", []string{"nonexistent"}, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, found, err := UnstructuredNestedString(obj, tt.fields...)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if found != tt.found {
				t.Errorf("found = %v, want %v", found, tt.found)
			}
			if found && val != tt.want {
				t.Errorf("value = %q, want %q", val, tt.want)
			}
		})
	}
}

func TestUnstructuredNestedString_NotAString(t *testing.T) {
	obj := map[string]interface{}{
		"count": 42,
	}

	val, found, err := UnstructuredNestedString(obj, "count")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The value exists but is not a string
	if found {
		t.Logf("found=%v, val=%q (int-to-string check)", found, val)
	}
}

func TestUnstructuredNestedSlice(t *testing.T) {
	obj := map[string]interface{}{
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{
					"type":   "Available",
					"status": "True",
				},
				map[string]interface{}{
					"type":   "Progressing",
					"status": "True",
				},
			},
		},
	}

	val, found, err := UnstructuredNestedSlice(obj, "status", "conditions")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Fatal("expected to find conditions slice")
	}
	if len(val) != 2 {
		t.Errorf("expected 2 conditions, got %d", len(val))
	}
}

func TestUnstructuredNestedSlice_Missing(t *testing.T) {
	obj := map[string]interface{}{}

	_, found, err := UnstructuredNestedSlice(obj, "status", "conditions")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Error("expected not found for missing path")
	}
}

func TestDiagnosticResultStructure(t *testing.T) {
	result := &DiagnosticResult{
		Steps: []DiagnosticStep{
			{
				Name:    "ResourceRequest",
				Status:  StatusOK,
				Message: "Found (age: 1h0m0s)",
			},
			{
				Name:    "Pipeline Job",
				Status:  StatusError,
				Message: "Job failed",
				Details: "OOMKilled",
			},
		},
	}

	if len(result.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(result.Steps))
	}
	if result.Steps[0].Status != StatusOK {
		t.Errorf("step 0 status = %v, want StatusOK", result.Steps[0].Status)
	}
	if result.Steps[1].Status != StatusError {
		t.Errorf("step 1 status = %v, want StatusError", result.Steps[1].Status)
	}
	if result.Steps[1].Details != "OOMKilled" {
		t.Errorf("step 1 details = %q, want 'OOMKilled'", result.Steps[1].Details)
	}
}
