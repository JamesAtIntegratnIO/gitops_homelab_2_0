package main

import (
	"fmt"
	"log"
	"strings"

	kratix "github.com/syntasso/kratix-go"

	u "github.com/jamesatintegratnio/gitops_homelab_2_0/promises/_shared/kratixutil"
)

// VClusterConfig holds all configuration for template rendering
type VClusterConfig struct {
	// Basic identity
	Name            string
	Namespace       string
	ProjectName     string
	TargetNamespace string

	// Vcluster configuration
	K8sVersion          string
	Preset              string
	Replicas            int
	CPURequest          string
	MemoryRequest       string
	CPULimit            string
	MemoryLimit         string
	PersistenceEnabled  bool
	PersistenceSize     string
	PersistenceClass    string
	CorednsReplicas     int
	ClusterDomain       string
	IsolationMode       string
	BackingStore        map[string]interface{}
	ExportKubeConfig    map[string]interface{}
	HelmOverrides       map[string]interface{}
	ValuesObject        map[string]interface{}
	ProxyExtraSANs      []string

	// Exposure configuration
	Hostname         string
	VIP              string
	Subnet           string
	APIPort          int
	ExternalServerURL string

	// Integration configuration
	CertManagerIssuerLabels        map[string]string
	ExternalSecretsStoreLabels     map[string]string
	ArgoCDEnvironment              string
	ArgoCDClusterLabels            map[string]string
	ArgoCDClusterAnnotations       map[string]string
	WorkloadRepoURL                string
	WorkloadRepoBasePath           string
	WorkloadRepoPath               string
	WorkloadRepoRevision           string

	// ArgoCD Application configuration
	ArgoCDRepoURL        string
	ArgoCDChart          string
	ArgoCDTargetRevision string
	ArgoCDDestServer     string
	ArgoCDSyncPolicy     map[string]interface{}

	// Network policy configuration
	EnableNFS   bool
	ExtraEgress []ExtraEgressRule

	// Derived values
	OnePasswordItem     string
	KubeconfigSyncJobName string
	BaseDomain          string
	BaseDomainSanitized string
	
	WorkflowContext WorkflowContext
}

type WorkflowContext struct {
	WorkflowAction string
	WorkflowType   string
	PromiseName    string
	PipelineName   string
}

func main() {
	sdk := kratix.New()

	log.Printf("=== VCluster Orchestrator V2 Pipeline ===")
	log.Printf("Action: %s", sdk.WorkflowAction())
	log.Printf("Type: %s", sdk.WorkflowType())
	log.Printf("Promise: %s", sdk.PromiseName())

	resource, err := sdk.ReadResourceInput()
	if err != nil {
		log.Fatalf("ERROR: Failed to read resource input: %v", err)
	}

	log.Printf("Processing resource: %s in namespace: %s",
		resource.GetName(), resource.GetNamespace())

	config, err := buildConfig(sdk, resource)
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

func buildConfig(sdk *kratix.KratixSDK, resource kratix.Resource) (*VClusterConfig, error) {
	config := &VClusterConfig{
		Namespace: resource.GetNamespace(),
		WorkflowContext: WorkflowContext{
			WorkflowAction: sdk.WorkflowAction(),
			WorkflowType:   sdk.WorkflowType(),
			PromiseName:    sdk.PromiseName(),
			PipelineName:   sdk.PipelineName(),
		},
	}

	// Extract basic fields
	var err error
	config.Name, err = u.GetStringValue(resource, "spec.name")
	if err != nil {
		return nil, fmt.Errorf("spec.name not found: %w", err)
	}

	config.TargetNamespace, _ = u.GetStringValue(resource, "spec.targetNamespace")
	if config.TargetNamespace == "" {
		config.TargetNamespace = config.Namespace
	}

	config.ProjectName, _ = u.GetStringValue(resource, "spec.projectName")
	if config.ProjectName == "" {
		config.ProjectName = "vcluster-" + config.Name
	}

	// Extract vcluster spec
	config.K8sVersion = u.GetStringValueWithDefault(resource, "spec.vcluster.k8sVersion", "v1.34.3")
	config.Preset = u.GetStringValueWithDefault(resource, "spec.vcluster.preset", "dev")
	config.IsolationMode = u.GetStringValueWithDefault(resource, "spec.vcluster.isolationMode", "standard")
	config.ClusterDomain = u.GetStringValueWithDefault(resource, "spec.vcluster.networking.clusterDomain", "cluster.local")
	config.PersistenceClass, _ = u.GetStringValue(resource, "spec.vcluster.persistence.storageClass")

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

	// Extract exposure configuration
	config.Hostname, _ = u.GetStringValue(resource, "spec.exposure.hostname")
	config.Subnet, _ = u.GetStringValue(resource, "spec.exposure.subnet")
	config.VIP, _ = u.GetStringValue(resource, "spec.exposure.vip")
	config.APIPort = u.GetIntValueWithDefault(resource, "spec.exposure.apiPort", 443)

	// Calculate VIP if needed (offset 200 aligns with MetalLB pool 10.0.4.200-253)
	if config.Subnet != "" && config.VIP == "" {
		vip, err := defaultVIPFromCIDR(config.Subnet, 200)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate VIP: %w", err)
		}
		config.VIP = vip
		log.Printf("Calculated default VIP: %s", vip)
	}

	// Validate VIP is in subnet
	if config.VIP != "" && config.Subnet != "" {
		if !ipInCIDR(config.VIP, config.Subnet) {
			return nil, fmt.Errorf("VIP %s is not within subnet %s", config.VIP, config.Subnet)
		}
	}

	// Set hostname if not specified
	config.BaseDomain, _ = u.GetStringValue(resource, "metadata.annotations.platform\\.integratn\\.tech/base-domain")
	if config.BaseDomain == "" || config.BaseDomain == "null" {
		config.BaseDomain = "integratn.tech"
	}
	if config.Hostname == "" {
		config.Hostname = fmt.Sprintf("%s.%s", config.Name, config.BaseDomain)
	}
	config.BaseDomainSanitized = strings.ReplaceAll(config.BaseDomain, ".", "-")

	// Calculate external server URL
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
		config.ExportKubeConfig = u.DeepMerge(defaultExport, config.ExportKubeConfig)
	} else if len(defaultExport) > 0 {
		config.ExportKubeConfig = defaultExport
	}

	// Build proxy extraSANs
	if config.Hostname != "" {
		config.ProxyExtraSANs = append(config.ProxyExtraSANs, config.Hostname)
	}
	if config.VIP != "" {
		config.ProxyExtraSANs = append(config.ProxyExtraSANs, config.VIP)
	}

	// Extract integration configuration
	config.CertManagerIssuerLabels = u.ExtractStringMap(resource, "spec.integrations.certManager.clusterIssuerSelectorLabels")
	if len(config.CertManagerIssuerLabels) == 0 {
		config.CertManagerIssuerLabels = map[string]string{"integratn.tech/cluster-issuer": "letsencrypt-prod"}
	}

	config.ExternalSecretsStoreLabels = u.ExtractStringMap(resource, "spec.integrations.externalSecrets.clusterStoreSelectorLabels")
	if len(config.ExternalSecretsStoreLabels) == 0 {
		config.ExternalSecretsStoreLabels = map[string]string{"integratn.tech/cluster-secret-store": "onepassword-store"}
	}

	config.ArgoCDEnvironment, _ = u.GetStringValue(resource, "spec.integrations.argocd.environment")
	if config.ArgoCDEnvironment == "" {
		if config.Preset == "prod" {
			config.ArgoCDEnvironment = "production"
		} else {
			config.ArgoCDEnvironment = "development"
		}
	}

	config.ArgoCDClusterLabels = u.ExtractStringMap(resource, "spec.integrations.argocd.clusterLabels")
	config.ArgoCDClusterAnnotations = u.ExtractStringMap(resource, "spec.integrations.argocd.clusterAnnotations")

	config.WorkloadRepoURL = u.GetStringValueWithDefault(resource, "spec.integrations.argocd.workloadRepo.url", "https://github.com/jamesatintegratnio/gitops_homelab_2_0")
	config.WorkloadRepoBasePath, _ = u.GetStringValue(resource, "spec.integrations.argocd.workloadRepo.basePath")
	config.WorkloadRepoPath = u.GetStringValueWithDefault(resource, "spec.integrations.argocd.workloadRepo.path", "workloads")
	config.WorkloadRepoRevision = u.GetStringValueWithDefault(resource, "spec.integrations.argocd.workloadRepo.revision", "main")

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
		"addons_repo_url":                            "https://github.com/jamesatintegratnio/gitops_homelab_2_0.git",
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

	if len(config.ArgoCDClusterLabels) == 0 {
		config.ArgoCDClusterLabels = map[string]string{}
	}
	for key, value := range defaultClusterLabels {
		if _, exists := config.ArgoCDClusterLabels[key]; !exists {
			config.ArgoCDClusterLabels[key] = value
		}
	}

	if len(config.ArgoCDClusterAnnotations) == 0 {
		config.ArgoCDClusterAnnotations = map[string]string{}
	}
	for key, value := range defaultClusterAnnotations {
		if _, exists := config.ArgoCDClusterAnnotations[key]; !exists {
			config.ArgoCDClusterAnnotations[key] = value
		}
	}

	// Extract ArgoCD application configuration
	config.ArgoCDRepoURL = u.GetStringValueWithDefault(resource, "spec.argocdApplication.repoURL", "https://charts.loft.sh")
	config.ArgoCDChart = u.GetStringValueWithDefault(resource, "spec.argocdApplication.chart", "vcluster")
	config.ArgoCDTargetRevision = u.GetStringValueWithDefault(resource, "spec.argocdApplication.targetRevision", "0.30.4")
	config.ArgoCDDestServer = u.GetStringValueWithDefault(resource, "spec.argocdApplication.destinationServer", "https://kubernetes.default.svc")

	// Extract sync policy
	if val, err := resource.GetValue("spec.argocdApplication.syncPolicy"); err == nil && val != nil {
		if m, ok := val.(map[string]interface{}); ok {
			config.ArgoCDSyncPolicy = m
		}
	}

	defaultSyncPolicy := map[string]interface{}{
		"automated": map[string]interface{}{
			"selfHeal": true,
			"prune":    true,
		},
		"syncOptions": []string{"CreateNamespace=true"},
	}
	if config.ArgoCDSyncPolicy == nil {
		config.ArgoCDSyncPolicy = defaultSyncPolicy
	} else {
		config.ArgoCDSyncPolicy = u.DeepMerge(defaultSyncPolicy, config.ArgoCDSyncPolicy)
	}

	// Extract network policy configuration
	if val, err := u.GetBoolValue(resource, "spec.networkPolicies.enableNFS"); err == nil {
		config.EnableNFS = val
	}
	config.ExtraEgress = extractExtraEgress(resource)

	// Set derived values
	config.OnePasswordItem = fmt.Sprintf("vcluster-%s-kubeconfig", config.Name)
	
	// Generate unique job name with reconcile token if present
	reconcileAt, _ := u.GetStringValue(resource, "metadata.annotations.platform\\.integratn\\.tech/reconcile-at")
	if reconcileAt != "" {
		// Extract just numbers from reconcile-at
		token := ""
		for _, c := range reconcileAt {
			if c >= '0' && c <= '9' {
				token += string(c)
			}
		}
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

func extractExtraEgress(resource kratix.Resource) []ExtraEgressRule {
	val, err := resource.GetValue("spec.networkPolicies.extraEgress")
	if err != nil {
		return nil
	}
	arr, ok := val.([]interface{})
	if !ok {
		return nil
	}

	var rules []ExtraEgressRule
	for _, item := range arr {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
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
