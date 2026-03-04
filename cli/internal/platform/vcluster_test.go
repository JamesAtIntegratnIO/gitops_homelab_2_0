package platform

import (
	"strings"
	"testing"
)

func TestApplyPreset_Dev(t *testing.T) {
	var spec VClusterSpec
	if err := ApplyPreset(&spec, "dev"); err != nil {
		t.Fatalf("ApplyPreset: %v", err)
	}
	if spec.VCluster.Replicas != 1 {
		t.Errorf("Replicas = %d, want 1", spec.VCluster.Replicas)
	}
	if spec.VCluster.IsolationMode != "standard" {
		t.Errorf("IsolationMode = %q, want %q", spec.VCluster.IsolationMode, "standard")
	}
	if spec.VCluster.Resources == nil || spec.VCluster.Resources.Requests["memory"] != "768Mi" {
		t.Error("expected memory request 768Mi")
	}
	if spec.VCluster.BackingStore != nil {
		t.Errorf("BackingStore should be nil for dev, got %v", spec.VCluster.BackingStore)
	}
	if spec.VCluster.Persistence != nil {
		t.Errorf("Persistence should be nil for dev, got %v", spec.VCluster.Persistence)
	}
	if spec.VCluster.CoreDNS == nil || spec.VCluster.CoreDNS.Replicas != 1 {
		t.Error("expected CoreDNS replicas 1")
	}
}

func TestApplyPreset_Prod(t *testing.T) {
	var spec VClusterSpec
	if err := ApplyPreset(&spec, "prod"); err != nil {
		t.Fatalf("ApplyPreset: %v", err)
	}
	if spec.VCluster.Replicas != 3 {
		t.Errorf("Replicas = %d, want 3", spec.VCluster.Replicas)
	}
	if spec.VCluster.Resources == nil || spec.VCluster.Resources.Requests["memory"] != "2Gi" {
		t.Error("expected memory request 2Gi")
	}
	if spec.VCluster.BackingStore == nil {
		t.Fatal("expected etcd backing store for prod")
	}
	if _, ok := spec.VCluster.BackingStore["etcd"]; !ok {
		t.Error("BackingStore missing 'etcd' key")
	}
	if spec.VCluster.Persistence == nil {
		t.Fatal("expected persistence for prod")
	}
	if !spec.VCluster.Persistence.Enabled {
		t.Error("Persistence.Enabled = false, want true")
	}
	if spec.VCluster.Persistence.Size != "10Gi" {
		t.Errorf("Persistence.Size = %q, want %q", spec.VCluster.Persistence.Size, "10Gi")
	}
	if spec.VCluster.CoreDNS == nil || spec.VCluster.CoreDNS.Replicas != 2 {
		t.Error("expected CoreDNS replicas 2")
	}
}

func TestApplyPreset_Unknown(t *testing.T) {
	var spec VClusterSpec
	err := ApplyPreset(&spec, "nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown preset")
	}
	if !strings.Contains(err.Error(), "unknown preset") {
		t.Errorf("error = %q, want substring %q", err.Error(), "unknown preset")
	}
}

func TestApplyPreset_DoesNotOverride(t *testing.T) {
	spec := VClusterSpec{
		VCluster: VClusterConfig{
			Replicas:      5,
			IsolationMode: "custom",
			Resources:     &ResourceRequirements{Requests: map[string]string{"memory": "4Gi"}},
		},
	}
	if err := ApplyPreset(&spec, "dev"); err != nil {
		t.Fatalf("ApplyPreset: %v", err)
	}
	if spec.VCluster.Replicas != 5 {
		t.Errorf("Replicas = %d, want 5 (should not be overridden)", spec.VCluster.Replicas)
	}
	if spec.VCluster.IsolationMode != "custom" {
		t.Errorf("IsolationMode = %q, want %q (should not be overridden)", spec.VCluster.IsolationMode, "custom")
	}
	if spec.VCluster.Resources.Requests["memory"] != "4Gi" {
		t.Error("Resources should not be overridden when already set")
	}
}

func TestPresetNames(t *testing.T) {
	names := PresetNames()
	need := map[string]bool{"dev": false, "prod": false}
	for _, n := range names {
		if _, ok := need[n]; ok {
			need[n] = true
		}
	}
	for k, found := range need {
		if !found {
			t.Errorf("PresetNames() missing %q", k)
		}
	}
}

func TestDefaultIntegrations(t *testing.T) {
	intg := DefaultIntegrations()
	if intg.CertManager == nil {
		t.Fatal("CertManager is nil")
	}
	if v := intg.CertManager.ClusterIssuerSelectorLabels["integratn.tech/cluster-issuer"]; v != "letsencrypt-prod" {
		t.Errorf("CertManager label = %q", v)
	}
	if intg.ExternalSecrets == nil {
		t.Fatal("ExternalSecrets is nil")
	}
	if v := intg.ExternalSecrets.ClusterStoreSelectorLabels["integratn.tech/cluster-secret-store"]; v != "onepassword-store" {
		t.Errorf("ExternalSecrets label = %q", v)
	}
	if intg.ArgoCD == nil {
		t.Fatal("ArgoCD is nil")
	}
	if intg.ArgoCD.Environment != "production" {
		t.Errorf("ArgoCD.Environment = %q", intg.ArgoCD.Environment)
	}
}

func TestDefaultArgocdApp(t *testing.T) {
	app := DefaultArgocdApp()
	if app.RepoURL != "https://charts.loft.sh" {
		t.Errorf("RepoURL = %q", app.RepoURL)
	}
	if app.Chart != "vcluster" {
		t.Errorf("Chart = %q", app.Chart)
	}
	if app.TargetRevision != "0.31.0" {
		t.Errorf("TargetRevision = %q", app.TargetRevision)
	}
	if app.SyncPolicy == nil {
		t.Fatal("SyncPolicy is nil")
	}
	auto, ok := app.SyncPolicy["automated"]
	if !ok {
		t.Fatal("SyncPolicy missing 'automated' key")
	}
	autoMap, ok := auto.(map[string]interface{})
	if !ok {
		t.Fatalf("automated is %T, want map", auto)
	}
	if autoMap["selfHeal"] != true {
		t.Error("selfHeal not true")
	}
	if autoMap["prune"] != true {
		t.Error("prune not true")
	}
}

func TestNewVClusterResource(t *testing.T) {
	spec := VClusterSpec{
		Name:            "media",
		TargetNamespace: "vcluster-media",
		ProjectName:     "media-project",
	}
	r := NewVClusterResource(spec, "platform-requests")

	if r.APIVersion != "platform.integratn.tech/v1alpha1" {
		t.Errorf("APIVersion = %q", r.APIVersion)
	}
	if r.Kind != "VClusterOrchestratorV2" {
		t.Errorf("Kind = %q", r.Kind)
	}
	if r.Metadata.Name != "media" {
		t.Errorf("Metadata.Name = %q", r.Metadata.Name)
	}
	if r.Metadata.Namespace != "platform-requests" {
		t.Errorf("Metadata.Namespace = %q", r.Metadata.Namespace)
	}
	if r.Spec.TargetNamespace != "vcluster-media" {
		t.Errorf("Spec.TargetNamespace = %q", r.Spec.TargetNamespace)
	}
	if r.Spec.ProjectName != "media-project" {
		t.Errorf("Spec.ProjectName = %q", r.Spec.ProjectName)
	}
}

func TestNewVClusterResource_DefaultsNamespace(t *testing.T) {
	spec := VClusterSpec{
		Name: "dev",
	}
	r := NewVClusterResource(spec, "ns")

	if r.Spec.TargetNamespace != "dev" {
		t.Errorf("TargetNamespace = %q, want %q (should default to Name)", r.Spec.TargetNamespace, "dev")
	}
	if r.Spec.ProjectName != "dev" {
		t.Errorf("ProjectName = %q, want %q (should default to Name)", r.Spec.ProjectName, "dev")
	}
}

func TestMarshal(t *testing.T) {
	spec := VClusterSpec{
		Name:            "test",
		TargetNamespace: "vcluster-test",
	}
	r := NewVClusterResource(spec, "default")

	data, err := r.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	yaml := string(data)
	for _, want := range []string{
		"apiVersion: platform.integratn.tech/v1alpha1",
		"kind: VClusterOrchestratorV2",
		"name: test",
		"namespace: default",
		"targetNamespace: vcluster-test",
	} {
		if !strings.Contains(yaml, want) {
			t.Errorf("Marshal output missing %q\n--- output ---\n%s", want, yaml)
		}
	}
}
