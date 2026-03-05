package platform

import (
	"context"
	"fmt"
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/jamesatintegratnio/hctl/internal/kube"
)

func TestProvisionResultTypes(t *testing.T) {
	// Ensure the struct can be instantiated with all fields
	r := ProvisionResult{
		Name:  "test",
		Phase: "Ready",
		Health: ProvisionHealth{
			Unhealthy: []string{"a", "b"},
		},
	}
	if r.Name != "test" {
		t.Error("Name should be 'test'")
	}
	if len(r.Health.Unhealthy) != 2 {
		t.Error("Unhealthy should have 2 items")
	}
}

// ---------------------------------------------------------------------------
// WaitForRequest tests
// ---------------------------------------------------------------------------

func TestWaitForRequest_Success(t *testing.T) {
	t.Parallel()
	vc := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "VClusterOrchestratorV2",
		"metadata": map[string]interface{}{
			"name":              "test-vc",
			"creationTimestamp": time.Now().Add(-2 * time.Minute).Format(time.RFC3339),
		},
	}}
	client := &fakeKubeClient{
		vcReturns: []*unstructured.Unstructured{vc},
		vcErrs:    []error{nil},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	msg, err := WaitForRequest(ctx, client, "platform", "test-vc", 10*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg == "" {
		t.Error("expected non-empty message")
	}
}

func TestWaitForRequest_Timeout(t *testing.T) {
	t.Parallel()
	client := &fakeKubeClient{
		vcReturns: []*unstructured.Unstructured{nil},
		vcErrs:    []error{fmt.Errorf("not found")},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := WaitForRequest(ctx, client, "platform", "missing", 10*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestWaitForRequest_EventuallyFound(t *testing.T) {
	t.Parallel()
	vc := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "VClusterOrchestratorV2",
		"metadata": map[string]interface{}{
			"name":              "test-vc",
			"creationTimestamp": time.Now().Format(time.RFC3339),
		},
	}}
	// First call: not found; second call: found
	client := &fakeKubeClient{
		vcReturns: []*unstructured.Unstructured{nil, vc},
		vcErrs:    []error{fmt.Errorf("not found"), nil},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	msg, err := WaitForRequest(ctx, client, "platform", "test-vc", 10*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg == "" {
		t.Error("expected non-empty message")
	}
}

// ---------------------------------------------------------------------------
// CollectProvisionResult tests
// ---------------------------------------------------------------------------

func TestCollectProvisionResult_StatusContract(t *testing.T) {
	t.Parallel()
	// Build a VCluster resource with a full status contract
	vc := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "VClusterOrchestratorV2",
		"metadata":   map[string]interface{}{"name": "test-vc"},
		"status": map[string]interface{}{
			"phase":   "Ready",
			"message": "All good",
			"endpoints": map[string]interface{}{
				"api":    "https://test-vc.example.com:443",
				"argocd": "https://argocd.test-vc.example.com",
			},
			"health": map[string]interface{}{
				"workloads": map[string]interface{}{
					"ready": int64(3),
					"total": int64(3),
				},
				"subApps": map[string]interface{}{
					"healthy": int64(5),
					"total":   int64(5),
				},
			},
		},
	}}
	client := &fakeKubeClient{
		vcReturns: []*unstructured.Unstructured{vc},
		vcErrs:    []error{nil},
	}

	result, err := CollectProvisionResult(context.Background(), client, "platform", "test-vc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Phase != "Ready" {
		t.Errorf("Phase = %q, want 'Ready'", result.Phase)
	}
	if !result.Healthy {
		t.Error("expected Healthy = true")
	}
	if result.Endpoints.API != "https://test-vc.example.com:443" {
		t.Errorf("API endpoint = %q", result.Endpoints.API)
	}
	if result.Health.ComponentsReady != 3 {
		t.Errorf("ComponentsReady = %d, want 3", result.Health.ComponentsReady)
	}
}

func TestCollectProvisionResult_Fallback(t *testing.T) {
	t.Parallel()
	// VCluster exists but has no status contract (empty status)
	vc := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "VClusterOrchestratorV2",
		"metadata":   map[string]interface{}{"name": "test-vc"},
	}}
	client := &fakeKubeClient{
		// First call for GetStatusContract, second for fallback queries
		vcReturns: []*unstructured.Unstructured{vc, vc},
		vcErrs:    []error{nil, nil},
		pods: []kube.PodInfo{
			{Name: "vc-0", Phase: "Running", ReadyContainers: 1, TotalContainers: 1},
			{Name: "vc-1", Phase: "Running", ReadyContainers: 1, TotalContainers: 1},
		},
		argoAppsCluster: []kube.ArgoAppInfo{
			{Name: "app1", SyncStatus: "Synced", HealthStatus: "Healthy"},
			{Name: "app2", SyncStatus: "OutOfSync", HealthStatus: "Degraded"},
		},
	}

	result, err := CollectProvisionResult(context.Background(), client, "platform", "test-vc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Fallback path: all pods ready → "Ready"
	if result.Health.ComponentsReady != 2 {
		t.Errorf("ComponentsReady = %d, want 2", result.Health.ComponentsReady)
	}
	if result.Health.SubAppsTotal != 2 {
		t.Errorf("SubAppsTotal = %d, want 2", result.Health.SubAppsTotal)
	}
	if result.Health.SubAppsHealthy != 1 {
		t.Errorf("SubAppsHealthy = %d, want 1", result.Health.SubAppsHealthy)
	}
	if len(result.Health.Unhealthy) != 1 || result.Health.Unhealthy[0] != "app2" {
		t.Errorf("Unhealthy = %v, want [app2]", result.Health.Unhealthy)
	}
}

func TestCollectProvisionResult_Fallback_NotAllPodsReady(t *testing.T) {
	t.Parallel()
	vc := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "VClusterOrchestratorV2",
		"metadata":   map[string]interface{}{"name": "test-vc"},
	}}
	client := &fakeKubeClient{
		vcReturns: []*unstructured.Unstructured{vc, vc},
		vcErrs:    []error{nil, nil},
		pods: []kube.PodInfo{
			{Name: "vc-0", Phase: "Running", ReadyContainers: 1, TotalContainers: 1},
			{Name: "vc-1", Phase: "Pending", ReadyContainers: 0, TotalContainers: 1},
		},
	}

	result, err := CollectProvisionResult(context.Background(), client, "platform", "test-vc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Healthy {
		t.Error("expected Healthy = false when not all pods are ready")
	}
	if result.Phase != "Progressing" {
		t.Errorf("Phase = %q, want 'Progressing'", result.Phase)
	}
}

// Ensure _ imports are used (prevent "imported and not used" errors).
var (
	_ = metav1.Now
	_ batchv1.Job
)
