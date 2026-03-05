package kratixutil

// ---------------------------------------------------------------------------
// Shared Network Policy Builders
//
// Common network policy patterns reused across promise pipelines. Each builder
// returns a Resource with podSelector: {} (all pods in namespace). Promise-
// specific policies that need narrower selectors should remain in their own
// modules.
// ---------------------------------------------------------------------------

// BuildDefaultDenyPolicy creates a default-deny-all NetworkPolicy that blocks
// all ingress and egress traffic in the namespace by default.
func BuildDefaultDenyPolicy(name, namespace string, labels map[string]string) Resource {
	return Resource{
		APIVersion: "networking.k8s.io/v1",
		Kind:       "NetworkPolicy",
		Metadata:   ResourceMeta(name, namespace, labels, nil),
		Spec: map[string]interface{}{
			"podSelector": map[string]interface{}{},
			"policyTypes": []string{"Ingress", "Egress"},
		},
	}
}

// BuildDNSEgressPolicy creates a NetworkPolicy allowing DNS egress to
// kube-system CoreDNS on port 53 (UDP and TCP).
func BuildDNSEgressPolicy(name, namespace string, labels map[string]string) Resource {
	return Resource{
		APIVersion: "networking.k8s.io/v1",
		Kind:       "NetworkPolicy",
		Metadata:   ResourceMeta(name, namespace, labels, nil),
		Spec: map[string]interface{}{
			"podSelector": map[string]interface{}{},
			"policyTypes": []string{"Egress"},
			"egress": []map[string]interface{}{
				{
					"to": []map[string]interface{}{
						{
							"namespaceSelector": map[string]interface{}{
								"matchLabels": map[string]string{
									"kubernetes.io/metadata.name": KubeSystemNamespace,
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
						{"protocol": "UDP", "port": DNSPort},
						{"protocol": "TCP", "port": DNSPort},
					},
				},
			},
		},
	}
}

// BuildMonitoringIngressPolicy creates a NetworkPolicy allowing ingress from
// the monitoring namespace for Prometheus scraping on the specified port.
func BuildMonitoringIngressPolicy(name, namespace string, labels map[string]string, appPort int) Resource {
	return Resource{
		APIVersion: "networking.k8s.io/v1",
		Kind:       "NetworkPolicy",
		Metadata:   ResourceMeta(name, namespace, labels, nil),
		Spec: map[string]interface{}{
			"podSelector": map[string]interface{}{},
			"policyTypes": []string{"Ingress"},
			"ingress": []map[string]interface{}{
				{
					"from": []map[string]interface{}{
						{
							"namespaceSelector": map[string]interface{}{
								"matchLabels": map[string]string{
									"kubernetes.io/metadata.name": MonitoringNamespace,
								},
							},
						},
					},
					"ports": []map[string]interface{}{
						{"protocol": "TCP", "port": appPort},
					},
				},
			},
		},
	}
}

// BuildNamespaceIngressPolicy creates a NetworkPolicy allowing ingress from
// all pods in a specific namespace.
func BuildNamespaceIngressPolicy(name, namespace string, labels map[string]string, fromNamespace string) Resource {
	return Resource{
		APIVersion: "networking.k8s.io/v1",
		Kind:       "NetworkPolicy",
		Metadata:   ResourceMeta(name, namespace, labels, nil),
		Spec: map[string]interface{}{
			"podSelector": map[string]interface{}{},
			"policyTypes": []string{"Ingress"},
			"ingress": []map[string]interface{}{
				{
					"from": []map[string]interface{}{
						{
							"namespaceSelector": map[string]interface{}{
								"matchLabels": map[string]string{
									"kubernetes.io/metadata.name": fromNamespace,
								},
							},
						},
					},
				},
			},
		},
	}
}
