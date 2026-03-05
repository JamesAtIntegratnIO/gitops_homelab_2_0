package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
// handleConfigure
// ============================================================================

func TestHandleConfigure_MinimalValid(t *testing.T) {
	sdk, dir := newTestSDK(t)
	resource := &mockResource{
		name: "test-project",
		data: map[string]interface{}{
			"spec": map[string]interface{}{
				"name": "my-project",
			},
		},
	}

	err := handleConfigure(sdk, resource)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := readOutput(t, dir, "resources/appproject.yaml")
	if !strings.Contains(output, "apiVersion: argoproj.io/v1alpha1") {
		t.Error("expected argoproj.io/v1alpha1")
	}
	if !strings.Contains(output, "kind: AppProject") {
		t.Error("expected kind: AppProject")
	}
	if !strings.Contains(output, "name: my-project") {
		t.Error("expected name: my-project")
	}
	if !strings.Contains(output, "namespace: argocd") {
		t.Error("expected default namespace: argocd")
	}
}

func TestHandleConfigure_WithAllFields(t *testing.T) {
	sdk, dir := newTestSDK(t)
	resource := &mockResource{
		name: "full-project",
		data: map[string]interface{}{
			"spec": map[string]interface{}{
				"name":        "full-project",
				"namespace":   "custom-argocd",
				"description": "Test project",
				"annotations": map[string]interface{}{
					"note": "test",
				},
				"labels": map[string]interface{}{
					"team": "platform",
				},
				"sourceRepos": []interface{}{
					"https://github.com/org/*",
					"https://charts.example.com",
				},
				"destinations": []interface{}{
					map[string]interface{}{"namespace": "*", "server": "https://kubernetes.default.svc"},
				},
				"clusterResourceWhitelist": []interface{}{
					map[string]interface{}{"group": "", "kind": "Namespace"},
				},
				"namespaceResourceWhitelist": []interface{}{
					map[string]interface{}{"group": "apps", "kind": "Deployment"},
				},
			},
		},
	}

	err := handleConfigure(sdk, resource)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := readOutput(t, dir, "resources/appproject.yaml")
	if !strings.Contains(output, "namespace: custom-argocd") {
		t.Error("expected custom namespace")
	}
	if !strings.Contains(output, "description: Test project") {
		t.Error("expected description")
	}
	if !strings.Contains(output, "note: test") {
		t.Error("expected annotation")
	}
	if !strings.Contains(output, "team: platform") {
		t.Error("expected label")
	}
	if !strings.Contains(output, "https://github.com/org/*") {
		t.Error("expected source repo")
	}
	if !strings.Contains(output, "kind: Namespace") {
		t.Error("expected clusterResourceWhitelist")
	}
}

func TestHandleConfigure_MissingName(t *testing.T) {
	sdk, _ := newTestSDK(t)
	resource := &mockResource{
		data: map[string]interface{}{
			"spec": map[string]interface{}{},
		},
	}

	err := handleConfigure(sdk, resource)
	if err == nil {
		t.Fatal("expected error for missing name")
	}
	if !strings.Contains(err.Error(), "spec.name is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ============================================================================
// handleDelete
// ============================================================================

func TestHandleDelete_Success(t *testing.T) {
	sdk, dir := newTestSDK(t)
	resource := &mockResource{
		data: map[string]interface{}{
			"spec": map[string]interface{}{
				"name":      "my-project",
				"namespace": "custom",
			},
		},
	}

	err := handleDelete(sdk, resource)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := readOutput(t, dir, "resources/delete-appproject.yaml")
	if !strings.Contains(output, "kind: AppProject") {
		t.Error("expected kind: AppProject")
	}
	if !strings.Contains(output, "name: my-project") {
		t.Error("expected name")
	}
	if !strings.Contains(output, "namespace: custom") {
		t.Error("expected custom namespace")
	}
}

func TestHandleDelete_DefaultNamespace(t *testing.T) {
	sdk, dir := newTestSDK(t)
	resource := &mockResource{
		data: map[string]interface{}{
			"spec": map[string]interface{}{
				"name": "my-project",
			},
		},
	}

	err := handleDelete(sdk, resource)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := readOutput(t, dir, "resources/delete-appproject.yaml")
	if !strings.Contains(output, "namespace: argocd") {
		t.Error("expected default namespace: argocd")
	}
}

func TestHandleDelete_MissingName(t *testing.T) {
	sdk, _ := newTestSDK(t)
	resource := &mockResource{
		data: map[string]interface{}{
			"spec": map[string]interface{}{},
		},
	}

	err := handleDelete(sdk, resource)
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}
