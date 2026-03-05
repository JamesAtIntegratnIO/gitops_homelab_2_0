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

// ============================================================================
// buildConfig
// ============================================================================

func TestBuildConfig_MinimalValid(t *testing.T) {
	resource := &mockResource{
		name: "my-secret",
		data: map[string]interface{}{
			"spec": map[string]interface{}{
				"namespace": "my-ns",
				"secrets": []interface{}{
					map[string]interface{}{
						"name":            "db-creds",
						"onePasswordItem": "db-item",
						"keys": []interface{}{
							map[string]interface{}{
								"secretKey": "password",
								"property":  "password",
							},
						},
					},
				},
			},
		},
	}

	config, err := buildConfig(resource)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if config.Namespace != "my-ns" {
		t.Errorf("expected namespace 'my-ns', got %q", config.Namespace)
	}
	if config.AppName != "my-secret" {
		t.Errorf("expected appName from resource name, got %q", config.AppName)
	}
	if config.SecretStoreName != defaultSecretStore {
		t.Errorf("expected default secret store, got %q", config.SecretStoreName)
	}
	if config.SecretStoreKind != defaultSecretStoreKind {
		t.Errorf("expected default secret store kind, got %q", config.SecretStoreKind)
	}
	if config.OwnerPromise != "external-secret" {
		t.Errorf("expected default owner promise, got %q", config.OwnerPromise)
	}
	if len(config.Secrets) != 1 {
		t.Fatalf("expected 1 secret, got %d", len(config.Secrets))
	}
}

func TestBuildConfig_WithOverrides(t *testing.T) {
	resource := &mockResource{
		name: "my-app",
		data: map[string]interface{}{
			"spec": map[string]interface{}{
				"namespace":       "prod",
				"appName":         "custom-app",
				"ownerPromise":    "http-service",
				"secretStoreName": "vault-store",
				"secretStoreKind": "SecretStore",
				"secrets": []interface{}{
					map[string]interface{}{
						"name":            "api-key",
						"onePasswordItem": "api-item",
						"keys": []interface{}{
							map[string]interface{}{
								"secretKey": "token",
								"property":  "api-token",
							},
						},
					},
				},
			},
		},
	}

	config, err := buildConfig(resource)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if config.AppName != "custom-app" {
		t.Errorf("expected 'custom-app', got %q", config.AppName)
	}
	if config.OwnerPromise != "http-service" {
		t.Errorf("expected 'http-service', got %q", config.OwnerPromise)
	}
	if config.SecretStoreName != "vault-store" {
		t.Errorf("expected 'vault-store', got %q", config.SecretStoreName)
	}
	if config.SecretStoreKind != "SecretStore" {
		t.Errorf("expected 'SecretStore', got %q", config.SecretStoreKind)
	}
}

func TestBuildConfig_MissingNamespace(t *testing.T) {
	resource := &mockResource{
		data: map[string]interface{}{
			"spec": map[string]interface{}{
				"secrets": []interface{}{
					map[string]interface{}{"name": "x", "onePasswordItem": "y"},
				},
			},
		},
	}
	_, err := buildConfig(resource)
	if err == nil {
		t.Fatal("expected error for missing namespace")
	}
	if !strings.Contains(err.Error(), "spec.namespace is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBuildConfig_NoSecrets(t *testing.T) {
	resource := &mockResource{
		data: map[string]interface{}{
			"spec": map[string]interface{}{
				"namespace": "ns",
			},
		},
	}
	_, err := buildConfig(resource)
	if err == nil {
		t.Fatal("expected error for missing secrets")
	}
	if !strings.Contains(err.Error(), "spec.secrets must contain at least one entry") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ============================================================================
// buildExternalSecrets
// ============================================================================

func TestBuildExternalSecrets_SingleSecret(t *testing.T) {
	config := &ExternalSecretConfig{
		AppName:         "my-app",
		Namespace:       "production",
		OwnerPromise:    "external-secret",
		SecretStoreName: "onepassword-store",
		SecretStoreKind: "ClusterSecretStore",
		Secrets: []u.SecretRef{
			{
				Name:            "db-creds",
				OnePasswordItem: "db-item",
				Keys: []u.SecretKey{
					{SecretKey: "password", Property: "password"},
					{SecretKey: "username", Property: "username"},
				},
			},
		},
	}

	resources := buildExternalSecrets(config)
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	es := resources[0]
	if es.APIVersion != "external-secrets.io/v1beta1" {
		t.Errorf("wrong apiVersion: %s", es.APIVersion)
	}
	if es.Kind != "ExternalSecret" {
		t.Errorf("wrong kind: %s", es.Kind)
	}
	if es.Metadata.Name != "db-creds" {
		t.Errorf("expected name 'db-creds', got %q", es.Metadata.Name)
	}
	if es.Metadata.Namespace != "production" {
		t.Errorf("expected namespace 'production', got %q", es.Metadata.Namespace)
	}
	if es.Metadata.Labels["kratix.io/promise-name"] != "external-secret" {
		t.Error("missing kratix promise label")
	}
}

func TestBuildExternalSecrets_DefaultName(t *testing.T) {
	config := &ExternalSecretConfig{
		AppName:         "my-app",
		Namespace:       "ns",
		SecretStoreName: "store",
		SecretStoreKind: "ClusterSecretStore",
		Secrets: []u.SecretRef{
			{
				OnePasswordItem: "api-token",
				Keys:            []u.SecretKey{{SecretKey: "token", Property: "credential"}},
			},
		},
	}

	resources := buildExternalSecrets(config)
	if resources[0].Metadata.Name != "my-app-api-token" {
		t.Errorf("expected default name 'my-app-api-token', got %q", resources[0].Metadata.Name)
	}
}

func TestBuildExternalSecrets_MultipleSecrets(t *testing.T) {
	config := &ExternalSecretConfig{
		AppName:         "app",
		Namespace:       "ns",
		SecretStoreName: "store",
		SecretStoreKind: "ClusterSecretStore",
		Secrets: []u.SecretRef{
			{Name: "secret-1", OnePasswordItem: "item-1", Keys: []u.SecretKey{{SecretKey: "k", Property: "p"}}},
			{Name: "secret-2", OnePasswordItem: "item-2", Keys: []u.SecretKey{{SecretKey: "k", Property: "p"}}},
		},
	}

	resources := buildExternalSecrets(config)
	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}
	if resources[0].Metadata.Name != "secret-1" {
		t.Errorf("expected 'secret-1', got %q", resources[0].Metadata.Name)
	}
	if resources[1].Metadata.Name != "secret-2" {
		t.Errorf("expected 'secret-2', got %q", resources[1].Metadata.Name)
	}
}

// ============================================================================
// handleConfigure
// ============================================================================

func TestHandleConfigure_WritesExternalSecrets(t *testing.T) {
	sdk, dir := newTestSDK(t)
	config := &ExternalSecretConfig{
		AppName:         "my-app",
		Namespace:       "production",
		OwnerPromise:    "external-secret",
		SecretStoreName: "onepassword-store",
		SecretStoreKind: "ClusterSecretStore",
		Secrets: []u.SecretRef{
			{
				Name:            "creds",
				OnePasswordItem: "item",
				Keys:            []u.SecretKey{{SecretKey: "pass", Property: "password"}},
			},
		},
	}

	err := handleConfigure(sdk, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := readOutput(t, dir, "resources/external-secrets.yaml")
	if !strings.Contains(output, "kind: ExternalSecret") {
		t.Error("expected ExternalSecret in output")
	}
	if !strings.Contains(output, "name: creds") {
		t.Error("expected secret name in output")
	}
}

// ============================================================================
// handleDelete
// ============================================================================

func TestHandleDelete_CreatesDeleteFiles(t *testing.T) {
	sdk, dir := newTestSDK(t)
	config := &ExternalSecretConfig{
		AppName:   "my-app",
		Namespace: "production",
		Secrets: []u.SecretRef{
			{Name: "secret-a", OnePasswordItem: "item-a"},
			{Name: "secret-b", OnePasswordItem: "item-b"},
		},
	}

	err := handleDelete(sdk, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check delete files for each secret
	for _, name := range []string{"secret-a", "secret-b"} {
		path := fmt.Sprintf("resources/delete-externalsecret-%s.yaml", name)
		output := readOutput(t, dir, path)
		if !strings.Contains(output, "kind: ExternalSecret") {
			t.Errorf("expected ExternalSecret in %s", path)
		}
		if !strings.Contains(output, fmt.Sprintf("name: %s", name)) {
			t.Errorf("expected name %s in %s", name, path)
		}
	}
}
