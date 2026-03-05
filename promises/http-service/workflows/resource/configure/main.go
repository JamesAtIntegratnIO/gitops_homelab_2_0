package main

import (
	"fmt"
	"log"

	kratix "github.com/syntasso/kratix-go"

	u "github.com/jamesatintegratnio/gitops_homelab_2_0/promises/_shared/kratixutil"
)

// Platform-wide defaults — baked into every HTTP service.
const (
	defaultBaseDomain      = "cluster.integratn.tech"
	defaultGatewayName     = "nginx-gateway"
	defaultGatewayNS       = "nginx-gateway"
	defaultSecretStore     = "onepassword-store"
	defaultSecretStoreKind = "ClusterSecretStore"
	stakaterChartRepo      = "https://stakater.github.io/stakater-charts"
	stakaterChartName      = "application"
	stakaterChartVersion   = "6.16.1"
	argoCDProject          = "default"
)

// HTTPServiceConfig holds the fully resolved config from the CR.
type HTTPServiceConfig struct {
	Name      string
	Namespace string
	Team      string

	// Image
	ImageRepository string
	ImageTag        string
	ImagePullPolicy string
	Command         []string
	Args            []string

	// Scaling
	Replicas      int
	CPURequest    string
	MemoryRequest string
	CPULimit      string
	MemoryLimit   string

	// Networking
	Port            int
	IngressEnabled  bool
	IngressHostname string
	IngressPath     string

	// Secrets
	Secrets []u.SecretRef

	// Environment
	Env            map[string]string
	EnvFromSecrets []string

	// Health checks
	HealthCheckPath string
	HealthCheckPort int

	// Monitoring
	MonitoringEnabled  bool
	MonitoringPath     string
	MonitoringInterval string

	// Storage
	PersistenceEnabled   bool
	PersistenceSize      string
	PersistenceClass     string
	PersistenceMountPath string

	// Security
	RunAsNonRoot           *bool
	ReadOnlyRootFilesystem *bool
	RunAsUser              *int64
	RunAsGroup             *int64

	// Escape hatch
	HelmOverrides map[string]interface{}

	// Platform defaults
	BaseDomain      string
	GatewayName     string
	GatewayNS       string
	SecretStoreName string
	SecretStoreKind string
}

func main() {
	sdk := kratix.New()

	log.Printf("=== HTTP Service Promise Pipeline ===")
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

// buildConfig extracts all fields from the CR with sensible defaults.
func buildConfig(resource kratix.Resource) (*HTTPServiceConfig, error) {
	config := &HTTPServiceConfig{
		BaseDomain:      defaultBaseDomain,
		GatewayName:     defaultGatewayName,
		GatewayNS:       defaultGatewayNS,
		SecretStoreName: defaultSecretStore,
		SecretStoreKind: defaultSecretStoreKind,
	}

	var err error
	config.Name, err = u.GetStringValue(resource, "spec.name")
	if err != nil {
		return nil, fmt.Errorf("spec.name is required: %w", err)
	}

	config.Namespace = u.GetStringValueWithDefault(resource, "spec.namespace", config.Name)
	config.Team = u.GetStringValueWithDefault(resource, "spec.team", "platform")

	// Image
	config.ImageRepository, err = u.GetStringValue(resource, "spec.image.repository")
	if err != nil {
		return nil, fmt.Errorf("spec.image.repository is required: %w", err)
	}
	config.ImageTag = u.GetStringValueWithDefault(resource, "spec.image.tag", "latest")
	config.ImagePullPolicy = u.GetStringValueWithDefault(resource, "spec.image.pullPolicy", "IfNotPresent")
	config.Command = u.ExtractStringSlice(resource, "spec.command")
	config.Args = u.ExtractStringSlice(resource, "spec.args")

	// Scaling
	config.Replicas = u.GetIntValueWithDefault(resource, "spec.replicas", 1)
	config.CPURequest = u.GetStringValueWithDefault(resource, "spec.resources.requests.cpu", "100m")
	config.MemoryRequest = u.GetStringValueWithDefault(resource, "spec.resources.requests.memory", "128Mi")
	config.CPULimit = u.GetStringValueWithDefault(resource, "spec.resources.limits.cpu", "500m")
	config.MemoryLimit = u.GetStringValueWithDefault(resource, "spec.resources.limits.memory", "256Mi")

	// Networking
	config.Port = u.GetIntValueWithDefault(resource, "spec.port", 8080)
	config.IngressEnabled = u.GetBoolValueWithDefault(resource, "spec.ingress.enabled", true)
	config.IngressHostname, _ = u.GetStringValue(resource, "spec.ingress.hostname")
	if config.IngressHostname == "" {
		config.IngressHostname = fmt.Sprintf("%s.%s", config.Name, config.BaseDomain)
	}
	config.IngressPath = u.GetStringValueWithDefault(resource, "spec.ingress.path", "/")

	// Secrets
	config.Secrets = u.ExtractSecrets(resource, "spec.secrets")

	// Environment
	config.Env = u.ExtractStringMap(resource, "spec.env")
	config.EnvFromSecrets = u.ExtractStringSlice(resource, "spec.envFromSecrets")

	// Health checks
	config.HealthCheckPath = u.GetStringValueWithDefault(resource, "spec.healthCheck.path", "/")
	config.HealthCheckPort = u.GetIntValueWithDefault(resource, "spec.healthCheck.port", config.Port)

	// Monitoring
	config.MonitoringEnabled = u.GetBoolValueWithDefault(resource, "spec.monitoring.enabled", false)
	config.MonitoringPath = u.GetStringValueWithDefault(resource, "spec.monitoring.path", "/metrics")
	config.MonitoringInterval = u.GetStringValueWithDefault(resource, "spec.monitoring.interval", "30s")

	// Storage
	config.PersistenceEnabled = u.GetBoolValueWithDefault(resource, "spec.persistence.enabled", false)
	config.PersistenceSize = u.GetStringValueWithDefault(resource, "spec.persistence.size", "1Gi")
	config.PersistenceClass, _ = u.GetStringValue(resource, "spec.persistence.storageClass")
	config.PersistenceMountPath = u.GetStringValueWithDefault(resource, "spec.persistence.mountPath", "/data")

	// Security context
	if v, err := u.GetBoolValue(resource, "spec.securityContext.runAsNonRoot"); err == nil {
		config.RunAsNonRoot = &v
	}
	if v, err := u.GetBoolValue(resource, "spec.securityContext.readOnlyRootFilesystem"); err == nil {
		config.ReadOnlyRootFilesystem = &v
	}
	if v, err := u.GetIntValue(resource, "spec.securityContext.runAsUser"); err == nil {
		v64 := int64(v)
		config.RunAsUser = &v64
	}
	if v, err := u.GetIntValue(resource, "spec.securityContext.runAsGroup"); err == nil {
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
	ns := u.Resource{
		APIVersion: "v1",
		Kind:       "Namespace",
		Metadata: u.ObjectMeta{
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
	if err := u.WriteYAML(sdk, "resources/namespace.yaml", ns); err != nil {
		return fmt.Errorf("write Namespace: %w", err)
	}
	log.Printf("✓ Rendered Namespace: %s", config.Namespace)

	// 1. Build Stakater application chart values
	values := buildStakaterValues(config)

	// 2. Deep-merge any helmOverrides on top
	if config.HelmOverrides != nil {
		values = u.DeepMerge(values, config.HelmOverrides)
	}

	// 3. Build ArgoCDApplication sub-ResourceRequest (delegates to the argocd-application promise)
	appLabels := map[string]string{
		"app.kubernetes.io/managed-by": "kratix",
		"kratix.io/promise-name":       "http-service",
		"app.kubernetes.io/part-of":    config.Name,
		"app.kubernetes.io/team":       config.Team,
	}

	appRequest := u.Resource{
		APIVersion: "platform.integratn.tech/v1alpha1",
		Kind:       "ArgoCDApplication",
		Metadata: u.ObjectMeta{
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
		Spec: u.ArgoCDApplicationSpec{
			Name:      config.Name,
			Namespace: "argocd",
			Annotations: map[string]string{
				"argocd.argoproj.io/sync-wave": "10",
			},
			Labels:     appLabels,
			Finalizers: []string{"resources-finalizer.argocd.argoproj.io"},
			Project:    argoCDProject,
			Source: u.AppSource{
				RepoURL:        stakaterChartRepo,
				Chart:          stakaterChartName,
				TargetRevision: stakaterChartVersion,
				Helm: &u.HelmSource{
					ReleaseName:  config.Name,
					ValuesObject: values,
				},
			},
			Destination: u.Destination{
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

	if err := u.WriteYAML(sdk, "resources/argocd-application-request.yaml", appRequest); err != nil {
		return fmt.Errorf("write ArgoCDApplication request: %w", err)
	}
	log.Printf("✓ Rendered ArgoCDApplication sub-ResourceRequest: %s", config.Name)

	// 4. Emit PlatformExternalSecret sub-ResourceRequest (delegates to external-secret promise)
	if len(config.Secrets) > 0 {
		esRequest := buildExternalSecretRequest(config)
		if err := u.WriteYAML(sdk, "resources/external-secret-request.yaml", esRequest); err != nil {
			return fmt.Errorf("write PlatformExternalSecret request: %w", err)
		}
		log.Printf("✓ Rendered PlatformExternalSecret sub-ResourceRequest (%d secret(s))", len(config.Secrets))
	}

	// 5. Build NetworkPolicies (remain inline — too variable for a sub-promise)
	netpols := buildNetworkPolicies(config)
	if err := u.WriteYAMLDocuments(sdk, "resources/network-policies.yaml", netpols); err != nil {
		return fmt.Errorf("write NetworkPolicies: %w", err)
	}
	log.Printf("✓ Rendered NetworkPolicies")

	// 6. Emit GatewayRoute sub-ResourceRequest (delegates to gateway-route promise)
	if config.IngressEnabled {
		gwRequest := buildGatewayRouteRequest(config)
		if err := u.WriteYAML(sdk, "resources/gateway-route-request.yaml", gwRequest); err != nil {
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
	appRequest := u.Resource{
		APIVersion: "platform.integratn.tech/v1alpha1",
		Kind:       "ArgoCDApplication",
		Metadata: u.ObjectMeta{
			Name:      config.Name,
			Namespace: "platform-requests",
		},
	}
	if err := u.WriteYAML(sdk, "resources/delete-argocdapplication-"+config.Name+".yaml", appRequest); err != nil {
		return fmt.Errorf("write delete ArgoCDApplication request: %w", err)
	}
	log.Printf("✓ Delete scheduled for ArgoCDApplication: %s", config.Name)

	// Delete PlatformExternalSecret sub-ResourceRequest
	if len(config.Secrets) > 0 {
		esRequest := u.Resource{
			APIVersion: "platform.integratn.tech/v1alpha1",
			Kind:       "PlatformExternalSecret",
			Metadata: u.ObjectMeta{
				Name:      fmt.Sprintf("%s-secrets", config.Name),
				Namespace: "platform-requests",
			},
		}
		if err := u.WriteYAML(sdk, "resources/delete-externalsecret-"+config.Name+".yaml", esRequest); err != nil {
			return fmt.Errorf("write delete PlatformExternalSecret request: %w", err)
		}
		log.Printf("✓ Delete scheduled for PlatformExternalSecret: %s", config.Name)
	}

	// Delete GatewayRoute sub-ResourceRequest
	if config.IngressEnabled {
		gwRequest := u.Resource{
			APIVersion: "platform.integratn.tech/v1alpha1",
			Kind:       "GatewayRoute",
			Metadata: u.ObjectMeta{
				Name:      fmt.Sprintf("%s-route", config.Name),
				Namespace: "platform-requests",
			},
		}
		if err := u.WriteYAML(sdk, "resources/delete-gatewayroute-"+config.Name+".yaml", gwRequest); err != nil {
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
