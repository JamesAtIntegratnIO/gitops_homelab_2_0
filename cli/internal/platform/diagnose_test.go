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
