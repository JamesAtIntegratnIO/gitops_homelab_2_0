package kratixutil

import (
	"strings"
	"testing"
)

func TestArgoCDApplicationSpec_Validate_Valid(t *testing.T) {
	s := &ArgoCDApplicationSpec{
		Name:    "my-app",
		Project: "default",
		Source: AppSource{
			RepoURL:        "https://charts.example.com",
			TargetRevision: "1.0.0",
		},
		Destination: Destination{
			Server:    "https://kubernetes.default.svc",
			Namespace: "default",
		},
	}
	if err := s.Validate(); err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
}

func TestArgoCDApplicationSpec_Validate_MissingName(t *testing.T) {
	s := &ArgoCDApplicationSpec{
		Source:      AppSource{RepoURL: "https://charts.example.com"},
		Destination: Destination{Server: "https://kubernetes.default.svc"},
	}
	err := s.Validate()
	if err == nil {
		t.Fatal("expected error for missing name")
	}
	if !strings.Contains(err.Error(), "name is required") {
		t.Errorf("expected 'name is required' error, got: %v", err)
	}
}

func TestArgoCDApplicationSpec_Validate_MissingRepoURL(t *testing.T) {
	s := &ArgoCDApplicationSpec{
		Name:        "my-app",
		Destination: Destination{Server: "https://kubernetes.default.svc"},
	}
	err := s.Validate()
	if err == nil {
		t.Fatal("expected error for missing repoURL")
	}
	if !strings.Contains(err.Error(), "repoURL is required") {
		t.Errorf("expected 'repoURL is required' error, got: %v", err)
	}
}

func TestArgoCDApplicationSpec_Validate_MissingServer(t *testing.T) {
	s := &ArgoCDApplicationSpec{
		Name:   "my-app",
		Source: AppSource{RepoURL: "https://charts.example.com"},
	}
	err := s.Validate()
	if err == nil {
		t.Fatal("expected error for missing server")
	}
	if !strings.Contains(err.Error(), "server is required") {
		t.Errorf("expected 'server is required' error, got: %v", err)
	}
}

func TestArgoCDProjectSpec_Validate_Valid(t *testing.T) {
	s := &ArgoCDProjectSpec{
		Name:      "my-project",
		Namespace: "argocd",
	}
	if err := s.Validate(); err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
}

func TestArgoCDProjectSpec_Validate_MissingName(t *testing.T) {
	s := &ArgoCDProjectSpec{Namespace: "argocd"}
	err := s.Validate()
	if err == nil {
		t.Fatal("expected error for missing name")
	}
	if !strings.Contains(err.Error(), "name is required") {
		t.Errorf("expected 'name is required' error, got: %v", err)
	}
}

func TestArgoCDProjectSpec_Validate_MissingNamespace(t *testing.T) {
	s := &ArgoCDProjectSpec{Name: "my-project"}
	err := s.Validate()
	if err == nil {
		t.Fatal("expected error for missing namespace")
	}
	if !strings.Contains(err.Error(), "namespace is required") {
		t.Errorf("expected 'namespace is required' error, got: %v", err)
	}
}
