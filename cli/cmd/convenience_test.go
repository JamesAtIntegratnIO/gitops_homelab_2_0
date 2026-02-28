package cmd

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
			result := filterWorkloadDeployments(deploys, tt.workload)
			if len(result) != tt.wantCount {
				t.Errorf("filterWorkloadDeployments(%q) got %d, want %d",
					tt.workload, len(result), tt.wantCount)
			}
		})
	}
}

func TestFilterWorkloadDeploymentsEmpty(t *testing.T) {
	result := filterWorkloadDeployments(nil, "anything")
	if len(result) != 0 {
		t.Errorf("expected empty result for nil input, got %d", len(result))
	}
}
