package main

import (
	"strings"
	"testing"

	ku "github.com/jamesatintegratnio/gitops_homelab_2_0/promises/_shared/kratixutil"
)

func TestHandleConfigure_MinimalValid(t *testing.T) {
	sdk, dir := ku.NewTestSDK(t)
	resource := &ku.MockResource{
		Name: "test-project",
		Data: map[string]interface{}{
			"spec": map[string]interface{}{
				"name": "my-project",
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

	output := ku.ReadOutput(t, dir, "resources/appproject.yaml")
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
	sdk, dir := ku.NewTestSDK(t)
	resource := &ku.MockResource{
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

	config, err := buildConfig(nil, resource)
	if err != nil {
		t.Fatalf("buildConfig: %v", err)
	}

	err = handleConfigure(sdk, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := ku.ReadOutput(t, dir, "resources/appproject.yaml")
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

func TestBuildConfig_MissingName(t *testing.T) {
	resource := &ku.MockResource{
		Data: map[string]interface{}{
			"spec": map[string]interface{}{},
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

func TestHandleDelete_Success(t *testing.T) {
	sdk, dir := ku.NewTestSDK(t)
	config := &ProjectConfig{
		Name:      "my-project",
		Namespace: "custom",
	}

	err := handleDelete(sdk, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := ku.ReadOutput(t, dir, "resources/delete-appproject.yaml")
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
	sdk, dir := ku.NewTestSDK(t)
	config := &ProjectConfig{
		Name:      "my-project",
		Namespace: "argocd",
	}

	err := handleDelete(sdk, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := ku.ReadOutput(t, dir, "resources/delete-appproject.yaml")
	if !strings.Contains(output, "namespace: argocd") {
		t.Error("expected default namespace: argocd")
	}
}

func TestHandleDelete_MissingName(t *testing.T) {
	sdk, _ := ku.NewTestSDK(t)
	// With typed config, an empty name still produces output
	// Validation now happens in buildConfig
	config := &ProjectConfig{}

	err := handleDelete(sdk, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildConfig_DefaultNamespace(t *testing.T) {
	resource := &ku.MockResource{
		Data: map[string]interface{}{
			"spec": map[string]interface{}{
				"name": "my-project",
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

func TestBuildConfig_WrongTypeSourceReposReturnsError(t *testing.T) {
	resource := &ku.MockResource{
		Data: map[string]interface{}{
			"spec": map[string]interface{}{
				"name":       "my-project",
				"sourceRepos": "not-a-slice",
			},
		},
	}

	_, err := buildConfig(nil, resource)
	if err == nil {
		t.Fatal("expected error for wrong-type sourceRepos")
	}
	if !strings.Contains(err.Error(), "sourceRepos") {
		t.Errorf("error should mention 'sourceRepos', got: %s", err.Error())
	}
}

func TestToProjectDestinations(t *testing.T) {
	tests := []struct {
		name     string
		input    []map[string]interface{}
		expected []ku.ProjectDestination
		wantErr  bool
	}{
		{
			name:     "nil returns nil",
			input:    nil,
			expected: nil,
		},
		{
			name:     "empty slice returns empty",
			input:    []map[string]interface{}{},
			expected: []ku.ProjectDestination{},
		},
		{
			name: "valid entries",
			input: []map[string]interface{}{
				{"namespace": "*", "server": "https://kubernetes.default.svc"},
				{"namespace": "production", "server": "https://prod.example.com"},
			},
			expected: []ku.ProjectDestination{
				{Namespace: "*", Server: "https://kubernetes.default.svc"},
				{Namespace: "production", Server: "https://prod.example.com"},
			},
		},
		{
			name: "missing keys produce zero-value fields",
			input: []map[string]interface{}{
				{"server": "https://kubernetes.default.svc"},
				{"namespace": "prod"},
			},
			expected: []ku.ProjectDestination{
				{Namespace: "", Server: "https://kubernetes.default.svc"},
				{Namespace: "prod", Server: ""},
			},
		},
		{
			name: "wrong-type values return error",
			input: []map[string]interface{}{
				{"namespace": 42, "server": true},
			},
			wantErr: true,
		},
		{
			name: "wrong-type namespace with valid server",
			input: []map[string]interface{}{
				{"namespace": 42, "server": "https://kubernetes.default.svc"},
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ku.FromMapSliceE[ku.ProjectDestination](tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.expected == nil {
				if got != nil {
					t.Fatalf("expected nil, got %v", got)
				}
				return
			}
			if len(got) != len(tc.expected) {
				t.Fatalf("expected %d items, got %d", len(tc.expected), len(got))
			}
			for i := range tc.expected {
				if got[i] != tc.expected[i] {
					t.Errorf("item %d: expected %+v, got %+v", i, tc.expected[i], got[i])
				}
			}
		})
	}
}

func TestToResourceFilters(t *testing.T) {
	tests := []struct {
		name     string
		input    []map[string]interface{}
		expected []ku.ResourceFilter
		wantErr  bool
	}{
		{
			name:     "nil returns nil",
			input:    nil,
			expected: nil,
		},
		{
			name:     "empty slice returns empty",
			input:    []map[string]interface{}{},
			expected: []ku.ResourceFilter{},
		},
		{
			name: "valid entries",
			input: []map[string]interface{}{
				{"group": "", "kind": "Namespace"},
				{"group": "apps", "kind": "Deployment"},
			},
			expected: []ku.ResourceFilter{
				{Group: "", Kind: "Namespace"},
				{Group: "apps", Kind: "Deployment"},
			},
		},
		{
			name: "missing keys produce zero-value fields",
			input: []map[string]interface{}{
				{"kind": "Secret"},
				{"group": "rbac.authorization.k8s.io"},
			},
			expected: []ku.ResourceFilter{
				{Group: "", Kind: "Secret"},
				{Group: "rbac.authorization.k8s.io", Kind: ""},
			},
		},
		{
			name: "wrong-type values return error",
			input: []map[string]interface{}{
				{"group": 42, "kind": []string{"a", "b"}},
			},
			wantErr: true,
		},
		{
			name: "wrong-type kind with valid group",
			input: []map[string]interface{}{
				{"group": "apps", "kind": 42},
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ku.FromMapSliceE[ku.ResourceFilter](tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.expected == nil {
				if got != nil {
					t.Fatalf("expected nil, got %v", got)
				}
				return
			}
			if len(got) != len(tc.expected) {
				t.Fatalf("expected %d items, got %d", len(tc.expected), len(got))
			}
			for i := range tc.expected {
				if got[i] != tc.expected[i] {
					t.Errorf("item %d: expected %+v, got %+v", i, tc.expected[i], got[i])
				}
			}
		})
	}
}
