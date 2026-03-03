package main

import (
	"fmt"
	"log"

	kratix "github.com/syntasso/kratix-go"

	u "github.com/jamesatintegratnio/gitops_homelab_2_0/promises/_shared/kratixutil"
)

const (
	defaultGatewayName     = "nginx-gateway"
	defaultGatewayNS       = "nginx-gateway"
	defaultHTTPSSection    = "https"
	defaultHTTPSection     = "http"
)

// GatewayRouteConfig holds the resolved configuration from the CR.
type GatewayRouteConfig struct {
	Name             string
	Namespace        string
	Hostname         string
	Path             string
	BackendName      string
	BackendPort      int
	GatewayName      string
	GatewayNS        string
	HTTPRedirect     bool
	OwnerPromise     string
	SectionName      string
	HTTPSectionName  string
}

func main() {
	sdk := kratix.New()

	log.Printf("=== Gateway Route Promise Pipeline ===")
	log.Printf("Action: %s", sdk.WorkflowAction())

	resource, err := sdk.ReadResourceInput()
	if err != nil {
		log.Fatalf("ERROR: Failed to read resource input: %v", err)
	}

	log.Printf("Processing resource: %s/%s",
		resource.GetNamespace(), resource.GetName())

	config, err := buildConfig(resource)
	if err != nil {
		log.Fatalf("ERROR: Failed to build config: %v", err)
	}

	if sdk.WorkflowAction() == "configure" {
		if err := handleConfigure(sdk, config); err != nil {
			log.Fatalf("ERROR: Configure failed: %v", err)
		}
	} else if sdk.WorkflowAction() == "delete" {
		if err := handleDelete(sdk, config); err != nil {
			log.Fatalf("ERROR: Delete failed: %v", err)
		}
	} else {
		log.Fatalf("ERROR: Unknown workflow action: %s", sdk.WorkflowAction())
	}

	log.Println("=== Pipeline completed successfully ===")
}

func buildConfig(resource kratix.Resource) (*GatewayRouteConfig, error) {
	config := &GatewayRouteConfig{
		GatewayName:     defaultGatewayName,
		GatewayNS:       defaultGatewayNS,
		SectionName:     defaultHTTPSSection,
		HTTPSectionName: defaultHTTPSection,
	}

	var err error
	config.Name, err = u.GetStringValue(resource, "spec.name")
	if err != nil {
		return nil, fmt.Errorf("spec.name is required: %w", err)
	}

	config.Namespace, err = u.GetStringValue(resource, "spec.namespace")
	if err != nil {
		return nil, fmt.Errorf("spec.namespace is required: %w", err)
	}

	config.Hostname, err = u.GetStringValue(resource, "spec.hostname")
	if err != nil {
		return nil, fmt.Errorf("spec.hostname is required: %w", err)
	}

	config.Path, _ = u.GetStringValueWithDefault(resource, "spec.path", "/")

	config.BackendName, err = u.GetStringValue(resource, "spec.backendRef.name")
	if err != nil {
		return nil, fmt.Errorf("spec.backendRef.name is required: %w", err)
	}

	config.BackendPort, err = u.GetIntValue(resource, "spec.backendRef.port")
	if err != nil {
		return nil, fmt.Errorf("spec.backendRef.port is required: %w", err)
	}

	if v, err := u.GetStringValue(resource, "spec.gateway.name"); err == nil && v != "" {
		config.GatewayName = v
	}
	if v, err := u.GetStringValue(resource, "spec.gateway.namespace"); err == nil && v != "" {
		config.GatewayNS = v
	}

	config.HTTPRedirect, _ = u.GetBoolValueWithDefault(resource, "spec.httpRedirect", true)

	config.OwnerPromise, _ = u.GetStringValueWithDefault(resource, "spec.ownerPromise", "gateway-route")

	if v, err := u.GetStringValue(resource, "spec.sectionName"); err == nil && v != "" {
		config.SectionName = v
	}
	if v, err := u.GetStringValue(resource, "spec.httpSectionName"); err == nil && v != "" {
		config.HTTPSectionName = v
	}

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
	if err := u.WriteYAML(sdk, "resources/httproute.yaml", httpsRoute); err != nil {
		return fmt.Errorf("write HTTPRoute: %w", err)
	}
	log.Printf("✓ Rendered HTTPS HTTPRoute: %s", config.Name)

	// 2. HTTP→HTTPS redirect route
	if config.HTTPRedirect {
		redirectRoute := buildHTTPRedirect(config, labels)
		if err := u.WriteYAML(sdk, "resources/http-redirect.yaml", redirectRoute); err != nil {
			return fmt.Errorf("write HTTP redirect: %w", err)
		}
		log.Printf("✓ Rendered HTTP→HTTPS redirect route: %s-http-redirect", config.Name)
	}

	// Write status
	status := kratix.NewStatus()
	status.Set("phase", "Configured")
	status.Set("hostname", config.Hostname)
	status.Set("url", fmt.Sprintf("https://%s%s", config.Hostname, config.Path))
	if config.HTTPRedirect {
		status.Set("httpRedirect", "enabled")
	}
	status.Set("message", fmt.Sprintf("Gateway route configured for %s", config.Hostname))

	if err := sdk.WriteStatus(status); err != nil {
		return fmt.Errorf("write status: %w", err)
	}

	return nil
}

func handleDelete(sdk *kratix.KratixSDK, config *GatewayRouteConfig) error {
	// HTTPS route
	httpsDelete := u.DeleteResource(
		"gateway.networking.k8s.io/v1",
		"HTTPRoute",
		config.Name,
		config.Namespace,
	)
	if err := u.WriteYAML(sdk, "resources/delete-httproute-"+config.Name+".yaml", httpsDelete); err != nil {
		return fmt.Errorf("write delete HTTPRoute: %w", err)
	}

	// HTTP redirect route
	if config.HTTPRedirect {
		redirectDelete := u.DeleteResource(
			"gateway.networking.k8s.io/v1",
			"HTTPRoute",
			fmt.Sprintf("%s-http-redirect", config.Name),
			config.Namespace,
		)
		if err := u.WriteYAML(sdk, "resources/delete-httproute-"+config.Name+"-redirect.yaml", redirectDelete); err != nil {
			return fmt.Errorf("write delete redirect HTTPRoute: %w", err)
		}
	}

	status := kratix.NewStatus()
	status.Set("phase", "Deleting")
	status.Set("message", fmt.Sprintf("Gateway routes for %s scheduled for deletion", config.Hostname))

	if err := sdk.WriteStatus(status); err != nil {
		return fmt.Errorf("write status: %w", err)
	}

	return nil
}

// buildHTTPSRoute creates the primary HTTPS HTTPRoute targeting the gateway's
// HTTPS listener with backend service routing.
func buildHTTPSRoute(config *GatewayRouteConfig, labels map[string]string) u.Resource {
	return u.Resource{
		APIVersion: "gateway.networking.k8s.io/v1",
		Kind:       "HTTPRoute",
		Metadata: u.ObjectMeta{
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
					"sectionName": config.SectionName,
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
func buildHTTPRedirect(config *GatewayRouteConfig, labels map[string]string) u.Resource {
	return u.Resource{
		APIVersion: "gateway.networking.k8s.io/v1",
		Kind:       "HTTPRoute",
		Metadata: u.ObjectMeta{
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
