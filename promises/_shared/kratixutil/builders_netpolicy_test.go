package kratixutil

import (
	"testing"
)

// ---------------------------------------------------------------------------
// BuildDefaultDenyPolicy
// ---------------------------------------------------------------------------

func TestBuildDefaultDenyPolicy(t *testing.T) {
	labels := map[string]string{"team": "platform", "env": "test"}
	p := BuildDefaultDenyPolicy("deny-all", "my-ns", labels)

	if p.APIVersion != "networking.k8s.io/v1" {
		t.Errorf("apiVersion = %q, want networking.k8s.io/v1", p.APIVersion)
	}
	if p.Kind != "NetworkPolicy" {
		t.Errorf("kind = %q, want NetworkPolicy", p.Kind)
	}
	if p.Metadata.Name != "deny-all" {
		t.Errorf("name = %q, want deny-all", p.Metadata.Name)
	}
	if p.Metadata.Namespace != "my-ns" {
		t.Errorf("namespace = %q, want my-ns", p.Metadata.Namespace)
	}
	if p.Metadata.Labels["team"] != "platform" {
		t.Errorf("missing label team=platform, got %v", p.Metadata.Labels)
	}

	spec, ok := p.Spec.(map[string]interface{})
	if !ok {
		t.Fatal("spec is not map[string]interface{}")
	}

	// podSelector should be empty (all pods)
	ps, ok := spec["podSelector"].(map[string]interface{})
	if !ok || len(ps) != 0 {
		t.Errorf("podSelector should be empty, got %v", ps)
	}

	types, ok := spec["policyTypes"].([]string)
	if !ok || len(types) != 2 {
		t.Fatalf("expected 2 policyTypes, got %v", types)
	}
	if types[0] != "Ingress" || types[1] != "Egress" {
		t.Errorf("policyTypes = %v, want [Ingress Egress]", types)
	}
}

// ---------------------------------------------------------------------------
// BuildDNSEgressPolicy
// ---------------------------------------------------------------------------

func TestBuildDNSEgressPolicy(t *testing.T) {
	labels := map[string]string{"app": "test"}
	p := BuildDNSEgressPolicy("allow-dns", "dns-ns", labels)

	if p.APIVersion != "networking.k8s.io/v1" {
		t.Errorf("apiVersion = %q, want networking.k8s.io/v1", p.APIVersion)
	}
	if p.Kind != "NetworkPolicy" {
		t.Errorf("kind = %q, want NetworkPolicy", p.Kind)
	}
	if p.Metadata.Name != "allow-dns" {
		t.Errorf("name = %q, want allow-dns", p.Metadata.Name)
	}
	if p.Metadata.Namespace != "dns-ns" {
		t.Errorf("namespace = %q, want dns-ns", p.Metadata.Namespace)
	}

	spec, ok := p.Spec.(map[string]interface{})
	if !ok {
		t.Fatal("spec is not map[string]interface{}")
	}

	// podSelector should be empty
	ps, ok := spec["podSelector"].(map[string]interface{})
	if !ok || len(ps) != 0 {
		t.Errorf("podSelector should be empty, got %v", ps)
	}

	types, ok := spec["policyTypes"].([]string)
	if !ok || len(types) != 1 || types[0] != "Egress" {
		t.Errorf("policyTypes = %v, want [Egress]", types)
	}

	egress, ok := spec["egress"].([]map[string]interface{})
	if !ok || len(egress) != 1 {
		t.Fatal("expected 1 egress rule")
	}

	// Verify "to" targets kube-system kube-dns
	to, ok := egress[0]["to"].([]map[string]interface{})
	if !ok || len(to) != 1 {
		t.Fatal("expected 1 'to' entry targeting kube-system")
	}
	nsSelector, ok := to[0]["namespaceSelector"].(map[string]interface{})
	if !ok {
		t.Fatal("expected namespaceSelector in 'to'")
	}
	nsLabels, ok := nsSelector["matchLabels"].(map[string]string)
	if !ok || nsLabels["kubernetes.io/metadata.name"] != KubeSystemNamespace {
		t.Errorf("expected kube-system namespace selector, got %v", nsLabels)
	}
	podSel, ok := to[0]["podSelector"].(map[string]interface{})
	if !ok {
		t.Fatal("expected podSelector in 'to'")
	}
	podLabels, ok := podSel["matchLabels"].(map[string]string)
	if !ok || podLabels["k8s-app"] != "kube-dns" {
		t.Errorf("expected kube-dns pod selector, got %v", podLabels)
	}

	// Verify ports (UDP + TCP on 53)
	ports, ok := egress[0]["ports"].([]map[string]interface{})
	if !ok || len(ports) != 2 {
		t.Fatalf("expected 2 DNS ports (UDP+TCP 53), got %d", len(ports))
	}
	if ports[0]["protocol"] != "UDP" || ports[0]["port"] != DNSPort {
		t.Errorf("first port = %v, want UDP/%d", ports[0], DNSPort)
	}
	if ports[1]["protocol"] != "TCP" || ports[1]["port"] != DNSPort {
		t.Errorf("second port = %v, want TCP/%d", ports[1], DNSPort)
	}
}

// ---------------------------------------------------------------------------
// BuildMonitoringIngressPolicy
// ---------------------------------------------------------------------------

func TestBuildMonitoringIngressPolicy(t *testing.T) {
	labels := map[string]string{"managed-by": "kratix"}
	p := BuildMonitoringIngressPolicy("allow-monitoring", "app-ns", labels, 8080)

	if p.APIVersion != "networking.k8s.io/v1" {
		t.Errorf("apiVersion = %q, want networking.k8s.io/v1", p.APIVersion)
	}
	if p.Kind != "NetworkPolicy" {
		t.Errorf("kind = %q, want NetworkPolicy", p.Kind)
	}
	if p.Metadata.Name != "allow-monitoring" {
		t.Errorf("name = %q, want allow-monitoring", p.Metadata.Name)
	}
	if p.Metadata.Namespace != "app-ns" {
		t.Errorf("namespace = %q, want app-ns", p.Metadata.Namespace)
	}

	spec, ok := p.Spec.(map[string]interface{})
	if !ok {
		t.Fatal("spec is not map[string]interface{}")
	}

	// podSelector should be empty
	ps, ok := spec["podSelector"].(map[string]interface{})
	if !ok || len(ps) != 0 {
		t.Errorf("podSelector should be empty, got %v", ps)
	}

	types, ok := spec["policyTypes"].([]string)
	if !ok || len(types) != 1 || types[0] != "Ingress" {
		t.Errorf("policyTypes = %v, want [Ingress]", types)
	}

	ingress, ok := spec["ingress"].([]map[string]interface{})
	if !ok || len(ingress) != 1 {
		t.Fatal("expected 1 ingress rule")
	}

	// Verify "from" targets monitoring namespace
	from, ok := ingress[0]["from"].([]map[string]interface{})
	if !ok || len(from) != 1 {
		t.Fatal("expected 1 'from' entry")
	}
	nsSelector, ok := from[0]["namespaceSelector"].(map[string]interface{})
	if !ok {
		t.Fatal("expected namespaceSelector in 'from'")
	}
	nsLabels, ok := nsSelector["matchLabels"].(map[string]string)
	if !ok || nsLabels["kubernetes.io/metadata.name"] != MonitoringNamespace {
		t.Errorf("expected monitoring namespace selector, got %v", nsLabels)
	}

	// Verify port
	ports, ok := ingress[0]["ports"].([]map[string]interface{})
	if !ok || len(ports) != 1 {
		t.Fatal("expected 1 port entry")
	}
	if ports[0]["protocol"] != "TCP" || ports[0]["port"] != 8080 {
		t.Errorf("port = %v, want TCP/8080", ports[0])
	}
}

func TestBuildMonitoringIngressPolicy_DifferentPort(t *testing.T) {
	p := BuildMonitoringIngressPolicy("mon", "ns", nil, 9090)

	spec := p.Spec.(map[string]interface{})
	ingress := spec["ingress"].([]map[string]interface{})
	ports := ingress[0]["ports"].([]map[string]interface{})
	if ports[0]["port"] != 9090 {
		t.Errorf("port = %v, want 9090", ports[0]["port"])
	}
}

// ---------------------------------------------------------------------------
// BuildNamespaceIngressPolicy
// ---------------------------------------------------------------------------

func TestBuildNamespaceIngressPolicy(t *testing.T) {
	labels := map[string]string{"role": "gateway"}
	p := BuildNamespaceIngressPolicy("allow-gateway", "app-ns", labels, "nginx-gateway")

	if p.APIVersion != "networking.k8s.io/v1" {
		t.Errorf("apiVersion = %q, want networking.k8s.io/v1", p.APIVersion)
	}
	if p.Kind != "NetworkPolicy" {
		t.Errorf("kind = %q, want NetworkPolicy", p.Kind)
	}
	if p.Metadata.Name != "allow-gateway" {
		t.Errorf("name = %q, want allow-gateway", p.Metadata.Name)
	}
	if p.Metadata.Namespace != "app-ns" {
		t.Errorf("namespace = %q, want app-ns", p.Metadata.Namespace)
	}
	if p.Metadata.Labels["role"] != "gateway" {
		t.Errorf("missing label role=gateway, got %v", p.Metadata.Labels)
	}

	spec, ok := p.Spec.(map[string]interface{})
	if !ok {
		t.Fatal("spec is not map[string]interface{}")
	}

	// podSelector should be empty
	ps, ok := spec["podSelector"].(map[string]interface{})
	if !ok || len(ps) != 0 {
		t.Errorf("podSelector should be empty, got %v", ps)
	}

	types, ok := spec["policyTypes"].([]string)
	if !ok || len(types) != 1 || types[0] != "Ingress" {
		t.Errorf("policyTypes = %v, want [Ingress]", types)
	}

	ingress, ok := spec["ingress"].([]map[string]interface{})
	if !ok || len(ingress) != 1 {
		t.Fatal("expected 1 ingress rule")
	}

	from, ok := ingress[0]["from"].([]map[string]interface{})
	if !ok || len(from) != 1 {
		t.Fatal("expected 1 'from' entry")
	}
	nsSelector, ok := from[0]["namespaceSelector"].(map[string]interface{})
	if !ok {
		t.Fatal("expected namespaceSelector in 'from'")
	}
	nsLabels, ok := nsSelector["matchLabels"].(map[string]string)
	if !ok || nsLabels["kubernetes.io/metadata.name"] != "nginx-gateway" {
		t.Errorf("expected nginx-gateway namespace selector, got %v", nsLabels)
	}
}

func TestBuildNamespaceIngressPolicy_DifferentNamespace(t *testing.T) {
	p := BuildNamespaceIngressPolicy("allow-argocd", "my-ns", nil, "argocd")

	spec := p.Spec.(map[string]interface{})
	ingress := spec["ingress"].([]map[string]interface{})
	from := ingress[0]["from"].([]map[string]interface{})
	nsSelector := from[0]["namespaceSelector"].(map[string]interface{})
	nsLabels := nsSelector["matchLabels"].(map[string]string)
	if nsLabels["kubernetes.io/metadata.name"] != "argocd" {
		t.Errorf("expected argocd namespace selector, got %v", nsLabels)
	}
}

// ---------------------------------------------------------------------------
// Nil labels edge case
// ---------------------------------------------------------------------------

func TestBuildNetpolicyHelpers_NilLabels(t *testing.T) {
	// All helpers should handle nil labels without panicking
	p1 := BuildDefaultDenyPolicy("deny", "ns", nil)
	if p1.Metadata.Labels != nil {
		t.Errorf("expected nil labels, got %v", p1.Metadata.Labels)
	}

	p2 := BuildDNSEgressPolicy("dns", "ns", nil)
	if p2.Metadata.Labels != nil {
		t.Errorf("expected nil labels, got %v", p2.Metadata.Labels)
	}

	p3 := BuildMonitoringIngressPolicy("mon", "ns", nil, 80)
	if p3.Metadata.Labels != nil {
		t.Errorf("expected nil labels, got %v", p3.Metadata.Labels)
	}

	p4 := BuildNamespaceIngressPolicy("ns-ingress", "ns", nil, "other-ns")
	if p4.Metadata.Labels != nil {
		t.Errorf("expected nil labels, got %v", p4.Metadata.Labels)
	}
}
