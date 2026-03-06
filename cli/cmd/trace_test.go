package cmd

import (
	"fmt"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/jamesatintegratnio/hctl/internal/config"
	"github.com/jamesatintegratnio/hctl/internal/kube"
	"github.com/jamesatintegratnio/hctl/internal/testutil"
	"github.com/jamesatintegratnio/hctl/internal/tui"
)

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestTraceCmdHasLongDescription(t *testing.T) {
	cmd := newTraceCmd()
	if cmd.Long == "" {
		t.Error("trace Long should not be empty")
	}
	if cmd.RunE == nil {
		t.Error("trace RunE should be set")
	}
}

func TestExecuteTrace_NothingFound(t *testing.T) {
	// Set structured output so executeTrace uses RenderOutput instead of fmt prints
	tui.SetOutputFormat("json")
	defer tui.SetOutputFormat("")

	cfg := config.Default()
	fake := &testutil.FakeKubeClient{
		VClusterErr: fmt.Errorf("not found"),
		ArgoAppErr:  fmt.Errorf("not found"),
		PodsErr:     fmt.Errorf("not found"),
	}

	err := runTraceWithClient(fake, "nonexistent", cfg)
	if err != nil {
		t.Fatalf("runTraceWithClient should not error when resources are not found, got: %v", err)
	}
}

func TestExecuteTrace_VClusterFound(t *testing.T) {
	tui.SetOutputFormat("json")
	defer tui.SetOutputFormat("")

	cfg := config.Default()

	vc := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "platform.integratn.tech/v1alpha1",
			"kind":       "VClusterOrchestratorV2",
			"metadata": map[string]interface{}{
				"name":      "test-vc",
				"namespace": "platform-requests",
			},
			"status": map[string]interface{}{
				"phase":   "Running",
				"message": "pipeline completed",
				"conditions": []interface{}{
					map[string]interface{}{
						"type":   "PipelineCompleted",
						"status": "True",
					},
				},
			},
		},
	}

	argoApp := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"name": "test-vc",
			},
			"status": map[string]interface{}{
				"sync": map[string]interface{}{
					"status":   "Synced",
					"revision": "abc1234567890",
				},
				"health": map[string]interface{}{
					"status": "Healthy",
				},
			},
		},
	}

	fake := &testutil.FakeKubeClient{
		VCluster: vc,
		ArgoApp:  argoApp,
		Pods: []kube.PodInfo{
			{Name: "test-vc-0", Phase: "Running", ReadyContainers: 1, TotalContainers: 1},
		},
	}

	err := runTraceWithClient(fake, "test-vc", cfg)
	if err != nil {
		t.Fatalf("runTraceWithClient should succeed, got: %v", err)
	}
}

func TestExecuteTrace_ArgoFoundWithSubApps(t *testing.T) {
	tui.SetOutputFormat("json")
	defer tui.SetOutputFormat("")

	cfg := config.Default()

	argoApp := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{"name": "my-app"},
			"status": map[string]interface{}{
				"sync":   map[string]interface{}{"status": "OutOfSync", "revision": "deadbeef"},
				"health": map[string]interface{}{"status": "Degraded"},
			},
		},
	}

	fake := &testutil.FakeKubeClient{
		VClusterErr: fmt.Errorf("not found"),
		ArgoApp:     argoApp,
		ArgoAppsCluster: []kube.ArgoAppInfo{
			{Name: "sub-1", SyncStatus: "Synced", HealthStatus: "Healthy"},
			{Name: "sub-2", SyncStatus: "OutOfSync", HealthStatus: "Degraded"},
		},
		PodsErr: fmt.Errorf("no pods"),
	}

	err := runTraceWithClient(fake, "my-app", cfg)
	if err != nil {
		t.Fatalf("runTraceWithClient should succeed, got: %v", err)
	}
}

func TestExecuteTrace_TextOutput(t *testing.T) {
	// Test with text output (not structured) — exercises the rendering path
	tui.SetOutputFormat("")

	cfg := config.Default()
	fake := &testutil.FakeKubeClient{
		VClusterErr: fmt.Errorf("not found"),
		ArgoAppErr:  fmt.Errorf("not found"),
		PodsErr:     fmt.Errorf("not found"),
	}

	err := runTraceWithClient(fake, "test-resource", cfg)
	if err != nil {
		t.Fatalf("runTraceWithClient text output should not error, got: %v", err)
	}
}


