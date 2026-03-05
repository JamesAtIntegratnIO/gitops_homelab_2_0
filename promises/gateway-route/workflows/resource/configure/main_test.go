package main

import (
	"strings"
	"testing"

	u "github.com/jamesatintegratnio/gitops_homelab_2_0/promises/_shared/kratixutil"
)

// ============================================================================
// buildConfig
// ============================================================================

func TestBuildConfig_MinimalValid(t *testing.T) {
	resource := &u.MockResource{
		Data: map[string]interface{}{
			"spec": map[string]interface{}{
				"name":      "my-route",
				"namespace": "my-ns",
				"hostname":  "app.example.com",
				"backendRef": map[string]interface{}{
					"name": "web-service",
					"port": 8080,
				},
			},
		},
	}

	config, err := buildConfig(resource)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if config.Name != "my-route" {
		t.Errorf("expected name 'my-route', got %q", config.Name)
	}
	if config.Namespace != "my-ns" {
		t.Errorf("expected namespace 'my-ns', got %q", config.Namespace)
	}
	if config.Hostname != "app.example.com" {
		t.Errorf("expected hostname 'app.example.com', got %q", config.Hostname)
	}
	if config.Path != "/" {
		t.Errorf("expected default path '/', got %q", config.Path)
	}
	if config.BackendName != "web-service" {
		t.Errorf("expected backendName 'web-service', got %q", config.BackendName)
	}
	if config.BackendPort != 8080 {
		t.Errorf("expected backendPort 8080, got %d", config.BackendPort)
	}
	if config.GatewayName != defaultGatewayName {
		t.Errorf("expected default gatewayName, got %q", config.GatewayName)
	}
	if config.GatewayNS != defaultGatewayNS {
		t.Errorf("expected default gatewayNS, got %q", config.GatewayNS)
	}
	if !config.HTTPRedirect {
		t.Error("expected HTTPRedirect default true")
	}
	if config.OwnerPromise != "gateway-route" {
		t.Errorf("expected default ownerPromise, got %q", config.OwnerPromise)
	}
	if config.HTTPSSectionName != defaultHTTPSSection {
		t.Errorf("expected default sectionName, got %q", config.HTTPSSectionName)
	}
}

func TestBuildConfig_WithOverrides(t *testing.T) {
	resource := &u.MockResource{
		Data: map[string]interface{}{
			"spec": map[string]interface{}{
				"name":      "custom-route",
				"namespace": "prod",
				"hostname":  "prod.example.com",
				"path":      "/api",
				"backendRef": map[string]interface{}{
					"name": "api-svc",
					"port": 3000,
				},
				"gateway": map[string]interface{}{
					"name":      "custom-gw",
					"namespace": "custom-gw-ns",
				},
				"httpRedirect":    false,
				"ownerPromise":    "http-service",
				"sectionName":     "custom-https",
				"httpSectionName": "custom-http",
			},
		},
	}

	config, err := buildConfig(resource)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if config.Path != "/api" {
		t.Errorf("expected '/api', got %q", config.Path)
	}
	if config.GatewayName != "custom-gw" {
		t.Errorf("expected 'custom-gw', got %q", config.GatewayName)
	}
	if config.GatewayNS != "custom-gw-ns" {
		t.Errorf("expected 'custom-gw-ns', got %q", config.GatewayNS)
	}
	if config.HTTPRedirect {
		t.Error("expected HTTPRedirect false")
	}
	if config.OwnerPromise != "http-service" {
		t.Errorf("expected 'http-service', got %q", config.OwnerPromise)
	}
	if config.HTTPSSectionName != "custom-https" {
		t.Errorf("expected 'custom-https', got %q", config.HTTPSSectionName)
	}
	if config.HTTPSectionName != "custom-http" {
		t.Errorf("expected 'custom-http', got %q", config.HTTPSectionName)
	}
}

func TestBuildConfig_MissingName(t *testing.T) {
	resource := &u.MockResource{
		Data: map[string]interface{}{
			"spec": map[string]interface{}{
				"namespace": "ns",
			},
		},
	}
	_, err := buildConfig(resource)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "spec.name is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBuildConfig_MissingNamespace(t *testing.T) {
	resource := &u.MockResource{
		Data: map[string]interface{}{
			"spec": map[string]interface{}{
				"name": "route",
			},
		},
	}
	_, err := buildConfig(resource)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "spec.namespace is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBuildConfig_MissingHostname(t *testing.T) {
	resource := &u.MockResource{
		Data: map[string]interface{}{
			"spec": map[string]interface{}{
				"name":      "route",
				"namespace": "ns",
			},
		},
	}
	_, err := buildConfig(resource)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "spec.hostname is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBuildConfig_MissingBackendName(t *testing.T) {
	resource := &u.MockResource{
		Data: map[string]interface{}{
			"spec": map[string]interface{}{
				"name":      "route",
				"namespace": "ns",
				"hostname":  "app.example.com",
			},
		},
	}
	_, err := buildConfig(resource)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "spec.backendRef.name is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBuildConfig_MissingBackendPort(t *testing.T) {
	resource := &u.MockResource{
		Data: map[string]interface{}{
			"spec": map[string]interface{}{
				"name":      "route",
				"namespace": "ns",
				"hostname":  "app.example.com",
				"backendRef": map[string]interface{}{
					"name": "svc",
				},
			},
		},
	}
	_, err := buildConfig(resource)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "spec.backendRef.port is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ============================================================================
// buildHTTPSRoute
// ============================================================================

func TestBuildHTTPSRoute(t *testing.T) {
	config := &GatewayRouteConfig{
		Name:        "my-route",
		Namespace:   "production",
		Hostname:    "app.example.com",
		Path:        "/api",
		BackendName: "web-svc",
		BackendPort: 8080,
		GatewayName:      "nginx-gateway",
		GatewayNS:        "nginx-gateway",
		HTTPSSectionName: "https",
	}
	labels := map[string]string{"app": "test"}
	route := buildHTTPSRoute(config, labels)

	if route.APIVersion != "gateway.networking.k8s.io/v1" {
		t.Errorf("wrong apiVersion: %s", route.APIVersion)
	}
	if route.Kind != "HTTPRoute" {
		t.Errorf("wrong kind: %s", route.Kind)
	}
	if route.Metadata.Name != "my-route" {
		t.Errorf("wrong name: %s", route.Metadata.Name)
	}
	if route.Metadata.Namespace != "production" {
		t.Errorf("wrong namespace: %s", route.Metadata.Namespace)
	}
	if route.Metadata.Annotations["argocd.argoproj.io/sync-wave"] != "10" {
		t.Error("expected sync-wave annotation")
	}
}

// ============================================================================
// buildHTTPRedirect
// ============================================================================

func TestBuildHTTPRedirect(t *testing.T) {
	config := &GatewayRouteConfig{
		Name:            "my-route",
		Namespace:       "production",
		Hostname:        "app.example.com",
		GatewayName:     "nginx-gateway",
		GatewayNS:       "nginx-gateway",
		HTTPSectionName: "http",
	}
	labels := map[string]string{"app": "test"}
	route := buildHTTPRedirect(config, labels)

	if route.Metadata.Name != "my-route-http-redirect" {
		t.Errorf("expected '-http-redirect' suffix, got %q", route.Metadata.Name)
	}
	if route.Kind != "HTTPRoute" {
		t.Errorf("wrong kind: %s", route.Kind)
	}
}

// ============================================================================
// handleConfigure
// ============================================================================

func TestHandleConfigure_WithRedirect(t *testing.T) {
	sdk, dir := u.NewTestSDK(t)
	config := &GatewayRouteConfig{
		Name:            "test-route",
		Namespace:       "ns",
		Hostname:        "test.example.com",
		Path:            "/",
		BackendName:     "svc",
		BackendPort:     80,
		GatewayName:      "nginx-gateway",
		GatewayNS:        "nginx-gateway",
		HTTPRedirect:     true,
		OwnerPromise:     "gateway-route",
		HTTPSSectionName: "https",
		HTTPSectionName:  "http",
	}

	err := handleConfigure(sdk, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !u.FileExists(dir, "resources/httproute.yaml") {
		t.Error("expected httproute.yaml")
	}
	if !u.FileExists(dir, "resources/http-redirect.yaml") {
		t.Error("expected http-redirect.yaml")
	}

	route := u.ReadOutput(t, dir, "resources/httproute.yaml")
	if !strings.Contains(route, "kind: HTTPRoute") {
		t.Error("expected HTTPRoute kind")
	}

	redirect := u.ReadOutput(t, dir, "resources/http-redirect.yaml")
	if !strings.Contains(redirect, "http-redirect") {
		t.Error("expected redirect route name")
	}
}

func TestHandleConfigure_WithoutRedirect(t *testing.T) {
	sdk, dir := u.NewTestSDK(t)
	config := &GatewayRouteConfig{
		Name:         "test-route",
		Namespace:    "ns",
		Hostname:     "test.example.com",
		Path:         "/",
		BackendName:  "svc",
		BackendPort:  80,
		GatewayName:      "nginx-gateway",
		GatewayNS:        "nginx-gateway",
		HTTPRedirect:     false,
		OwnerPromise:     "gateway-route",
		HTTPSSectionName: "https",
	}

	err := handleConfigure(sdk, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !u.FileExists(dir, "resources/httproute.yaml") {
		t.Error("expected httproute.yaml")
	}
	if u.FileExists(dir, "resources/http-redirect.yaml") {
		t.Error("should not create redirect route when HTTPRedirect is false")
	}
}

// ============================================================================
// handleDelete
// ============================================================================

func TestHandleDelete_WithRedirect(t *testing.T) {
	sdk, dir := u.NewTestSDK(t)
	config := &GatewayRouteConfig{
		Name:         "my-route",
		Namespace:    "ns",
		Hostname:     "app.example.com",
		HTTPRedirect: true,
	}

	err := handleDelete(sdk, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	httpsDelete := u.ReadOutput(t, dir, "resources/delete-httproute-my-route.yaml")
	if !strings.Contains(httpsDelete, "kind: HTTPRoute") {
		t.Error("expected HTTPRoute in delete")
	}
	if !strings.Contains(httpsDelete, "name: my-route") {
		t.Error("expected route name in delete")
	}

	redirectDelete := u.ReadOutput(t, dir, "resources/delete-httproute-my-route-redirect.yaml")
	if !strings.Contains(redirectDelete, "my-route-http-redirect") {
		t.Error("expected redirect route name in delete")
	}
}

func TestHandleDelete_WithoutRedirect(t *testing.T) {
	sdk, dir := u.NewTestSDK(t)
	config := &GatewayRouteConfig{
		Name:         "my-route",
		Namespace:    "ns",
		Hostname:     "app.example.com",
		HTTPRedirect: false,
	}

	err := handleDelete(sdk, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !u.FileExists(dir, "resources/delete-httproute-my-route.yaml") {
		t.Error("expected delete file for HTTPS route")
	}
	if u.FileExists(dir, "resources/delete-httproute-my-route-redirect.yaml") {
		t.Error("should not create delete redirect when HTTPRedirect is false")
	}
}
