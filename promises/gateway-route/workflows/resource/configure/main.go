package main

import (
	"fmt"
	"log"

	kratix "github.com/syntasso/kratix-go"

	ku "github.com/jamesatintegratnio/gitops_homelab_2_0/promises/_shared/kratixutil"
)

const (
	defaultHTTPSSection    = "https"
	defaultHTTPSection     = "http"
)

func main() {
	ku.RunPromiseWithConfig("Gateway Route", buildConfig, handleConfigure, handleDelete)
}

func buildConfig(_ *kratix.KratixSDK, resource kratix.Resource) (*GatewayRouteConfig, error) {
	config := &GatewayRouteConfig{}

	var err error
	config.Name, err = ku.GetStringValue(resource, "spec.name")
	if err != nil {
		return nil, fmt.Errorf("spec.name is required: %w", err)
	}

	config.Namespace, err = ku.GetStringValue(resource, "spec.namespace")
	if err != nil {
		return nil, fmt.Errorf("spec.namespace is required: %w", err)
	}

	config.Hostname, err = ku.GetStringValue(resource, "spec.hostname")
	if err != nil {
		return nil, fmt.Errorf("spec.hostname is required: %w", err)
	}

	config.Path = ku.GetStringValueWithDefault(resource, "spec.path", "/")

	config.BackendName, err = ku.GetStringValue(resource, "spec.backendRef.name")
	if err != nil {
		return nil, fmt.Errorf("spec.backendRef.name is required: %w", err)
	}

	config.BackendPort, err = ku.GetIntValue(resource, "spec.backendRef.port")
	if err != nil {
		return nil, fmt.Errorf("spec.backendRef.port is required: %w", err)
	}

	config.GatewayName = ku.GetStringValueWithDefault(resource, "spec.gateway.name", ku.DefaultGatewayName)
	config.GatewayNS = ku.GetStringValueWithDefault(resource, "spec.gateway.namespace", ku.DefaultGatewayNamespace)

	config.HTTPRedirect = ku.GetBoolValueWithDefault(resource, "spec.httpRedirect", true)

	config.OwnerPromise = ku.GetStringValueWithDefault(resource, "spec.ownerPromise", "gateway-route")

	config.HTTPSSectionName = ku.GetStringValueWithDefault(resource, "spec.sectionName", defaultHTTPSSection)
	config.HTTPSectionName = ku.GetStringValueWithDefault(resource, "spec.httpSectionName", defaultHTTPSection)

	return config, nil
}

func handleConfigure(sdk *kratix.KratixSDK, config *GatewayRouteConfig) error {
	labels := map[string]string{
		"app.kubernetes.io/managed-by": "kratix",
		"kratix.io/promise-name":       config.OwnerPromise,
		"app.kubernetes.io/part-of":    config.Name,
	}

	// 1. HTTPS HTTPRoute (primary route)
	httpsRoute := buildHTTPSRoute(config, labels)
	if err := ku.WriteYAML(sdk, "resources/httproute.yaml", httpsRoute); err != nil {
		return fmt.Errorf("write HTTPRoute: %w", err)
	}
	log.Printf("✓ Rendered HTTPS HTTPRoute: %s", config.Name)

	// 2. HTTP→HTTPS redirect route
	if config.HTTPRedirect {
		redirectRoute := buildHTTPRedirect(config, labels)
		if err := ku.WriteYAML(sdk, "resources/http-redirect.yaml", redirectRoute); err != nil {
			return fmt.Errorf("write HTTP redirect: %w", err)
		}
		log.Printf("✓ Rendered HTTP→HTTPS redirect route: %s-http-redirect", config.Name)
	}

	fields := map[string]interface{}{
		"hostname": config.Hostname,
		"url":      fmt.Sprintf("https://%s%s", config.Hostname, config.Path),
	}
	if config.HTTPRedirect {
		fields["httpRedirect"] = "enabled"
	}
	if err := ku.WritePromiseStatus(sdk, "Configured",
		fmt.Sprintf("Gateway route configured for %s", config.Hostname), fields); err != nil {
		return fmt.Errorf("write status: %w", err)
	}

	return nil
}

func handleDelete(sdk *kratix.KratixSDK, config *GatewayRouteConfig) error {
	// HTTPS route
	httpsDelete := ku.DeleteResource(
		"gateway.networking.k8s.io/v1",
		"HTTPRoute",
		config.Name,
		config.Namespace,
	)
	if err := ku.WriteYAML(sdk, "resources/delete-httproute-"+config.Name+".yaml", httpsDelete); err != nil {
		return fmt.Errorf("write delete HTTPRoute: %w", err)
	}

	// HTTP redirect route
	if config.HTTPRedirect {
		redirectDelete := ku.DeleteResource(
			"gateway.networking.k8s.io/v1",
			"HTTPRoute",
			fmt.Sprintf("%s-http-redirect", config.Name),
			config.Namespace,
		)
		if err := ku.WriteYAML(sdk, "resources/delete-httproute-"+config.Name+"-redirect.yaml", redirectDelete); err != nil {
			return fmt.Errorf("write delete redirect HTTPRoute: %w", err)
		}
	}

	if err := ku.WritePromiseStatus(sdk, "Deleting",
		fmt.Sprintf("Gateway routes for %s scheduled for deletion", config.Hostname), nil); err != nil {
		return fmt.Errorf("write status: %w", err)
	}

	return nil
}

// buildHTTPSRoute creates the primary HTTPS HTTPRoute targeting the gateway's
// HTTPS listener with backend service routing.
func buildHTTPSRoute(config *GatewayRouteConfig, labels map[string]string) ku.Resource {
	return ku.Resource{
		APIVersion: "gateway.networking.k8s.io/v1",
		Kind:       "HTTPRoute",
		Metadata: ku.ObjectMeta{
			Name:      config.Name,
			Namespace: config.Namespace,
			Labels:    labels,
			Annotations: map[string]string{
				"argocd.argoproj.io/sync-wave": "10",
			},
		},
		Spec: map[string]interface{}{
			"hostnames": []string{config.Hostname},
			"parentRefs": []map[string]interface{}{
				{
					"name":        config.GatewayName,
					"namespace":   config.GatewayNS,
					"sectionName": config.HTTPSSectionName,
				},
			},
			"rules": []map[string]interface{}{
				{
					"matches": []map[string]interface{}{
						{
							"path": map[string]interface{}{
								"type":  "PathPrefix",
								"value": config.Path,
							},
						},
					},
					"backendRefs": []map[string]interface{}{
						{
							"name": config.BackendName,
							"port": config.BackendPort,
						},
					},
				},
			},
		},
	}
}

// buildHTTPRedirect creates an HTTPRoute that redirects HTTP→HTTPS (301).
// It targets the gateway's HTTP listener so plain HTTP requests are automatically
// redirected to the HTTPS equivalent.
func buildHTTPRedirect(config *GatewayRouteConfig, labels map[string]string) ku.Resource {
	return ku.Resource{
		APIVersion: "gateway.networking.k8s.io/v1",
		Kind:       "HTTPRoute",
		Metadata: ku.ObjectMeta{
			Name:      fmt.Sprintf("%s-http-redirect", config.Name),
			Namespace: config.Namespace,
			Labels:    labels,
			Annotations: map[string]string{
				"argocd.argoproj.io/sync-wave": "10",
			},
		},
		Spec: map[string]interface{}{
			"hostnames": []string{config.Hostname},
			"parentRefs": []map[string]interface{}{
				{
					"group":       "gateway.networking.k8s.io",
					"kind":        "Gateway",
					"name":        config.GatewayName,
					"namespace":   config.GatewayNS,
					"sectionName": config.HTTPSectionName,
				},
			},
			"rules": []map[string]interface{}{
				{
					"matches": []map[string]interface{}{
						{
							"path": map[string]interface{}{
								"type":  "PathPrefix",
								"value": "/",
							},
						},
					},
					"filters": []map[string]interface{}{
						{
							"type": "RequestRedirect",
							"requestRedirect": map[string]interface{}{
								"scheme":     "https",
								"statusCode": 301,
							},
						},
					},
				},
			},
		},
	}
}
