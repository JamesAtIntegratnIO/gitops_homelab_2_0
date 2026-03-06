package main

import (
	"fmt"
	"strings"
	"testing"

	ku "github.com/jamesatintegratnio/gitops_homelab_2_0/promises/_shared/kratixutil"
)

func TestBuildConfig_MinimalValid(t *testing.T) {
	resource := &ku.MockResource{
		Name: "my-secret",
		Data: map[string]interface{}{
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

	config, err := buildConfig(nil, resource)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if config.Namespace != "my-ns" {
		t.Errorf("expected namespace 'my-ns', got %q", config.Namespace)
	}
	if config.AppName != "my-secret" {
		t.Errorf("expected appName from resource name, got %q", config.AppName)
	}
	if config.SecretStoreName != ku.DefaultSecretStoreName {
		t.Errorf("expected default secret store, got %q", config.SecretStoreName)
	}
	if config.SecretStoreKind != ku.DefaultSecretStoreKind {
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
	resource := &ku.MockResource{
		Name: "my-app",
		Data: map[string]interface{}{
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

	config, err := buildConfig(nil, resource)
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
	resource := &ku.MockResource{
		Data: map[string]interface{}{
			"spec": map[string]interface{}{
				"secrets": []interface{}{
					map[string]interface{}{"name": "x", "onePasswordItem": "y"},
				},
			},
		},
	}
	_, err := buildConfig(nil, resource)
	if err == nil {
		t.Fatal("expected error for missing namespace")
	}
	if !strings.Contains(err.Error(), "spec.namespace is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBuildConfig_NoSecrets(t *testing.T) {
	resource := &ku.MockResource{
		Data: map[string]interface{}{
			"spec": map[string]interface{}{
				"namespace": "ns",
			},
		},
	}
	_, err := buildConfig(nil, resource)
	if err == nil {
		t.Fatal("expected error for missing secrets")
	}
	if !strings.Contains(err.Error(), "spec.secrets must contain at least one entry") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBuildConfig_WrongTypeSecretsReturnsError(t *testing.T) {
	resource := &ku.MockResource{
		Data: map[string]interface{}{
			"spec": map[string]interface{}{
				"namespace": "my-ns",
				"secrets":   "not-an-array",
			},
		},
	}

	_, err := buildConfig(nil, resource)
	if err == nil {
		t.Fatal("expected error for wrong-type secrets")
	}
	if !strings.Contains(err.Error(), "secrets") {
		t.Errorf("error should mention 'secrets', got: %s", err.Error())
	}
}

func TestBuildExternalSecrets_SingleSecret(t *testing.T) {
	config := &ExternalSecretConfig{
		AppName:         "my-app",
		Namespace:       "production",
		OwnerPromise:    "external-secret",
		SecretStoreName: "onepassword-store",
		SecretStoreKind: "ClusterSecretStore",
		Secrets: []ku.SecretRef{
			{
				Name:            "db-creds",
				OnePasswordItem: "db-item",
				Keys: []ku.SecretKey{
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
		Secrets: []ku.SecretRef{
			{
				OnePasswordItem: "api-token",
				Keys:            []ku.SecretKey{{SecretKey: "token", Property: "credential"}},
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
		Secrets: []ku.SecretRef{
			{Name: "secret-1", OnePasswordItem: "item-1", Keys: []ku.SecretKey{{SecretKey: "k", Property: "p"}}},
			{Name: "secret-2", OnePasswordItem: "item-2", Keys: []ku.SecretKey{{SecretKey: "k", Property: "p"}}},
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

func TestHandleConfigure_WritesExternalSecrets(t *testing.T) {
	sdk, dir := ku.NewTestSDK(t)
	config := &ExternalSecretConfig{
		AppName:         "my-app",
		Namespace:       "production",
		OwnerPromise:    "external-secret",
		SecretStoreName: "onepassword-store",
		SecretStoreKind: "ClusterSecretStore",
		Secrets: []ku.SecretRef{
			{
				Name:            "creds",
				OnePasswordItem: "item",
				Keys:            []ku.SecretKey{{SecretKey: "pass", Property: "password"}},
			},
		},
	}

	err := handleConfigure(sdk, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := ku.ReadOutput(t, dir, "resources/external-secrets.yaml")
	if !strings.Contains(output, "kind: ExternalSecret") {
		t.Error("expected ExternalSecret in output")
	}
	if !strings.Contains(output, "name: creds") {
		t.Error("expected secret name in output")
	}
}

func TestHandleDelete_CreatesDeleteFiles(t *testing.T) {
	sdk, dir := ku.NewTestSDK(t)
	config := &ExternalSecretConfig{
		AppName:   "my-app",
		Namespace: "production",
		Secrets: []ku.SecretRef{
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
		output := ku.ReadOutput(t, dir, path)
		if !strings.Contains(output, "kind: ExternalSecret") {
			t.Errorf("expected ExternalSecret in %s", path)
		}
		if !strings.Contains(output, fmt.Sprintf("name: %s", name)) {
			t.Errorf("expected name %s in %s", name, path)
		}
	}
}
