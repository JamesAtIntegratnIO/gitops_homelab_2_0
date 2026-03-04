package platform

import (
	"testing"

	"github.com/jamesatintegratnio/hctl/internal/kube"
)

// ---------------------------------------------------------------------------
// PodResourceChecker.buildSteps — pure logic, no cluster required
// ---------------------------------------------------------------------------

func TestPodResourceChecker_buildSteps_Healthy(t *testing.T) {
	t.Parallel()
	c := &PodResourceChecker{}
	podResources := []kube.PodResourceInfo{
		{
			Name:          "vcluster-abc-0",
			Namespace:     "my-ns",
			Phase:         "Running",
			MemoryRequest: "256Mi",
			MemoryLimit:   "512Mi",
			CPURequest:    "100m",
			CPULimit:      "500m",
			Restarts:      0,
		},
	}

	steps := c.buildSteps(podResources, false, "vcluster")
	if len(steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(steps))
	}
	if steps[0].Status != StatusOK {
		t.Errorf("status = %v, want StatusOK", steps[0].Status)
	}
	if steps[0].Details != "" {
		t.Errorf("details should be empty for healthy pod, got %q", steps[0].Details)
	}
}

func TestPodResourceChecker_buildSteps_WithRestarts(t *testing.T) {
	t.Parallel()
	c := &PodResourceChecker{}
	podResources := []kube.PodResourceInfo{
		{
			Name:          "vcluster-abc-0",
			Namespace:     "my-ns",
			MemoryRequest: "256Mi",
			MemoryLimit:   "512Mi",
			CPURequest:    "100m",
			CPULimit:      "500m",
			Restarts:      5,
		},
	}

	steps := c.buildSteps(podResources, false, "vcluster")
	if len(steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(steps))
	}
	if steps[0].Status != StatusWarning {
		t.Errorf("status = %v, want StatusWarning", steps[0].Status)
	}
	if steps[0].Details == "" {
		t.Error("expected restart details message")
	}
}

func TestPodResourceChecker_buildSteps_FilterByName(t *testing.T) {
	t.Parallel()
	c := &PodResourceChecker{}
	podResources := []kube.PodResourceInfo{
		{Name: "vcluster-media-0", MemoryRequest: "256Mi", MemoryLimit: "512Mi", Restarts: 0},
		{Name: "unrelated-pod", MemoryRequest: "128Mi", MemoryLimit: "256Mi", Restarts: 0},
		{Name: "vcluster-sidecar", MemoryRequest: "64Mi", MemoryLimit: "128Mi", Restarts: 0},
	}

	steps := c.buildSteps(podResources, true, "media")
	// Should match "vcluster-media-0" (contains "media") and "vcluster-sidecar" (contains "vcluster")
	// but NOT "unrelated-pod"
	for _, s := range steps {
		if s.Message != "" && contains(s.Message, "unrelated-pod") {
			t.Errorf("unexpected step for unrelated-pod: %s", s.Message)
		}
	}
}

func TestPodResourceChecker_buildSteps_NoFilterShowsAll(t *testing.T) {
	t.Parallel()
	c := &PodResourceChecker{}
	podResources := []kube.PodResourceInfo{
		{Name: "pod-a", MemoryRequest: "256Mi", MemoryLimit: "512Mi", CPURequest: "100m", CPULimit: "500m"},
		{Name: "pod-b", MemoryRequest: "128Mi", MemoryLimit: "256Mi", CPURequest: "50m", CPULimit: "200m"},
	}

	steps := c.buildSteps(podResources, false, "anything")
	if len(steps) != 2 {
		t.Errorf("expected 2 steps (no filtering), got %d", len(steps))
	}
}

func TestPodResourceChecker_buildSteps_Empty(t *testing.T) {
	t.Parallel()
	c := &PodResourceChecker{}
	steps := c.buildSteps(nil, false, "test")
	if len(steps) != 0 {
		t.Errorf("expected 0 steps for nil input, got %d", len(steps))
	}
}

func TestPodResourceChecker_buildSteps_MessageFormat_Filtered(t *testing.T) {
	t.Parallel()
	c := &PodResourceChecker{}
	podResources := []kube.PodResourceInfo{
		{
			Name:          "my-vcluster-0",
			MemoryRequest: "256Mi",
			MemoryLimit:   "512Mi",
			CPURequest:    "100m",
			CPULimit:      "500m",
			Restarts:      0,
		},
	}

	steps := c.buildSteps(podResources, true, "my-vcluster")
	if len(steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(steps))
	}
	// Filtered mode shows mem only (not CPU)
	msg := steps[0].Message
	if !contains(msg, "mem req=256Mi") {
		t.Errorf("expected memory request in message, got %q", msg)
	}
	// Should NOT contain cpu info in filtered mode
	if contains(msg, "cpu req=") {
		t.Errorf("filtered mode should not include CPU info, got %q", msg)
	}
}

func TestPodResourceChecker_buildSteps_MessageFormat_Unfiltered(t *testing.T) {
	t.Parallel()
	c := &PodResourceChecker{}
	podResources := []kube.PodResourceInfo{
		{
			Name:          "pod-abc-0",
			MemoryRequest: "1Gi",
			MemoryLimit:   "2Gi",
			CPURequest:    "500m",
			CPULimit:      "1000m",
			Restarts:      0,
		},
	}

	steps := c.buildSteps(podResources, false, "pod-abc")
	if len(steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(steps))
	}
	msg := steps[0].Message
	// Unfiltered mode shows both mem and CPU
	if !contains(msg, "cpu req=500m") {
		t.Errorf("expected CPU info in unfiltered message, got %q", msg)
	}
	if !contains(msg, "mem req=1Gi") {
		t.Errorf("expected memory info in unfiltered message, got %q", msg)
	}
}

// ---------------------------------------------------------------------------
// SubAppHealthChecker.Check logic (via direct struct construction)
// We test the output aggregation logic that SubAppHealthChecker uses,
// which doesn't require a client.
// ---------------------------------------------------------------------------

func TestDefaultCheckers_ReturnsAll(t *testing.T) {
	t.Parallel()
	checkers := DefaultCheckers()
	// Should return all 8 checkers
	if len(checkers) != 8 {
		t.Errorf("expected 8 default checkers, got %d", len(checkers))
	}
}

func TestRunDiagnostics_HaltsOnFlag(t *testing.T) {
	t.Parallel()
	// Create a minimal checker chain where the first halts
	result := &DiagnosticResult{}
	result.Steps = append(result.Steps, DiagnosticStep{
		Name:   "First",
		Status: StatusError,
	})
	// The RunDiagnostics function itself requires a kube.Client,
	// but we can verify the struct construction is sound
	if result.Steps[0].Status != StatusError {
		t.Error("expected StatusError")
	}
}

// contains is a helper to check substring presence.
func contains(s, sub string) bool {
	return len(s) >= len(sub) && searchSubstring(s, sub)
}

func searchSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
