package kube

import "testing"

func TestSplitFirst(t *testing.T) {
	tests := []struct {
		s, sep, want string
	}{
		{"app:argocd/myapp:Deployment:default/deploy", ":", "app"},
		{"no-separator-here", ":", "no-separator-here"},
		{"", ":", ""},
		{":leading", ":", ""},
		{"a::b", "::", "a"},
	}
	for _, tt := range tests {
		got := splitFirst(tt.s, tt.sep)
		if got != tt.want {
			t.Errorf("splitFirst(%q, %q) = %q, want %q", tt.s, tt.sep, got, tt.want)
		}
	}
}

func TestDeploymentInfoStruct(t *testing.T) {
	// Verify DeploymentInfo is constructable with expected fields
	info := DeploymentInfo{
		Name:     "nginx",
		Replicas: 3,
		ArgoApp:  "my-app",
	}
	if info.Name != "nginx" {
		t.Errorf("Name = %q, want %q", info.Name, "nginx")
	}
	if info.Replicas != 3 {
		t.Errorf("Replicas = %d, want %d", info.Replicas, 3)
	}
	if info.ArgoApp != "my-app" {
		t.Errorf("ArgoApp = %q, want %q", info.ArgoApp, "my-app")
	}
}
