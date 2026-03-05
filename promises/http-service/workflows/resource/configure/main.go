package main

import (
	"fmt"
	"log"

	kratix "github.com/syntasso/kratix-go"

	ku "github.com/jamesatintegratnio/gitops_homelab_2_0/promises/_shared/kratixutil"
)

// Platform-wide defaults — baked into every HTTP service.
const (
	defaultBaseDomain      = "cluster.integratn.tech"
	stakaterChartRepo      = "https://stakater.github.io/stakater-charts"
	stakaterChartName      = "application"
	stakaterChartVersion   = "6.16.1"
	argoCDProject          = "default"
)

func main() {
	ku.RunPromiseWithConfig("HTTP Service", buildConfig, handleConfigure, handleDelete)
}

// buildConfig extracts all fields from the CR with sensible defaults.
func buildConfig(_ *kratix.KratixSDK, resource kratix.Resource) (*HTTPServiceConfig, error) {
	config := &HTTPServiceConfig{
		BaseDomain:      defaultBaseDomain,
		GatewayName:     ku.DefaultGatewayName,
		GatewayNS:       ku.DefaultGatewayNamespace,
		SecretStoreName: ku.DefaultSecretStoreName,
		SecretStoreKind: ku.DefaultSecretStoreKind,
	}

	var err error
	config.Name, err = ku.GetStringValue(resource, "spec.name")
	if err != nil {
		return nil, fmt.Errorf("spec.name is required: %w", err)
	}

	config.Namespace = ku.GetStringValueWithDefault(resource, "spec.namespace", config.Name)
	config.Team = ku.GetStringValueWithDefault(resource, "spec.team", "platform")

	// Image
	config.ImageRepository, err = ku.GetStringValue(resource, "spec.image.repository")
	if err != nil {
		return nil, fmt.Errorf("spec.image.repository is required: %w", err)
	}
	config.ImageTag = ku.GetStringValueWithDefault(resource, "spec.image.tag", "latest")
	config.ImagePullPolicy = ku.GetStringValueWithDefault(resource, "spec.image.pullPolicy", "IfNotPresent")
	config.Command = ku.ExtractStringSlice(resource, "spec.command")
	config.Args = ku.ExtractStringSlice(resource, "spec.args")

	// Scaling
	config.Replicas = ku.GetIntValueWithDefault(resource, "spec.replicas", 1)
	config.CPURequest = ku.GetStringValueWithDefault(resource, "spec.resources.requests.cpu", "100m")
	config.MemoryRequest = ku.GetStringValueWithDefault(resource, "spec.resources.requests.memory", "128Mi")
	config.CPULimit = ku.GetStringValueWithDefault(resource, "spec.resources.limits.cpu", "500m")
	config.MemoryLimit = ku.GetStringValueWithDefault(resource, "spec.resources.limits.memory", "256Mi")

	// Networking
	config.Port = ku.GetIntValueWithDefault(resource, "spec.port", 8080)
	config.IngressEnabled = ku.GetBoolValueWithDefault(resource, "spec.ingress.enabled", true)
	config.IngressHostname, _ = ku.GetStringValue(resource, "spec.ingress.hostname")
	if config.IngressHostname == "" {
		config.IngressHostname = fmt.Sprintf("%s.%s", config.Name, config.BaseDomain)
	}
	config.IngressPath = ku.GetStringValueWithDefault(resource, "spec.ingress.path", "/")

	// Secrets
	config.Secrets = ku.ExtractSecrets(resource, "spec.secrets")

	// Environment
	config.Env = ku.ExtractStringMap(resource, "spec.env")
	config.EnvFromSecrets = ku.ExtractStringSlice(resource, "spec.envFromSecrets")

	// Health checks
	config.HealthCheckPath = ku.GetStringValueWithDefault(resource, "spec.healthCheck.path", "/")
	config.HealthCheckPort = ku.GetIntValueWithDefault(resource, "spec.healthCheck.port", config.Port)

	// Monitoring
	config.MonitoringEnabled = ku.GetBoolValueWithDefault(resource, "spec.monitoring.enabled", false)
	config.MonitoringPath = ku.GetStringValueWithDefault(resource, "spec.monitoring.path", "/metrics")
	config.MonitoringInterval = ku.GetStringValueWithDefault(resource, "spec.monitoring.interval", "30s")

	// Storage
	config.PersistenceEnabled = ku.GetBoolValueWithDefault(resource, "spec.persistence.enabled", false)
	config.PersistenceSize = ku.GetStringValueWithDefault(resource, "spec.persistence.size", "1Gi")
	config.PersistenceClass, _ = ku.GetStringValue(resource, "spec.persistence.storageClass")
	config.PersistenceMountPath = ku.GetStringValueWithDefault(resource, "spec.persistence.mountPath", "/data")

	// Security context
	if v, err := ku.GetBoolValue(resource, "spec.securityContext.runAsNonRoot"); err == nil {
		config.RunAsNonRoot = &v
	}
	if v, err := ku.GetBoolValue(resource, "spec.securityContext.readOnlyRootFilesystem"); err == nil {
		config.ReadOnlyRootFilesystem = &v
	}
	if v, err := ku.GetIntValue(resource, "spec.securityContext.runAsUser"); err == nil {
		v64 := int64(v)
		config.RunAsUser = &v64
	}
	if v, err := ku.GetIntValue(resource, "spec.securityContext.runAsGroup"); err == nil {
		v64 := int64(v)
		config.RunAsGroup = &v64
	}

	// Helm overrides
	if val, err := resource.GetValue("spec.helmOverrides"); err == nil && val != nil {
		if m, ok := val.(map[string]interface{}); ok {
			config.HelmOverrides = m
		}
	}

	return config, nil
}

// handleConfigure generates the Namespace + ArgoCD app + sub-ResourceRequests + NetworkPolicies.
func handleConfigure(sdk *kratix.KratixSDK, config *HTTPServiceConfig) error {
	// 0. Create the target Namespace first (low sync-wave so it exists before everything else)
	ns := ku.Resource{
		APIVersion: "v1",
		Kind:       "Namespace",
		Metadata: ku.ObjectMeta{
			Name: config.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by":           "kratix",
				"kratix.io/promise-name":                 "http-service",
				"app.kubernetes.io/part-of":              config.Name,
				"app.kubernetes.io/team":                 config.Team,
				"platform.integratn.tech/gateway-access": "true",
			},
			Annotations: map[string]string{
				"argocd.argoproj.io/sync-wave": "0",
			},
		},
	}
	if err := ku.WriteYAML(sdk, "resources/namespace.yaml", ns); err != nil {
		return fmt.Errorf("write Namespace: %w", err)
	}
	log.Printf("✓ Rendered Namespace: %s", config.Namespace)

	// 1. Build Stakater application chart values
	values := buildStakaterValues(config)

	// 2. Deep-merge any helmOverrides on top
	if config.HelmOverrides != nil {
		values = ku.DeepMerge(values, config.HelmOverrides)
	}

	// 3. Build ArgoCDApplication sub-ResourceRequest (delegates to the argocd-application promise)
	appLabels := map[string]string{
		"app.kubernetes.io/managed-by": "kratix",
		"kratix.io/promise-name":       "http-service",
		"app.kubernetes.io/part-of":    config.Name,
		"app.kubernetes.io/team":       config.Team,
	}

	appRequest := ku.Resource{
		APIVersion: "platform.integratn.tech/v1alpha1",
		Kind:       "ArgoCDApplication",
		Metadata: ku.ObjectMeta{
			Name:      config.Name,
			Namespace: "platform-requests",
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "kratix",
				"app.kubernetes.io/name":       "argocd-application",
				"kratix.io/promise-name":       "http-service",
				"app.kubernetes.io/part-of":    config.Name,
				"app.kubernetes.io/team":       config.Team,
			},
		},
		Spec: ku.ArgoCDApplicationSpec{
			Name:      config.Name,
			Namespace: "argocd",
			Annotations: map[string]string{
				"argocd.argoproj.io/sync-wave": "10",
			},
			Labels:     appLabels,
			Finalizers: []string{"resources-finalizer.argocd.argoproj.io"},
			Project:    argoCDProject,
			Source: ku.AppSource{
				RepoURL:        stakaterChartRepo,
				Chart:          stakaterChartName,
				TargetRevision: stakaterChartVersion,
				Helm: &ku.HelmSource{
					ReleaseName:  config.Name,
					ValuesObject: values,
				},
			},
			Destination: ku.Destination{
				Server:    "https://kubernetes.default.svc",
				Namespace: config.Namespace,
			},
			SyncPolicy: map[string]interface{}{
				"automated": map[string]interface{}{
					"selfHeal": true,
					"prune":    true,
				},
				"syncOptions": []string{
					"CreateNamespace=true",
					"ServerSideApply=true",
				},
			},
		},
	}

	if err := ku.WriteYAML(sdk, "resources/argocd-application-request.yaml", appRequest); err != nil {
		return fmt.Errorf("write ArgoCDApplication request: %w", err)
	}
	log.Printf("✓ Rendered ArgoCDApplication sub-ResourceRequest: %s", config.Name)

	// 4. Emit PlatformExternalSecret sub-ResourceRequest (delegates to external-secret promise)
	if len(config.Secrets) > 0 {
		esRequest := buildExternalSecretRequest(config)
		if err := ku.WriteYAML(sdk, "resources/external-secret-request.yaml", esRequest); err != nil {
			return fmt.Errorf("write PlatformExternalSecret request: %w", err)
		}
		log.Printf("✓ Rendered PlatformExternalSecret sub-ResourceRequest (%d secret(s))", len(config.Secrets))
	}

	// 5. Build NetworkPolicies (remain inline — too variable for a sub-promise)
	netpols := buildNetworkPolicies(config)
	if err := ku.WriteYAMLDocuments(sdk, "resources/network-policies.yaml", netpols); err != nil {
		return fmt.Errorf("write NetworkPolicies: %w", err)
	}
	log.Printf("✓ Rendered NetworkPolicies")

	// 6. Emit GatewayRoute sub-ResourceRequest (delegates to gateway-route promise)
	if config.IngressEnabled {
		gwRequest := buildGatewayRouteRequest(config)
		if err := ku.WriteYAML(sdk, "resources/gateway-route-request.yaml", gwRequest); err != nil {
			return fmt.Errorf("write GatewayRoute request: %w", err)
		}
		log.Printf("✓ Rendered GatewayRoute sub-ResourceRequest")
	}

	// 7. Write status
	status := kratix.NewStatus()
	status.Set("phase", "Configured")
	status.Set("message", fmt.Sprintf("HTTP Service %s configured", config.Name))
	status.Set("namespace", config.Namespace)
	if config.IngressEnabled {
		status.Set("url", fmt.Sprintf("https://%s%s", config.IngressHostname, config.IngressPath))
	}

	if err := sdk.WriteStatus(status); err != nil {
		return fmt.Errorf("write status: %w", err)
	}

	return nil
}

// handleDelete cleans up sub-ResourceRequests.
func handleDelete(sdk *kratix.KratixSDK, config *HTTPServiceConfig) error {
	// Delete ArgoCDApplication sub-ResourceRequest
	appRequest := ku.Resource{
		APIVersion: "platform.integratn.tech/v1alpha1",
		Kind:       "ArgoCDApplication",
		Metadata: ku.ObjectMeta{
			Name:      config.Name,
			Namespace: "platform-requests",
		},
	}
	if err := ku.WriteYAML(sdk, "resources/delete-argocdapplication-"+config.Name+".yaml", appRequest); err != nil {
		return fmt.Errorf("write delete ArgoCDApplication request: %w", err)
	}
	log.Printf("✓ Delete scheduled for ArgoCDApplication: %s", config.Name)

	// Delete PlatformExternalSecret sub-ResourceRequest
	if len(config.Secrets) > 0 {
		esRequest := ku.Resource{
			APIVersion: "platform.integratn.tech/v1alpha1",
			Kind:       "PlatformExternalSecret",
			Metadata: ku.ObjectMeta{
				Name:      fmt.Sprintf("%s-secrets", config.Name),
				Namespace: "platform-requests",
			},
		}
		if err := ku.WriteYAML(sdk, "resources/delete-externalsecret-"+config.Name+".yaml", esRequest); err != nil {
			return fmt.Errorf("write delete PlatformExternalSecret request: %w", err)
		}
		log.Printf("✓ Delete scheduled for PlatformExternalSecret: %s", config.Name)
	}

	// Delete GatewayRoute sub-ResourceRequest
	if config.IngressEnabled {
		gwRequest := ku.Resource{
			APIVersion: "platform.integratn.tech/v1alpha1",
			Kind:       "GatewayRoute",
			Metadata: ku.ObjectMeta{
				Name:      fmt.Sprintf("%s-route", config.Name),
				Namespace: "platform-requests",
			},
		}
		if err := ku.WriteYAML(sdk, "resources/delete-gatewayroute-"+config.Name+".yaml", gwRequest); err != nil {
			return fmt.Errorf("write delete GatewayRoute request: %w", err)
		}
		log.Printf("✓ Delete scheduled for GatewayRoute: %s", config.Name)
	}

	status := kratix.NewStatus()
	status.Set("phase", "Deleting")
	status.Set("message", fmt.Sprintf("HTTP Service %s scheduled for deletion", config.Name))

	if err := sdk.WriteStatus(status); err != nil {
		return fmt.Errorf("write status: %w", err)
	}

	return nil
}
