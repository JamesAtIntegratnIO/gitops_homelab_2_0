package cmd

import (
	"errors"
	"testing"

	"github.com/jamesatintegratnio/hctl/internal/config"
	"github.com/jamesatintegratnio/hctl/internal/kube"
	"github.com/jamesatintegratnio/hctl/internal/testutil"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestRunDiagnoseWithClient_ResourceNotFound(t *testing.T) {
	fake := &testutil.FakeKubeClient{
		VClusterErr: errors.New("not found"),
	}

	cfg := &config.Config{
		Platform: config.PlatformConfig{
			PlatformNamespace: "kratix-platform-system",
		},
	}

	// Diagnose should run through checkers and produce steps (no error —
	// DiagnoseVCluster always returns nil error, steps carry the status).
	err := runDiagnoseWithClient(fake, cfg, "missing-cluster")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunDiagnoseWithClient_HealthyCluster(t *testing.T) {
	vc := &unstructured.Unstructured{}
	vc.SetName("test-vc")
	vc.SetNamespace("kratix-platform-system")
	vc.Object["spec"] = map[string]interface{}{}

	argoApp := unstructured.Unstructured{}
	argoApp.SetName("test-vc")
	argoApp.Object = map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      "test-vc",
			"namespace": "argocd",
		},
		"status": map[string]interface{}{
			"sync":   map[string]interface{}{"status": "Synced"},
			"health": map[string]interface{}{"status": "Healthy"},
		},
	}

	fake := &testutil.FakeKubeClient{
		VCluster: vc,
		ArgoApp:  &argoApp,
		Pods: []kube.PodInfo{
			{Name: "test-vc-0", Phase: "Running", ReadyContainers: 1, TotalContainers: 1},
		},
	}

	cfg := &config.Config{
		Platform: config.PlatformConfig{
			PlatformNamespace: "kratix-platform-system",
		},
	}

	err := runDiagnoseWithClient(fake, cfg, "test-vc")
	if err != nil {
		t.Fatalf("unexpected error for healthy cluster: %v", err)
	}
}
