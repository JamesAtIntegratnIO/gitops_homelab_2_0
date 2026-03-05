package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	u "github.com/jamesatintegratnio/gitops_homelab_2_0/promises/_shared/kratixutil"
	kratix "github.com/syntasso/kratix-go"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type mockResource struct {
	data map[string]interface{}
	name string
	ns   string
}

var _ kratix.Resource = (*mockResource)(nil)

func (m *mockResource) GetValue(path string) (interface{}, error) {
	keys := strings.Split(strings.TrimPrefix(path, "."), ".")
	var current interface{} = m.data
	for _, key := range keys {
		if cm, ok := current.(map[string]interface{}); ok {
			val, found := cm[key]
			if !found {
				return nil, fmt.Errorf("path %s not found", path)
			}
			current = val
		} else {
			return nil, fmt.Errorf("path %s not found", path)
		}
	}
	return current, nil
}

func (m *mockResource) GetStatus() (kratix.Status, error) { return nil, nil }
func (m *mockResource) GetName() string                    { return m.name }
func (m *mockResource) GetNamespace() string               { return m.ns }
func (m *mockResource) GetGroupVersionKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{}
}
func (m *mockResource) GetLabels() map[string]string      { return nil }
func (m *mockResource) GetAnnotations() map[string]string { return nil }
func (m *mockResource) ToUnstructured() unstructured.Unstructured {
	return unstructured.Unstructured{}
}

func newTestSDK(t *testing.T) (*kratix.KratixSDK, string) {
	t.Helper()
	dir := t.TempDir()
	sdk := kratix.New(kratix.WithOutputDir(dir), kratix.WithMetadataDir(dir))
	return sdk, dir
}

func readOutput(t *testing.T, dir, path string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, path))
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}
	return string(data)
}

func fileExists(dir, path string) bool {
	_, err := os.Stat(filepath.Join(dir, path))
	return err == nil
}

// ============================================================================
// buildConfig
// ============================================================================

func TestBuildConfig_MinimalValid(t *testing.T) {
	sdk := kratix.New()
	resource := &mockResource{
		data: map[string]interface{}{
			"spec": map[string]interface{}{
				"name":              "dev-cluster",
				"targetNamespace":   "vcluster-dev",
				"kubeconfigSecret":  "dev-kubeconfig",
				"externalServerURL": "https://dev.cluster.integratn.tech:6443",
			},
		},
	}

	config, err := buildConfig(sdk, resource)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if config.Name != "dev-cluster" {
		t.Errorf("expected name 'dev-cluster', got %q", config.Name)
	}
	if config.KubeconfigKey != "config" {
		t.Errorf("expected default kubeconfigKey 'config', got %q", config.KubeconfigKey)
	}
	if config.OnePasswordItem != "dev-cluster-kubeconfig" {
		t.Errorf("expected default onePasswordItem, got %q", config.OnePasswordItem)
	}
	if config.Environment != "development" {
		t.Errorf("expected default environment 'development', got %q", config.Environment)
	}
	if config.BaseDomain != "integratn.tech" {
		t.Errorf("expected default baseDomain, got %q", config.BaseDomain)
	}
	if config.BaseDomainSanitized != "integratn-tech" {
		t.Errorf("expected sanitized domain 'integratn-tech', got %q", config.BaseDomainSanitized)
	}
	if config.SyncJobName != "dev-cluster-kubeconfig-sync" {
		t.Errorf("expected default syncJobName, got %q", config.SyncJobName)
	}
}

func TestBuildConfig_WithOverrides(t *testing.T) {
	sdk := kratix.New()
	resource := &mockResource{
		data: map[string]interface{}{
			"spec": map[string]interface{}{
				"name":                   "prod-cluster",
				"targetNamespace":        "vcluster-prod",
				"kubeconfigSecret":       "prod-kubeconfig",
				"externalServerURL":      "https://prod.example.com:6443",
				"kubeconfigKey":          "admin.conf",
				"onePasswordItem":        "custom-item",
				"onePasswordConnectHost": "https://op.example.com",
				"environment":            "production",
				"baseDomain":             "example.com",
				"baseDomainSanitized":    "example-com",
				"syncJobName":            "custom-sync",
				"clusterLabels": map[string]interface{}{
					"env": "prod",
				},
				"clusterAnnotations": map[string]interface{}{
					"note": "production cluster",
				},
			},
		},
	}

	config, err := buildConfig(sdk, resource)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if config.KubeconfigKey != "admin.conf" {
		t.Errorf("expected kubeconfigKey 'admin.conf', got %q", config.KubeconfigKey)
	}
	if config.OnePasswordItem != "custom-item" {
		t.Errorf("expected custom onePasswordItem, got %q", config.OnePasswordItem)
	}
	if config.Environment != "production" {
		t.Errorf("expected 'production', got %q", config.Environment)
	}
	if config.BaseDomainSanitized != "example-com" {
		t.Errorf("expected 'example-com', got %q", config.BaseDomainSanitized)
	}
	if config.SyncJobName != "custom-sync" {
		t.Errorf("expected 'custom-sync', got %q", config.SyncJobName)
	}
	if config.ClusterLabels["env"] != "prod" {
		t.Errorf("expected clusterLabels, got %v", config.ClusterLabels)
	}
	if config.ClusterAnnotations["note"] != "production cluster" {
		t.Errorf("expected clusterAnnotations, got %v", config.ClusterAnnotations)
	}
}

func TestBuildConfig_MissingName(t *testing.T) {
	sdk := kratix.New()
	resource := &mockResource{
		data: map[string]interface{}{
			"spec": map[string]interface{}{
				"targetNamespace": "ns",
			},
		},
	}

	_, err := buildConfig(sdk, resource)
	if err == nil {
		t.Fatal("expected error for missing name")
	}
	if !strings.Contains(err.Error(), "spec.name is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBuildConfig_MissingTargetNamespace(t *testing.T) {
	sdk := kratix.New()
	resource := &mockResource{
		data: map[string]interface{}{
			"spec": map[string]interface{}{
				"name": "test",
			},
		},
	}

	_, err := buildConfig(sdk, resource)
	if err == nil {
		t.Fatal("expected error for missing targetNamespace")
	}
	if !strings.Contains(err.Error(), "spec.targetNamespace is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBuildConfig_MissingKubeconfigSecret(t *testing.T) {
	sdk := kratix.New()
	resource := &mockResource{
		data: map[string]interface{}{
			"spec": map[string]interface{}{
				"name":            "test",
				"targetNamespace": "ns",
			},
		},
	}

	_, err := buildConfig(sdk, resource)
	if err == nil {
		t.Fatal("expected error for missing kubeconfigSecret")
	}
}

func TestBuildConfig_MissingExternalServerURL(t *testing.T) {
	sdk := kratix.New()
	resource := &mockResource{
		data: map[string]interface{}{
			"spec": map[string]interface{}{
				"name":             "test",
				"targetNamespace":  "ns",
				"kubeconfigSecret": "secret",
			},
		},
	}

	_, err := buildConfig(sdk, resource)
	if err == nil {
		t.Fatal("expected error for missing externalServerURL")
	}
}

// ============================================================================
// Builder functions
// ============================================================================

func newTestConfig() *RegistrationConfig {
	return &RegistrationConfig{
		Name:                   "test-cluster",
		TargetNamespace:        "vcluster-test",
		KubeconfigSecret:       "test-kubeconfig",
		KubeconfigKey:          "config",
		ExternalServerURL:      "https://test.example.com:6443",
		OnePasswordItem:        "test-cluster-kubeconfig",
		OnePasswordConnectHost: "https://connect.integratn.tech",
		Environment:            "development",
		BaseDomain:             "integratn.tech",
		BaseDomainSanitized:    "integratn-tech",
		SyncJobName:            "test-cluster-kubeconfig-sync",
		PromiseName:            "argocd-cluster-registration",
	}
}

func TestBuildKubeconfigExternalSecret(t *testing.T) {
	config := newTestConfig()
	es := buildKubeconfigExternalSecret(config)

	if es.APIVersion != "external-secrets.io/v1beta1" {
		t.Errorf("wrong apiVersion: %s", es.APIVersion)
	}
	if es.Kind != "ExternalSecret" {
		t.Errorf("wrong kind: %s", es.Kind)
	}
	if es.Metadata.Name != "test-cluster-kubeconfig" {
		t.Errorf("wrong name: %s", es.Metadata.Name)
	}
	if es.Metadata.Namespace != "vcluster-test" {
		t.Errorf("wrong namespace: %s", es.Metadata.Namespace)
	}
	if es.Metadata.Labels["kratix.io/promise-name"] != "argocd-cluster-registration" {
		t.Error("missing kratix promise label")
	}
	if es.Metadata.Labels["app.kubernetes.io/component"] != "kubeconfig" {
		t.Error("missing component label")
	}
}

func TestBuildKubeconfigSyncRBAC(t *testing.T) {
	config := newTestConfig()
	resources := buildKubeconfigSyncRBAC(config)

	if len(resources) != 4 {
		t.Fatalf("expected 4 RBAC resources, got %d", len(resources))
	}

	// ExternalSecret
	if resources[0].Kind != "ExternalSecret" {
		t.Errorf("expected ExternalSecret, got %s", resources[0].Kind)
	}

	// ServiceAccount
	if resources[1].Kind != "ServiceAccount" {
		t.Errorf("expected ServiceAccount, got %s", resources[1].Kind)
	}
	if resources[1].Metadata.Name != "test-cluster-kubeconfig-sync" {
		t.Errorf("wrong SA name: %s", resources[1].Metadata.Name)
	}

	// Role
	if resources[2].Kind != "Role" {
		t.Errorf("expected Role, got %s", resources[2].Kind)
	}

	// RoleBinding
	if resources[3].Kind != "RoleBinding" {
		t.Errorf("expected RoleBinding, got %s", resources[3].Kind)
	}
	if resources[3].RoleRef == nil {
		t.Fatal("expected RoleRef in RoleBinding")
	}
	if resources[3].RoleRef.Kind != "Role" {
		t.Errorf("expected Role ref, got %s", resources[3].RoleRef.Kind)
	}
	if len(resources[3].Subjects) != 1 {
		t.Fatalf("expected 1 subject, got %d", len(resources[3].Subjects))
	}
	if resources[3].Subjects[0].Kind != "ServiceAccount" {
		t.Errorf("expected ServiceAccount subject, got %s", resources[3].Subjects[0].Kind)
	}
}

func TestBuildKubeconfigSyncJob(t *testing.T) {
	config := newTestConfig()
	job := buildKubeconfigSyncJob(config)

	if job.APIVersion != "batch/v1" {
		t.Errorf("wrong apiVersion: %s", job.APIVersion)
	}
	if job.Kind != "Job" {
		t.Errorf("wrong kind: %s", job.Kind)
	}
	if job.Metadata.Name != "test-cluster-kubeconfig-sync" {
		t.Errorf("wrong name: %s", job.Metadata.Name)
	}

	spec, ok := job.Spec.(u.JobSpec)
	if !ok {
		t.Fatal("Spec is not JobSpec")
	}
	if spec.BackoffLimit != 3 {
		t.Errorf("expected backoffLimit 3, got %d", spec.BackoffLimit)
	}
	if spec.Template.Spec.RestartPolicy != "OnFailure" {
		t.Errorf("wrong restartPolicy: %s", spec.Template.Spec.RestartPolicy)
	}
	if len(spec.Template.Spec.InitContainers) != 1 {
		t.Fatalf("expected 1 init container, got %d", len(spec.Template.Spec.InitContainers))
	}
	if len(spec.Template.Spec.Containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(spec.Template.Spec.Containers))
	}
	if len(spec.Template.Spec.Volumes) != 1 {
		t.Fatalf("expected 1 volume, got %d", len(spec.Template.Spec.Volumes))
	}

	container := spec.Template.Spec.Containers[0]
	if container.Name != "sync-to-onepassword" {
		t.Errorf("wrong container name: %s", container.Name)
	}

	// Check env vars contain expected values
	envMap := map[string]string{}
	for _, e := range container.Env {
		if e.Value != "" {
			envMap[e.Name] = e.Value
		}
	}
	if envMap["CLUSTER_NAME"] != "test-cluster" {
		t.Errorf("wrong CLUSTER_NAME env: %s", envMap["CLUSTER_NAME"])
	}
	if envMap["KUBECONFIG_KEY"] != "config" {
		t.Errorf("wrong KUBECONFIG_KEY env: %s", envMap["KUBECONFIG_KEY"])
	}
}

func TestBuildArgoCDClusterExternalSecret(t *testing.T) {
	config := newTestConfig()
	es := buildArgoCDClusterExternalSecret(config)

	if es.Metadata.Name != "test-cluster-argocd-cluster" {
		t.Errorf("wrong name: %s", es.Metadata.Name)
	}
	if es.Metadata.Namespace != "argocd" {
		t.Errorf("expected namespace 'argocd', got %s", es.Metadata.Namespace)
	}
	if es.Metadata.Labels["argocd.argoproj.io/secret-type"] != "cluster" {
		t.Error("missing argocd secret-type label")
	}
}

func TestBuildArgoCDClusterExternalSecret_WithClusterLabels(t *testing.T) {
	config := newTestConfig()
	config.ClusterLabels = map[string]string{"env": "staging"}

	es := buildArgoCDClusterExternalSecret(config)
	if es.Metadata.Labels["env"] != "staging" {
		t.Error("cluster labels should be merged into resource labels")
	}
}

func TestBuildArgoCDClusterExternalSecret_WithClusterAnnotations(t *testing.T) {
	config := newTestConfig()
	config.ClusterAnnotations = map[string]string{"note": "test"}

	es := buildArgoCDClusterExternalSecret(config)
	if es.Metadata.Annotations == nil || es.Metadata.Annotations["note"] != "test" {
		t.Error("cluster annotations should be included")
	}
}

// ============================================================================
// handleConfigure
// ============================================================================

func TestHandleConfigure_Success(t *testing.T) {
	sdk, dir := newTestSDK(t)
	config := newTestConfig()

	err := handleConfigure(sdk, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check all 4 output files exist
	expectedFiles := []string{
		"resources/kubeconfig-sync-rbac.yaml",
		"resources/kubeconfig-sync-job.yaml",
		"resources/kubeconfig-external-secret.yaml",
		"resources/argocd-cluster-external-secret.yaml",
	}
	for _, f := range expectedFiles {
		if !fileExists(dir, f) {
			t.Errorf("expected output file %s to exist", f)
		}
	}

	// Check RBAC file contains multi-doc
	rbac := readOutput(t, dir, "resources/kubeconfig-sync-rbac.yaml")
	if !strings.Contains(rbac, "---") {
		t.Error("expected multi-doc YAML in RBAC file")
	}
	if !strings.Contains(rbac, "kind: ServiceAccount") {
		t.Error("expected ServiceAccount in RBAC")
	}
	if !strings.Contains(rbac, "kind: Role") {
		t.Error("expected Role in RBAC")
	}
}

// ============================================================================
// handleDelete
// ============================================================================

func TestHandleDelete_CreatesDeleteResources(t *testing.T) {
	sdk, dir := newTestSDK(t)
	config := newTestConfig()

	err := handleDelete(sdk, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should create delete files for each resource
	entries, err := os.ReadDir(filepath.Join(dir, "resources"))
	if err != nil {
		t.Fatalf("failed to read output dir: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected delete resource files")
	}

	// All delete files should contain minimal resources
	for _, entry := range entries {
		content := readOutput(t, dir, "resources/"+entry.Name())
		if !strings.HasPrefix(entry.Name(), "delete-") {
			t.Errorf("unexpected file: %s", entry.Name())
		}
		if !strings.Contains(content, "apiVersion:") {
			t.Errorf("expected apiVersion in %s", entry.Name())
		}
	}
}

// ============================================================================
// deleteOutputPath
// ============================================================================

func TestDeleteOutputPath_DefaultPrefix(t *testing.T) {
	r := u.Resource{Kind: "Service", Metadata: u.ObjectMeta{Name: "web"}}
	path := u.DeleteOutputPathForResource("", r)
	if path != "resources/delete-service-web.yaml" {
		t.Errorf("expected 'resources/delete-service-web.yaml', got %q", path)
	}
}

func TestDeleteOutputPath_CustomPrefix(t *testing.T) {
	r := u.Resource{Kind: "Job", Metadata: u.ObjectMeta{Name: "sync"}}
	path := u.DeleteOutputPathForResource("output", r)
	if path != "output/delete-job-sync.yaml" {
		t.Errorf("expected 'output/delete-job-sync.yaml', got %q", path)
	}
}
