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
		Name: "test-app",
		Ns:   "default",
		Data: map[string]interface{}{
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

	output := u.ReadOutput(t, dir, "resources/application.yaml")
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
	sdk, dir := u.NewTestSDK(t)
	resource := &u.MockResource{
		Name: "full-app",
		Data: map[string]interface{}{
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

	output := u.ReadOutput(t, dir, "resources/application.yaml")
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
	sdk, _ := u.NewTestSDK(t)
	resource := &u.MockResource{
		Data: map[string]interface{}{
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
	sdk, _ := u.NewTestSDK(t)
	resource := &u.MockResource{
		Data: map[string]interface{}{
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
	sdk, _ := u.NewTestSDK(t)
	resource := &u.MockResource{
		Data: map[string]interface{}{
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
	sdk, _ := u.NewTestSDK(t)
	resource := &u.MockResource{
		Data: map[string]interface{}{
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
	sdk, dir := u.NewTestSDK(t)
	resource := &u.MockResource{
		Data: map[string]interface{}{
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

	output := u.ReadOutput(t, dir, "resources/delete-application.yaml")
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
	sdk, dir := u.NewTestSDK(t)
	resource := &u.MockResource{
		Data: map[string]interface{}{
			"spec": map[string]interface{}{
				"name": "my-app",
			},
		},
	}

	err := handleDelete(sdk, resource)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := u.ReadOutput(t, dir, "resources/delete-application.yaml")
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
