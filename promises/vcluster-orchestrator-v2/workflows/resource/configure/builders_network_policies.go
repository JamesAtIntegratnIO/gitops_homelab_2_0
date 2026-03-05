package main

import (
	"fmt"

	ku "github.com/jamesatintegratnio/gitops_homelab_2_0/promises/_shared/kratixutil"
)

// ---------------------------------------------------------------------------
// Network Policy Builders – Kubernetes vs Cilium
// ---------------------------------------------------------------------------
//
// This file produces two kinds of network policy resources:
//
// Standard Kubernetes NetworkPolicy (networking.k8s.io/v1):
//   - buildDefaultDenyPolicy      – namespace-level default-deny-all baseline
//   - buildDNSEgressPolicy         – allow DNS egress to kube-system CoreDNS
//   - buildIntraNamespacePolicy    – allow full intra-namespace communication
//   - buildVClusterExternalPolicy  – allow external ingress/egress (ArgoCD, gateway, registries)
//   - buildNFSEgressPolicy         – optional NFS egress (opt-in)
//   - buildExtraEgressPolicy       – custom egress rules (e.g., PostgreSQL)
//
// Cilium CiliumNetworkPolicy (cilium.io/v2):
//   - buildKubeAPIPolicy           – allow egress to kube-apiserver entity
//   - buildCorednsHostDNSPolicy    – allow egress to Talos node-local DNS (169.254.116.108)
//   - buildVClusterLBSNATPolicy    – allow SNAT'd LB traffic via Cilium security identities
//
// Why both are used:
//   Cilium CiliumNetworkPolicy is required for workload-level policies that
//   reference Cilium-specific constructs (entities like "kube-apiserver",
//   "host", "remote-node", "world"; toCIDR for link-local addresses; and
//   security-identity-based fromEntities). Standard Kubernetes NetworkPolicy
//   is used for namespace-level baseline rules (default-deny, DNS, intra-NS,
//   IP-block based ingress/egress) that don't need Cilium extensions and
//   remain portable across CNI implementations.
// ---------------------------------------------------------------------------

// buildNetworkPolicies generates the complete set of host-cluster network policies
// for a vcluster namespace. This includes:
//   - Generic baseline policies (default-deny, DNS, kube-api, intra-namespace, external)
//   - Cilium-specific policies (kube-apiserver entity, Talos node-local DNS)
//   - Optional NFS egress (if enableNFS is true)
//   - Custom extra egress rules (e.g., PostgreSQL)
//
// All policies are emitted to the Kratix state repo and synced to the host cluster.
func buildNetworkPolicies(config *VClusterConfig) []ku.Resource {
	var policies []ku.Resource

	// --- Generic baseline policies (every vcluster gets these) ---
	policies = append(policies,
		buildDefaultDenyPolicy(config),
		buildDNSEgressPolicy(config),
		buildKubeAPIPolicy(config),
		buildCorednsHostDNSPolicy(config),
		buildIntraNamespacePolicy(config),
		buildVClusterExternalPolicy(config),
		buildVClusterLBSNATPolicy(config),
	)

	// --- Optional policies ---

	// NFS egress (opt-in)
	if config.EnableNFS {
		policies = append(policies, buildNFSEgressPolicy(config))
	}

	// Custom extra egress rules
	for _, rule := range config.ExtraEgress {
		policies = append(policies, buildExtraEgressPolicy(config, rule))
	}

	return policies
}

func netpolicyLabels(config *VClusterConfig, name string) map[string]string {
	return ku.MergeStringMap(map[string]string{
		"app.kubernetes.io/name":       name,
		"app.kubernetes.io/component":  "network-policy",
		"platform.integratn.tech/type": "vcluster-policy",
	}, ku.BaseLabels(config.WorkflowContext.PromiseName, config.Name))
}

// --- Generic baseline policies ---

// buildDefaultDenyPolicy creates the default-deny-all policy.
func buildDefaultDenyPolicy(config *VClusterConfig) ku.Resource {
	return ku.BuildDefaultDenyPolicy(
		"default-deny-all",
		config.TargetNamespace,
		netpolicyLabels(config, "default-deny-all"),
	)
}

// buildDNSEgressPolicy allows egress to kube-system CoreDNS on port 53.
func buildDNSEgressPolicy(config *VClusterConfig) ku.Resource {
	return ku.BuildDNSEgressPolicy(
		"allow-dns",
		config.TargetNamespace,
		netpolicyLabels(config, "allow-dns"),
	)
}

// buildKubeAPIPolicy allows egress to the kube-apiserver entity (CiliumNetworkPolicy).
func buildKubeAPIPolicy(config *VClusterConfig) ku.Resource {
	return ku.Resource{
		APIVersion: "cilium.io/v2",
		Kind:       "CiliumNetworkPolicy",
		Metadata: ku.ResourceMeta(
			"allow-kube-api",
			config.TargetNamespace,
			netpolicyLabels(config, "allow-kube-api"),
			nil,
		),
		Spec: map[string]interface{}{
			"endpointSelector": map[string]interface{}{},
			"egress": []map[string]interface{}{
				{
					"toEntities": []string{"kube-apiserver"},
				},
			},
		},
	}
}

// buildCorednsHostDNSPolicy allows egress to Talos node-local DNS (169.254.116.108).
// This link-local address is classified as 'world' by Cilium, not 'host'.
func buildCorednsHostDNSPolicy(config *VClusterConfig) ku.Resource {
	return ku.Resource{
		APIVersion: "cilium.io/v2",
		Kind:       "CiliumNetworkPolicy",
		Metadata: ku.ResourceMeta(
			"allow-coredns-to-host-dns",
			config.TargetNamespace,
			netpolicyLabels(config, "allow-coredns-to-host-dns"),
			nil,
		),
		Spec: map[string]interface{}{
			"endpointSelector": map[string]interface{}{},
			"egress": []map[string]interface{}{
				{
					"toCIDR": []string{ku.TalosNodeLocalDNSCIDR},
					"toPorts": []map[string]interface{}{
						{
							"ports": []map[string]interface{}{
								{"port": "53", "protocol": "UDP"},
								{"port": "53", "protocol": "TCP"},
							},
						},
					},
				},
			},
		},
	}
}

// buildIntraNamespacePolicy allows full intra-namespace communication.
func buildIntraNamespacePolicy(config *VClusterConfig) ku.Resource {
	return ku.Resource{
		APIVersion: "networking.k8s.io/v1",
		Kind:       "NetworkPolicy",
		Metadata: ku.ResourceMeta(
			"allow-intra-namespace",
			config.TargetNamespace,
			netpolicyLabels(config, "allow-intra-namespace"),
			nil,
		),
		Spec: map[string]interface{}{
			"podSelector": map[string]interface{}{},
			"policyTypes": []string{"Ingress", "Egress"},
			"ingress": []map[string]interface{}{
				{
					"from": []map[string]interface{}{
						{"podSelector": map[string]interface{}{}},
					},
				},
			},
			"egress": []map[string]interface{}{
				{
					"to": []map[string]interface{}{
						{"podSelector": map[string]interface{}{}},
					},
				},
			},
		},
	}
}

// buildVClusterExternalPolicy allows generic external ingress and egress
// that every vcluster needs: ArgoCD, nginx-gateway, monitoring ingress;
// 1Password Connect and public HTTPS egress.
func buildVClusterExternalPolicy(config *VClusterConfig) ku.Resource {
	return ku.Resource{
		APIVersion: "networking.k8s.io/v1",
		Kind:       "NetworkPolicy",
		Metadata: ku.ResourceMeta(
			"allow-vcluster-external",
			config.TargetNamespace,
			netpolicyLabels(config, "allow-vcluster-external"),
			nil,
		),
		Spec: map[string]interface{}{
			"podSelector": map[string]interface{}{},
			"policyTypes": []string{"Ingress", "Egress"},
			"ingress": []map[string]interface{}{
				// vCluster API LB: accept from ArgoCD namespace and RFC-1918
				// networks on port 8443.
				// NOTE: vCluster Service maps port 443 → targetPort 8443.
				// Cilium (kube-proxy replacement) DNATs to the targetPort before
				// policy evaluation, so the ingress port must be 8443.
				//
				// IMPORTANT: With Cilium KPR + externalTrafficPolicy: Cluster,
				// external traffic is SNAT'd to the cilium_host IP (a pod-CIDR
				// address). Cilium excludes cluster-internal IPs from ipBlock
				// matching per K8s spec. The companion CiliumNetworkPolicy
				// "allow-vcluster-lb-snat" handles this via fromEntities.
				// This ipBlock rule covers direct pod-to-pod or ETP:Local cases.
				{
					"from": []map[string]interface{}{
						{
							"namespaceSelector": map[string]interface{}{
								"matchLabels": map[string]string{
									"kubernetes.io/metadata.name": ku.DefaultArgoCDNamespace,
								},
							},
						},
						{"ipBlock": map[string]interface{}{"cidr": ku.RFC1918Class10}},
						{"ipBlock": map[string]interface{}{"cidr": ku.RFC1918Class192}},
					},
					"ports": []map[string]interface{}{
						{"protocol": "TCP", "port": 8443},
					},
				},
				// nginx-gateway routes traffic to vCluster workloads
				{
					"from": []map[string]interface{}{
						{
							"namespaceSelector": map[string]interface{}{
								"matchLabels": map[string]string{
									"kubernetes.io/metadata.name": ku.DefaultGatewayNamespace,
								},
							},
						},
					},
				},
				// Prometheus scrapes vCluster metrics
				{
					"from": []map[string]interface{}{
						{
							"namespaceSelector": map[string]interface{}{
								"matchLabels": map[string]string{
									"kubernetes.io/metadata.name": ku.MonitoringNamespace,
								},
							},
						},
					},
				},
				// nginx-gateway LoadBalancer: accept HTTP/HTTPS from any external client.
				// With externalTrafficPolicy: Local, Cilium preserves original source IP,
				// so we must allow 0.0.0.0/0 (not just private ranges).
				{
					"from": []map[string]interface{}{
						{"ipBlock": map[string]interface{}{"cidr": ku.AllIPv4}},
					},
					"ports": []map[string]interface{}{
						{"protocol": "TCP", "port": ku.HTTPPort},
						{"protocol": "TCP", "port": ku.HTTPSPort},
					},
				},
			},
			"egress": []map[string]interface{}{
				// 1Password Connect server (kubeconfig-sync job)
				{
					"to": []map[string]interface{}{
						{"ipBlock": map[string]interface{}{"cidr": ku.OnePasswordConnectCIDR}},
					},
					"ports": []map[string]interface{}{
						{"protocol": "TCP", "port": ku.HTTPSPort},
					},
				},
				// External HTTPS (container registries, APIs)
				// Also allows DNS (53/UDP+TCP) to public nameservers for
				// cert-manager DNS-01 challenge propagation checks against
				// authoritative nameservers (e.g., Cloudflare 162.159.x.x).
				{
					"to": []map[string]interface{}{
						{
							"ipBlock": map[string]interface{}{
								"cidr": ku.AllIPv4,
								"except": []string{
									ku.RFC1918Class10,
									ku.RFC1918Class172,
									ku.RFC1918Class192,
								},
							},
						},
					},
					"ports": []map[string]interface{}{
						{"protocol": "TCP", "port": ku.HTTPSPort},
						{"protocol": "TCP", "port": ku.HTTPPort},
						{"protocol": "UDP", "port": ku.DNSPort},
						{"protocol": "TCP", "port": ku.DNSPort},
					},
				},
			},
		},
	}
}

// --- Optional per-vcluster policies ---

// buildVClusterLBSNATPolicy creates a CiliumNetworkPolicy that allows external
// traffic reaching the vCluster API LoadBalancer.
//
// With Cilium kube-proxy replacement + externalTrafficPolicy: Cluster, external
// packets are DNAT'd + SNAT'd at the MetalLB announcing node. The SNAT source
// IP is the cilium_host0 address (a pod-CIDR IP, e.g. 10.244.x.x). Because
// Cilium excludes cluster-internal IPs from K8s NetworkPolicy ipBlock matching
// (per the K8s spec: "ipBlock selects cluster-external IPs"), standard
// NetworkPolicy rules cannot match this traffic.
//
// This CiliumNetworkPolicy uses fromEntities to match by Cilium security
// identity instead of IP ranges:
//   - host: traffic from the local node (cilium_host0 SNAT on same node)
//   - remote-node: traffic from other nodes (cilium_host0 SNAT cross-node)
//   - world: traffic with preserved external source IP (e.g., ETP:Local)
func buildVClusterLBSNATPolicy(config *VClusterConfig) ku.Resource {
	return ku.Resource{
		APIVersion: "cilium.io/v2",
		Kind:       "CiliumNetworkPolicy",
		Metadata: ku.ResourceMeta(
			"allow-vcluster-lb-snat",
			config.TargetNamespace,
			netpolicyLabels(config, "allow-vcluster-lb-snat"),
			nil,
		),
		Spec: map[string]interface{}{
			"endpointSelector": map[string]interface{}{
				"matchLabels": map[string]string{
					"app": "vcluster",
				},
			},
			"ingress": []map[string]interface{}{
				{
					"fromEntities": []string{"host", "remote-node", "world"},
					"toPorts": []map[string]interface{}{
						{
							"ports": []map[string]interface{}{
								{"port": "8443", "protocol": "TCP"},
							},
						},
					},
				},
			},
		},
	}
}

// buildNFSEgressPolicy creates a NetworkPolicy allowing NFS egress.
func buildNFSEgressPolicy(config *VClusterConfig) ku.Resource {
	return ku.Resource{
		APIVersion: "networking.k8s.io/v1",
		Kind:       "NetworkPolicy",
		Metadata: ku.ResourceMeta(
			"allow-nfs-egress",
			config.TargetNamespace,
			netpolicyLabels(config, "allow-nfs-egress"),
			nil,
		),
		Spec: map[string]interface{}{
			"podSelector": map[string]interface{}{},
			"policyTypes": []string{"Egress"},
			"egress": []map[string]interface{}{
				{
					"to": []map[string]interface{}{
						{"ipBlock": map[string]interface{}{"cidr": ku.RFC1918Class10}},
					},
					"ports": []map[string]interface{}{
						{"protocol": "TCP", "port": ku.NFSPort},
					},
				},
			},
		},
	}
}

// buildExtraEgressPolicy creates a NetworkPolicy for a custom egress rule.
func buildExtraEgressPolicy(config *VClusterConfig, rule ExtraEgressRule) ku.Resource {
	policyName := fmt.Sprintf("allow-%s-egress", rule.Name)

	return ku.Resource{
		APIVersion: "networking.k8s.io/v1",
		Kind:       "NetworkPolicy",
		Metadata: ku.ResourceMeta(
			policyName,
			config.TargetNamespace,
			netpolicyLabels(config, policyName),
			nil,
		),
		Spec: map[string]interface{}{
			"podSelector": map[string]interface{}{},
			"policyTypes": []string{"Egress"},
			"egress": []map[string]interface{}{
				{
					"to": []map[string]interface{}{
						{"ipBlock": map[string]interface{}{"cidr": rule.CIDR}},
					},
					"ports": []map[string]interface{}{
						{"protocol": rule.Protocol, "port": rule.Port},
					},
				},
			},
		},
	}
}
