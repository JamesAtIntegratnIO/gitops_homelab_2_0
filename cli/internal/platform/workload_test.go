package platform

import (
	"testing"

	"github.com/jamesatintegratnio/hctl/internal/kube"
)

func TestFilterWorkloadDeployments(t *testing.T) {
	deploys := []kube.DeploymentInfo{
		{Name: "myapp", Replicas: 1, ArgoApp: "myapp"},
		{Name: "myapp-worker", Replicas: 1, ArgoApp: "myapp-worker"},
		{Name: "other-service", Replicas: 1, ArgoApp: "other-service"},
		{Name: "nginx", Replicas: 1, ArgoApp: "nginx"},
	}

	tests := []struct {
		name      string
		workload  string
		wantCount int
	}{
		{"exact match", "myapp", 2},       // "myapp" and "myapp-worker"
		{"no match", "nonexistent", 0},
		{"partial match", "nginx", 1},
		{"argoapp match", "other-service", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FilterWorkloadDeployments(deploys, tt.workload)
			if len(result) != tt.wantCount {
				t.Errorf("FilterWorkloadDeployments(%q) got %d, want %d",
					tt.workload, len(result), tt.wantCount)
			}
		})
	}
}

func TestFilterWorkloadDeploymentsEmpty(t *testing.T) {
	result := FilterWorkloadDeployments(nil, "anything")
	if len(result) != 0 {
		t.Errorf("expected empty result for nil input, got %d", len(result))
	}
}

func TestMatchDeployments(t *testing.T) {
	deploys := []kube.DeploymentInfo{
		{Name: "myapp", Replicas: 1, ArgoApp: "myapp"},
		{Name: "other", Replicas: 1, ArgoApp: "other"},
	}

	t.Run("specific match", func(t *testing.T) {
		got, err := MatchDeployments(deploys, "myapp", "ns")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 1 {
			t.Errorf("got %d deployments, want 1", len(got))
		}
	})

	t.Run("no match falls back to all", func(t *testing.T) {
		got, err := MatchDeployments(deploys, "nonexistent", "ns")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != len(deploys) {
			t.Errorf("got %d deployments, want %d (fallback)", len(got), len(deploys))
		}
	})

	t.Run("empty list returns error", func(t *testing.T) {
		_, err := MatchDeployments(nil, "myapp", "ns")
		if err == nil {
			t.Error("expected error for empty deployment list, got nil")
		}
	})
}

func TestResolveWorkloadAndCluster(t *testing.T) {
	t.Run("workload from args with default cluster", func(t *testing.T) {
		name, cluster, err := ResolveWorkloadAndCluster([]string{"myapp"}, "prod")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if name != "myapp" {
			t.Errorf("name = %q, want %q", name, "myapp")
		}
		if cluster != "prod" {
			t.Errorf("cluster = %q, want %q", cluster, "prod")
		}
	})

	t.Run("no args and no score.yaml", func(t *testing.T) {
		_, _, err := ResolveWorkloadAndCluster(nil, "prod")
		if err == nil {
			t.Error("expected error when no args and no score.yaml")
		}
	})

	t.Run("no cluster fallback", func(t *testing.T) {
		_, _, err := ResolveWorkloadAndCluster([]string{"myapp"}, "")
		if err == nil {
			t.Error("expected error when no cluster available")
		}
	})
}
