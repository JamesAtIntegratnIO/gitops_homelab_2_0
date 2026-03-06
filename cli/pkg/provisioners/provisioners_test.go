package provisioners

import (
	"strings"
	"testing"

	"github.com/jamesatintegratnio/hctl/internal/score"
)

// --- Registry Tests ---

func TestNewRegistry_HasAllProvisionerTypes(t *testing.T) {
	r := NewRegistry()
	expected := []string{"postgres", "redis", "route", "volume", "dns"}
	types := r.Types()

	if len(types) != len(expected) {
		t.Errorf("expected %d provisioner types, got %d", len(expected), len(types))
	}

	typeSet := make(map[string]bool)
	for _, tp := range types {
		typeSet[tp] = true
	}
	for _, e := range expected {
		if !typeSet[e] {
			t.Errorf("missing provisioner type %q", e)
		}
	}
}

func TestRegistry_Get_Existing(t *testing.T) {
	r := NewRegistry()
	for _, tp := range []string{"postgres", "redis", "route", "volume", "dns"} {
		p, err := r.Get(tp)
		if err != nil {
			t.Errorf("Get(%q) returned error: %v", tp, err)
		}
		if p == nil {
			t.Errorf("Get(%q) returned nil provisioner", tp)
		}
		if p.Type() != tp {
			t.Errorf("Get(%q).Type() = %q", tp, p.Type())
		}
	}
}

func TestRegistry_Get_NotFound(t *testing.T) {
	r := NewRegistry()
	_, err := r.Get("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent provisioner type")
	}
	if !strings.Contains(err.Error(), "no provisioner") {
		t.Errorf("error should mention 'no provisioner', got: %v", err)
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("error should mention the requested type, got: %v", err)
	}
}

func TestRegistry_Register_Custom(t *testing.T) {
	r := &Registry{provisioners: make(map[string]Provisioner)}
	r.Register(&PostgresProvisioner{})
	if len(r.Types()) != 1 {
		t.Fatalf("expected 1 type, got %d", len(r.Types()))
	}
	p, err := r.Get("postgres")
	if err != nil {
		t.Fatal(err)
	}
	if p.Type() != "postgres" {
		t.Errorf("expected type 'postgres', got %q", p.Type())
	}
}

func TestRegistry_Register_Overwrites(t *testing.T) {
	r := &Registry{provisioners: make(map[string]Provisioner)}
	r.Register(&PostgresProvisioner{})
	r.Register(&PostgresProvisioner{}) // overwrite
	if len(r.Types()) != 1 {
		t.Errorf("expected 1 type after overwrite, got %d", len(r.Types()))
	}
}

func TestRegistry_Types_Empty(t *testing.T) {
	r := &Registry{provisioners: make(map[string]Provisioner)}
	if len(r.Types()) != 0 {
		t.Errorf("expected 0 types for empty registry, got %d", len(r.Types()))
	}
}

// --- Postgres Provisioner Tests ---

func TestPostgresProvisioner_Type(t *testing.T) {
	p := &PostgresProvisioner{}
	if p.Type() != "postgres" {
		t.Errorf("Type() = %q, want 'postgres'", p.Type())
	}
}

func TestPostgresProvisioner_Provision(t *testing.T) {
	p := &PostgresProvisioner{}
	res := score.Resource{Type: "postgres"}
	result, err := p.Provision("db", res, "my-app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check outputs
	expectedOutputKeys := []string{"host", "port", "name", "database", "username", "password"}
	for _, k := range expectedOutputKeys {
		if _, ok := result.Outputs[k]; !ok {
			t.Errorf("missing output key %q", k)
		}
	}

	// Outputs should reference the secret name
	secretName := "my-app-db-credentials"
	for k, v := range result.Outputs {
		if !strings.Contains(v, secretName) {
			t.Errorf("output %q = %q, expected to contain %q", k, v, secretName)
		}
	}

	// Check manifests
	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}
	m := result.Manifests[0]
	if m["kind"] != "ExternalSecret" {
		t.Errorf("manifest kind = %v, want ExternalSecret", m["kind"])
	}
	if m["apiVersion"] != "external-secrets.io/v1beta1" {
		t.Errorf("manifest apiVersion = %v, want external-secrets.io/v1beta1", m["apiVersion"])
	}

	meta := m["metadata"].(map[string]interface{})
	if meta["name"] != secretName {
		t.Errorf("metadata.name = %v, want %q", meta["name"], secretName)
	}
}

// --- Redis Provisioner Tests ---

func TestRedisProvisioner_Type(t *testing.T) {
	p := &RedisProvisioner{}
	if p.Type() != "redis" {
		t.Errorf("Type() = %q, want 'redis'", p.Type())
	}
}

func TestRedisProvisioner_Provision(t *testing.T) {
	p := &RedisProvisioner{}
	res := score.Resource{Type: "redis"}
	result, err := p.Provision("cache", res, "my-app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedOutputKeys := []string{"host", "port", "password"}
	for _, k := range expectedOutputKeys {
		if _, ok := result.Outputs[k]; !ok {
			t.Errorf("missing output key %q", k)
		}
	}

	secretName := "my-app-cache-credentials"
	for k, v := range result.Outputs {
		if !strings.Contains(v, secretName) {
			t.Errorf("output %q = %q, expected to contain %q", k, v, secretName)
		}
	}

	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}
	if result.Manifests[0]["kind"] != "ExternalSecret" {
		t.Errorf("manifest kind = %v, want ExternalSecret", result.Manifests[0]["kind"])
	}
}

// --- Route Provisioner Tests ---

func TestRouteProvisioner_Type(t *testing.T) {
	p := &RouteProvisioner{}
	if p.Type() != "route" {
		t.Errorf("Type() = %q, want 'route'", p.Type())
	}
}

func TestRouteProvisioner_Provision_Valid(t *testing.T) {
	p := &RouteProvisioner{}
	res := score.Resource{
		Type: "route",
		Params: map[string]interface{}{
			"host": "myapp.example.com",
			"path": "/api",
			"port": 9090,
		},
	}

	result, err := p.Provision("web", res, "my-app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}
	m := result.Manifests[0]
	if m["kind"] != "HTTPRoute" {
		t.Errorf("manifest kind = %v, want HTTPRoute", m["kind"])
	}

	meta := m["metadata"].(map[string]interface{})
	if meta["name"] != "my-app-web" {
		t.Errorf("manifest name = %v, want 'my-app-web'", meta["name"])
	}
}

func TestRouteProvisioner_Provision_DefaultPath(t *testing.T) {
	p := &RouteProvisioner{}
	res := score.Resource{
		Type: "route",
		Params: map[string]interface{}{
			"host": "myapp.example.com",
		},
	}

	result, err := p.Provision("web", res, "my-app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should default path to "/"
	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}
}

func TestRouteProvisioner_Provision_MissingHost(t *testing.T) {
	p := &RouteProvisioner{}
	res := score.Resource{
		Type:   "route",
		Params: map[string]interface{}{},
	}

	_, err := p.Provision("web", res, "my-app")
	if err == nil {
		t.Fatal("expected error when host is missing")
	}
	if !strings.Contains(err.Error(), "requires params.host") {
		t.Errorf("error message should mention host requirement, got: %v", err)
	}
}

func TestRouteProvisioner_Provision_NilParams(t *testing.T) {
	p := &RouteProvisioner{}
	res := score.Resource{
		Type: "route",
	}

	_, err := p.Provision("web", res, "my-app")
	if err == nil {
		t.Fatal("expected error when params is nil")
	}
}

// --- Volume Provisioner Tests ---

func TestVolumeProvisioner_Type(t *testing.T) {
	p := &VolumeProvisioner{}
	if p.Type() != "volume" {
		t.Errorf("Type() = %q, want 'volume'", p.Type())
	}
}

func TestVolumeProvisioner_Provision_DefaultSize(t *testing.T) {
	p := &VolumeProvisioner{}
	res := score.Resource{Type: "volume"}
	result, err := p.Provision("data", res, "my-app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}
	m := result.Manifests[0]
	if m["kind"] != "PersistentVolumeClaim" {
		t.Errorf("manifest kind = %v, want PersistentVolumeClaim", m["kind"])
	}

	meta := m["metadata"].(map[string]interface{})
	if meta["name"] != "my-app-data" {
		t.Errorf("PVC name = %v, want 'my-app-data'", meta["name"])
	}

	spec := m["spec"].(map[string]interface{})
	resources := spec["resources"].(map[string]interface{})
	requests := resources["requests"].(map[string]interface{})
	if requests["storage"] != "1Gi" {
		t.Errorf("default storage = %v, want '1Gi'", requests["storage"])
	}

	// Check output
	if result.Outputs["source"] != "my-app-data" {
		t.Errorf("output source = %q, want 'my-app-data'", result.Outputs["source"])
	}
}

func TestVolumeProvisioner_Provision_CustomSize(t *testing.T) {
	p := &VolumeProvisioner{}
	res := score.Resource{
		Type: "volume",
		Params: map[string]interface{}{
			"size": "50Gi",
		},
	}
	result, err := p.Provision("storage", res, "myworkload")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	spec := result.Manifests[0]["spec"].(map[string]interface{})
	resources := spec["resources"].(map[string]interface{})
	requests := resources["requests"].(map[string]interface{})
	if requests["storage"] != "50Gi" {
		t.Errorf("storage = %v, want '50Gi'", requests["storage"])
	}
}

// --- DNS Provisioner Tests ---

func TestDNSProvisioner_Type(t *testing.T) {
	p := &DNSProvisioner{}
	if p.Type() != "dns" {
		t.Errorf("Type() = %q, want 'dns'", p.Type())
	}
}

func TestDNSProvisioner_Provision_DefaultHost(t *testing.T) {
	p := &DNSProvisioner{}
	res := score.Resource{Type: "dns"}
	result, err := p.Provision("records", res, "my-app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Outputs["host"] != "my-app.cluster.integratn.tech" {
		t.Errorf("host = %q, want 'my-app.cluster.integratn.tech'", result.Outputs["host"])
	}
	if result.Manifests != nil {
		t.Errorf("expected nil manifests for DNS, got %d", len(result.Manifests))
	}
}

func TestDNSProvisioner_Provision_CustomHost(t *testing.T) {
	p := &DNSProvisioner{}
	res := score.Resource{
		Type: "dns",
		Params: map[string]interface{}{
			"host": "custom.example.com",
		},
	}
	result, err := p.Provision("records", res, "my-app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Outputs["host"] != "custom.example.com" {
		t.Errorf("host = %q, want 'custom.example.com'", result.Outputs["host"])
	}
}

// --- MarshalManifests Tests ---

func TestMarshalManifests_Single(t *testing.T) {
	manifests := []map[string]interface{}{
		{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name": "test",
			},
		},
	}
	data, err := MarshalManifests(manifests)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := string(data)
	if !strings.Contains(s, "kind: ConfigMap") {
		t.Errorf("expected YAML to contain 'kind: ConfigMap', got:\n%s", s)
	}
	if strings.Contains(s, "---") {
		t.Errorf("single manifest should not have separator")
	}
}

func TestMarshalManifests_Multiple(t *testing.T) {
	manifests := []map[string]interface{}{
		{"kind": "ConfigMap"},
		{"kind": "Secret"},
	}
	data, err := MarshalManifests(manifests)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := string(data)
	if !strings.Contains(s, "---") {
		t.Error("multiple manifests should be separated by '---'")
	}
}

func TestMarshalManifests_Empty(t *testing.T) {
	data, err := MarshalManifests(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data) != 0 {
		t.Errorf("expected empty output for nil manifests, got %q", string(data))
	}
}
