package main

import (
	"strings"
	"testing"

	ku "github.com/jamesatintegratnio/gitops_homelab_2_0/promises/_shared/kratixutil"
)

func TestHandleConfigure_MinimalValid(t *testing.T) {
	sdk, dir := ku.NewTestSDK(t)
	resource := &ku.MockResource{
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

	config, err := buildConfig(nil, resource)
	if err != nil {
		t.Fatalf("buildConfig: %v", err)
	}

	err = handleConfigure(sdk, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resources := ku.ReadOutputAsResources(t, dir, "resources/application.yaml")
	app := ku.FindResource(resources, "Application", "my-app")
	if app == nil {
		t.Fatal("expected Application resource named 'my-app'")
	}
	if app.APIVersion != "argoproj.io/v1alpha1" {
		t.Errorf("expected apiVersion argoproj.io/v1alpha1, got %q", app.APIVersion)
	}
	if app.Metadata.Namespace != "argocd" {
		t.Errorf("expected namespace argocd, got %q", app.Metadata.Namespace)
	}
	// Verify spec fields via raw string check for nested fields not in Resource struct
	output := ku.ReadOutput(t, dir, "resources/application.yaml")
	if !strings.Contains(output, "project: default") {
		t.Error("expected project: default")
	}
	if !strings.Contains(output, "repoURL: https://charts.example.com") {
		t.Error("expected repoURL")
	}
}

func TestHandleConfigure_WithAllFields(t *testing.T) {
	sdk, dir := ku.NewTestSDK(t)
	resource := &ku.MockResource{
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

	config, err := buildConfig(nil, resource)
	if err != nil {
		t.Fatalf("buildConfig: %v", err)
	}

	err = handleConfigure(sdk, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resources := ku.ReadOutputAsResources(t, dir, "resources/application.yaml")
	app := ku.FindResource(resources, "Application", "full-app")
	if app == nil {
		t.Fatal("expected Application resource named 'full-app'")
	}
	if app.Metadata.Namespace != "custom-ns" {
		t.Errorf("expected namespace custom-ns, got %q", app.Metadata.Namespace)
	}
	if app.Metadata.Annotations["note"] != "test" {
		t.Error("expected annotation note=test")
	}
	if app.Metadata.Labels["team"] != "platform" {
		t.Error("expected label team=platform")
	}
	// Verify nested spec fields via string check (spec is interface{})
	output := ku.ReadOutput(t, dir, "resources/application.yaml")
	if !strings.Contains(output, "chart: nginx") {
		t.Error("expected chart: nginx")
	}
	if !strings.Contains(output, "releaseName: my-release") {
		t.Error("expected releaseName")
	}
	if !strings.Contains(output, "resources-finalizer.argocd.argoproj.io") {
		t.Error("expected finalizer")
	}
}

func TestBuildConfig_MissingName(t *testing.T) {
	resource := &ku.MockResource{
		Data: map[string]interface{}{
			"spec": map[string]interface{}{
				"project": "default",
			},
		},
	}

	_, err := buildConfig(nil, resource)
	if err == nil {
		t.Fatal("expected error for missing name")
	}
	if !strings.Contains(err.Error(), "spec.name is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBuildConfig_MissingProject(t *testing.T) {
	resource := &ku.MockResource{
		Data: map[string]interface{}{
			"spec": map[string]interface{}{
				"name": "my-app",
			},
		},
	}

	_, err := buildConfig(nil, resource)
	if err == nil {
		t.Fatal("expected error for missing project")
	}
	if !strings.Contains(err.Error(), "spec.project is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBuildConfig_MissingSource(t *testing.T) {
	resource := &ku.MockResource{
		Data: map[string]interface{}{
			"spec": map[string]interface{}{
				"name":    "my-app",
				"project": "default",
			},
		},
	}

	_, err := buildConfig(nil, resource)
	if err == nil {
		t.Fatal("expected error for missing source")
	}
	if !strings.Contains(err.Error(), "spec.source.repoURL is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBuildConfig_MissingDestination(t *testing.T) {
	resource := &ku.MockResource{
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

	_, err := buildConfig(nil, resource)
	if err == nil {
		t.Fatal("expected error for missing destination")
	}
	if !strings.Contains(err.Error(), "spec.destination.server is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleDelete_Success(t *testing.T) {
	sdk, dir := ku.NewTestSDK(t)
	config := &AppConfig{
		Name:      "my-app",
		Namespace: "custom",
	}

	err := handleDelete(sdk, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resources := ku.ReadOutputAsResources(t, dir, "resources/delete-application.yaml")
	app := ku.FindResource(resources, "Application", "my-app")
	if app == nil {
		t.Fatal("expected Application delete resource named 'my-app'")
	}
	if app.Metadata.Namespace != "custom" {
		t.Errorf("expected namespace custom, got %q", app.Metadata.Namespace)
	}
}

func TestHandleDelete_DefaultNamespace(t *testing.T) {
	sdk, dir := ku.NewTestSDK(t)
	config := &AppConfig{
		Name:      "my-app",
		Namespace: "argocd",
	}

	err := handleDelete(sdk, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := ku.ReadOutput(t, dir, "resources/delete-application.yaml")
	if !strings.Contains(output, "namespace: argocd") {
		t.Error("expected default namespace: argocd")
	}
}

func TestHandleDelete_MissingName(t *testing.T) {
	sdk, _ := ku.NewTestSDK(t)
	// With typed config, an empty name will still produce output (no validation in handleDelete)
	// The validation now happens in buildConfig
	config := &AppConfig{}

	err := handleDelete(sdk, config)
	// handleDelete with an empty config should still work (writes empty name)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildConfig_DefaultNamespace(t *testing.T) {
	resource := &ku.MockResource{
		Data: map[string]interface{}{
			"spec": map[string]interface{}{
				"name":    "my-app",
				"project": "default",
				"source": map[string]interface{}{
					"repoURL":        "https://example.com",
					"targetRevision": "1.0.0",
				},
				"destination": map[string]interface{}{
					"server":    "https://kubernetes.default.svc",
					"namespace": "production",
				},
			},
		},
	}

	config, err := buildConfig(nil, resource)
	if err != nil {
		t.Fatalf("buildConfig: %v", err)
	}
	if config.Namespace != "argocd" {
		t.Errorf("expected default namespace 'argocd', got %q", config.Namespace)
	}
}

func TestBuildConfig_WrongTypeAnnotationsReturnsError(t *testing.T) {
	resource := &ku.MockResource{
		Data: map[string]interface{}{
			"spec": map[string]interface{}{
				"name":        "my-app",
				"project":     "default",
				"annotations": "not-a-map",
				"source": map[string]interface{}{
					"repoURL":        "https://example.com",
					"targetRevision": "1.0.0",
				},
				"destination": map[string]interface{}{
					"server":    "https://kubernetes.default.svc",
					"namespace": "production",
				},
			},
		},
	}

	_, err := buildConfig(nil, resource)
	if err == nil {
		t.Fatal("expected error for wrong-type annotations")
	}
	if !strings.Contains(err.Error(), "annotations") {
		t.Errorf("error should mention 'annotations', got: %s", err.Error())
	}
}
