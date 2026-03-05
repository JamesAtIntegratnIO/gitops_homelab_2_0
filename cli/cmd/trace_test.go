package cmd

import (
	"context"
	"fmt"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/jamesatintegratnio/hctl/internal/config"
	"github.com/jamesatintegratnio/hctl/internal/kube"
	"github.com/jamesatintegratnio/hctl/internal/tui"
)

// ---------------------------------------------------------------------------
// fakeKubeClient — implements platform.KubeClient for cmd-level tests
// ---------------------------------------------------------------------------

type fakeKubeClient struct {
	vcluster        *unstructured.Unstructured
	vclusterErr     error
	vclusters       []unstructured.Unstructured
	vclustersErr    error
	argoApps        []unstructured.Unstructured
	argoAppsErr     error
	argoApp         *unstructured.Unstructured
	argoAppErr      error
	argoAppsCluster []kube.ArgoAppInfo
	argoAppsClErr   error
	argoAppsSel     []unstructured.Unstructured
	argoAppsSelErr  error
	pods            []kube.PodInfo
	podsErr         error
	podResources    []kube.PodResourceInfo
	podResourcesErr error
	jobs            []batchv1.Job
	jobsErr         error
	works           []unstructured.Unstructured
	worksErr        error
	workPlacements  []unstructured.Unstructured
	workPlErr       error
	nodes           []kube.NodeInfo
	nodesErr        error
	promises        []unstructured.Unstructured
	promisesErr     error
}

func (f *fakeKubeClient) GetVCluster(_ context.Context, _, _ string) (*unstructured.Unstructured, error) {
	return f.vcluster, f.vclusterErr
}
func (f *fakeKubeClient) ListVClusters(_ context.Context, _ string) ([]unstructured.Unstructured, error) {
	return f.vclusters, f.vclustersErr
}
func (f *fakeKubeClient) ListArgoApps(_ context.Context, _ string) ([]unstructured.Unstructured, error) {
	return f.argoApps, f.argoAppsErr
}
func (f *fakeKubeClient) GetArgoApp(_ context.Context, _, _ string) (*unstructured.Unstructured, error) {
	return f.argoApp, f.argoAppErr
}
func (f *fakeKubeClient) ListArgoAppsForCluster(_ context.Context, _, _ string) ([]kube.ArgoAppInfo, error) {
	return f.argoAppsCluster, f.argoAppsClErr
}
func (f *fakeKubeClient) ListArgoAppsWithSelector(_ context.Context, _, _ string) ([]unstructured.Unstructured, error) {
	return f.argoAppsSel, f.argoAppsSelErr
}
func (f *fakeKubeClient) ListPods(_ context.Context, _, _ string) ([]kube.PodInfo, error) {
	return f.pods, f.podsErr
}
func (f *fakeKubeClient) GetPodResourceInfo(_ context.Context, _, _ string) ([]kube.PodResourceInfo, error) {
	return f.podResources, f.podResourcesErr
}
func (f *fakeKubeClient) ListJobs(_ context.Context, _, _ string) ([]batchv1.Job, error) {
	return f.jobs, f.jobsErr
}
func (f *fakeKubeClient) ListWorks(_ context.Context, _ string) ([]unstructured.Unstructured, error) {
	return f.works, f.worksErr
}
func (f *fakeKubeClient) ListWorkPlacements(_ context.Context, _ string) ([]unstructured.Unstructured, error) {
	return f.workPlacements, f.workPlErr
}
func (f *fakeKubeClient) ListNodes(_ context.Context) ([]kube.NodeInfo, error) {
	return f.nodes, f.nodesErr
}
func (f *fakeKubeClient) ListPromises(_ context.Context) ([]unstructured.Unstructured, error) {
	return f.promises, f.promisesErr
}

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
	fake := &fakeKubeClient{
		vclusterErr: fmt.Errorf("not found"),
		argoAppErr:  fmt.Errorf("not found"),
		podsErr:     fmt.Errorf("not found"),
	}

	err := executeTrace(fake, "nonexistent", cfg)
	if err != nil {
		t.Fatalf("executeTrace should not error when resources are not found, got: %v", err)
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

	fake := &fakeKubeClient{
		vcluster: vc,
		argoApp:  argoApp,
		pods: []kube.PodInfo{
			{Name: "test-vc-0", Phase: "Running", ReadyContainers: 1, TotalContainers: 1},
		},
	}

	err := executeTrace(fake, "test-vc", cfg)
	if err != nil {
		t.Fatalf("executeTrace should succeed, got: %v", err)
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

	fake := &fakeKubeClient{
		vclusterErr: fmt.Errorf("not found"),
		argoApp:     argoApp,
		argoAppsCluster: []kube.ArgoAppInfo{
			{Name: "sub-1", SyncStatus: "Synced", HealthStatus: "Healthy"},
			{Name: "sub-2", SyncStatus: "OutOfSync", HealthStatus: "Degraded"},
		},
		podsErr: fmt.Errorf("no pods"),
	}

	err := executeTrace(fake, "my-app", cfg)
	if err != nil {
		t.Fatalf("executeTrace should succeed, got: %v", err)
	}
}

func TestExecuteTrace_TextOutput(t *testing.T) {
	// Test with text output (not structured) — exercises the rendering path
	tui.SetOutputFormat("")

	cfg := config.Default()
	fake := &fakeKubeClient{
		vclusterErr: fmt.Errorf("not found"),
		argoAppErr:  fmt.Errorf("not found"),
		podsErr:     fmt.Errorf("not found"),
	}

	err := executeTrace(fake, "test-resource", cfg)
	if err != nil {
		t.Fatalf("executeTrace text output should not error, got: %v", err)
	}
}

func TestMin(t *testing.T) {
	tests := []struct {
		a, b, want int
	}{
		{0, 0, 0},
		{1, 2, 1},
		{5, 3, 3},
		{-1, 1, -1},
		{7, 7, 7},
	}
	for _, tt := range tests {
		got := min(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("min(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}
