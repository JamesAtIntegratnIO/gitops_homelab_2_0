package main

import (
	"fmt"
	"testing"

	ku "github.com/jamesatintegratnio/gitops_homelab_2_0/promises/_shared/kratixutil"
)

// newResourceTestConfig returns a config with all fields needed by resource builders.
func newResourceTestConfig() *HTTPServiceConfig {
	return &HTTPServiceConfig{
		Name:      "myapp",
		Namespace: "production",
		Team:      "backend",
		HTTPNetworkConfig: HTTPNetworkConfig{
			Port:            8080,
			IngressEnabled:  true,
			IngressHostname: "myapp.cluster.integratn.tech",
			IngressPath:     "/",
		},
		GatewayName:     "nginx-gateway",
		GatewayNS:       "nginx-gateway",
		SecretStoreName: ku.DefaultSecretStoreName,
		SecretStoreKind: ku.DefaultSecretStoreKind,
	}
}

// ---------------------------------------------------------------------------
// buildExternalSecretRequest
// ---------------------------------------------------------------------------

func TestBuildExternalSecretRequest_UnnamedSecret(t *testing.T) {
	config := newResourceTestConfig()
	config.Secrets = []ku.SecretRef{
		{
			OnePasswordItem: "db-creds",
			Keys:            []ku.SecretKey{{SecretKey: "password", Property: "password"}},
			// No Name — should fall through without setting "name" in output
		},
	}

	r := buildExternalSecretRequest(config)
	spec := r.Spec.(map[string]interface{})
	secrets := spec["secrets"].([]map[string]interface{})
	if len(secrets) != 1 {
		t.Fatalf("expected 1 secret, got %d", len(secrets))
	}
	if _, hasName := secrets[0]["name"]; hasName {
		t.Error("unnamed secret should not have 'name' key")
	}
}

func TestBuildExternalSecretRequest_NamedSecret(t *testing.T) {
	config := newResourceTestConfig()
	config.Secrets = []ku.SecretRef{
		{
			Name:            "custom-name",
			OnePasswordItem: "vault-item",
			Keys:            []ku.SecretKey{{SecretKey: "token", Property: "token"}},
		},
	}

	r := buildExternalSecretRequest(config)
	spec := r.Spec.(map[string]interface{})
	secrets := spec["secrets"].([]map[string]interface{})
	if secrets[0]["name"] != "custom-name" {
		t.Errorf("expected name 'custom-name', got %v", secrets[0]["name"])
	}
}

func TestBuildExternalSecretRequest_MultipleSecrets(t *testing.T) {
	config := newResourceTestConfig()
	config.Secrets = []ku.SecretRef{
		{OnePasswordItem: "secret-a", Keys: []ku.SecretKey{{SecretKey: "a", Property: "a"}}},
		{OnePasswordItem: "secret-b", Keys: []ku.SecretKey{{SecretKey: "b1", Property: "b1"}, {SecretKey: "b2", Property: "b2"}}},
	}

	r := buildExternalSecretRequest(config)
	spec := r.Spec.(map[string]interface{})
	secrets := spec["secrets"].([]map[string]interface{})
	if len(secrets) != 2 {
		t.Fatalf("expected 2 secrets, got %d", len(secrets))
	}
	// Second secret should have 2 keys
	keys := secrets[1]["keys"].([]map[string]string)
	if len(keys) != 2 {
		t.Errorf("expected 2 keys in second secret, got %d", len(keys))
	}
}

func TestBuildExternalSecretRequest_SpecFields(t *testing.T) {
	config := newResourceTestConfig()
	config.Secrets = []ku.SecretRef{
		{OnePasswordItem: "item", Keys: []ku.SecretKey{{SecretKey: "k", Property: "p"}}},
	}

	r := buildExternalSecretRequest(config)
	spec := r.Spec.(map[string]interface{})

	if spec["namespace"] != "production" {
		t.Errorf("expected namespace 'production', got %v", spec["namespace"])
	}
	if spec["appName"] != "myapp" {
		t.Errorf("expected appName 'myapp', got %v", spec["appName"])
	}
	if spec["secretStoreName"] != ku.DefaultSecretStoreName {
		t.Errorf("expected secret store %q, got %v", ku.DefaultSecretStoreName, spec["secretStoreName"])
	}
	if spec["secretStoreKind"] != ku.DefaultSecretStoreKind {
		t.Errorf("expected secret store kind %q, got %v", ku.DefaultSecretStoreKind, spec["secretStoreKind"])
	}
	if spec["ownerPromise"] != "http-service" {
		t.Errorf("expected ownerPromise 'http-service', got %v", spec["ownerPromise"])
	}
}

func TestBuildExternalSecretRequest_Labels(t *testing.T) {
	config := newResourceTestConfig()
	config.Secrets = []ku.SecretRef{
		{OnePasswordItem: "item", Keys: []ku.SecretKey{{SecretKey: "k", Property: "p"}}},
	}

	r := buildExternalSecretRequest(config)
	labels := r.Metadata.Labels
	if labels["app.kubernetes.io/managed-by"] != "kratix" {
		t.Error("missing managed-by label")
	}
	if labels["kratix.io/promise-name"] != "http-service" {
		t.Error("missing promise-name label")
	}
	if labels["app.kubernetes.io/part-of"] != "myapp" {
		t.Error("missing part-of label")
	}
}

func TestBuildExternalSecretRequest_APIVersionKind(t *testing.T) {
	config := newResourceTestConfig()
	config.Secrets = []ku.SecretRef{
		{OnePasswordItem: "item", Keys: []ku.SecretKey{{SecretKey: "k", Property: "p"}}},
	}

	r := buildExternalSecretRequest(config)
	if r.APIVersion != "platform.integratn.tech/v1alpha1" {
		t.Errorf("wrong apiVersion: %s", r.APIVersion)
	}
	if r.Kind != "PlatformExternalSecret" {
		t.Errorf("wrong kind: %s", r.Kind)
	}
}

// ---------------------------------------------------------------------------
// buildGatewayRouteRequest
// ---------------------------------------------------------------------------

func TestBuildGatewayRouteRequest_FullSpec(t *testing.T) {
	config := newResourceTestConfig()
	r := buildGatewayRouteRequest(config)

	spec := r.Spec.(map[string]interface{})
	if spec["name"] != "myapp" {
		t.Errorf("expected name 'myapp', got %v", spec["name"])
	}
	if spec["namespace"] != "production" {
		t.Errorf("expected namespace 'production', got %v", spec["namespace"])
	}
	if spec["hostname"] != "myapp.cluster.integratn.tech" {
		t.Errorf("expected hostname, got %v", spec["hostname"])
	}
	if spec["path"] != "/" {
		t.Errorf("expected path '/', got %v", spec["path"])
	}
	if spec["httpRedirect"] != true {
		t.Error("expected httpRedirect true")
	}
	if spec["ownerPromise"] != "http-service" {
		t.Errorf("expected ownerPromise 'http-service', got %v", spec["ownerPromise"])
	}
}

func TestBuildGatewayRouteRequest_BackendRef(t *testing.T) {
	config := newResourceTestConfig()
	r := buildGatewayRouteRequest(config)

	spec := r.Spec.(map[string]interface{})
	backend := spec["backendRef"].(map[string]interface{})
	if backend["name"] != "myapp" {
		t.Errorf("expected backend name 'myapp', got %v", backend["name"])
	}
	if backend["port"] != 8080 {
		t.Errorf("expected backend port 8080, got %v", backend["port"])
	}
}

func TestBuildGatewayRouteRequest_GatewayRef(t *testing.T) {
	config := newResourceTestConfig()
	r := buildGatewayRouteRequest(config)

	spec := r.Spec.(map[string]interface{})
	gw := spec["gateway"].(map[string]interface{})
	if gw["name"] != "nginx-gateway" {
		t.Errorf("expected gateway name, got %v", gw["name"])
	}
	if gw["namespace"] != "nginx-gateway" {
		t.Errorf("expected gateway namespace, got %v", gw["namespace"])
	}
}

func TestBuildGatewayRouteRequest_MetadataNamespace(t *testing.T) {
	config := newResourceTestConfig()
	r := buildGatewayRouteRequest(config)

	if r.Metadata.Namespace != ku.DefaultPlatformRequestsNamespace {
		t.Errorf("expected namespace %q, got %q", ku.DefaultPlatformRequestsNamespace, r.Metadata.Namespace)
	}
	if r.Metadata.Name != "myapp-route" {
		t.Errorf("expected name 'myapp-route', got %q", r.Metadata.Name)
	}
}

// ---------------------------------------------------------------------------
// buildNetworkPolicies — spec-level verification
// ---------------------------------------------------------------------------

func TestBuildNetworkPolicies_GatewayPolicySpec(t *testing.T) {
	config := newResourceTestConfig()
	policies := buildNetworkPolicies(config)

	gp := policies[0]
	spec := gp.Spec.(map[string]interface{})

	// podSelector
	ps := spec["podSelector"].(map[string]interface{})
	ml := ps["matchLabels"].(map[string]string)
	if ml["app.kubernetes.io/name"] != "myapp" {
		t.Error("expected podSelector with app name")
	}

	// policyTypes
	pt := spec["policyTypes"].([]string)
	if len(pt) != 1 || pt[0] != "Ingress" {
		t.Errorf("expected [Ingress], got %v", pt)
	}

	// ingress rules
	ingress := spec["ingress"].([]map[string]interface{})
	if len(ingress) != 1 {
		t.Fatalf("expected 1 ingress rule, got %d", len(ingress))
	}
	from := ingress[0]["from"].([]map[string]interface{})
	nsSelector := from[0]["namespaceSelector"].(map[string]interface{})
	nsLabels := nsSelector["matchLabels"].(map[string]string)
	if nsLabels["kubernetes.io/metadata.name"] != "nginx-gateway" {
		t.Error("expected gateway namespace in from selector")
	}
}

func TestBuildNetworkPolicies_DnsPolicySpec(t *testing.T) {
	config := newResourceTestConfig()
	policies := buildNetworkPolicies(config)

	// DNS is last policy (no monitoring)
	dns := policies[len(policies)-1]
	spec := dns.Spec.(map[string]interface{})

	pt := spec["policyTypes"].([]string)
	if len(pt) != 1 || pt[0] != "Egress" {
		t.Errorf("expected [Egress], got %v", pt)
	}

	egress := spec["egress"].([]map[string]interface{})
	if len(egress) != 1 {
		t.Fatalf("expected 1 egress rule, got %d", len(egress))
	}
	ports := egress[0]["ports"].([]map[string]interface{})
	if len(ports) != 2 {
		t.Fatalf("expected 2 DNS ports (UDP+TCP), got %d", len(ports))
	}
}

func TestBuildNetworkPolicies_SyncWaveAnnotation(t *testing.T) {
	config := newResourceTestConfig()
	policies := buildNetworkPolicies(config)

	for _, p := range policies {
		if p.Metadata.Annotations["argocd.argoproj.io/sync-wave"] != "5" {
			t.Errorf("policy %q missing sync-wave annotation", p.Metadata.Name)
		}
	}
}

func TestBuildNetworkPolicies_MonitoringPolicySpec(t *testing.T) {
	config := newResourceTestConfig()
	config.MonitoringEnabled = true

	policies := buildNetworkPolicies(config)
	if len(policies) != 3 {
		t.Fatalf("expected 3 policies with monitoring, got %d", len(policies))
	}

	mon := policies[1]
	if mon.Metadata.Name != fmt.Sprintf("%s-allow-monitoring", config.Name) {
		t.Errorf("wrong monitoring policy name: %q", mon.Metadata.Name)
	}

	spec := mon.Spec.(map[string]interface{})
	ingress := spec["ingress"].([]map[string]interface{})
	from := ingress[0]["from"].([]map[string]interface{})
	nsSelector := from[0]["namespaceSelector"].(map[string]interface{})
	nsLabels := nsSelector["matchLabels"].(map[string]string)
	if nsLabels["kubernetes.io/metadata.name"] != "monitoring" {
		t.Error("expected monitoring namespace in from selector")
	}
}

func TestBuildNetworkPolicies_GatewayPortMatchesConfig(t *testing.T) {
	config := newResourceTestConfig()
	config.Port = 3000

	policies := buildNetworkPolicies(config)
	gp := policies[0]
	spec := gp.Spec.(map[string]interface{})
	ingress := spec["ingress"].([]map[string]interface{})
	ports := ingress[0]["ports"].([]map[string]interface{})
	if ports[0]["port"] != 3000 {
		t.Errorf("expected port 3000, got %v", ports[0]["port"])
	}
}

// ---------------------------------------------------------------------------
// Edge-case / error-path tests — empty, nil, and minimal configs
// ---------------------------------------------------------------------------

func TestBuildExternalSecretRequest_EmptySecrets(t *testing.T) {
	config := newResourceTestConfig()
	config.Secrets = []ku.SecretRef{} // empty slice, no secrets

	r := buildExternalSecretRequest(config)
	spec := r.Spec.(map[string]interface{})
	secrets := spec["secrets"].([]map[string]interface{})
	if len(secrets) != 0 {
		t.Errorf("expected 0 secrets for empty input, got %d", len(secrets))
	}
	// Name and namespace should still be set correctly
	if r.Metadata.Name != "myapp-secrets" {
		t.Errorf("expected name 'myapp-secrets', got %q", r.Metadata.Name)
	}
	if spec["appName"] != "myapp" {
		t.Errorf("expected appName 'myapp', got %v", spec["appName"])
	}
}

func TestBuildExternalSecretRequest_NilSecrets(t *testing.T) {
	config := newResourceTestConfig()
	config.Secrets = nil // nil slice

	r := buildExternalSecretRequest(config)
	spec := r.Spec.(map[string]interface{})
	secrets := spec["secrets"].([]map[string]interface{})
	if len(secrets) != 0 {
		t.Errorf("expected 0 secrets for nil input, got %d", len(secrets))
	}
}

func TestBuildExternalSecretRequest_SecretWithEmptyKeys(t *testing.T) {
	config := newResourceTestConfig()
	config.Secrets = []ku.SecretRef{
		{
			OnePasswordItem: "vault-item",
			Keys:            []ku.SecretKey{}, // no keys
		},
	}

	r := buildExternalSecretRequest(config)
	spec := r.Spec.(map[string]interface{})
	secrets := spec["secrets"].([]map[string]interface{})
	if len(secrets) != 1 {
		t.Fatalf("expected 1 secret, got %d", len(secrets))
	}
	keys := secrets[0]["keys"].([]map[string]string)
	if len(keys) != 0 {
		t.Errorf("expected 0 keys, got %d", len(keys))
	}
}

func TestBuildGatewayRouteRequest_MinimalConfig(t *testing.T) {
	// Minimal: only required fields set, optional fields at zero values
	config := &HTTPServiceConfig{
		Name:      "minimal",
		Namespace: "default",
	}

	r := buildGatewayRouteRequest(config)
	if r.Kind != "GatewayRoute" {
		t.Errorf("expected kind GatewayRoute, got %q", r.Kind)
	}
	if r.Metadata.Name != "minimal-route" {
		t.Errorf("expected name 'minimal-route', got %q", r.Metadata.Name)
	}
	spec := r.Spec.(map[string]interface{})
	if spec["namespace"] != "default" {
		t.Errorf("expected namespace 'default', got %v", spec["namespace"])
	}
	// Zero-value fields should be present but empty/zero
	if spec["hostname"] != "" {
		t.Errorf("expected empty hostname, got %v", spec["hostname"])
	}
	if spec["path"] != "" {
		t.Errorf("expected empty path, got %v", spec["path"])
	}
	backend := spec["backendRef"].(map[string]interface{})
	if backend["port"] != 0 {
		t.Errorf("expected port 0 for minimal config, got %v", backend["port"])
	}
}

func TestBuildNetworkPolicies_MinimalConfig(t *testing.T) {
	config := &HTTPServiceConfig{
		Name:      "minimal",
		Namespace: "default",
	}

	policies := buildNetworkPolicies(config)
	// Should still produce gateway + DNS policies (the baseline 2)
	if len(policies) < 2 {
		t.Fatalf("expected at least 2 baseline policies (gateway + dns), got %d", len(policies))
	}
	// First policy should be the gateway policy
	if policies[0].Metadata.Name != "minimal-allow-gateway" {
		t.Errorf("expected gateway policy name 'minimal-allow-gateway', got %q", policies[0].Metadata.Name)
	}
	// Last policy should be DNS
	dns := policies[len(policies)-1]
	if dns.Metadata.Name != "minimal-allow-dns" {
		t.Errorf("expected DNS policy name 'minimal-allow-dns', got %q", dns.Metadata.Name)
	}
}

func TestBuildNetworkPolicies_MonitoringDisabled(t *testing.T) {
	config := newResourceTestConfig()
	config.MonitoringEnabled = false

	policies := buildNetworkPolicies(config)
	// Should have exactly 2 policies: gateway + DNS (no monitoring)
	if len(policies) != 2 {
		t.Fatalf("expected 2 policies when monitoring disabled, got %d", len(policies))
	}
	for _, p := range policies {
		if p.Metadata.Name == fmt.Sprintf("%s-allow-monitoring", config.Name) {
			t.Error("should not have monitoring policy when monitoring disabled")
		}
	}
}

func TestBuildNetworkPolicies_AllLabelsPresent(t *testing.T) {
	config := newResourceTestConfig()
	policies := buildNetworkPolicies(config)

	for _, p := range policies {
		if p.Metadata.Labels["app.kubernetes.io/managed-by"] != "kratix" {
			t.Errorf("policy %q missing managed-by label", p.Metadata.Name)
		}
		if p.Metadata.Labels["kratix.io/promise-name"] != "http-service" {
			t.Errorf("policy %q missing promise-name label", p.Metadata.Name)
		}
		if p.Metadata.Labels["app.kubernetes.io/part-of"] != config.Name {
			t.Errorf("policy %q missing part-of label", p.Metadata.Name)
		}
	}
}
