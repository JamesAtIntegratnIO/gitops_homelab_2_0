package main

import (
	"strings"
	"testing"

	u "github.com/jamesatintegratnio/gitops_homelab_2_0/promises/_shared/kratixutil"
)

// ============================================================================
// handleConfigure
// ============================================================================

func TestHandleConfigure_MinimalValid(t *testing.T) {
	sdk, dir := u.NewTestSDK(t)
	resource := &u.MockResource{
		Name: "test-project",
		Data: map[string]interface{}{
			"spec": map[string]interface{}{
				"name": "my-project",
			},
		},
	}

	err := handleConfigure(sdk, resource)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := u.ReadOutput(t, dir, "resources/appproject.yaml")
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
	sdk, dir := u.NewTestSDK(t)
	resource := &u.MockResource{
		Name: "full-project",
		Data: map[string]interface{}{
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

	output := u.ReadOutput(t, dir, "resources/appproject.yaml")
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
	sdk, _ := u.NewTestSDK(t)
	resource := &u.MockResource{
		Data: map[string]interface{}{
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
	sdk, dir := u.NewTestSDK(t)
	resource := &u.MockResource{
		Data: map[string]interface{}{
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

	output := u.ReadOutput(t, dir, "resources/delete-appproject.yaml")
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
	sdk, dir := u.NewTestSDK(t)
	resource := &u.MockResource{
		Data: map[string]interface{}{
			"spec": map[string]interface{}{
				"name": "my-project",
			},
		},
	}

	err := handleDelete(sdk, resource)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := u.ReadOutput(t, dir, "resources/delete-appproject.yaml")
	if !strings.Contains(output, "namespace: argocd") {
		t.Error("expected default namespace: argocd")
	}
}

func TestHandleDelete_MissingName(t *testing.T) {
	sdk, _ := u.NewTestSDK(t)
	resource := &u.MockResource{
		Data: map[string]interface{}{
			"spec": map[string]interface{}{},
		},
	}

	err := handleDelete(sdk, resource)
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}
