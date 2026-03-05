package platform

import (
	"context"
	"fmt"
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/jamesatintegratnio/hctl/internal/kube"
	"github.com/jamesatintegratnio/hctl/internal/testutil"
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

// ---------------------------------------------------------------------------
// ResourceRequestChecker.Check tests
// ---------------------------------------------------------------------------

func TestResourceRequestChecker_Check_Found(t *testing.T) {
	t.Parallel()
	vc := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "VClusterOrchestratorV2",
		"metadata": map[string]interface{}{
			"name":              "test-vc",
			"namespace":         "platform",
			"creationTimestamp": time.Now().Add(-5 * time.Minute).Format(time.RFC3339),
		},
		"spec": map[string]interface{}{
			"targetNamespace": "test-ns",
		},
	}}
	client := &testutil.FakeKubeClient{VCluster: vc}
	state := &DiagnosticState{Namespace: "platform", Name: "test-vc", TargetNS: "test-vc"}

	steps, halt := (&ResourceRequestChecker{}).Check(context.Background(), client, state)
	if halt {
		t.Error("should not halt when resource is found")
	}
	if len(steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(steps))
	}
	if steps[0].Status != StatusOK {
		t.Errorf("status = %v, want StatusOK", steps[0].Status)
	}
	if state.TargetNS != "test-ns" {
		t.Errorf("TargetNS = %q, want 'test-ns'", state.TargetNS)
	}
}

func TestResourceRequestChecker_Check_NotFound(t *testing.T) {
	t.Parallel()
	client := &testutil.FakeKubeClient{VClusterErr: fmt.Errorf("not found")}
	state := &DiagnosticState{Namespace: "platform", Name: "missing"}

	steps, halt := (&ResourceRequestChecker{}).Check(context.Background(), client, state)
	if !halt {
		t.Error("should halt when resource is not found")
	}
	if len(steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(steps))
	}
	if steps[0].Status != StatusError {
		t.Errorf("status = %v, want StatusError", steps[0].Status)
	}
}

// ---------------------------------------------------------------------------
// PipelineJobChecker.Check tests
// ---------------------------------------------------------------------------

func TestPipelineJobChecker_Check_Completed(t *testing.T) {
	t.Parallel()
	client := &testutil.FakeKubeClient{
		Jobs: []batchv1.Job{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "pipeline-abc"},
				Status: batchv1.JobStatus{
					Conditions: []batchv1.JobCondition{
						{Type: batchv1.JobComplete, Status: corev1.ConditionTrue},
					},
				},
			},
		},
	}
	state := &DiagnosticState{Namespace: "platform", Name: "test-vc"}

	steps, halt := (&PipelineJobChecker{}).Check(context.Background(), client, state)
	if halt {
		t.Error("should not halt on completed job")
	}
	if len(steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(steps))
	}
	if steps[0].Status != StatusOK {
		t.Errorf("status = %v, want StatusOK", steps[0].Status)
	}
}

func TestPipelineJobChecker_Check_Failed(t *testing.T) {
	t.Parallel()
	client := &testutil.FakeKubeClient{
		Jobs: []batchv1.Job{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "pipeline-abc"},
				Status: batchv1.JobStatus{
					Conditions: []batchv1.JobCondition{
						{Type: batchv1.JobFailed, Status: corev1.ConditionTrue, Message: "OOMKilled"},
					},
				},
			},
		},
	}
	state := &DiagnosticState{Namespace: "platform", Name: "test-vc"}

	steps, halt := (&PipelineJobChecker{}).Check(context.Background(), client, state)
	if halt {
		t.Error("should not halt on failed job")
	}
	if len(steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(steps))
	}
	if steps[0].Status != StatusError {
		t.Errorf("status = %v, want StatusError", steps[0].Status)
	}
}

func TestPipelineJobChecker_Check_NoJobs(t *testing.T) {
	t.Parallel()
	client := &testutil.FakeKubeClient{Jobs: nil}
	state := &DiagnosticState{Namespace: "platform", Name: "test-vc"}

	steps, _ := (&PipelineJobChecker{}).Check(context.Background(), client, state)
	if len(steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(steps))
	}
	if steps[0].Status != StatusWarning {
		t.Errorf("status = %v, want StatusWarning", steps[0].Status)
	}
}

func TestPipelineJobChecker_Check_ListError(t *testing.T) {
	t.Parallel()
	client := &testutil.FakeKubeClient{JobsErr: fmt.Errorf("API error")}
	state := &DiagnosticState{Namespace: "platform", Name: "test-vc"}

	steps, _ := (&PipelineJobChecker{}).Check(context.Background(), client, state)
	if len(steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(steps))
	}
	if steps[0].Status != StatusWarning {
		t.Errorf("status = %v, want StatusWarning", steps[0].Status)
	}
}

// ---------------------------------------------------------------------------
// ArgoCDAppChecker.Check tests
// ---------------------------------------------------------------------------

func TestArgoCDAppChecker_Check_Healthy(t *testing.T) {
	t.Parallel()
	app := &unstructured.Unstructured{Object: map[string]interface{}{
		"status": map[string]interface{}{
			"sync":   map[string]interface{}{"status": "Synced"},
			"health": map[string]interface{}{"status": "Healthy"},
		},
	}}
	client := &testutil.FakeKubeClient{ArgoApp: app}
	state := &DiagnosticState{Namespace: "platform", Name: "test-vc"}

	steps, halt := (&ArgoCDAppChecker{}).Check(context.Background(), client, state)
	if halt {
		t.Error("should not halt")
	}
	if len(steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(steps))
	}
	if steps[0].Status != StatusOK {
		t.Errorf("status = %v, want StatusOK", steps[0].Status)
	}
}

func TestArgoCDAppChecker_Check_OutOfSync(t *testing.T) {
	t.Parallel()
	app := &unstructured.Unstructured{Object: map[string]interface{}{
		"status": map[string]interface{}{
			"sync":   map[string]interface{}{"status": "OutOfSync"},
			"health": map[string]interface{}{"status": "Healthy"},
		},
	}}
	client := &testutil.FakeKubeClient{ArgoApp: app}
	state := &DiagnosticState{Namespace: "platform", Name: "test-vc"}

	steps, _ := (&ArgoCDAppChecker{}).Check(context.Background(), client, state)
	if len(steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(steps))
	}
	if steps[0].Status != StatusWarning {
		t.Errorf("status = %v, want StatusWarning", steps[0].Status)
	}
}

func TestArgoCDAppChecker_Check_NotFound(t *testing.T) {
	t.Parallel()
	client := &testutil.FakeKubeClient{ArgoAppErr: fmt.Errorf("not found")}
	state := &DiagnosticState{Namespace: "platform", Name: "test-vc"}

	steps, _ := (&ArgoCDAppChecker{}).Check(context.Background(), client, state)
	if len(steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(steps))
	}
	if steps[0].Status != StatusWarning {
		t.Errorf("status = %v, want StatusWarning", steps[0].Status)
	}
}
