package main

import (
	"fmt"
	"log"
	"strings"

	kratix "github.com/syntasso/kratix-go"

	ku "github.com/jamesatintegratnio/gitops_homelab_2_0/promises/_shared/kratixutil"
)

func main() {
	ku.RunPromiseWithConfig("VCluster Orchestrator V2", buildConfig, handleConfigure, handleDelete)
}

func buildConfig(sdk *kratix.KratixSDK, resource kratix.Resource) (*VClusterConfig, error) {
	config := &VClusterConfig{
		Namespace:  resource.GetNamespace(),
		KubeClient: InClusterClientFactory{},
		WorkflowContext: WorkflowContext{
			WorkflowAction: sdk.WorkflowAction(),
			WorkflowType:   sdk.WorkflowType(),
			PromiseName:    sdk.PromiseName(),
			PipelineName:   sdk.PipelineName(),
		},
	}

	if err := extractBasicFields(config, resource); err != nil {
		return nil, err
	}
	if err := configureExposure(config, resource); err != nil {
		return nil, err
	}
	configureIntegrations(config, resource)
	if err := configureArgoCD(config, resource); err != nil {
		return nil, err
	}

	// Extract network policy configuration
	if val, err := ku.GetBoolValue(resource, "spec.networkPolicies.enableNFS"); err == nil {
		config.EnableNFS = val
	}
	config.ExtraEgress = extractExtraEgress(resource)

	// Set derived values
	config.OnePasswordItem = fmt.Sprintf("vcluster-%s-kubeconfig", config.Name)
	
	// Generate unique job name with reconcile token if present
	reconcileAt, _ := ku.GetStringValue(resource, "metadata.annotations.platform\\.integratn\\.tech/reconcile-at")
	if reconcileAt != "" {
		token := strings.Map(func(r rune) rune {
			if r >= '0' && r <= '9' {
				return r
			}
			return -1
		}, reconcileAt)
		if token != "" {
			config.KubeconfigSyncJobName = fmt.Sprintf("vcluster-%s-kubeconfig-sync-%s", config.Name, token)
		}
	}
	if config.KubeconfigSyncJobName == "" {
		config.KubeconfigSyncJobName = fmt.Sprintf("vcluster-%s-kubeconfig-sync", config.Name)
	}

	valuesObj, err := buildValuesObject(config)
	if err != nil {
		return nil, fmt.Errorf("failed to build values object: %w", err)
	}
	config.ValuesObject = valuesObj

	return config, nil
}

// extractBasicFields extracts name, namespace, project, vcluster spec fields from the resource
// and applies preset defaults.
func extractBasicFields(config *VClusterConfig, resource kratix.Resource) error {
	var err error
	config.Name, err = ku.GetStringValue(resource, "spec.name")
	if err != nil {
		return fmt.Errorf("spec.name not found: %w", err)
	}

	config.TargetNamespace, _ = ku.GetStringValue(resource, "spec.targetNamespace")
	if config.TargetNamespace == "" {
		config.TargetNamespace = config.Namespace
	}

	config.ProjectName, _ = ku.GetStringValue(resource, "spec.projectName")
	if config.ProjectName == "" {
		config.ProjectName = "vcluster-" + config.Name
	}

	// Extract vcluster spec
	config.K8sVersion = ku.GetStringValueWithDefault(resource, "spec.vcluster.k8sVersion", "v1.34.3")
	config.Preset = ku.GetStringValueWithDefault(resource, "spec.vcluster.preset", "dev")
	config.IsolationMode = ku.GetStringValueWithDefault(resource, "spec.vcluster.isolationMode", "standard")
	config.ClusterDomain = ku.GetStringValueWithDefault(resource, "spec.vcluster.networking.clusterDomain", "cluster.local")
	config.PersistenceClass, _ = ku.GetStringValue(resource, "spec.vcluster.persistence.storageClass")

	// Apply preset defaults
	applyPresetDefaults(config, resource)

	// Extract backing store and helm overrides
	if val, err := resource.GetValue("spec.vcluster.backingStore"); err == nil && val != nil {
		if m, ok := val.(map[string]interface{}); ok {
			config.BackingStore = m
		}
	}

	if val, err := resource.GetValue("spec.vcluster.exportKubeConfig"); err == nil && val != nil {
		if m, ok := val.(map[string]interface{}); ok {
			config.ExportKubeConfig = m
		}
	}

	if val, err := resource.GetValue("spec.vcluster.helmOverrides"); err == nil && val != nil {
		if m, ok := val.(map[string]interface{}); ok {
			config.HelmOverrides = m
		}
	}

	return nil
}

// configureExposure extracts and calculates exposure settings: hostname, subnet,
// VIP, apiPort, external server URL, exportKubeConfig merge, and proxy extraSANs.
func configureExposure(config *VClusterConfig, resource kratix.Resource) error {
	config.Hostname, _ = ku.GetStringValue(resource, "spec.exposure.hostname")
	config.Subnet, _ = ku.GetStringValue(resource, "spec.exposure.subnet")
	config.VIP, _ = ku.GetStringValue(resource, "spec.exposure.vip")
	config.APIPort = ku.GetIntValueWithDefault(resource, "spec.exposure.apiPort", 443)

	if err := calculateVIP(config); err != nil {
		return err
	}
	resolveHostname(config, resource)
	resolveExposureDefaults(config)
	return nil
}

// calculateVIP calculates a default VIP from the subnet CIDR if not explicitly
// set, and validates the VIP is within the subnet.
func calculateVIP(config *VClusterConfig) error {
	if config.Subnet != "" && config.VIP == "" {
		vip, err := defaultVIPFromCIDR(config.Subnet, ku.DefaultMetalLBPoolOffset)
		if err != nil {
			return fmt.Errorf("failed to calculate VIP: %w", err)
		}
		config.VIP = vip
		log.Printf("Calculated default VIP: %s", vip)
	}
	if config.VIP != "" && config.Subnet != "" {
		if !ipInCIDR(config.VIP, config.Subnet) {
			return fmt.Errorf("VIP %s is not within subnet %s", config.VIP, config.Subnet)
		}
	}
	return nil
}

// resolveHostname determines the hostname and baseDomain from the resource
// annotations, falling back to sensible defaults.
func resolveHostname(config *VClusterConfig, resource kratix.Resource) {
	config.BaseDomain, _ = ku.GetStringValue(resource, "metadata.annotations.platform\\.integratn\\.tech/base-domain")
	if config.BaseDomain == "" || config.BaseDomain == "null" {
		config.BaseDomain = ku.DefaultBaseDomain
	}
	if config.Hostname == "" {
		config.Hostname = fmt.Sprintf("%s.%s", config.Name, config.BaseDomain)
	}
	config.BaseDomainSanitized = strings.ReplaceAll(config.BaseDomain, ".", "-")
}

// resolveExposureDefaults derives the ExternalServerURL, merges ExportKubeConfig
// defaults, and populates ProxyExtraSANs from the resolved hostname and VIP.
func resolveExposureDefaults(config *VClusterConfig) {
	if config.Hostname != "" {
		config.ExternalServerURL = fmt.Sprintf("https://%s:%d", config.Hostname, config.APIPort)
	} else if config.VIP != "" {
		config.ExternalServerURL = fmt.Sprintf("https://%s:%d", config.VIP, config.APIPort)
	}

	defaultExport := map[string]interface{}{}
	if config.ExternalServerURL != "" {
		defaultExport["server"] = config.ExternalServerURL
	}
	if len(config.ExportKubeConfig) > 0 {
		config.ExportKubeConfig = ku.DeepMerge(defaultExport, config.ExportKubeConfig)
	} else if len(defaultExport) > 0 {
		config.ExportKubeConfig = defaultExport
	}

	if config.Hostname != "" {
		config.ProxyExtraSANs = append(config.ProxyExtraSANs, config.Hostname)
	}
	if config.VIP != "" {
		config.ProxyExtraSANs = append(config.ProxyExtraSANs, config.VIP)
	}
}

// configureIntegrations sets up cert-manager, external-secrets, and ArgoCD integration
// configuration including cluster labels, annotations, and workload repo settings.
func configureIntegrations(config *VClusterConfig, resource kratix.Resource) {
	configureCertManager(config, resource)
	configureExternalSecrets(config, resource)
	configureArgoCDEnvironment(config, resource)
	configureWorkloadRepo(config, resource)
	configureClusterMetadata(config)
}

// configureCertManager extracts cert-manager issuer selector labels with a default.
func configureCertManager(config *VClusterConfig, resource kratix.Resource) {
	config.CertManagerIssuerLabels = ku.ExtractStringMap(resource, "spec.integrations.certManager.clusterIssuerSelectorLabels")
	if len(config.CertManagerIssuerLabels) == 0 {
		config.CertManagerIssuerLabels = map[string]string{ku.DefaultCertManagerIssuerLabel: ku.DefaultCertManagerIssuer}
	}
}

// configureExternalSecrets extracts external-secrets store selector labels with a default.
func configureExternalSecrets(config *VClusterConfig, resource kratix.Resource) {
	config.ExternalSecretsStoreLabels = ku.ExtractStringMap(resource, "spec.integrations.externalSecrets.clusterStoreSelectorLabels")
	if len(config.ExternalSecretsStoreLabels) == 0 {
		config.ExternalSecretsStoreLabels = map[string]string{ku.DefaultExternalSecretsStoreLabel: ku.DefaultExternalSecretsStore}
	}
}

// configureArgoCDEnvironment resolves the ArgoCD environment label from the resource
// or derives it from the vcluster preset.
func configureArgoCDEnvironment(config *VClusterConfig, resource kratix.Resource) {
	config.ArgoCDEnvironment, _ = ku.GetStringValue(resource, "spec.integrations.argocd.environment")
	if config.ArgoCDEnvironment == "" {
		if config.Preset == "prod" {
			config.ArgoCDEnvironment = "production"
		} else {
			config.ArgoCDEnvironment = "development"
		}
	}
}

// configureWorkloadRepo extracts ArgoCD workload repository settings and user-provided
// cluster labels/annotations from the resource.
func configureWorkloadRepo(config *VClusterConfig, resource kratix.Resource) {
	config.ArgoCDClusterLabels = ku.ExtractStringMap(resource, "spec.integrations.argocd.clusterLabels")
	config.ArgoCDClusterAnnotations = ku.ExtractStringMap(resource, "spec.integrations.argocd.clusterAnnotations")

	config.WorkloadRepoURL = ku.GetStringValueWithDefault(resource, "spec.integrations.argocd.workloadRepo.url", ku.PlatformRepoURL)
	config.WorkloadRepoBasePath, _ = ku.GetStringValue(resource, "spec.integrations.argocd.workloadRepo.basePath")
	config.WorkloadRepoPath = ku.GetStringValueWithDefault(resource, "spec.integrations.argocd.workloadRepo.path", "workloads")
	config.WorkloadRepoRevision = ku.GetStringValueWithDefault(resource, "spec.integrations.argocd.workloadRepo.revision", "main")
}

// configureClusterMetadata merges user-provided cluster labels/annotations with defaults
// to produce the final ArgoCD cluster secret metadata.
func configureClusterMetadata(config *VClusterConfig) {
	defaultClusterLabels := map[string]string{
		"argocd.argoproj.io/secret-type": "cluster",
		"akuity.io/argo-cd-cluster-name":  config.Name,
		"cluster_name":                    config.Name,
		"cluster_role":                    "vcluster",
		"cluster_type":                    "vcluster",
		"enable_argocd":                   "true",
		"enable_gateway_api_crds":         "true",
		"enable_nginx_gateway_fabric":     "true",
		"enable_cert_manager":             "true",
		"enable_external_secrets":         "true",
		"enable_external_dns":             "true",
		"environment":                     config.ArgoCDEnvironment,
	}
	defaultClusterAnnotations := map[string]string{
		"addons_repo_url":                            ku.PlatformRepoGitURL,
		"addons_repo_revision":                       "main",
		"addons_repo_basepath":                       "addons/",
		"addons_repo_path":                           "charts/application-sets",
		"managed-by":                                 "argocd.argoproj.io",
		"cert_manager_namespace":                     "cert-manager",
		"external_dns_namespace":                     "external-dns",
		"nfs_subdir_external_provisioner_namespace":   "nfs-provisioner",
		"cluster_name":                               config.Name,
		"environment":                                config.ArgoCDEnvironment,
		"platform.integratn.tech/base-domain":         config.BaseDomain,
		"platform.integratn.tech/base-domain-sanitized": config.BaseDomainSanitized,
		"workload_repo_url":                          config.WorkloadRepoURL,
		"workload_repo_basepath":                     config.WorkloadRepoBasePath,
		"workload_repo_path":                         config.WorkloadRepoPath,
		"workload_repo_revision":                     config.WorkloadRepoRevision,
	}

	config.ArgoCDClusterLabels = ku.MergeStringMap(defaultClusterLabels, config.ArgoCDClusterLabels)
	config.ArgoCDClusterAnnotations = ku.MergeStringMap(defaultClusterAnnotations, config.ArgoCDClusterAnnotations)
}

// configureArgoCD sets up ArgoCD application configuration including repo URL,
// chart, target revision, destination server, and sync policy with defaults.
func configureArgoCD(config *VClusterConfig, resource kratix.Resource) error {
	config.ArgoCDRepoURL = ku.GetStringValueWithDefault(resource, "spec.argocdApplication.repoURL", "https://charts.loft.sh")
	config.ArgoCDChart = ku.GetStringValueWithDefault(resource, "spec.argocdApplication.chart", "vcluster")
	config.ArgoCDTargetRevision = ku.GetStringValueWithDefault(resource, "spec.argocdApplication.targetRevision", "0.30.4")
	config.ArgoCDDestServer = ku.GetStringValueWithDefault(resource, "spec.argocdApplication.destinationServer", "https://kubernetes.default.svc")

	// Extract sync policy
	if val, err := resource.GetValue("spec.argocdApplication.syncPolicy"); err == nil && val != nil {
		config.ArgoCDSyncPolicy = ku.ParseSyncPolicy(val)
	}

	defaultSyncPolicy := &ku.SyncPolicy{
		Automated: &ku.AutomatedSync{
			SelfHeal: true,
			Prune:    true,
		},
		SyncOptions: []string{"CreateNamespace=true"},
	}
	if config.ArgoCDSyncPolicy == nil {
		config.ArgoCDSyncPolicy = defaultSyncPolicy
	} else {
		// Merge: user-provided values win over defaults
		if config.ArgoCDSyncPolicy.Automated == nil {
			config.ArgoCDSyncPolicy.Automated = defaultSyncPolicy.Automated
		}
		if len(config.ArgoCDSyncPolicy.SyncOptions) == 0 {
			config.ArgoCDSyncPolicy.SyncOptions = defaultSyncPolicy.SyncOptions
		}
	}

	return nil
}

func extractExtraEgress(resource kratix.Resource) []ExtraEgressRule {
	raw := ku.ExtractObjectSlice(resource, "spec.networkPolicies.extraEgress")
	if raw == nil {
		return nil
	}

	var rules []ExtraEgressRule
	for _, m := range raw {
		rule := ExtraEgressRule{
			Protocol: "TCP", // default
		}
		if v, ok := m["name"].(string); ok {
			rule.Name = v
		}
		if v, ok := m["cidr"].(string); ok {
			rule.CIDR = v
		}
		if v, ok := m["port"].(float64); ok {
			rule.Port = int(v)
		}
		if v, ok := m["protocol"].(string); ok && v != "" {
			rule.Protocol = v
		}
		if rule.Name != "" && rule.CIDR != "" && rule.Port > 0 {
			rules = append(rules, rule)
		}
	}
	return rules
}
