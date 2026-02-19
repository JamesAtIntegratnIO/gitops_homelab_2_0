package main

import "fmt"

// buildNetworkPolicies generates the complete set of host-cluster network policies
// for a vcluster namespace. This includes:
//   - Generic baseline policies (default-deny, DNS, kube-api, intra-namespace, external)
//   - Cilium-specific policies (kube-apiserver entity, Talos node-local DNS)
//   - Optional NFS egress (if enableNFS is true)
//   - Custom extra egress rules (e.g., PostgreSQL)
//
// All policies are emitted to the Kratix state repo and synced to the host cluster.
func buildNetworkPolicies(config *VClusterConfig) []Resource {
	var policies []Resource

	// --- Generic baseline policies (every vcluster gets these) ---
	policies = append(policies,
		buildDefaultDenyPolicy(config),
		buildDNSEgressPolicy(config),
		buildKubeAPIPolicy(config),
		buildCorednsHostDNSPolicy(config),
		buildIntraNamespacePolicy(config),
		buildVClusterExternalPolicy(config),
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
	return mergeStringMap(map[string]string{
		"app.kubernetes.io/name":       name,
		"app.kubernetes.io/component":  "network-policy",
		"platform.integratn.tech/type": "vcluster-policy",
	}, baseLabels(config, config.Name))
}

// --- Generic baseline policies ---

// buildDefaultDenyPolicy creates the default-deny-all policy.
func buildDefaultDenyPolicy(config *VClusterConfig) Resource {
	return Resource{
		APIVersion: "networking.k8s.io/v1",
		Kind:       "NetworkPolicy",
		Metadata: resourceMeta(
			"default-deny-all",
			config.TargetNamespace,
			netpolicyLabels(config, "default-deny-all"),
			nil,
		),
		Spec: map[string]interface{}{
			"podSelector": map[string]interface{}{},
			"policyTypes": []string{"Ingress", "Egress"},
		},
	}
}

// buildDNSEgressPolicy allows egress to kube-system CoreDNS on port 53.
func buildDNSEgressPolicy(config *VClusterConfig) Resource {
	return Resource{
		APIVersion: "networking.k8s.io/v1",
		Kind:       "NetworkPolicy",
		Metadata: resourceMeta(
			"allow-dns",
			config.TargetNamespace,
			netpolicyLabels(config, "allow-dns"),
			nil,
		),
		Spec: map[string]interface{}{
			"podSelector": map[string]interface{}{},
			"policyTypes": []string{"Egress"},
			"egress": []map[string]interface{}{
				{
					"to": []map[string]interface{}{
						{
							"namespaceSelector": map[string]interface{}{
								"matchLabels": map[string]string{
									"kubernetes.io/metadata.name": "kube-system",
								},
							},
							"podSelector": map[string]interface{}{
								"matchLabels": map[string]string{
									"k8s-app": "kube-dns",
								},
							},
						},
					},
					"ports": []map[string]interface{}{
						{"protocol": "UDP", "port": 53},
						{"protocol": "TCP", "port": 53},
					},
				},
			},
		},
	}
}

// buildKubeAPIPolicy allows egress to the kube-apiserver entity (CiliumNetworkPolicy).
func buildKubeAPIPolicy(config *VClusterConfig) Resource {
	return Resource{
		APIVersion: "cilium.io/v2",
		Kind:       "CiliumNetworkPolicy",
		Metadata: resourceMeta(
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
func buildCorednsHostDNSPolicy(config *VClusterConfig) Resource {
	return Resource{
		APIVersion: "cilium.io/v2",
		Kind:       "CiliumNetworkPolicy",
		Metadata: resourceMeta(
			"allow-coredns-to-host-dns",
			config.TargetNamespace,
			netpolicyLabels(config, "allow-coredns-to-host-dns"),
			nil,
		),
		Spec: map[string]interface{}{
			"endpointSelector": map[string]interface{}{},
			"egress": []map[string]interface{}{
				{
					"toCIDR": []string{"169.254.116.108/32"},
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
func buildIntraNamespacePolicy(config *VClusterConfig) Resource {
	return Resource{
		APIVersion: "networking.k8s.io/v1",
		Kind:       "NetworkPolicy",
		Metadata: resourceMeta(
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
func buildVClusterExternalPolicy(config *VClusterConfig) Resource {
	return Resource{
		APIVersion: "networking.k8s.io/v1",
		Kind:       "NetworkPolicy",
		Metadata: resourceMeta(
			"allow-vcluster-external",
			config.TargetNamespace,
			netpolicyLabels(config, "allow-vcluster-external"),
			nil,
		),
		Spec: map[string]interface{}{
			"podSelector": map[string]interface{}{},
			"policyTypes": []string{"Ingress", "Egress"},
			"ingress": []map[string]interface{}{
				// vCluster API LB: ArgoCD app-controller + LAN clients
				// NOTE: vCluster Service maps port 443 → targetPort 8443.
				// Cilium (kube-proxy replacement) DNATs to the targetPort before
				// policy evaluation, so the ingress port must be 8443.
				{
					"from": []map[string]interface{}{
						{
							"namespaceSelector": map[string]interface{}{
								"matchLabels": map[string]string{
									"kubernetes.io/metadata.name": "argocd",
								},
							},
						},
						{"ipBlock": map[string]interface{}{"cidr": "10.0.0.0/8"}},
						{"ipBlock": map[string]interface{}{"cidr": "192.168.0.0/16"}},
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
									"kubernetes.io/metadata.name": "nginx-gateway",
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
									"kubernetes.io/metadata.name": "monitoring",
								},
							},
						},
					},
				},
			},
			"egress": []map[string]interface{}{
				// 1Password Connect server (kubeconfig-sync job)
				{
					"to": []map[string]interface{}{
						{"ipBlock": map[string]interface{}{"cidr": "10.0.1.139/32"}},
					},
					"ports": []map[string]interface{}{
						{"protocol": "TCP", "port": 443},
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
								"cidr": "0.0.0.0/0",
								"except": []string{
									"10.0.0.0/8",
									"172.16.0.0/12",
									"192.168.0.0/16",
								},
							},
						},
					},
					"ports": []map[string]interface{}{
						{"protocol": "TCP", "port": 443},
						{"protocol": "TCP", "port": 80},
						{"protocol": "UDP", "port": 53},
						{"protocol": "TCP", "port": 53},
					},
				},
			},
		},
	}
}

// --- Optional per-vcluster policies ---

// buildNFSEgressPolicy creates a NetworkPolicy allowing NFS egress.
func buildNFSEgressPolicy(config *VClusterConfig) Resource {
	return Resource{
		APIVersion: "networking.k8s.io/v1",
		Kind:       "NetworkPolicy",
		Metadata: resourceMeta(
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
						{"ipBlock": map[string]interface{}{"cidr": "10.0.0.0/8"}},
					},
					"ports": []map[string]interface{}{
						{"protocol": "TCP", "port": 2049},
					},
				},
			},
		},
	}
}

// buildExtraEgressPolicy creates a NetworkPolicy for a custom egress rule.
func buildExtraEgressPolicy(config *VClusterConfig, rule ExtraEgressRule) Resource {
	policyName := fmt.Sprintf("allow-%s-egress", rule.Name)

	return Resource{
		APIVersion: "networking.k8s.io/v1",
		Kind:       "NetworkPolicy",
		Metadata: resourceMeta(
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
