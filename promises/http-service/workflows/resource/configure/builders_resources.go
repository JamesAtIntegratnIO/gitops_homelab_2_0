package main

import (
	"fmt"

	ku "github.com/jamesatintegratnio/gitops_homelab_2_0/promises/_shared/kratixutil"
)

// buildExternalSecretRequest creates a PlatformExternalSecret sub-ResourceRequest
// that delegates to the external-secret promise.
func buildExternalSecretRequest(config *HTTPServiceConfig) ku.Resource {
	// Convert SecretRef slice to the typed format expected by the external-secret promise
	secrets := make([]ku.PlatformExternalSecretItem, 0, len(config.Secrets))
	for _, s := range config.Secrets {
		secrets = append(secrets, ku.PlatformExternalSecretItem{
			Name:            s.Name,
			OnePasswordItem: s.OnePasswordItem,
			Keys:            s.Keys,
		})
	}

	return ku.Resource{
		APIVersion: "platform.integratn.tech/v1alpha1",
		Kind:       "PlatformExternalSecret",
		Metadata: ku.ObjectMeta{
			Name:      fmt.Sprintf("%s-secrets", config.Name),
			Namespace: ku.DefaultPlatformRequestsNamespace,
			Labels: ku.MergeStringMap(ku.BaseLabels("http-service", config.Name), map[string]string{
				"app.kubernetes.io/part-of": config.Name,
			}),
		},
		Spec: ku.PlatformExternalSecretSpec{
			Namespace:       config.Namespace,
			AppName:         config.Name,
			SecretStoreName: config.SecretStoreName,
			SecretStoreKind: config.SecretStoreKind,
			OwnerPromise:    "http-service",
			Secrets:         secrets,
		},
	}
}

// buildGatewayRouteRequest creates a GatewayRoute sub-ResourceRequest
// that delegates to the gateway-route promise.
func buildGatewayRouteRequest(config *HTTPServiceConfig) ku.Resource {
	return ku.Resource{
		APIVersion: "platform.integratn.tech/v1alpha1",
		Kind:       "GatewayRoute",
		Metadata: ku.ObjectMeta{
			Name:      fmt.Sprintf("%s-route", config.Name),
			Namespace: ku.DefaultPlatformRequestsNamespace,
			Labels: ku.MergeStringMap(ku.BaseLabels("http-service", config.Name), map[string]string{
				"app.kubernetes.io/part-of": config.Name,
			}),
		},
		Spec: ku.GatewayRouteSpec{
			Name:      config.Name,
			Namespace: config.Namespace,
			Hostname:  config.IngressHostname,
			Path:      config.IngressPath,
			BackendRef: ku.GatewayBackendRef{
				Name: config.Name,
				Port: config.Port,
			},
			Gateway: ku.GatewayRef{
				Name:      config.GatewayName,
				Namespace: config.GatewayNS,
			},
			HTTPRedirect: true,
			OwnerPromise: "http-service",
		},
	}
}

// buildNetworkPolicies creates allow-ingress-from-gateway + allow-dns policies.
// NOTE: We do NOT generate a default-deny policy here because the platform's
// Kyverno ClusterPolicy (generate-default-deny-netpol) automatically creates a
// default-deny-all NetworkPolicy in every new namespace.
func buildNetworkPolicies(config *HTTPServiceConfig) []ku.Resource {
	var policies []ku.Resource

	// Allow ingress from the gateway namespace
	policies = append(policies, ku.Resource{
		APIVersion: "networking.k8s.io/v1",
		Kind:       "NetworkPolicy",
		Metadata: ku.ObjectMeta{
			Name:      fmt.Sprintf("%s-allow-gateway", config.Name),
			Namespace: config.Namespace,
			Labels: ku.MergeStringMap(ku.BaseLabels("http-service", config.Name), map[string]string{
				"app.kubernetes.io/part-of": config.Name,
			}),
			Annotations: map[string]string{
				"argocd.argoproj.io/sync-wave": "5",
			},
		},
		Spec: map[string]interface{}{
			"podSelector": map[string]interface{}{
				"matchLabels": map[string]string{
					"app.kubernetes.io/name": config.Name,
				},
			},
			"policyTypes": []string{"Ingress"},
			"ingress": []map[string]interface{}{
				{
					"from": []map[string]interface{}{
						{
							"namespaceSelector": map[string]interface{}{
								"matchLabels": map[string]string{
									"kubernetes.io/metadata.name": config.GatewayNS,
								},
							},
						},
					},
					"ports": []map[string]interface{}{
						{
							"protocol": "TCP",
							"port":     config.Port,
						},
					},
				},
			},
		},
	})

	// Common labels and annotations for network policies
	netpolLabels := ku.MergeStringMap(ku.BaseLabels("http-service", config.Name), map[string]string{
		"app.kubernetes.io/part-of": config.Name,
	})
	syncWaveAnnotations := map[string]string{
		"argocd.argoproj.io/sync-wave": "5",
	}

	// Allow monitoring scrape if enabled
	if config.MonitoringEnabled {
		mon := ku.BuildMonitoringIngressPolicy(
			fmt.Sprintf("%s-allow-monitoring", config.Name),
			config.Namespace,
			netpolLabels,
			config.Port,
		)
		mon.Metadata.Annotations = syncWaveAnnotations
		policies = append(policies, mon)
	}

	// Allow DNS egress to kube-system CoreDNS (all pods need this)
	dns := ku.BuildDNSEgressPolicy(
		fmt.Sprintf("%s-allow-dns", config.Name),
		config.Namespace,
		netpolLabels,
	)
	dns.Metadata.Annotations = syncWaveAnnotations
	policies = append(policies, dns)

	return policies
}
