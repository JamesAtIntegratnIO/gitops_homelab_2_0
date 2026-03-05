package main

import (
	"testing"

	ku "github.com/jamesatintegratnio/gitops_homelab_2_0/promises/_shared/kratixutil"
)

func TestNetpolicyLabels(t *testing.T) {
	config := minimalConfig()
	labels := netpolicyLabels(config, "my-policy")

	expected := map[string]string{
		"app.kubernetes.io/name":       "my-policy",
		"app.kubernetes.io/component":  "network-policy",
		"platform.integratn.tech/type": "vcluster-policy",
	}
	for k, want := range expected {
		got, ok := labels[k]
		if !ok {
			t.Errorf("missing label %q", k)
		} else if got != want {
			t.Errorf("label %q = %q, want %q", k, got, want)
		}
	}

	// Should also include BaseLabels keys
	base := ku.BaseLabels(config.WorkflowContext.PromiseName, config.Name)
	for k, want := range base {
		got, ok := labels[k]
		if !ok {
			t.Errorf("missing base label %q", k)
		} else if got != want {
			t.Errorf("base label %q = %q, want %q", k, got, want)
		}
	}
}

func TestBuildDefaultDenyPolicy(t *testing.T) {
	config := minimalConfig()
	p := buildDefaultDenyPolicy(config)

	if p.APIVersion != "networking.k8s.io/v1" {
		t.Errorf("apiVersion = %q, want networking.k8s.io/v1", p.APIVersion)
	}
	if p.Kind != "NetworkPolicy" {
		t.Errorf("kind = %q, want NetworkPolicy", p.Kind)
	}
	if p.Metadata.Name != "default-deny-all" {
		t.Errorf("name = %q, want default-deny-all", p.Metadata.Name)
	}
	if p.Metadata.Namespace != config.TargetNamespace {
		t.Errorf("namespace = %q, want %q", p.Metadata.Namespace, config.TargetNamespace)
	}

	spec, ok := p.Spec.(map[string]interface{})
	if !ok {
		t.Fatal("spec is not map[string]interface{}")
	}
	types, ok := spec["policyTypes"].([]string)
	if !ok {
		t.Fatal("policyTypes is not []string")
	}
	if len(types) != 2 {
		t.Fatalf("expected 2 policyTypes, got %d", len(types))
	}
	if types[0] != "Ingress" || types[1] != "Egress" {
		t.Errorf("policyTypes = %v, want [Ingress Egress]", types)
	}
}

func TestBuildDNSEgressPolicy(t *testing.T) {
	config := minimalConfig()
	p := buildDNSEgressPolicy(config)

	if p.APIVersion != "networking.k8s.io/v1" {
		t.Errorf("apiVersion = %q, want networking.k8s.io/v1", p.APIVersion)
	}
	if p.Kind != "NetworkPolicy" {
		t.Errorf("kind = %q, want NetworkPolicy", p.Kind)
	}
	if p.Metadata.Name != "allow-dns" {
		t.Errorf("name = %q, want allow-dns", p.Metadata.Name)
	}
	if p.Metadata.Namespace != config.TargetNamespace {
		t.Errorf("namespace = %q, want %q", p.Metadata.Namespace, config.TargetNamespace)
	}

	spec, ok := p.Spec.(map[string]interface{})
	if !ok {
		t.Fatal("spec is not map[string]interface{}")
	}

	types, ok := spec["policyTypes"].([]string)
	if !ok || len(types) != 1 || types[0] != "Egress" {
		t.Errorf("policyTypes = %v, want [Egress]", types)
	}

	egress, ok := spec["egress"].([]map[string]interface{})
	if !ok || len(egress) != 1 {
		t.Fatal("expected 1 egress rule")
	}

	ports, ok := egress[0]["ports"].([]map[string]interface{})
	if !ok || len(ports) != 2 {
		t.Fatalf("expected 2 ports (UDP+TCP 53), got %d", len(ports))
	}
	if ports[0]["protocol"] != "UDP" || ports[0]["port"] != 53 {
		t.Errorf("first port = %v, want UDP/53", ports[0])
	}
	if ports[1]["protocol"] != "TCP" || ports[1]["port"] != 53 {
		t.Errorf("second port = %v, want TCP/53", ports[1])
	}
}

func TestBuildKubeAPIPolicy(t *testing.T) {
	config := minimalConfig()
	p := buildKubeAPIPolicy(config)

	if p.APIVersion != "cilium.io/v2" {
		t.Errorf("apiVersion = %q, want cilium.io/v2", p.APIVersion)
	}
	if p.Kind != "CiliumNetworkPolicy" {
		t.Errorf("kind = %q, want CiliumNetworkPolicy", p.Kind)
	}
	if p.Metadata.Name != "allow-kube-api" {
		t.Errorf("name = %q, want allow-kube-api", p.Metadata.Name)
	}
	if p.Metadata.Namespace != config.TargetNamespace {
		t.Errorf("namespace = %q, want %q", p.Metadata.Namespace, config.TargetNamespace)
	}

	spec, ok := p.Spec.(map[string]interface{})
	if !ok {
		t.Fatal("spec is not map[string]interface{}")
	}

	egress, ok := spec["egress"].([]map[string]interface{})
	if !ok || len(egress) != 1 {
		t.Fatal("expected 1 egress rule")
	}

	entities, ok := egress[0]["toEntities"].([]string)
	if !ok || len(entities) != 1 || entities[0] != "kube-apiserver" {
		t.Errorf("toEntities = %v, want [kube-apiserver]", entities)
	}
}

func TestBuildCorednsHostDNSPolicy(t *testing.T) {
	config := minimalConfig()
	p := buildCorednsHostDNSPolicy(config)

	if p.APIVersion != "cilium.io/v2" {
		t.Errorf("apiVersion = %q, want cilium.io/v2", p.APIVersion)
	}
	if p.Kind != "CiliumNetworkPolicy" {
		t.Errorf("kind = %q, want CiliumNetworkPolicy", p.Kind)
	}
	if p.Metadata.Name != "allow-coredns-to-host-dns" {
		t.Errorf("name = %q, want allow-coredns-to-host-dns", p.Metadata.Name)
	}

	spec, ok := p.Spec.(map[string]interface{})
	if !ok {
		t.Fatal("spec is not map[string]interface{}")
	}

	egress, ok := spec["egress"].([]map[string]interface{})
	if !ok || len(egress) != 1 {
		t.Fatal("expected 1 egress rule")
	}

	cidrs, ok := egress[0]["toCIDR"].([]string)
	if !ok || len(cidrs) != 1 || cidrs[0] != ku.TalosNodeLocalDNSCIDR {
		t.Errorf("toCIDR = %v, want [%s]", cidrs, ku.TalosNodeLocalDNSCIDR)
	}
}

func TestBuildIntraNamespacePolicy(t *testing.T) {
	config := minimalConfig()
	p := buildIntraNamespacePolicy(config)

	if p.APIVersion != "networking.k8s.io/v1" {
		t.Errorf("apiVersion = %q, want networking.k8s.io/v1", p.APIVersion)
	}
	if p.Kind != "NetworkPolicy" {
		t.Errorf("kind = %q, want NetworkPolicy", p.Kind)
	}
	if p.Metadata.Name != "allow-intra-namespace" {
		t.Errorf("name = %q, want allow-intra-namespace", p.Metadata.Name)
	}

	spec, ok := p.Spec.(map[string]interface{})
	if !ok {
		t.Fatal("spec is not map[string]interface{}")
	}

	types, ok := spec["policyTypes"].([]string)
	if !ok || len(types) != 2 {
		t.Fatal("expected policyTypes [Ingress, Egress]")
	}

	ingress, ok := spec["ingress"].([]map[string]interface{})
	if !ok || len(ingress) != 1 {
		t.Fatal("expected 1 ingress rule")
	}

	egress, ok := spec["egress"].([]map[string]interface{})
	if !ok || len(egress) != 1 {
		t.Fatal("expected 1 egress rule")
	}
}

func TestBuildVClusterExternalPolicy(t *testing.T) {
	config := minimalConfig()
	p := buildVClusterExternalPolicy(config)

	if p.APIVersion != "networking.k8s.io/v1" {
		t.Errorf("apiVersion = %q, want networking.k8s.io/v1", p.APIVersion)
	}
	if p.Kind != "NetworkPolicy" {
		t.Errorf("kind = %q, want NetworkPolicy", p.Kind)
	}
	if p.Metadata.Name != "allow-vcluster-external" {
		t.Errorf("name = %q, want allow-vcluster-external", p.Metadata.Name)
	}

	spec, ok := p.Spec.(map[string]interface{})
	if !ok {
		t.Fatal("spec is not map[string]interface{}")
	}

	types, ok := spec["policyTypes"].([]string)
	if !ok || len(types) != 2 {
		t.Fatal("expected policyTypes [Ingress, Egress]")
	}

	ingress, ok := spec["ingress"].([]map[string]interface{})
	if !ok {
		t.Fatal("expected ingress rules")
	}
	// 4 ingress blocks: argocd+RFC1918, nginx-gateway, monitoring, LB HTTP/HTTPS
	if len(ingress) != 4 {
		t.Errorf("expected 4 ingress rules, got %d", len(ingress))
	}

	egress, ok := spec["egress"].([]map[string]interface{})
	if !ok {
		t.Fatal("expected egress rules")
	}
	// 2 egress blocks: 1Password Connect, external HTTPS
	if len(egress) != 2 {
		t.Errorf("expected 2 egress rules, got %d", len(egress))
	}
}

func TestBuildVClusterLBSNATPolicy(t *testing.T) {
	config := minimalConfig()
	p := buildVClusterLBSNATPolicy(config)

	if p.APIVersion != "cilium.io/v2" {
		t.Errorf("apiVersion = %q, want cilium.io/v2", p.APIVersion)
	}
	if p.Kind != "CiliumNetworkPolicy" {
		t.Errorf("kind = %q, want CiliumNetworkPolicy", p.Kind)
	}
	if p.Metadata.Name != "allow-vcluster-lb-snat" {
		t.Errorf("name = %q, want allow-vcluster-lb-snat", p.Metadata.Name)
	}

	spec, ok := p.Spec.(map[string]interface{})
	if !ok {
		t.Fatal("spec is not map[string]interface{}")
	}

	// endpointSelector should target app=vcluster
	epSel, ok := spec["endpointSelector"].(map[string]interface{})
	if !ok {
		t.Fatal("expected endpointSelector")
	}
	ml, ok := epSel["matchLabels"].(map[string]string)
	if !ok || ml["app"] != "vcluster" {
		t.Errorf("endpointSelector matchLabels = %v, want app=vcluster", ml)
	}

	ingress, ok := spec["ingress"].([]map[string]interface{})
	if !ok || len(ingress) != 1 {
		t.Fatal("expected 1 ingress rule")
	}

	entities, ok := ingress[0]["fromEntities"].([]string)
	if !ok {
		t.Fatal("expected fromEntities")
	}
	wantEntities := map[string]bool{"host": true, "remote-node": true, "world": true}
	if len(entities) != 3 {
		t.Fatalf("expected 3 entities, got %d", len(entities))
	}
	for _, e := range entities {
		if !wantEntities[e] {
			t.Errorf("unexpected entity %q", e)
		}
	}
}

func TestBuildNFSEgressPolicy(t *testing.T) {
	config := minimalConfig()
	p := buildNFSEgressPolicy(config)

	if p.APIVersion != "networking.k8s.io/v1" {
		t.Errorf("apiVersion = %q, want networking.k8s.io/v1", p.APIVersion)
	}
	if p.Kind != "NetworkPolicy" {
		t.Errorf("kind = %q, want NetworkPolicy", p.Kind)
	}
	if p.Metadata.Name != "allow-nfs-egress" {
		t.Errorf("name = %q, want allow-nfs-egress", p.Metadata.Name)
	}
	if p.Metadata.Namespace != config.TargetNamespace {
		t.Errorf("namespace = %q, want %q", p.Metadata.Namespace, config.TargetNamespace)
	}

	spec, ok := p.Spec.(map[string]interface{})
	if !ok {
		t.Fatal("spec is not map[string]interface{}")
	}

	types, ok := spec["policyTypes"].([]string)
	if !ok || len(types) != 1 || types[0] != "Egress" {
		t.Errorf("policyTypes = %v, want [Egress]", types)
	}

	egress, ok := spec["egress"].([]map[string]interface{})
	if !ok || len(egress) != 1 {
		t.Fatal("expected 1 egress rule")
	}

	ports, ok := egress[0]["ports"].([]map[string]interface{})
	if !ok || len(ports) != 1 {
		t.Fatal("expected 1 port entry")
	}
	if ports[0]["protocol"] != "TCP" || ports[0]["port"] != 2049 {
		t.Errorf("port = %v, want TCP/2049", ports[0])
	}
}

func TestBuildExtraEgressPolicy_DetailedSpec(t *testing.T) {
	tests := []struct {
		name     string
		rule     ExtraEgressRule
		wantName string
		wantCIDR string
		wantPort int
		wantProto string
	}{
		{
			name:      "postgres",
			rule:      ExtraEgressRule{Name: "postgres", CIDR: "10.0.1.50/32", Port: 5432, Protocol: "TCP"},
			wantName:  "allow-postgres-egress",
			wantCIDR:  "10.0.1.50/32",
			wantPort:  5432,
			wantProto: "TCP",
		},
		{
			name:      "redis UDP",
			rule:      ExtraEgressRule{Name: "redis", CIDR: "10.0.1.60/32", Port: 6379, Protocol: "UDP"},
			wantName:  "allow-redis-egress",
			wantCIDR:  "10.0.1.60/32",
			wantPort:  6379,
			wantProto: "UDP",
		},
	}

	config := minimalConfig()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := buildExtraEgressPolicy(config, tt.rule)

			if p.Metadata.Name != tt.wantName {
				t.Errorf("name = %q, want %q", p.Metadata.Name, tt.wantName)
			}
			if p.APIVersion != "networking.k8s.io/v1" {
				t.Errorf("apiVersion = %q, want networking.k8s.io/v1", p.APIVersion)
			}
			if p.Kind != "NetworkPolicy" {
				t.Errorf("kind = %q, want NetworkPolicy", p.Kind)
			}
			if p.Metadata.Namespace != config.TargetNamespace {
				t.Errorf("namespace = %q, want %q", p.Metadata.Namespace, config.TargetNamespace)
			}

			spec := p.Spec.(map[string]interface{})
			egress := spec["egress"].([]map[string]interface{})
			if len(egress) != 1 {
				t.Fatalf("expected 1 egress rule, got %d", len(egress))
			}

			to := egress[0]["to"].([]map[string]interface{})
			ipBlock := to[0]["ipBlock"].(map[string]interface{})
			if ipBlock["cidr"] != tt.wantCIDR {
				t.Errorf("cidr = %v, want %q", ipBlock["cidr"], tt.wantCIDR)
			}

			ports := egress[0]["ports"].([]map[string]interface{})
			if ports[0]["port"] != tt.wantPort {
				t.Errorf("port = %v, want %d", ports[0]["port"], tt.wantPort)
			}
			if ports[0]["protocol"] != tt.wantProto {
				t.Errorf("protocol = %v, want %q", ports[0]["protocol"], tt.wantProto)
			}
		})
	}
}

func TestBuildNetworkPolicies_AllNFSAndExtraEgress(t *testing.T) {
	config := minimalConfig()
	config.EnableNFS = true
	config.ExtraEgress = []ExtraEgressRule{
		{Name: "pg", CIDR: "10.0.1.50/32", Port: 5432, Protocol: "TCP"},
	}

	policies := buildNetworkPolicies(config)

	// 7 baseline + 1 NFS + 1 extra = 9
	if len(policies) != 9 {
		t.Fatalf("expected 9 policies, got %d", len(policies))
	}

	names := make(map[string]bool)
	for _, p := range policies {
		names[p.Metadata.Name] = true
	}

	want := []string{
		"default-deny-all",
		"allow-dns",
		"allow-kube-api",
		"allow-coredns-to-host-dns",
		"allow-intra-namespace",
		"allow-vcluster-external",
		"allow-vcluster-lb-snat",
		"allow-nfs-egress",
		"allow-pg-egress",
	}
	for _, w := range want {
		if !names[w] {
			t.Errorf("missing policy %q", w)
		}
	}
}

func TestBuildNetworkPolicies_NoNFSNoExtra(t *testing.T) {
	config := minimalConfig()
	config.EnableNFS = false
	config.ExtraEgress = nil

	policies := buildNetworkPolicies(config)
	if len(policies) != 7 {
		t.Errorf("expected 7 baseline policies, got %d", len(policies))
	}
}

func TestPolicyNamespaceMatchesConfig(t *testing.T) {
	config := minimalConfig()
	config.TargetNamespace = "vcluster-custom-ns"

	builders := []struct {
		name string
		fn   func(*VClusterConfig) ku.Resource
	}{
		{"defaultDeny", buildDefaultDenyPolicy},
		{"dns", buildDNSEgressPolicy},
		{"kubeAPI", buildKubeAPIPolicy},
		{"corednsHostDNS", buildCorednsHostDNSPolicy},
		{"intraNamespace", buildIntraNamespacePolicy},
		{"vclusterExternal", buildVClusterExternalPolicy},
		{"vclusterLBSNAT", buildVClusterLBSNATPolicy},
		{"nfsEgress", buildNFSEgressPolicy},
	}

	for _, b := range builders {
		t.Run(b.name, func(t *testing.T) {
			p := b.fn(config)
			if p.Metadata.Namespace != "vcluster-custom-ns" {
				t.Errorf("namespace = %q, want vcluster-custom-ns", p.Metadata.Namespace)
			}
		})
	}
}
