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

// mockResource implements kratix.Resource for testing.
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
		name: "test-app",
		ns:   "default",
		data: map[string]interface{}{
			"spec": map[string]interface{}{
				"name":    "my-app",
				"project": "default",
				"source": map[string]interface{}{
					"repoURL":        "https://charts.example.com",
					"targetRevision": "1.0.0",
				},
				"destination": map[string]interface{}{
					"server":    "https://kubernetes.default.svc",
					"namespace": "production",
				},
			},
		},
	}

	err := handleConfigure(sdk, resource)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := readOutput(t, dir, "resources/application.yaml")
	if !strings.Contains(output, "apiVersion: argoproj.io/v1alpha1") {
		t.Error("expected argoproj.io/v1alpha1 apiVersion")
	}
	if !strings.Contains(output, "kind: Application") {
		t.Error("expected kind: Application")
	}
	if !strings.Contains(output, "name: my-app") {
		t.Error("expected name: my-app")
	}
	if !strings.Contains(output, "namespace: argocd") {
		t.Error("expected default namespace: argocd")
	}
	if !strings.Contains(output, "project: default") {
		t.Error("expected project: default")
	}
	if !strings.Contains(output, "repoURL: https://charts.example.com") {
		t.Error("expected repoURL")
	}
}

func TestHandleConfigure_WithAllFields(t *testing.T) {
	sdk, dir := newTestSDK(t)
	resource := &mockResource{
		name: "full-app",
		data: map[string]interface{}{
			"spec": map[string]interface{}{
				"name":      "full-app",
				"namespace": "custom-ns",
				"project":   "my-project",
				"annotations": map[string]interface{}{
					"note": "test",
				},
				"labels": map[string]interface{}{
					"team": "platform",
				},
				"finalizers": []interface{}{
					"resources-finalizer.argocd.argoproj.io",
				},
				"source": map[string]interface{}{
					"repoURL":        "https://charts.example.com",
					"chart":          "nginx",
					"targetRevision": "2.0.0",
					"helm": map[string]interface{}{
						"releaseName": "my-release",
						"valuesObject": map[string]interface{}{
							"replicaCount": 3,
						},
					},
				},
				"destination": map[string]interface{}{
					"server":    "https://kubernetes.default.svc",
					"namespace": "production",
				},
				"syncPolicy": map[string]interface{}{
					"automated": map[string]interface{}{
						"selfHeal": true,
						"prune":    true,
					},
				},
			},
		},
	}

	err := handleConfigure(sdk, resource)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := readOutput(t, dir, "resources/application.yaml")
	if !strings.Contains(output, "namespace: custom-ns") {
		t.Error("expected custom namespace")
	}
	if !strings.Contains(output, "chart: nginx") {
		t.Error("expected chart: nginx")
	}
	if !strings.Contains(output, "releaseName: my-release") {
		t.Error("expected releaseName")
	}
	if !strings.Contains(output, "note: test") {
		t.Error("expected annotation")
	}
	if !strings.Contains(output, "team: platform") {
		t.Error("expected label")
	}
	if !strings.Contains(output, "resources-finalizer.argocd.argoproj.io") {
		t.Error("expected finalizer")
	}
}

func TestHandleConfigure_MissingName(t *testing.T) {
	sdk, _ := newTestSDK(t)
	resource := &mockResource{
		data: map[string]interface{}{
			"spec": map[string]interface{}{
				"project": "default",
			},
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

func TestHandleConfigure_MissingProject(t *testing.T) {
	sdk, _ := newTestSDK(t)
	resource := &mockResource{
		data: map[string]interface{}{
			"spec": map[string]interface{}{
				"name": "my-app",
			},
		},
	}

	err := handleConfigure(sdk, resource)
	if err == nil {
		t.Fatal("expected error for missing project")
	}
	if !strings.Contains(err.Error(), "spec.project is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleConfigure_MissingSource(t *testing.T) {
	sdk, _ := newTestSDK(t)
	resource := &mockResource{
		data: map[string]interface{}{
			"spec": map[string]interface{}{
				"name":    "my-app",
				"project": "default",
			},
		},
	}

	err := handleConfigure(sdk, resource)
	if err == nil {
		t.Fatal("expected error for missing source")
	}
	if !strings.Contains(err.Error(), "spec.source.repoURL is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleConfigure_MissingDestination(t *testing.T) {
	sdk, _ := newTestSDK(t)
	resource := &mockResource{
		data: map[string]interface{}{
			"spec": map[string]interface{}{
				"name":    "my-app",
				"project": "default",
				"source": map[string]interface{}{
					"repoURL":        "https://example.com",
					"targetRevision": "1.0.0",
				},
			},
		},
	}

	err := handleConfigure(sdk, resource)
	if err == nil {
		t.Fatal("expected error for missing destination")
	}
	if !strings.Contains(err.Error(), "spec.destination.server is required") {
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
				"name":      "my-app",
				"namespace": "custom",
			},
		},
	}

	err := handleDelete(sdk, resource)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := readOutput(t, dir, "resources/delete-application.yaml")
	if !strings.Contains(output, "kind: Application") {
		t.Error("expected kind: Application")
	}
	if !strings.Contains(output, "name: my-app") {
		t.Error("expected name: my-app")
	}
	if !strings.Contains(output, "namespace: custom") {
		t.Error("expected namespace: custom")
	}
}

func TestHandleDelete_DefaultNamespace(t *testing.T) {
	sdk, dir := newTestSDK(t)
	resource := &mockResource{
		data: map[string]interface{}{
			"spec": map[string]interface{}{
				"name": "my-app",
			},
		},
	}

	err := handleDelete(sdk, resource)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := readOutput(t, dir, "resources/delete-application.yaml")
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
