package main

import (
	"fmt"
	"testing"

	ku "github.com/jamesatintegratnio/gitops_homelab_2_0/promises/_shared/kratixutil"
)

func TestBuildArgoCDProjectRequest_APIVersion(t *testing.T) {
	config := minimalConfig()
	res := buildArgoCDProjectRequest(config)

	if res.APIVersion != "platform.integratn.tech/v1alpha1" {
		t.Errorf("expected apiVersion 'platform.integratn.tech/v1alpha1', got %q", res.APIVersion)
	}
}

func TestBuildArgoCDProjectRequest_Labels(t *testing.T) {
	config := minimalConfig()
	res := buildArgoCDProjectRequest(config)

	wantLabels := map[string]string{
		"app.kubernetes.io/name":       "argocd-project",
		"app.kubernetes.io/managed-by": "kratix",
		"kratix.io/promise-name":       config.PromiseName,
		"kratix.io/resource-name":      config.Name,
	}
	for k, want := range wantLabels {
		got := res.Metadata.Labels[k]
		if got != want {
			t.Errorf("label %q: got %q, want %q", k, got, want)
		}
	}
}

func TestBuildArgoCDProjectRequest_SpecDetails(t *testing.T) {
	config := minimalConfig()
	res := buildArgoCDProjectRequest(config)

	spec, ok := res.Spec.(ku.ArgoCDProjectSpec)
	if !ok {
		t.Fatal("expected ArgoCDProjectSpec type")
	}

	if spec.Namespace != "argocd" {
		t.Errorf("expected spec namespace 'argocd', got %q", spec.Namespace)
	}
	if spec.Description != fmt.Sprintf("VCluster project for %s", config.Name) {
		t.Errorf("unexpected description: %q", spec.Description)
	}
	if spec.Annotations["argocd.argoproj.io/sync-wave"] != "-1" {
		t.Error("expected sync-wave annotation '-1'")
	}
	if len(spec.SourceRepos) != 1 || spec.SourceRepos[0] != "https://charts.loft.sh" {
		t.Errorf("unexpected sourceRepos: %v", spec.SourceRepos)
	}
	if len(spec.Destinations) != 1 {
		t.Fatalf("expected 1 destination, got %d", len(spec.Destinations))
	}
	if spec.Destinations[0].Namespace != config.TargetNamespace {
		t.Errorf("expected destination namespace %q, got %q", config.TargetNamespace, spec.Destinations[0].Namespace)
	}
	if spec.Destinations[0].Server != "https://kubernetes.default.svc" {
		t.Errorf("expected destination server 'https://kubernetes.default.svc', got %q", spec.Destinations[0].Server)
	}
	if len(spec.ClusterResourceWhitelist) != 1 || spec.ClusterResourceWhitelist[0].Group != "*" {
		t.Error("expected wildcard ClusterResourceWhitelist")
	}
	if len(spec.NamespaceResourceWhitelist) != 1 || spec.NamespaceResourceWhitelist[0].Kind != "*" {
		t.Error("expected wildcard NamespaceResourceWhitelist")
	}
}

func TestBuildArgoCDProjectRequest_SpecLabels(t *testing.T) {
	config := minimalConfig()
	res := buildArgoCDProjectRequest(config)

	spec := res.Spec.(ku.ArgoCDProjectSpec)
	wantSpecLabels := map[string]string{
		"app.kubernetes.io/managed-by":     "kratix",
		"argocd.argoproj.io/project-group": "appteam",
		"kratix.io/promise-name":           config.PromiseName,
		"kratix.io/resource-name":          config.Name,
	}
	for k, want := range wantSpecLabels {
		got := spec.Labels[k]
		if got != want {
			t.Errorf("spec label %q: got %q, want %q", k, got, want)
		}
	}
}

func TestBuildArgoCDApplicationRequest_APIVersionAndLabels(t *testing.T) {
	config := minimalConfig()
	res := buildArgoCDApplicationRequest(config)

	if res.APIVersion != "platform.integratn.tech/v1alpha1" {
		t.Errorf("expected apiVersion 'platform.integratn.tech/v1alpha1', got %q", res.APIVersion)
	}
	if res.Metadata.Labels["app.kubernetes.io/name"] != "argocd-application" {
		t.Error("expected label app.kubernetes.io/name = argocd-application")
	}
}

func TestBuildArgoCDApplicationRequest_SpecSourceAndDestination(t *testing.T) {
	config := minimalConfig()
	config.ValuesObject = map[string]interface{}{"foo": "bar"}
	res := buildArgoCDApplicationRequest(config)

	spec, ok := res.Spec.(ku.ArgoCDApplicationSpec)
	if !ok {
		t.Fatal("expected ArgoCDApplicationSpec type")
	}

	if spec.Namespace != "argocd" {
		t.Errorf("expected spec namespace 'argocd', got %q", spec.Namespace)
	}
	if spec.Annotations["argocd.argoproj.io/sync-wave"] != "0" {
		t.Error("expected sync-wave annotation '0'")
	}
	if len(spec.Finalizers) != 1 || spec.Finalizers[0] != "resources-finalizer.argocd.argoproj.io" {
		t.Errorf("expected ArgoCD resource finalizer, got %v", spec.Finalizers)
	}
	if spec.Source.RepoURL != config.ArgoCDRepoURL {
		t.Errorf("expected source repoURL %q, got %q", config.ArgoCDRepoURL, spec.Source.RepoURL)
	}
	if spec.Source.TargetRevision != config.ArgoCDTargetRevision {
		t.Errorf("expected targetRevision %q, got %q", config.ArgoCDTargetRevision, spec.Source.TargetRevision)
	}
	if spec.Source.Helm == nil {
		t.Fatal("expected helm source")
	}
	if spec.Source.Helm.ReleaseName != config.Name {
		t.Errorf("expected helm releaseName %q, got %q", config.Name, spec.Source.Helm.ReleaseName)
	}
	valuesMap, ok := spec.Source.Helm.ValuesObject.(map[string]interface{})
	if !ok {
		t.Fatal("expected ValuesObject as map")
	}
	if valuesMap["foo"] != "bar" {
		t.Errorf("expected ValuesObject foo=bar, got %v", valuesMap["foo"])
	}
	if spec.Destination.Server != config.ArgoCDDestServer {
		t.Errorf("expected dest server %q, got %q", config.ArgoCDDestServer, spec.Destination.Server)
	}
	if spec.Destination.Namespace != config.TargetNamespace {
		t.Errorf("expected dest namespace %q, got %q", config.TargetNamespace, spec.Destination.Namespace)
	}
}

func TestBuildArgoCDApplicationRequest_SyncPolicyPropagated(t *testing.T) {
	config := minimalConfig()
	res := buildArgoCDApplicationRequest(config)

	spec := res.Spec.(ku.ArgoCDApplicationSpec)
	if spec.SyncPolicy == nil {
		t.Error("expected SyncPolicy to be propagated from config")
	}
}

func TestBuildArgoCDClusterRegistrationRequest_APIVersionAndLabels(t *testing.T) {
	config := minimalConfig()
	res := buildArgoCDClusterRegistrationRequest(config)

	if res.APIVersion != "platform.integratn.tech/v1alpha1" {
		t.Errorf("expected apiVersion 'platform.integratn.tech/v1alpha1', got %q", res.APIVersion)
	}
	if res.Metadata.Labels["app.kubernetes.io/name"] != "argocd-cluster-registration" {
		t.Error("expected label app.kubernetes.io/name = argocd-cluster-registration")
	}
	if res.Metadata.Namespace != config.Namespace {
		t.Errorf("expected namespace %q, got %q", config.Namespace, res.Metadata.Namespace)
	}
}

func TestBuildArgoCDClusterRegistrationRequest_SpecAllFields(t *testing.T) {
	config := minimalConfig()
	res := buildArgoCDClusterRegistrationRequest(config)

	spec, ok := res.Spec.(ku.ArgoCDClusterRegistrationSpec)
	if !ok {
		t.Fatal("expected ArgoCDClusterRegistrationSpec type")
	}

	if spec.TargetNamespace != config.TargetNamespace {
		t.Errorf("expected targetNamespace %q, got %q", config.TargetNamespace, spec.TargetNamespace)
	}
	wantSecret := fmt.Sprintf("vc-%s", config.Name)
	if spec.KubeconfigSecret != wantSecret {
		t.Errorf("expected kubeconfigSecret %q, got %q", wantSecret, spec.KubeconfigSecret)
	}
	if spec.Environment != config.ArgoCDEnvironment {
		t.Errorf("expected environment %q, got %q", config.ArgoCDEnvironment, spec.Environment)
	}
	if spec.BaseDomain != config.BaseDomain {
		t.Errorf("expected baseDomain %q, got %q", config.BaseDomain, spec.BaseDomain)
	}
	if spec.BaseDomainSanitized != config.BaseDomainSanitized {
		t.Errorf("expected baseDomainSanitized %q, got %q", config.BaseDomainSanitized, spec.BaseDomainSanitized)
	}
	if spec.SyncJobName != config.KubeconfigSyncJobName {
		t.Errorf("expected syncJobName %q, got %q", config.KubeconfigSyncJobName, spec.SyncJobName)
	}
	if len(spec.ClusterLabels) == 0 {
		t.Error("expected clusterLabels to be set")
	}
	if len(spec.ClusterAnnotations) == 0 {
		t.Error("expected clusterAnnotations to be set")
	}
}

func TestBuildArgoCDBuilders_DifferentConfigs(t *testing.T) {
	tests := []struct {
		name        string
		configName  string
		projectName string
	}{
		{"short name", "vc1", "vcluster-vc1"},
		{"longer name", "my-production-vcluster", "vcluster-my-production-vcluster"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := minimalConfig()
			config.Name = tt.configName
			config.ProjectName = tt.projectName

			project := buildArgoCDProjectRequest(config)
			if project.Metadata.Name != tt.projectName {
				t.Errorf("project name: got %q, want %q", project.Metadata.Name, tt.projectName)
			}

			app := buildArgoCDApplicationRequest(config)
			wantAppName := fmt.Sprintf("vcluster-%s", tt.configName)
			if app.Metadata.Name != wantAppName {
				t.Errorf("app name: got %q, want %q", app.Metadata.Name, wantAppName)
			}

			reg := buildArgoCDClusterRegistrationRequest(config)
			wantRegName := fmt.Sprintf("%s-cluster-registration", tt.configName)
			if reg.Metadata.Name != wantRegName {
				t.Errorf("registration name: got %q, want %q", reg.Metadata.Name, wantRegName)
			}
		})
	}
}
