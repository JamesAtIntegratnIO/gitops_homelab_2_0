package main

import (
	"fmt"

	kratix "github.com/syntasso/kratix-go"

	ku "github.com/jamesatintegratnio/gitops_homelab_2_0/promises/_shared/kratixutil"
)

// Platform-wide defaults — baked into every HTTP service.
const (
	defaultBaseDomain      = ku.DefaultClusterBaseDomain
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
	config.Command, err = ku.ExtractStringSliceFromResource(resource, "spec.command")
	if err != nil {
		return nil, err
	}
	config.Args, err = ku.ExtractStringSliceFromResource(resource, "spec.args")
	if err != nil {
		return nil, err
	}

	// Scaling
	config.Replicas = ku.GetIntValueWithDefault(resource, "spec.replicas", 1)
	config.CPURequest = ku.GetStringValueWithDefault(resource, "spec.resources.requests.cpu", "100m")
	config.MemoryRequest = ku.GetStringValueWithDefault(resource, "spec.resources.requests.memory", "128Mi")
	config.CPULimit = ku.GetStringValueWithDefault(resource, "spec.resources.limits.cpu", "500m")
	config.MemoryLimit = ku.GetStringValueWithDefault(resource, "spec.resources.limits.memory", "256Mi")

	// Networking
	config.Port = ku.GetIntValueWithDefault(resource, "spec.port", 8080)
	config.IngressEnabled = ku.GetBoolValueWithDefault(resource, "spec.ingress.enabled", true)
	config.IngressHostname, err = ku.GetOptionalStringValue(resource, "spec.ingress.hostname")
	if err != nil {
		return nil, err
	}
	if config.IngressHostname == "" {
		config.IngressHostname = fmt.Sprintf("%s.%s", config.Name, config.BaseDomain)
	}
	config.IngressPath = ku.GetStringValueWithDefault(resource, "spec.ingress.path", "/")

	// Secrets
	config.Secrets, err = ku.ExtractSecretsFromResource(resource, "spec.secrets")
	if err != nil {
		return nil, err
	}

	// Environment
	config.Env, err = ku.ExtractStringMapFromResource(resource, "spec.env")
	if err != nil {
		return nil, err
	}
	config.EnvFromSecrets, err = ku.ExtractStringSliceFromResource(resource, "spec.envFromSecrets")
	if err != nil {
		return nil, err
	}

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
	config.PersistenceClass, err = ku.GetOptionalStringValue(resource, "spec.persistence.storageClass")
	if err != nil {
		return nil, err
	}
	config.PersistenceMountPath = ku.GetStringValueWithDefault(resource, "spec.persistence.mountPath", "/data")

	// Security context
	if v, err := ku.GetBoolValue(resource, "spec.securityContext.runAsNonRoot"); err == nil {
		config.RunAsNonRoot = &v
	} else if rawVal, _ := resource.GetValue("spec.securityContext.runAsNonRoot"); rawVal != nil {
		return nil, fmt.Errorf("spec.securityContext.runAsNonRoot: %w", err)
	}
	if v, err := ku.GetBoolValue(resource, "spec.securityContext.readOnlyRootFilesystem"); err == nil {
		config.ReadOnlyRootFilesystem = &v
	} else if rawVal, _ := resource.GetValue("spec.securityContext.readOnlyRootFilesystem"); rawVal != nil {
		return nil, fmt.Errorf("spec.securityContext.readOnlyRootFilesystem: %w", err)
	}
	if v, err := ku.GetIntValue(resource, "spec.securityContext.runAsUser"); err == nil {
		v64 := int64(v)
		config.RunAsUser = &v64
	} else if rawVal, _ := resource.GetValue("spec.securityContext.runAsUser"); rawVal != nil {
		return nil, fmt.Errorf("spec.securityContext.runAsUser: %w", err)
	}
	if v, err := ku.GetIntValue(resource, "spec.securityContext.runAsGroup"); err == nil {
		v64 := int64(v)
		config.RunAsGroup = &v64
	} else if rawVal, _ := resource.GetValue("spec.securityContext.runAsGroup"); rawVal != nil {
		return nil, fmt.Errorf("spec.securityContext.runAsGroup: %w", err)
	}

	// Helm overrides
	config.HelmOverrides, err = ku.ExtractMapFromResource(resource, "spec.helmOverrides")
	if err != nil {
		return nil, err
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
			Namespace: ku.DefaultPlatformRequestsNamespace,
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
			Namespace: ku.DefaultArgoCDNamespace,
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
			SyncPolicy: &ku.SyncPolicy{
				Automated: &ku.AutomatedSync{
					SelfHeal: true,
					Prune:    true,
				},
				SyncOptions: []string{
					"CreateNamespace=true",
					"ServerSideApply=true",
				},
			},
		},
	}

	if err := ku.WriteYAML(sdk, "resources/argocd-application-request.yaml", appRequest); err != nil {
		return fmt.Errorf("write ArgoCDApplication request: %w", err)
	}

	// 4. Emit PlatformExternalSecret sub-ResourceRequest (delegates to external-secret promise)
	if len(config.Secrets) > 0 {
		esRequest := buildExternalSecretRequest(config)
		if err := ku.WriteYAML(sdk, "resources/external-secret-request.yaml", esRequest); err != nil {
			return fmt.Errorf("write PlatformExternalSecret request: %w", err)
		}
	}

	// 5. Build NetworkPolicies (remain inline — too variable for a sub-promise)
	netpols := buildNetworkPolicies(config)
	if err := ku.WriteYAMLDocuments(sdk, "resources/network-policies.yaml", netpols); err != nil {
		return fmt.Errorf("write NetworkPolicies: %w", err)
	}

	// 6. Emit GatewayRoute sub-ResourceRequest (delegates to gateway-route promise)
	if config.IngressEnabled {
		gwRequest := buildGatewayRouteRequest(config)
		if err := ku.WriteYAML(sdk, "resources/gateway-route-request.yaml", gwRequest); err != nil {
			return fmt.Errorf("write GatewayRoute request: %w", err)
		}
	}

	// 7. Write status
	statusFields := map[string]interface{}{"namespace": config.Namespace}
	if config.IngressEnabled {
		statusFields["url"] = fmt.Sprintf("https://%s%s", config.IngressHostname, config.IngressPath)
	}
	if err := ku.WritePromiseStatus(sdk, ku.PhaseConfigured,
		fmt.Sprintf("HTTP Service %s configured", config.Name), statusFields); err != nil {
		return fmt.Errorf("write status: %w", err)
	}

	return nil
}

// handleDelete cleans up sub-ResourceRequests.
func handleDelete(sdk *kratix.KratixSDK, config *HTTPServiceConfig) error {
	// Delete ArgoCDApplication sub-ResourceRequest
	appRequest := ku.DeleteFromResource(ku.Resource{
		APIVersion: "platform.integratn.tech/v1alpha1",
		Kind:       "ArgoCDApplication",
		Metadata: ku.ObjectMeta{
			Name:      config.Name,
			Namespace: ku.DefaultPlatformRequestsNamespace,
		},
	})
	if err := ku.WriteYAML(sdk, "resources/delete-argocdapplication-"+config.Name+".yaml", appRequest); err != nil {
		return fmt.Errorf("write delete ArgoCDApplication request: %w", err)
	}

	// Delete PlatformExternalSecret sub-ResourceRequest
	if len(config.Secrets) > 0 {
		esRequest := ku.DeleteFromResource(ku.Resource{
			APIVersion: "platform.integratn.tech/v1alpha1",
			Kind:       "PlatformExternalSecret",
			Metadata: ku.ObjectMeta{
				Name:      fmt.Sprintf("%s-secrets", config.Name),
				Namespace: ku.DefaultPlatformRequestsNamespace,
			},
		})
		if err := ku.WriteYAML(sdk, "resources/delete-externalsecret-"+config.Name+".yaml", esRequest); err != nil {
			return fmt.Errorf("write delete PlatformExternalSecret request: %w", err)
		}
	}

	// Delete GatewayRoute sub-ResourceRequest
	if config.IngressEnabled {
		gwRequest := ku.DeleteFromResource(ku.Resource{
			APIVersion: "platform.integratn.tech/v1alpha1",
			Kind:       "GatewayRoute",
			Metadata: ku.ObjectMeta{
				Name:      fmt.Sprintf("%s-route", config.Name),
				Namespace: ku.DefaultPlatformRequestsNamespace,
			},
		})
		if err := ku.WriteYAML(sdk, "resources/delete-gatewayroute-"+config.Name+".yaml", gwRequest); err != nil {
			return fmt.Errorf("write delete GatewayRoute request: %w", err)
		}
	}

	if err := ku.WritePromiseStatus(sdk, ku.PhaseDeleting,
		fmt.Sprintf("HTTP Service %s scheduled for deletion", config.Name), nil); err != nil {
		return fmt.Errorf("write status: %w", err)
	}

	return nil
}
