package vcluster

import (
	"testing"

	"github.com/jamesatintegratnio/hctl/internal/platform"
)

// --- collectNameAndPreset Tests (non-interactive paths) ---

func TestCollectNameAndPreset_NameFromArgs(t *testing.T) {
	cmd := newCreateCmd()
	opts := &CreateOptions{Core: CoreOpts{Preset: "dev"}}

	name, preset, err := collectNameAndPreset(cmd, []string{"my-cluster"}, opts, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "my-cluster" {
		t.Errorf("name = %q, want 'my-cluster'", name)
	}
	if preset != "dev" {
		t.Errorf("preset = %q, want 'dev'", preset)
	}
}

func TestCollectNameAndPreset_DefaultPreset(t *testing.T) {
	cmd := newCreateCmd()
	opts := &CreateOptions{}

	name, preset, err := collectNameAndPreset(cmd, []string{"test"}, opts, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "test" {
		t.Errorf("name = %q, want 'test'", name)
	}
	if preset != "dev" {
		t.Errorf("preset should default to 'dev', got %q", preset)
	}
}

func TestCollectNameAndPreset_EmptyName_NonInteractive(t *testing.T) {
	cmd := newCreateCmd()
	opts := &CreateOptions{}

	_, _, err := collectNameAndPreset(cmd, []string{}, opts, false)
	if err == nil {
		t.Fatal("expected error when name is empty and non-interactive")
	}
}

func TestCollectNameAndPreset_ProdPreset(t *testing.T) {
	cmd := newCreateCmd()
	opts := &CreateOptions{Core: CoreOpts{Preset: "prod"}}

	_, preset, err := collectNameAndPreset(cmd, []string{"production"}, opts, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if preset != "prod" {
		t.Errorf("preset = %q, want 'prod'", preset)
	}
}

// --- applyClusterMetadata Tests ---

func TestApplyClusterMetadata_Labels(t *testing.T) {
	cmd := newCreateCmd()
	opts := &CreateOptions{
		ClusterLabels: []string{"env=production", "team=platform"},
	}
	spec := &platform.VClusterSpec{
		Integrations: platform.DefaultIntegrations(),
	}

	err := applyClusterMetadata(cmd, opts, spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if spec.Integrations.ArgoCD.ClusterLabels["env"] != "production" {
		t.Errorf("label env = %q, want 'production'", spec.Integrations.ArgoCD.ClusterLabels["env"])
	}
	if spec.Integrations.ArgoCD.ClusterLabels["team"] != "platform" {
		t.Errorf("label team = %q, want 'platform'", spec.Integrations.ArgoCD.ClusterLabels["team"])
	}
}

func TestApplyClusterMetadata_Annotations(t *testing.T) {
	cmd := newCreateCmd()
	opts := &CreateOptions{
		ClusterAnnotations: []string{"note=test-cluster"},
	}
	spec := &platform.VClusterSpec{
		Integrations: platform.DefaultIntegrations(),
	}

	err := applyClusterMetadata(cmd, opts, spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if spec.Integrations.ArgoCD.ClusterAnnotations["note"] != "test-cluster" {
		t.Errorf("annotation note = %q, want 'test-cluster'", spec.Integrations.ArgoCD.ClusterAnnotations["note"])
	}
}

func TestApplyClusterMetadata_InvalidLabel(t *testing.T) {
	cmd := newCreateCmd()
	opts := &CreateOptions{
		ClusterLabels: []string{"noequalssign"},
	}
	spec := &platform.VClusterSpec{
		Integrations: platform.DefaultIntegrations(),
	}

	err := applyClusterMetadata(cmd, opts, spec)
	if err == nil {
		t.Fatal("expected error for invalid label format")
	}
}

func TestApplyClusterMetadata_InvalidAnnotation(t *testing.T) {
	cmd := newCreateCmd()
	opts := &CreateOptions{
		ClusterAnnotations: []string{"bad-format"},
	}
	spec := &platform.VClusterSpec{
		Integrations: platform.DefaultIntegrations(),
	}

	err := applyClusterMetadata(cmd, opts, spec)
	if err == nil {
		t.Fatal("expected error for invalid annotation format")
	}
}

func TestApplyClusterMetadata_NoArgoCD(t *testing.T) {
	cmd := newCreateCmd()
	opts := &CreateOptions{
		ClusterLabels: []string{"env=prod"},
	}
	spec := &platform.VClusterSpec{
		Integrations: platform.IntegrationsCfg{
			ArgoCD: nil,
		},
	}

	// Should not panic when ArgoCD is nil
	err := applyClusterMetadata(cmd, opts, spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestApplyClusterMetadata_EmptyLabels(t *testing.T) {
	cmd := newCreateCmd()
	opts := &CreateOptions{}
	spec := &platform.VClusterSpec{
		Integrations: platform.DefaultIntegrations(),
	}

	err := applyClusterMetadata(cmd, opts, spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should not create labels map if none provided
	if spec.Integrations.ArgoCD.ClusterLabels != nil {
		t.Error("expected nil ClusterLabels when none provided")
	}
}

// --- collectWorkloadRepo Tests (non-interactive) ---

func TestCollectWorkloadRepo_WithFlags(t *testing.T) {
	cmd := newCreateCmd()
	opts := &CreateOptions{
		WorkloadRepo: WorkloadRepoOpts{
			URL:      "https://github.com/example/repo",
			BasePath: "clusters/dev",
			Path:     "workloads",
			Revision: "main",
		},
	}
	spec := &platform.VClusterSpec{
		Integrations: platform.DefaultIntegrations(),
	}

	err := collectWorkloadRepo(cmd, opts, spec, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if spec.Integrations.ArgoCD.WorkloadRepo == nil {
		t.Fatal("expected WorkloadRepo to be set")
	}
	if spec.Integrations.ArgoCD.WorkloadRepo.URL != "https://github.com/example/repo" {
		t.Errorf("URL = %q", spec.Integrations.ArgoCD.WorkloadRepo.URL)
	}
	if spec.Integrations.ArgoCD.WorkloadRepo.BasePath != "clusters/dev" {
		t.Errorf("BasePath = %q", spec.Integrations.ArgoCD.WorkloadRepo.BasePath)
	}
	if spec.Integrations.ArgoCD.WorkloadRepo.Path != "workloads" {
		t.Errorf("Path = %q", spec.Integrations.ArgoCD.WorkloadRepo.Path)
	}
	if spec.Integrations.ArgoCD.WorkloadRepo.Revision != "main" {
		t.Errorf("Revision = %q", spec.Integrations.ArgoCD.WorkloadRepo.Revision)
	}
}

func TestCollectWorkloadRepo_NoFlags(t *testing.T) {
	cmd := newCreateCmd()
	opts := &CreateOptions{}
	spec := &platform.VClusterSpec{
		Integrations: platform.DefaultIntegrations(),
	}

	err := collectWorkloadRepo(cmd, opts, spec, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should not create WorkloadRepo if no flags set
	if spec.Integrations.ArgoCD.WorkloadRepo != nil {
		t.Error("expected nil WorkloadRepo when no flags set")
	}
}

func TestCollectWorkloadRepo_PartialFlags(t *testing.T) {
	cmd := newCreateCmd()
	opts := &CreateOptions{
		WorkloadRepo: WorkloadRepoOpts{
			URL: "https://github.com/example/partial",
		},
	}
	spec := &platform.VClusterSpec{
		Integrations: platform.DefaultIntegrations(),
	}

	err := collectWorkloadRepo(cmd, opts, spec, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if spec.Integrations.ArgoCD.WorkloadRepo == nil {
		t.Fatal("expected WorkloadRepo to be set with partial flags")
	}
	if spec.Integrations.ArgoCD.WorkloadRepo.URL != "https://github.com/example/partial" {
		t.Errorf("URL = %q", spec.Integrations.ArgoCD.WorkloadRepo.URL)
	}
	// Other fields should be empty
	if spec.Integrations.ArgoCD.WorkloadRepo.BasePath != "" {
		t.Errorf("BasePath should be empty, got %q", spec.Integrations.ArgoCD.WorkloadRepo.BasePath)
	}
}

func TestCollectWorkloadRepo_NilArgoCD(t *testing.T) {
	cmd := newCreateCmd()
	opts := &CreateOptions{
		WorkloadRepo: WorkloadRepoOpts{
			URL: "https://github.com/example/repo",
		},
	}
	spec := &platform.VClusterSpec{
		Integrations: platform.IntegrationsCfg{
			ArgoCD: nil,
		},
	}

	// Should not panic when ArgoCD is nil
	err := collectWorkloadRepo(cmd, opts, spec, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
