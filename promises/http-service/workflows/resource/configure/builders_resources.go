package main

import (
	"fmt"

	ku "github.com/jamesatintegratnio/gitops_homelab_2_0/promises/_shared/kratixutil"
)

// buildExternalSecretRequest creates a PlatformExternalSecret sub-ResourceRequest
// that delegates to the external-secret promise.
func buildExternalSecretRequest(config *HTTPServiceConfig) ku.Resource {
	// Convert SecretRef slice to the format expected by the external-secret promise
	secrets := []map[string]interface{}{}
	for _, s := range config.Secrets {
		keys := []map[string]string{}
		for _, k := range s.Keys {
			keys = append(keys, map[string]string{
				"secretKey": k.SecretKey,
				"property":  k.Property,
			})
		}

		secret := map[string]interface{}{
			"onePasswordItem": s.OnePasswordItem,
			"keys":            keys,
		}
		if s.Name != "" {
			secret["name"] = s.Name
		}
		secrets = append(secrets, secret)
	}

	return ku.Resource{
		APIVersion: "platform.integratn.tech/v1alpha1",
		Kind:       "PlatformExternalSecret",
		Metadata: ku.ObjectMeta{
			Name:      fmt.Sprintf("%s-secrets", config.Name),
			Namespace: ku.DefaultPlatformRequestsNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "kratix",
				"kratix.io/promise-name":       "http-service",
				"app.kubernetes.io/part-of":    config.Name,
			},
		},
		Spec: map[string]interface{}{
			"namespace":       config.Namespace,
			"appName":         config.Name,
			"secretStoreName": config.SecretStoreName,
			"secretStoreKind": config.SecretStoreKind,
			"ownerPromise":    "http-service",
			"secrets":         secrets,
		},
	}
}

// buildGatewayRouteRequest creates a GatewayRoute sub-ResourceRequest
// that delegates to the gateway-route promise.
func buildGatewayRouteRequest(config *HTTPServiceConfig) ku.Resource {
	spec := map[string]interface{}{
		"name":      config.Name,
		"namespace": config.Namespace,
		"hostname":  config.IngressHostname,
		"path":      config.IngressPath,
		"backendRef": map[string]interface{}{
			"name": config.Name,
			"port": config.Port,
		},
		"gateway": map[string]interface{}{
			"name":      config.GatewayName,
			"namespace": config.GatewayNS,
		},
		"httpRedirect": true,
		"ownerPromise": "http-service",
	}

	return ku.Resource{
		APIVersion: "platform.integratn.tech/v1alpha1",
		Kind:       "GatewayRoute",
		Metadata: ku.ObjectMeta{
			Name:      fmt.Sprintf("%s-route", config.Name),
			Namespace: ku.DefaultPlatformRequestsNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "kratix",
				"kratix.io/promise-name":       "http-service",
				"app.kubernetes.io/part-of":    config.Name,
			},
		},
		Spec: spec,
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
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "kratix",
				"kratix.io/promise-name":       "http-service",
				"app.kubernetes.io/part-of":    config.Name,
			},
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

	// Allow monitoring scrape if enabled
	if config.MonitoringEnabled {
		policies = append(policies, ku.Resource{
			APIVersion: "networking.k8s.io/v1",
			Kind:       "NetworkPolicy",
			Metadata: ku.ObjectMeta{
				Name:      fmt.Sprintf("%s-allow-monitoring", config.Name),
				Namespace: config.Namespace,
				Labels: map[string]string{
					"app.kubernetes.io/managed-by": "kratix",
					"kratix.io/promise-name":       "http-service",
					"app.kubernetes.io/part-of":    config.Name,
				},
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
										"kubernetes.io/metadata.name": ku.MonitoringNamespace,
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
	}

	// Allow DNS egress (all pods need this)
	policies = append(policies, ku.Resource{
		APIVersion: "networking.k8s.io/v1",
		Kind:       "NetworkPolicy",
		Metadata: ku.ObjectMeta{
			Name:      fmt.Sprintf("%s-allow-dns", config.Name),
			Namespace: config.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "kratix",
				"kratix.io/promise-name":       "http-service",
				"app.kubernetes.io/part-of":    config.Name,
			},
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
			"policyTypes": []string{"Egress"},
			"egress": []map[string]interface{}{
				{
					"ports": []map[string]interface{}{
						{"protocol": "UDP", "port": 53},
						{"protocol": "TCP", "port": 53},
					},
				},
			},
		},
	})

	return policies
}
