package main

import (
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"

	kratix "github.com/syntasso/kratix-go"
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
	config.Name, err = getStringValue(resource, "spec.name")
	if err != nil {
		return nil, fmt.Errorf("spec.name not found: %w", err)
	}

	config.TargetNamespace, _ = getStringValue(resource, "spec.targetNamespace")
	if config.TargetNamespace == "" {
		config.TargetNamespace = config.Namespace
	}

	config.ProjectName, _ = getStringValue(resource, "spec.projectName")
	if config.ProjectName == "" {
		config.ProjectName = "vcluster-" + config.Name
	}

	// Extract vcluster spec
	config.K8sVersion, _ = getStringValueWithDefault(resource, "spec.vcluster.k8sVersion", "v1.34.3")
	config.Preset, _ = getStringValueWithDefault(resource, "spec.vcluster.preset", "dev")
	config.IsolationMode, _ = getStringValueWithDefault(resource, "spec.vcluster.isolationMode", "standard")
	config.ClusterDomain, _ = getStringValueWithDefault(resource, "spec.vcluster.networking.clusterDomain", "cluster.local")
	config.PersistenceClass, _ = getStringValue(resource, "spec.vcluster.persistence.storageClass")

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
	config.Hostname, _ = getStringValue(resource, "spec.exposure.hostname")
	config.Subnet, _ = getStringValue(resource, "spec.exposure.subnet")
	config.VIP, _ = getStringValue(resource, "spec.exposure.vip")
	config.APIPort, _ = getIntValueWithDefault(resource, "spec.exposure.apiPort", 443)

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
	config.BaseDomain, _ = getStringValue(resource, "metadata.annotations.platform\\.integratn\\.tech/base-domain")
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
		config.ExportKubeConfig = mergeMaps(defaultExport, config.ExportKubeConfig)
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
	config.CertManagerIssuerLabels = extractLabels(resource, "spec.integrations.certManager.clusterIssuerSelectorLabels")
	if len(config.CertManagerIssuerLabels) == 0 {
		config.CertManagerIssuerLabels = map[string]string{"integratn.tech/cluster-issuer": "letsencrypt-prod"}
	}

	config.ExternalSecretsStoreLabels = extractLabels(resource, "spec.integrations.externalSecrets.clusterStoreSelectorLabels")
	if len(config.ExternalSecretsStoreLabels) == 0 {
		config.ExternalSecretsStoreLabels = map[string]string{"integratn.tech/cluster-secret-store": "onepassword-store"}
	}

	config.ArgoCDEnvironment, _ = getStringValue(resource, "spec.integrations.argocd.environment")
	if config.ArgoCDEnvironment == "" {
		if config.Preset == "prod" {
			config.ArgoCDEnvironment = "production"
		} else {
			config.ArgoCDEnvironment = "development"
		}
	}

	config.ArgoCDClusterLabels = extractLabels(resource, "spec.integrations.argocd.clusterLabels")
	config.ArgoCDClusterAnnotations = extractLabels(resource, "spec.integrations.argocd.clusterAnnotations")

	config.WorkloadRepoURL, _ = getStringValueWithDefault(resource, "spec.integrations.argocd.workloadRepo.url", "https://github.com/jamesatintegratnio/gitops_homelab_2_0")
	config.WorkloadRepoBasePath, _ = getStringValue(resource, "spec.integrations.argocd.workloadRepo.basePath")
	config.WorkloadRepoPath, _ = getStringValueWithDefault(resource, "spec.integrations.argocd.workloadRepo.path", "workloads")
	config.WorkloadRepoRevision, _ = getStringValueWithDefault(resource, "spec.integrations.argocd.workloadRepo.revision", "main")

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
		"enable_metrics_server":           "true",
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
	config.ArgoCDRepoURL, _ = getStringValueWithDefault(resource, "spec.argocdApplication.repoURL", "https://charts.loft.sh")
	config.ArgoCDChart, _ = getStringValueWithDefault(resource, "spec.argocdApplication.chart", "vcluster")
	config.ArgoCDTargetRevision, _ = getStringValueWithDefault(resource, "spec.argocdApplication.targetRevision", "0.30.4")
	config.ArgoCDDestServer, _ = getStringValueWithDefault(resource, "spec.argocdApplication.destinationServer", "https://kubernetes.default.svc")

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
		config.ArgoCDSyncPolicy = mergeMaps(defaultSyncPolicy, config.ArgoCDSyncPolicy)
	}

	// Extract network policy configuration
	if val, err := getBoolValue(resource, "spec.networkPolicies.enableNFS"); err == nil {
		config.EnableNFS = val
	}
	config.ExtraEgress = extractExtraEgress(resource)

	// Set derived values
	config.OnePasswordItem = fmt.Sprintf("vcluster-%s-kubeconfig", config.Name)
	
	// Generate unique job name with reconcile token if present
	reconcileAt, _ := getStringValue(resource, "metadata.annotations.platform\\.integratn\\.tech/reconcile-at")
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

	config.ValuesObject = buildValuesObject(config)

	return config, nil
}

func buildValuesObject(config *VClusterConfig) map[string]interface{} {
	cp := ControlPlane{
		Distro: DistroConfig{
			K8s: K8sDistro{
				Enabled: true,
				Version: config.K8sVersion,
			},
		},
		ServiceMonitor: ServiceMonitor{
			Enabled: true,
			Labels: map[string]string{
				"vcluster_name":      config.Name,
				"vcluster_namespace": config.TargetNamespace,
				"environment":        config.ArgoCDEnvironment,
				"cluster_role":       "vcluster",
			},
		},
		StatefulSet: StatefulSetConfig{
			HighAvailability: HAConfig{Replicas: config.Replicas},
			Scheduling: SchedulingConfig{
				PodManagementPolicy: "Parallel",
				PriorityClassName:   "system-cluster-critical",
			},
			ImagePullPolicy: "Always",
			Image:           ImageConfig{Repository: "loft-sh/vcluster-oss"},
			Persistence: PersistenceConfig{
				VolumeClaim: VolumeClaimConfig{
					Enabled: config.PersistenceEnabled,
					Size:    config.PersistenceSize,
				},
			},
			Resources: ResourcesConfig{
				Requests: ResourceValues{CPU: config.CPURequest, Memory: config.MemoryRequest},
				Limits:   ResourceValues{CPU: config.CPULimit, Memory: config.MemoryLimit},
			},
		},
		CoreDNS: CoreDNSConfig{
			Enabled: true,
			Deployment: DeploymentConfig{Replicas: config.CorednsReplicas},
			OverwriteConfig: fmt.Sprintf(`.:1053 {
  errors
  health
  ready
  kubernetes %s in-addr.arpa ip6.arpa {
    pods insecure
    fallthrough in-addr.arpa ip6.arpa
    ttl 30
  }
  prometheus 0.0.0.0:9153
  forward . /etc/resolv.conf
  cache 30
  loop
  reload
  loadbalance
}`,
				config.ClusterDomain,
			),
		},
		Ingress: EnabledFlag{Enabled: false},
		Advanced: AdvancedConfig{
			PodDisruptionBudget: PDBConfig{Enabled: true, MinAvailable: 1},
		},
		Service: ServiceConfig{
			Enabled: true,
			Annotations: map[string]string{
				"external-dns.alpha.kubernetes.io/hostname": config.Hostname,
			},
			Spec: ServiceSpecConfig{
				Type: "LoadBalancer",
				Ports: []ServicePort{
					{
						Name:       "https",
						Port:       config.APIPort,
						TargetPort: 8443,
						Protocol:   "TCP",
					},
				},
			},
		},
	}

	if config.PersistenceClass != "" {
		cp.StatefulSet.Persistence.VolumeClaim.StorageClass = config.PersistenceClass
	}

	if config.BackingStore != nil {
		cp.BackingStore = config.BackingStore
	}

	if len(config.ProxyExtraSANs) > 0 {
		cp.Proxy = &ProxyConfig{ExtraSANs: config.ProxyExtraSANs}
	}

	if config.VIP != "" {
		cp.Service.Spec.LoadBalancerIP = config.VIP
	}

	if config.APIPort != 443 {
		cp.Service.Spec.Ports = append(cp.Service.Spec.Ports, ServicePort{
			Name:       "https-internal",
			Port:       443,
			TargetPort: 8443,
			Protocol:   "TCP",
		})
	}

	values := VClusterValues{
		ControlPlane: cp,
		Deploy: DeployConfig{
			MetalLB: EnabledFlag{Enabled: true},
		},
		Integrations: Integrations{
			ExternalSecrets: IntegrationExternalSecrets{
				Enabled: true,
				Webhook: EnabledFlag{Enabled: true},
				Sync: ESSyncConfig{
					FromHost: ESFromHostConfig{
						ClusterStores: ClusterStoresConfig{
							Enabled: true,
							Selector: LabelSelector{
								MatchLabels: config.ExternalSecretsStoreLabels,
							},
						},
					},
				},
			},
			MetricsServer: EnabledFlag{Enabled: true},
			CertManager: IntegrationCertManager{
				Enabled: true,
				Sync: CMSyncConfig{
					FromHost: CMFromHostConfig{
						ClusterIssuers: ClusterIssuersConfig{
							Enabled: true,
							Selector: LabelSelector{
								Labels: config.CertManagerIssuerLabels,
							},
						},
					},
				},
			},
		},
		Telemetry: EnabledFlag{Enabled: false},
		Logging:   LoggingConfig{Encoding: "json"},
		Networking: NetworkingConfig{
			Advanced: NetworkAdvanced{ClusterDomain: config.ClusterDomain},
			ReplicateServices: ReplicateServices{
				FromHost: []ServiceMapping{
					{From: "default/kubernetes", To: "default/kubernetes"},
				},
			},
		},
		Sync: SyncConfig{
			ToHost: SyncToHost{
				Pods:              EnabledFlag{Enabled: true},
				PersistentVolumes: EnabledFlag{Enabled: true},
				Ingresses:         EnabledFlag{Enabled: true},
				NetworkPolicies:   EnabledFlag{Enabled: false},
			},
			FromHost: SyncFromHost{
				StorageClasses: EnabledFlag{Enabled: true},
				IngressClasses: EnabledFlag{Enabled: true},
				Secrets: SecretSyncConfig{
					Enabled: true,
					Mappings: SecretMappings{
						ByName: map[string]string{
							"external-secrets/eso-onepassword-token": "external-secrets/eso-onepassword-token",
						},
					},
				},
			},
		},
		RBAC: RBACConfig{
			ClusterRole: ClusterRoleConfig{
				Enabled: true,
				ExtraRules: []PolicyRule{
					{
						APIGroups:     []string{""},
						Resources:     []string{"secrets"},
						Verbs:         []string{"get", "list", "watch"},
						ResourceNames: []string{"eso-onepassword-token"},
					},
				},
			},
		},
	}

	if len(config.ExportKubeConfig) > 0 {
		values.ExportKubeConfig = config.ExportKubeConfig
	}

	// Convert typed struct to map for merging with HelmOverrides
	valuesMap, err := toMap(values)
	if err != nil {
		log.Fatalf("ERROR: Failed to convert values to map: %v", err)
	}

	return mergeMaps(valuesMap, config.HelmOverrides)
}

func mergeMaps(dst, src map[string]interface{}) map[string]interface{} {
	if dst == nil {
		dst = map[string]interface{}{}
	}
	if src == nil {
		return dst
	}
	for key, value := range src {
		if srcMap, ok := value.(map[string]interface{}); ok {
			if dstMap, ok := dst[key].(map[string]interface{}); ok {
				dst[key] = mergeMaps(dstMap, srcMap)
				continue
			}
			dst[key] = mergeMaps(map[string]interface{}{}, srcMap)
			continue
		}
		dst[key] = value
	}
	return dst
}

func applyPresetDefaults(config *VClusterConfig, resource kratix.Resource) {
	presetDefaults := map[string]PresetDefaults{
		"dev": {
			Replicas:           1,
			CPURequest:         "200m",
			MemoryRequest:      "512Mi",
			CPULimit:           "1000m",
			MemoryLimit:        "1Gi",
			PersistenceEnabled: false,
			PersistenceSize:    "5Gi",
			CorednsReplicas:    1,
		},
		"prod": {
			Replicas:           3,
			CPURequest:         "500m",
			MemoryRequest:      "1Gi",
			CPULimit:           "2",
			MemoryLimit:        "2Gi",
			PersistenceEnabled: true,
			PersistenceSize:    "10Gi",
			CorednsReplicas:    2,
		},
	}

	defaults := presetDefaults[config.Preset]
	if defaults == (PresetDefaults{}) {
		defaults = presetDefaults["dev"]
	}

	// Apply replicas
	if val, err := getIntValue(resource, "spec.vcluster.replicas"); err == nil && val > 0 {
		config.Replicas = val
	} else {
		config.Replicas = defaults.Replicas
	}

	// Apply resource requests/limits
	config.CPURequest, _ = getStringValueWithDefault(resource, "spec.vcluster.resources.requests.cpu", defaults.CPURequest)
	config.MemoryRequest, _ = getStringValueWithDefault(resource, "spec.vcluster.resources.requests.memory", defaults.MemoryRequest)
	config.CPULimit, _ = getStringValueWithDefault(resource, "spec.vcluster.resources.limits.cpu", defaults.CPULimit)
	config.MemoryLimit, _ = getStringValueWithDefault(resource, "spec.vcluster.resources.limits.memory", defaults.MemoryLimit)

	// Apply persistence
	if val, err := getBoolValue(resource, "spec.vcluster.persistence.enabled"); err == nil {
		config.PersistenceEnabled = val
	} else {
		config.PersistenceEnabled = defaults.PersistenceEnabled
	}

	config.PersistenceSize, _ = getStringValueWithDefault(resource, "spec.vcluster.persistence.size", defaults.PersistenceSize)

	// Apply coredns replicas
	if val, err := getIntValue(resource, "spec.vcluster.coredns.replicas"); err == nil && val > 0 {
		config.CorednsReplicas = val
	} else {
		config.CorednsReplicas = defaults.CorednsReplicas
	}
}

func handleConfigure(sdk *kratix.KratixSDK, config *VClusterConfig) error {
	log.Println("--- Rendering orchestrator resources ---")

	resourceRequests := map[string]Resource{
		"resources/argocd-project-request.yaml":              buildArgoCDProjectRequest(config),
		"resources/argocd-application-request.yaml":          buildArgoCDApplicationRequest(config),
		"resources/argocd-cluster-registration-request.yaml": buildArgoCDClusterRegistrationRequest(config),
	}

	for path, obj := range resourceRequests {
		if err := writeYAML(sdk, path, obj); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
		log.Printf("✓ Rendered: %s", path)
	}

	if err := writeYAML(sdk, "resources/namespace.yaml", buildNamespace(config)); err != nil {
		return fmt.Errorf("write namespace: %w", err)
	}
	log.Printf("✓ Rendered: %s", "resources/namespace.yaml")

	if docs := buildEtcdCertificates(config); len(docs) > 0 {
		if err := writeYAMLDocuments(sdk, "resources/etcd-certificates.yaml", docs); err != nil {
			return fmt.Errorf("write etcd certificates: %w", err)
		}
		log.Printf("✓ Rendered: %s", "resources/etcd-certificates.yaml")
	}

	if err := writeYAML(sdk, "resources/coredns-configmap.yaml", buildCorednsConfigMap(config)); err != nil {
		return fmt.Errorf("write coredns configmap: %w", err)
	}
	log.Printf("✓ Rendered: %s", "resources/coredns-configmap.yaml")

	// Per-vcluster network policies (NFS, extra egress)
	netPolicies := buildNetworkPolicies(config)
	if len(netPolicies) > 0 {
		if err := writeYAMLDocuments(sdk, "resources/network-policies.yaml", netPolicies); err != nil {
			return fmt.Errorf("write network policies: %w", err)
		}
		log.Printf("✓ Rendered: resources/network-policies.yaml (%d policies)", len(netPolicies))
	}

	directResources := 2 // namespace + coredns configmap
	if etcdEnabled(config) {
		directResources++
	}
	if len(netPolicies) > 0 {
		directResources++
	}

	status := kratix.NewStatus()
	status.Set("phase", "Scheduled")
	status.Set("message", "VCluster resources scheduled for creation")
	status.Set("resourceRequestsGenerated", len(resourceRequests))
	status.Set("directResourcesGenerated", directResources)
	status.Set("vclusterName", config.Name)
	status.Set("targetNamespace", config.TargetNamespace)
	status.Set("hostname", config.Hostname)
	status.Set("environment", config.ArgoCDEnvironment)

	if err := sdk.WriteStatus(status); err != nil {
		return fmt.Errorf("failed to write status: %w", err)
	}

	log.Println("✓ Status updated")
	return nil
}

func handleDelete(sdk *kratix.KratixSDK, config *VClusterConfig) error {
	log.Printf("--- Handling delete for vcluster: %s ---", config.Name)

	status := kratix.NewStatus()
	status.Set("phase", "Deleting")
	status.Set("message", "VCluster resources scheduled for deletion")
	status.Set("vclusterName", config.Name)

	if err := sdk.WriteStatus(status); err != nil {
		return fmt.Errorf("failed to write status: %w", err)
	}

	outputs := map[string]Resource{}

	// Delete all created resources
	allResources := []Resource{
		buildArgoCDProjectRequest(config),
		buildArgoCDApplicationRequest(config),
		buildArgoCDClusterRegistrationRequest(config),
		buildCorednsConfigMap(config),
	}

	for _, obj := range allResources {
		deleteObj := deleteFromResource(obj)
		path := deleteOutputPathForResource("resources", obj)
		outputs[path] = deleteObj
	}

	// Delete per-vcluster network policies
	for _, obj := range buildNetworkPolicies(config) {
		deleteObj := deleteFromResource(obj)
		path := deleteOutputPathForResource("resources", obj)
		outputs[path] = deleteObj
	}

	if etcdEnabled(config) {
		for _, obj := range buildEtcdCertificates(config) {
			deleteObj := deleteFromResource(obj)
			path := deleteOutputPathForResource("resources", obj)
			outputs[path] = deleteObj
		}
	}

	outputs["resources/delete-vcluster-clusterrole.yaml"] = deleteResource(
		"rbac.authorization.k8s.io/v1",
		"ClusterRole",
		fmt.Sprintf("vc-%s-v-%s", config.Name, config.TargetNamespace),
		"",
	)
	outputs["resources/delete-vcluster-clusterrolebinding.yaml"] = deleteResource(
		"rbac.authorization.k8s.io/v1",
		"ClusterRoleBinding",
		fmt.Sprintf("vc-%s-v-%s", config.Name, config.TargetNamespace),
		"",
	)

	if etcdEnabled(config) {
		outputs["resources/delete-etcd-ca-secret.yaml"] = deleteResource(
			"v1",
			"Secret",
			fmt.Sprintf("%s-etcd-ca", config.Name),
			config.TargetNamespace,
		)
		outputs["resources/delete-etcd-server-secret.yaml"] = deleteResource(
			"v1",
			"Secret",
			fmt.Sprintf("%s-etcd-server", config.Name),
			config.TargetNamespace,
		)
		outputs["resources/delete-etcd-peer-secret.yaml"] = deleteResource(
			"v1",
			"Secret",
			fmt.Sprintf("%s-etcd-peer", config.Name),
			config.TargetNamespace,
		)
		outputs["resources/delete-etcd-merged-secret.yaml"] = deleteResource(
			"v1",
			"Secret",
			fmt.Sprintf("%s-etcd-certs", config.Name),
			config.TargetNamespace,
		)
	}

	for path, obj := range outputs {
		if err := writeYAML(sdk, path, obj); err != nil {
			return fmt.Errorf("write delete output %s: %w", path, err)
		}
	}

	return nil
}

// Helper functions
func getStringValue(resource kratix.Resource, path string) (string, error) {
	val, err := resource.GetValue(path)
	if err != nil {
		return "", err
	}
	if str, ok := val.(string); ok {
		return str, nil
	}
	return "", fmt.Errorf("%s is not a string", path)
}

func getStringValueWithDefault(resource kratix.Resource, path, defaultValue string) (string, error) {
	val, err := getStringValue(resource, path)
	if err != nil || val == "" || val == "null" {
		return defaultValue, nil
	}
	return val, nil
}

func getIntValue(resource kratix.Resource, path string) (int, error) {
	val, err := resource.GetValue(path)
	if err != nil {
		return 0, err
	}
	switch v := val.(type) {
	case int:
		return v, nil
	case float64:
		return int(v), nil
	case string:
		return strconv.Atoi(v)
	}
	return 0, fmt.Errorf("%s is not an integer", path)
}

func getIntValueWithDefault(resource kratix.Resource, path string, defaultValue int) (int, error) {
	val, err := getIntValue(resource, path)
	if err != nil || val == 0 {
		return defaultValue, nil
	}
	return val, nil
}

func getBoolValue(resource kratix.Resource, path string) (bool, error) {
	val, err := resource.GetValue(path)
	if err != nil {
		return false, err
	}
	if b, ok := val.(bool); ok {
		return b, nil
	}
	return false, fmt.Errorf("%s is not a boolean", path)
}

func extractLabels(resource kratix.Resource, path string) map[string]string {
	val, err := resource.GetValue(path)
	if err != nil {
		return nil
	}

	labels := make(map[string]string)
	if m, ok := val.(map[string]interface{}); ok {
		for k, v := range m {
			if str, ok := v.(string); ok {
				labels[k] = str
			}
		}
	}

	return labels
}

// IP utility functions
func defaultVIPFromCIDR(cidr string, offset int) (string, error) {
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return "", fmt.Errorf("invalid CIDR: %w", err)
	}

	networkIP := ipNet.IP.Mask(ipNet.Mask)
	networkInt := ipToInt(networkIP)
	vipInt := networkInt + uint32(offset)
	vip := intToIP(vipInt)

	return vip.String(), nil
}

func ipInCIDR(ipStr, cidr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}

	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return false
	}

	return ipNet.Contains(ip)
}

func ipToInt(ip net.IP) uint32 {
	ip = ip.To4()
	if ip == nil {
		return 0
	}
	return uint32(ip[0])<<24 | uint32(ip[1])<<16 | uint32(ip[2])<<8 | uint32(ip[3])
}

func intToIP(n uint32) net.IP {
	return net.IPv4(byte(n>>24), byte(n>>16), byte(n>>8), byte(n))
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
