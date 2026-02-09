package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"

	kratix "github.com/syntasso/kratix-go"
	"sigs.k8s.io/yaml"
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

	// Calculate VIP if needed
	if config.Subnet != "" && config.VIP == "" {
		vip, err := defaultVIPFromCIDR(config.Subnet, 100)
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
	controlPlane := map[string]interface{}{
		"distro": map[string]interface{}{
			"k8s": map[string]interface{}{
				"enabled": true,
				"version": config.K8sVersion,
			},
		},
		"serviceMonitor": map[string]interface{}{
			"enabled": true,
			"labels": map[string]interface{}{
				"vcluster_name":      config.Name,
				"vcluster_namespace": config.TargetNamespace,
				"environment":        config.ArgoCDEnvironment,
				"cluster_role":       "vcluster",
			},
		},
		"statefulSet": map[string]interface{}{
			"highAvailability": map[string]interface{}{
				"replicas": config.Replicas,
			},
			"scheduling": map[string]interface{}{
				"podManagementPolicy": "Parallel",
				"priorityClassName":   "system-cluster-critical",
			},
			"imagePullPolicy": "Always",
			"image": map[string]interface{}{
				"repository": "loft-sh/vcluster-oss",
			},
			"persistence": map[string]interface{}{
				"volumeClaim": map[string]interface{}{
					"enabled": config.PersistenceEnabled,
					"size":    config.PersistenceSize,
				},
			},
			"resources": map[string]interface{}{
				"requests": map[string]interface{}{
					"cpu":    config.CPURequest,
					"memory": config.MemoryRequest,
				},
				"limits": map[string]interface{}{
					"cpu":    config.CPULimit,
					"memory": config.MemoryLimit,
				},
			},
		},
		"coredns": map[string]interface{}{
			"enabled": true,
			"deployment": map[string]interface{}{
				"replicas": config.CorednsReplicas,
			},
			"overwriteConfig": fmt.Sprintf(`.:1053 {
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
		"ingress": map[string]interface{}{
			"enabled": false,
		},
		"service": map[string]interface{}{
			"enabled": true,
			"annotations": map[string]interface{}{
				"external-dns.alpha.kubernetes.io/hostname": config.Hostname,
			},
			"spec": map[string]interface{}{
				"type": "LoadBalancer",
				"ports": []map[string]interface{}{
					{
						"name":       "https",
						"port":       config.APIPort,
						"targetPort": 8443,
						"protocol":   "TCP",
					},
				},
			},
		},
	}

	if config.PersistenceClass != "" {
		volumeClaim := controlPlane["statefulSet"].(map[string]interface{})["persistence"].(map[string]interface{})["volumeClaim"].(map[string]interface{})
		volumeClaim["storageClass"] = config.PersistenceClass
	}

	if config.BackingStore != nil {
		controlPlane["backingStore"] = config.BackingStore
	}

	if len(config.ProxyExtraSANs) > 0 {
		controlPlane["proxy"] = map[string]interface{}{
			"extraSANs": config.ProxyExtraSANs,
		}
	}

	if config.VIP != "" {
		serviceSpec := controlPlane["service"].(map[string]interface{})["spec"].(map[string]interface{})
		serviceSpec["loadBalancerIP"] = config.VIP
	}

	if config.APIPort != 443 {
		serviceSpec := controlPlane["service"].(map[string]interface{})["spec"].(map[string]interface{})
		ports := serviceSpec["ports"].([]map[string]interface{})
		ports = append(ports, map[string]interface{}{
			"name":       "https-internal",
			"port":       443,
			"targetPort": 8443,
			"protocol":   "TCP",
		})
		serviceSpec["ports"] = ports
	}

	values := map[string]interface{}{
		"controlPlane": controlPlane,
		"deploy": map[string]interface{}{
			"metallb": map[string]interface{}{
				"enabled": true,
			},
		},
		"integrations": map[string]interface{}{
			"externalSecrets": map[string]interface{}{
				"enabled": true,
				"webhook": map[string]interface{}{
					"enabled": true,
				},
				"sync": map[string]interface{}{
					"fromHost": map[string]interface{}{
						"clusterStores": map[string]interface{}{
							"enabled": true,
							"selector": map[string]interface{}{
								"matchLabels": config.ExternalSecretsStoreLabels,
							},
						},
					},
				},
			},
			"metricsServer": map[string]interface{}{
				"enabled": true,
			},
			"certManager": map[string]interface{}{
				"enabled": true,
				"sync": map[string]interface{}{
					"fromHost": map[string]interface{}{
						"clusterIssuers": map[string]interface{}{
							"enabled": true,
							"selector": map[string]interface{}{
								"labels": config.CertManagerIssuerLabels,
							},
						},
					},
				},
			},
		},
		"telemetry": map[string]interface{}{
			"enabled": false,
		},
		"logging": map[string]interface{}{
			"encoding": "json",
		},
		"networking": map[string]interface{}{
			"advanced": map[string]interface{}{
				"clusterDomain": config.ClusterDomain,
			},
			"replicateServices": map[string]interface{}{
				"fromHost": []map[string]interface{}{
					{
						"from": "default/kubernetes",
						"to":   "default/kubernetes",
					},
				},
			},
		},
		"sync": map[string]interface{}{
			"toHost": map[string]interface{}{
				"pods": map[string]interface{}{
					"enabled": true,
				},
				"persistentVolumes": map[string]interface{}{
					"enabled": true,
				},
				"ingresses": map[string]interface{}{
					"enabled": true,
				},
				"networkPolicies": map[string]interface{}{
					"enabled": false,
				},
			},
			"fromHost": map[string]interface{}{
				"storageClasses": map[string]interface{}{
					"enabled": true,
				},
				"ingressClasses": map[string]interface{}{
					"enabled": true,
				},
				"secrets": map[string]interface{}{
					"enabled": true,
					"mappings": map[string]interface{}{
						"byName": map[string]interface{}{
							"external-secrets/eso-onepassword-token": "external-secrets/eso-onepassword-token",
						},
					},
				},
			},
		},
		"rbac": map[string]interface{}{
			"clusterRole": map[string]interface{}{
				"enabled": true,
				"extraRules": []map[string]interface{}{
					{
						"apiGroups": []string{""},
						"resources": []string{"secrets"},
						"verbs":     []string{"get", "list", "watch"},
						"resourceNames": []string{"eso-onepassword-token"},
					},
				},
			},
		},
	}

	if len(config.ExportKubeConfig) > 0 {
		values["exportKubeConfig"] = config.ExportKubeConfig
	}

	return mergeMaps(values, config.HelmOverrides)
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
	presetDefaults := map[string]map[string]interface{}{
		"dev": {
			"replicas":          1,
			"cpuRequest":        "200m",
			"memoryRequest":     "512Mi",
			"cpuLimit":          "1000m",
			"memoryLimit":       "1Gi",
			"persistenceEnabled": false,
			"persistenceSize":   "5Gi",
			"corednsReplicas":   1,
		},
		"prod": {
			"replicas":          3,
			"cpuRequest":        "500m",
			"memoryRequest":     "1Gi",
			"cpuLimit":          "2",
			"memoryLimit":       "2Gi",
			"persistenceEnabled": true,
			"persistenceSize":   "10Gi",
			"corednsReplicas":   2,
		},
	}

	defaults := presetDefaults[config.Preset]
	if defaults == nil {
		defaults = presetDefaults["dev"]
	}

	// Apply replicas
	if val, err := getIntValue(resource, "spec.vcluster.replicas"); err == nil && val > 0 {
		config.Replicas = val
	} else {
		config.Replicas = defaults["replicas"].(int)
	}

	// Apply resource requests/limits
	config.CPURequest, _ = getStringValueWithDefault(resource, "spec.vcluster.resources.requests.cpu", defaults["cpuRequest"].(string))
	config.MemoryRequest, _ = getStringValueWithDefault(resource, "spec.vcluster.resources.requests.memory", defaults["memoryRequest"].(string))
	config.CPULimit, _ = getStringValueWithDefault(resource, "spec.vcluster.resources.limits.cpu", defaults["cpuLimit"].(string))
	config.MemoryLimit, _ = getStringValueWithDefault(resource, "spec.vcluster.resources.limits.memory", defaults["memoryLimit"].(string))

	// Apply persistence
	if val, err := getBoolValue(resource, "spec.vcluster.persistence.enabled"); err == nil {
		config.PersistenceEnabled = val
	} else {
		config.PersistenceEnabled = defaults["persistenceEnabled"].(bool)
	}

	config.PersistenceSize, _ = getStringValueWithDefault(resource, "spec.vcluster.persistence.size", defaults["persistenceSize"].(string))

	// Apply coredns replicas
	if val, err := getIntValue(resource, "spec.vcluster.coredns.replicas"); err == nil && val > 0 {
		config.CorednsReplicas = val
	} else {
		config.CorednsReplicas = defaults["corednsReplicas"].(int)
	}
}

func handleConfigure(sdk *kratix.KratixSDK, config *VClusterConfig) error {
	log.Println("--- Rendering orchestrator resources ---")

	resourceRequests := map[string]interface{}{
		"resources/argocd-project-request.yaml":     buildArgoCDProjectRequest(config),
		"resources/argocd-application-request.yaml": buildArgoCDApplicationRequest(config),
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

	if err := writeYAMLDocuments(sdk, "resources/kubeconfig-sync-rbac.yaml", buildKubeconfigSyncRBAC(config)); err != nil {
		return fmt.Errorf("write kubeconfig sync rbac: %w", err)
	}
	log.Printf("✓ Rendered: %s", "resources/kubeconfig-sync-rbac.yaml")

	if err := writeYAML(sdk, "resources/kubeconfig-sync-job.yaml", buildKubeconfigSyncJob(config)); err != nil {
		return fmt.Errorf("write kubeconfig sync job: %w", err)
	}
	log.Printf("✓ Rendered: %s", "resources/kubeconfig-sync-job.yaml")

	if err := writeYAML(sdk, "resources/kubeconfig-external-secret.yaml", buildKubeconfigExternalSecret(config)); err != nil {
		return fmt.Errorf("write kubeconfig external secret: %w", err)
	}
	log.Printf("✓ Rendered: %s", "resources/kubeconfig-external-secret.yaml")

	if err := writeYAML(sdk, "resources/argocd-cluster-external-secret.yaml", buildArgoCDClusterExternalSecret(config)); err != nil {
		return fmt.Errorf("write argocd cluster external secret: %w", err)
	}
	log.Printf("✓ Rendered: %s", "resources/argocd-cluster-external-secret.yaml")

	directResources := 6
	if etcdEnabled(config) {
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

func writeYAML(sdk *kratix.KratixSDK, path string, obj interface{}) error {
	data, err := yaml.Marshal(obj)
	if err != nil {
		return fmt.Errorf("marshal %s: %w", path, err)
	}
	if err := sdk.WriteOutput(path, data); err != nil {
		return fmt.Errorf("write output %s: %w", path, err)
	}
	return nil
}

func writeYAMLDocuments(sdk *kratix.KratixSDK, path string, docs []interface{}) error {
	if len(docs) == 0 {
		return nil
	}

	var buf bytes.Buffer
	for i, doc := range docs {
		data, err := yaml.Marshal(doc)
		if err != nil {
			return fmt.Errorf("marshal %s: %w", path, err)
		}
		if i > 0 {
			buf.WriteString("---\n")
		}
		buf.Write(data)
	}

	if err := sdk.WriteOutput(path, buf.Bytes()); err != nil {
		return fmt.Errorf("write output %s: %w", path, err)
	}
	return nil
}

func etcdEnabled(config *VClusterConfig) bool {
	if config.BackingStore == nil {
		return false
	}
	etcd, ok := config.BackingStore["etcd"].(map[string]interface{})
	if !ok {
		return false
	}
	deploy, ok := etcd["deploy"].(map[string]interface{})
	if !ok {
		return false
	}
	enabled, ok := deploy["enabled"].(bool)
	return ok && enabled
}

func resourceMeta(name, namespace string, labels, annotations map[string]string) map[string]interface{} {
	meta := map[string]interface{}{
		"name": name,
	}
	if namespace != "" {
		meta["namespace"] = namespace
	}
	if len(labels) > 0 {
		meta["labels"] = labels
	}
	if len(annotations) > 0 {
		meta["annotations"] = annotations
	}
	return meta
}

func mergeStringMap(dst, src map[string]string) map[string]string {
	if dst == nil {
		dst = map[string]string{}
	}
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func baseLabels(config *VClusterConfig, name string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/managed-by": "kratix",
		"kratix.io/promise-name":       config.WorkflowContext.PromiseName,
		"kratix.io/resource-name":      name,
	}
}

func deleteResource(apiVersion, kind, name, namespace string) map[string]interface{} {
	meta := map[string]interface{}{
		"name": name,
	}
	if namespace != "" {
		meta["namespace"] = namespace
	}
	return map[string]interface{}{
		"apiVersion": apiVersion,
		"kind":       kind,
		"metadata":   meta,
	}
}

func buildArgoCDProjectRequest(config *VClusterConfig) map[string]interface{} {
	metadataLabels := mergeStringMap(map[string]string{
		"app.kubernetes.io/name": "argocd-project",
	}, baseLabels(config, config.Name))

	specLabels := map[string]string{
		"app.kubernetes.io/managed-by":     "kratix",
		"argocd.argoproj.io/project-group": "appteam",
		"kratix.io/promise-name":           config.WorkflowContext.PromiseName,
		"kratix.io/resource-name":          config.Name,
	}

	return map[string]interface{}{
		"apiVersion": "platform.integratn.tech/v1alpha1",
		"kind":       "ArgoCDProject",
		"metadata": resourceMeta(
			config.ProjectName,
			config.Namespace,
			metadataLabels,
			nil,
		),
		"spec": map[string]interface{}{
			"namespace":   "argocd",
			"name":        config.ProjectName,
			"description": fmt.Sprintf("VCluster project for %s", config.Name),
			"annotations": map[string]string{
				"argocd.argoproj.io/sync-wave": "-1",
			},
			"labels": specLabels,
			"sourceRepos": []string{
				"https://charts.loft.sh",
			},
			"destinations": []map[string]interface{}{
				{
					"namespace": config.TargetNamespace,
					"server":    "https://kubernetes.default.svc",
				},
			},
			"clusterResourceWhitelist": []map[string]interface{}{
				{
					"group": "*",
					"kind":  "*",
				},
			},
			"namespaceResourceWhitelist": []map[string]interface{}{
				{
					"group": "*",
					"kind":  "*",
				},
			},
		},
	}
}

func buildArgoCDApplicationRequest(config *VClusterConfig) map[string]interface{} {
	metadataLabels := mergeStringMap(map[string]string{
		"app.kubernetes.io/name": "argocd-application",
	}, baseLabels(config, config.Name))

	spec := map[string]interface{}{
		"name":      fmt.Sprintf("vcluster-%s", config.Name),
		"namespace": "argocd",
		"annotations": map[string]string{
			"argocd.argoproj.io/sync-wave": "0",
		},
		"finalizers": []string{"resources-finalizer.argocd.argoproj.io"},
		"project":    config.ProjectName,
		"destination": map[string]interface{}{
			"server":    config.ArgoCDDestServer,
			"namespace": config.TargetNamespace,
		},
		"source": map[string]interface{}{
			"repoURL":        config.ArgoCDRepoURL,
			"chart":          config.ArgoCDChart,
			"targetRevision": config.ArgoCDTargetRevision,
			"helm": map[string]interface{}{
				"releaseName":  config.Name,
				"valuesObject": config.ValuesObject,
			},
		},
	}
	if config.ArgoCDSyncPolicy != nil {
		spec["syncPolicy"] = config.ArgoCDSyncPolicy
	}

	return map[string]interface{}{
		"apiVersion": "platform.integratn.tech/v1alpha1",
		"kind":       "ArgoCDApplication",
		"metadata": resourceMeta(
			fmt.Sprintf("vcluster-%s", config.Name),
			config.Namespace,
			metadataLabels,
			nil,
		),
		"spec": spec,
	}
}

func buildNamespace(config *VClusterConfig) map[string]interface{} {
	labels := mergeStringMap(map[string]string{
		"app.kubernetes.io/name":     "vcluster-namespace",
		"vcluster.loft.sh/namespace": "true",
	}, baseLabels(config, config.Name))

	return map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Namespace",
		"metadata": resourceMeta(
			config.TargetNamespace,
			"",
			labels,
			map[string]string{"argocd.argoproj.io/sync-wave": "-2"},
		),
	}
}

func buildCorednsConfigMap(config *VClusterConfig) map[string]interface{} {
	labels := mergeStringMap(map[string]string{
		"app.kubernetes.io/name":     "coredns",
		"app.kubernetes.io/instance": fmt.Sprintf("vc-%s", config.Name),
	}, baseLabels(config, config.Name))

	corefile := fmt.Sprintf(`.:1053 {
    errors
    health
    ready
    kubernetes %s in-addr.arpa ip6.arpa {
        pods insecure
        fallthrough in-addr.arpa ip6.arpa
    }
    hosts /etc/coredns/NodeHosts {
        ttl 60
        reload 15s
        fallthrough
    }
    prometheus :9153
    forward . /etc/resolv.conf
    cache 30
    loop
    reload
    loadbalance
}

import /etc/coredns/custom/*.server
`, config.ClusterDomain)

	return map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": resourceMeta(
			fmt.Sprintf("vc-%s-coredns", config.Name),
			config.TargetNamespace,
			labels,
			nil,
		),
		"data": map[string]string{
			"Corefile":  corefile,
			"NodeHosts": "",
		},
	}
}

func buildKubeconfigExternalSecret(config *VClusterConfig) map[string]interface{} {
	labels := mergeStringMap(map[string]string{
		"app.kubernetes.io/name":      "external-secret",
		"app.kubernetes.io/component": "kubeconfig",
	}, baseLabels(config, config.Name))

	return map[string]interface{}{
		"apiVersion": "external-secrets.io/v1beta1",
		"kind":       "ExternalSecret",
		"metadata": resourceMeta(
			fmt.Sprintf("%s-kubeconfig", config.Name),
			config.TargetNamespace,
			labels,
			nil,
		),
		"spec": map[string]interface{}{
			"secretStoreRef": map[string]interface{}{
				"name": "onepassword-store",
				"kind": "ClusterSecretStore",
			},
			"target": map[string]interface{}{
				"name": fmt.Sprintf("vcluster-%s-kubeconfig-external", config.Name),
				"template": map[string]interface{}{
					"engineVersion": "v2",
					"data": map[string]string{
						"config": "{{ .kubeconfig }}\n",
					},
				},
			},
			"dataFrom": []map[string]interface{}{
				{
					"extract": map[string]interface{}{
						"key": config.OnePasswordItem,
					},
				},
			},
			"refreshInterval": "15m",
		},
	}
}

func buildArgoCDClusterExternalSecret(config *VClusterConfig) map[string]interface{} {
	labels := mergeStringMap(map[string]string{
		"app.kubernetes.io/name":        "external-secret",
		"app.kubernetes.io/component":   "argocd-cluster",
		"argocd.argoproj.io/secret-type": "cluster",
	}, baseLabels(config, config.Name))
	labels = mergeStringMap(labels, config.ArgoCDClusterLabels)

	metadataAnnotations := map[string]string{}
	if len(config.ArgoCDClusterAnnotations) > 0 {
		metadataAnnotations = mergeStringMap(metadataAnnotations, config.ArgoCDClusterAnnotations)
	}

	targetLabels := mergeStringMap(map[string]string{
		"argocd.argoproj.io/secret-type": "cluster",
		"integratn.tech/vcluster-name":  config.Name,
		"integratn.tech/environment":    config.ArgoCDEnvironment,
	}, config.ArgoCDClusterLabels)

	targetAnnotations := map[string]string{}
	if len(config.ArgoCDClusterAnnotations) > 0 {
		targetAnnotations = mergeStringMap(targetAnnotations, config.ArgoCDClusterAnnotations)
	}

	metadata := resourceMeta(
		fmt.Sprintf("%s-argocd-cluster", config.Name),
		"argocd",
		labels,
		metadataAnnotations,
	)

	targetMetadata := map[string]interface{}{
		"labels": targetLabels,
	}
	if len(targetAnnotations) > 0 {
		targetMetadata["annotations"] = targetAnnotations
	}

	return map[string]interface{}{
		"apiVersion": "external-secrets.io/v1beta1",
		"kind":       "ExternalSecret",
		"metadata":   metadata,
		"spec": map[string]interface{}{
			"secretStoreRef": map[string]interface{}{
				"name": "onepassword-store",
				"kind": "ClusterSecretStore",
			},
			"target": map[string]interface{}{
				"name": fmt.Sprintf("vcluster-%s", config.Name),
				"template": map[string]interface{}{
					"engineVersion": "v2",
					"type":         "Opaque",
					"metadata":     targetMetadata,
					"data": map[string]string{
						"name":   "{{ index . \"argocd-name\" }}",
						"server": "{{ index . \"argocd-server\" }}",
						"config": "{{ index . \"argocd-config\" }}",
					},
				},
			},
			"dataFrom": []map[string]interface{}{
				{
					"extract": map[string]interface{}{
						"key":                config.OnePasswordItem,
						"conversionStrategy": "Default",
						"decodingStrategy":   "None",
					},
				},
			},
			"refreshInterval": "15m",
		},
	}
}

func buildKubeconfigSyncRBAC(config *VClusterConfig) []interface{} {
	labels := mergeStringMap(map[string]string{
		"app.kubernetes.io/name":      "external-secret",
		"app.kubernetes.io/component": "kubeconfig-sync",
	}, baseLabels(config, config.Name))

	externalSecret := map[string]interface{}{
		"apiVersion": "external-secrets.io/v1beta1",
		"kind":       "ExternalSecret",
		"metadata": resourceMeta(
			fmt.Sprintf("%s-onepassword-token", config.Name),
			config.TargetNamespace,
			labels,
			nil,
		),
		"spec": map[string]interface{}{
			"secretStoreRef": map[string]interface{}{
				"name": "onepassword-store",
				"kind": "ClusterSecretStore",
			},
			"target": map[string]interface{}{
				"name": fmt.Sprintf("vcluster-%s-onepassword-token", config.Name),
			},
			"data": []map[string]interface{}{
				{
					"secretKey": "token",
					"remoteRef": map[string]interface{}{
						"key":      "onepassword-access-token",
						"property": "credential",
					},
				},
				{
					"secretKey": "vault",
					"remoteRef": map[string]interface{}{
						"key":      "onepassword-access-token",
						"property": "vault",
					},
				},
			},
		},
	}

	baseRBACLabels := mergeStringMap(map[string]string{
		"app.kubernetes.io/name": "kubeconfig-sync",
	}, baseLabels(config, config.Name))

	serviceAccount := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ServiceAccount",
		"metadata": resourceMeta(
			fmt.Sprintf("%s-kubeconfig-sync", config.Name),
			config.TargetNamespace,
			baseRBACLabels,
			nil,
		),
	}

	role := map[string]interface{}{
		"apiVersion": "rbac.authorization.k8s.io/v1",
		"kind":       "Role",
		"metadata": resourceMeta(
			fmt.Sprintf("%s-kubeconfig-sync", config.Name),
			config.TargetNamespace,
			baseRBACLabels,
			nil,
		),
		"rules": []map[string]interface{}{
			{
				"apiGroups":     []string{""},
				"resources":     []string{"secrets"},
				"resourceNames": []string{fmt.Sprintf("vc-%s", config.Name), fmt.Sprintf("vcluster-%s-onepassword-token", config.Name)},
				"verbs":         []string{"get"},
			},
		},
	}

	roleBinding := map[string]interface{}{
		"apiVersion": "rbac.authorization.k8s.io/v1",
		"kind":       "RoleBinding",
		"metadata": resourceMeta(
			fmt.Sprintf("%s-kubeconfig-sync", config.Name),
			config.TargetNamespace,
			baseRBACLabels,
			nil,
		),
		"roleRef": map[string]interface{}{
			"apiGroup": "rbac.authorization.k8s.io",
			"kind":     "Role",
			"name":     fmt.Sprintf("%s-kubeconfig-sync", config.Name),
		},
		"subjects": []map[string]interface{}{
			{
				"kind":      "ServiceAccount",
				"name":      fmt.Sprintf("%s-kubeconfig-sync", config.Name),
				"namespace": config.TargetNamespace,
			},
		},
	}

	return []interface{}{externalSecret, serviceAccount, role, roleBinding}
}

func buildKubeconfigSyncJob(config *VClusterConfig) map[string]interface{} {
	labels := mergeStringMap(map[string]string{
		"app.kubernetes.io/name": "kubeconfig-sync",
	}, baseLabels(config, config.Name))

	initCommand := fmt.Sprintf(`echo "Waiting for secret vc-%s to exist..."
until [ -f /kubeconfig/config ]; do
  echo "Kubeconfig not found yet, sleeping..."
  sleep 5
done
echo "Kubeconfig found!"`, config.Name)

	syncCommand := `set -e

apk add --no-cache curl jq >/dev/null 2>&1

echo "=== VCluster Kubeconfig Sync to 1Password ==="
echo "VCluster: $VCLUSTER_NAME"
echo "1Password Item: $OP_ITEM_NAME"
echo "Vault: $OP_VAULT"

# Read kubeconfig from secret
KUBECONFIG_CONTENT=$(cat /kubeconfig/config)

# Build ArgoCD cluster config
ARGOCD_CONFIG=$(cat <<EOF
{
  "tlsClientConfig": {
    "insecure": false
  }
}
EOF
)

# Check if item exists
echo "Checking if item exists..."
ITEM_SEARCH=$(curl -s -X POST "$OP_CONNECT_HOST/v1/vaults/homelab/items" \
  -H "Authorization: Bearer $OP_CONNECT_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"title\":\"$OP_ITEM_NAME\"}" || echo "{}")

ITEM_ID=$(echo "$ITEM_SEARCH" | grep -o '"id":"[^"]*' | head -1 | cut -d'"' -f4 || echo "")

if [ -z "$ITEM_ID" ]; then
  echo "Creating new 1Password item..."
  RESPONSE=$(curl -s -X POST "$OP_CONNECT_HOST/v1/vaults/homelab/items" \
    -H "Authorization: Bearer $OP_CONNECT_TOKEN" \
    -H "Content-Type: application/json" \
    -d "{
      \"title\": \"$OP_ITEM_NAME\",
      \"category\": \"SERVER\",
      \"tags\": [\"vcluster\", \"kubeconfig\", \"$ARGOCD_ENVIRONMENT\"],
      \"fields\": [
        {
          \"id\": \"kubeconfig\",
          \"type\": \"CONCEALED\",
          \"label\": \"kubeconfig\",
          \"value\": $(echo \"$KUBECONFIG_CONTENT\" | jq -Rs .)
        },
        {
          \"id\": \"argocd-name\",
          \"type\": \"STRING\",
          \"label\": \"argocd-name\",
          \"value\": \"$VCLUSTER_NAME.$BASE_DOMAIN_SANITIZED\"
        },
        {
          \"id\": \"argocd-server\",
          \"type\": \"STRING\",
          \"label\": \"argocd-server\",
          \"value\": \"$EXTERNAL_SERVER_URL\"
        },
        {
          \"id\": \"argocd-config\",
          \"type\": \"CONCEALED\",
          \"label\": \"argocd-config\",
          \"value\": $(echo \"$ARGOCD_CONFIG\" | jq -Rc .)
        }
      ]
    }")
  ITEM_ID=$(echo "$RESPONSE" | grep -o '"id":"[^"]*' | head -1 | cut -d'"' -f4)
  echo "Created item with ID: $ITEM_ID"
else
  echo "Updating existing item ID: $ITEM_ID"
  curl -s -X PATCH "$OP_CONNECT_HOST/v1/vaults/homelab/items/$ITEM_ID" \
    -H "Authorization: Bearer $OP_CONNECT_TOKEN" \
    -H "Content-Type: application/json" \
    -d "{
      \"fields\": [
        {
          \"id\": \"kubeconfig\",
          \"value\": $(echo \"$KUBECONFIG_CONTENT\" | jq -Rs .)
        },
        {
          \"id\": \"argocd-name\",
          \"value\": \"$VCLUSTER_NAME.$BASE_DOMAIN_SANITIZED\"
        },
        {
          \"id\": \"argocd-server\",
          \"value\": \"$EXTERNAL_SERVER_URL\"
        },
        {
          \"id\": \"argocd-config\",
          \"value\": $(echo \"$ARGOCD_CONFIG\" | jq -Rc .)
        }
      ]
    }"
fi

echo "✓ Kubeconfig synced to 1Password successfully"`

	return map[string]interface{}{
		"apiVersion": "batch/v1",
		"kind":       "Job",
		"metadata": resourceMeta(
			config.KubeconfigSyncJobName,
			config.TargetNamespace,
			labels,
			nil,
		),
		"spec": map[string]interface{}{
			"backoffLimit":            3,
			"ttlSecondsAfterFinished": 600,
			"template": map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": map[string]string{
						"app.kubernetes.io/name":     "kubeconfig-sync",
						"app.kubernetes.io/instance": config.Name,
					},
				},
				"spec": map[string]interface{}{
					"serviceAccountName": fmt.Sprintf("%s-kubeconfig-sync", config.Name),
					"restartPolicy":      "OnFailure",
					"initContainers": []map[string]interface{}{
						{
							"name":    "wait-for-kubeconfig",
							"image":   "busybox:1.36",
							"command": []string{"sh", "-c", initCommand},
							"volumeMounts": []map[string]interface{}{
								{
									"name":      "kubeconfig",
									"mountPath": "/kubeconfig",
								},
							},
						},
					},
					"containers": []map[string]interface{}{
						{
							"name":  "sync-to-onepassword",
							"image": "alpine:3.20",
							"env": []map[string]interface{}{
								{"name": "OP_CONNECT_HOST", "value": "https://connect.integratn.tech"},
								{
									"name": "OP_CONNECT_TOKEN",
									"valueFrom": map[string]interface{}{
										"secretKeyRef": map[string]interface{}{
											"name": fmt.Sprintf("vcluster-%s-onepassword-token", config.Name),
											"key":  "token",
										},
									},
								},
								{
									"name": "OP_VAULT",
									"valueFrom": map[string]interface{}{
										"secretKeyRef": map[string]interface{}{
											"name": fmt.Sprintf("vcluster-%s-onepassword-token", config.Name),
											"key":  "vault",
										},
									},
								},
								{"name": "VCLUSTER_NAME", "value": config.Name},
								{"name": "OP_ITEM_NAME", "value": config.OnePasswordItem},
								{"name": "BASE_DOMAIN", "value": config.BaseDomain},
								{"name": "BASE_DOMAIN_SANITIZED", "value": config.BaseDomainSanitized},
								{"name": "EXTERNAL_SERVER_URL", "value": config.ExternalServerURL},
								{"name": "ARGOCD_ENVIRONMENT", "value": config.ArgoCDEnvironment},
							},
							"command": []string{"sh", "-c", syncCommand},
							"volumeMounts": []map[string]interface{}{
								{
									"name":      "kubeconfig",
									"mountPath": "/kubeconfig",
									"readOnly":  true,
								},
							},
						},
					},
					"volumes": []map[string]interface{}{
						{
							"name": "kubeconfig",
							"secret": map[string]interface{}{
								"secretName": fmt.Sprintf("vc-%s", config.Name),
								"optional":   false,
							},
						},
					},
				},
			},
		},
	}
}

func buildEtcdCertificates(config *VClusterConfig) []interface{} {
	if !etcdEnabled(config) {
		return nil
	}

	labels := func(name string) map[string]string {
		return mergeStringMap(map[string]string{
			"app.kubernetes.io/instance": config.Name,
			"app.kubernetes.io/name":     name,
		}, baseLabels(config, config.Name))
	}

	caCert := map[string]interface{}{
		"apiVersion": "cert-manager.io/v1",
		"kind":       "Certificate",
		"metadata": resourceMeta(
			fmt.Sprintf("%s-etcd-ca", config.Name),
			config.TargetNamespace,
			labels("etcd-ca"),
			nil,
		),
		"spec": map[string]interface{}{
			"isCA":       true,
			"commonName": fmt.Sprintf("%s-etcd-ca", config.Name),
			"secretName": fmt.Sprintf("%s-etcd-ca", config.Name),
			"privateKey": map[string]interface{}{
				"algorithm": "RSA",
				"size":      2048,
			},
			"issuerRef": map[string]interface{}{
				"name":  fmt.Sprintf("%s-etcd-selfsigned", config.Name),
				"kind":  "Issuer",
				"group": "cert-manager.io",
			},
		},
	}

	selfsignedIssuer := map[string]interface{}{
		"apiVersion": "cert-manager.io/v1",
		"kind":       "Issuer",
		"metadata": resourceMeta(
			fmt.Sprintf("%s-etcd-selfsigned", config.Name),
			config.TargetNamespace,
			labels("etcd-issuer"),
			nil,
		),
		"spec": map[string]interface{}{
			"selfSigned": map[string]interface{}{},
		},
	}

	caIssuer := map[string]interface{}{
		"apiVersion": "cert-manager.io/v1",
		"kind":       "Issuer",
		"metadata": resourceMeta(
			fmt.Sprintf("%s-etcd-ca", config.Name),
			config.TargetNamespace,
			labels("etcd-ca-issuer"),
			nil,
		),
		"spec": map[string]interface{}{
			"ca": map[string]interface{}{
				"secretName": fmt.Sprintf("%s-etcd-ca", config.Name),
			},
		},
	}

	mergeJob := map[string]interface{}{
		"apiVersion": "batch/v1",
		"kind":       "Job",
		"metadata": resourceMeta(
			fmt.Sprintf("%s-etcd-certs-merge", config.Name),
			config.TargetNamespace,
			labels("etcd-certs-job"),
			nil,
		),
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": map[string]string{
						"app": "etcd-certs-merge",
					},
				},
				"spec": map[string]interface{}{
					"restartPolicy":      "OnFailure",
					"serviceAccountName": fmt.Sprintf("vc-%s", config.Name),
					"containers": []map[string]interface{}{
						{
							"name":    "merge-certs",
							"image":   "bitnami/kubectl:latest",
							"command": []string{"/bin/bash", "-c", buildEtcdMergeScript(config)},
						},
					},
				},
			},
		},
	}

	serverCert := map[string]interface{}{
		"apiVersion": "cert-manager.io/v1",
		"kind":       "Certificate",
		"metadata": resourceMeta(
			fmt.Sprintf("%s-etcd-server", config.Name),
			config.TargetNamespace,
			labels("etcd-server-cert"),
			nil,
		),
		"spec": map[string]interface{}{
			"secretName": fmt.Sprintf("%s-etcd-server", config.Name),
			"commonName": fmt.Sprintf("%s-etcd", config.Name),
			"dnsNames":   buildEtcdDNSNames(config),
			"ipAddresses": []string{
				"127.0.0.1",
			},
			"privateKey": map[string]interface{}{
				"algorithm": "RSA",
				"size":      2048,
			},
			"usages": []string{"server auth", "client auth"},
			"issuerRef": map[string]interface{}{
				"name":  fmt.Sprintf("%s-etcd-ca", config.Name),
				"kind":  "Issuer",
				"group": "cert-manager.io",
			},
			"secretTemplate": map[string]interface{}{
				"labels": map[string]string{
					"app.kubernetes.io/name":     "etcd-server-cert",
					"app.kubernetes.io/instance": config.Name,
				},
			},
		},
	}

	peerCert := map[string]interface{}{
		"apiVersion": "cert-manager.io/v1",
		"kind":       "Certificate",
		"metadata": resourceMeta(
			fmt.Sprintf("%s-etcd-peer", config.Name),
			config.TargetNamespace,
			labels("etcd-peer-cert"),
			nil,
		),
		"spec": map[string]interface{}{
			"secretName": fmt.Sprintf("%s-etcd-peer", config.Name),
			"commonName": fmt.Sprintf("%s-etcd", config.Name),
			"dnsNames":   buildEtcdDNSNames(config),
			"privateKey": map[string]interface{}{
				"algorithm": "RSA",
				"size":      2048,
			},
			"usages": []string{"server auth", "client auth"},
			"issuerRef": map[string]interface{}{
				"name":  fmt.Sprintf("%s-etcd-ca", config.Name),
				"kind":  "Issuer",
				"group": "cert-manager.io",
			},
			"secretTemplate": map[string]interface{}{
				"labels": map[string]string{
					"app.kubernetes.io/name":     "etcd-peer-cert",
					"app.kubernetes.io/instance": config.Name,
				},
			},
		},
	}

	return []interface{}{caCert, selfsignedIssuer, caIssuer, mergeJob, serverCert, peerCert}
}

func buildEtcdDNSNames(config *VClusterConfig) []string {
	base := []string{
		fmt.Sprintf("%s-etcd", config.Name),
		fmt.Sprintf("%s-etcd.%s", config.Name, config.TargetNamespace),
		fmt.Sprintf("%s-etcd.%s.svc", config.Name, config.TargetNamespace),
		fmt.Sprintf("%s-etcd.%s.svc.cluster.local", config.Name, config.TargetNamespace),
		fmt.Sprintf("%s-etcd-headless", config.Name),
		fmt.Sprintf("%s-etcd-headless.%s", config.Name, config.TargetNamespace),
		fmt.Sprintf("%s-etcd-headless.%s.svc", config.Name, config.TargetNamespace),
		fmt.Sprintf("%s-etcd-headless.%s.svc.cluster.local", config.Name, config.TargetNamespace),
	}
	for i := 0; i < 3; i++ {
		base = append(base,
			fmt.Sprintf("%s-etcd-%d", config.Name, i),
			fmt.Sprintf("%s-etcd-%d.%s-etcd-headless.%s", config.Name, i, config.Name, config.TargetNamespace),
			fmt.Sprintf("%s-etcd-%d.%s-etcd-headless.%s.svc", config.Name, i, config.Name, config.TargetNamespace),
			fmt.Sprintf("%s-etcd-%d.%s-etcd-headless.%s.svc.cluster.local", config.Name, i, config.Name, config.TargetNamespace),
		)
	}
	base = append(base, "localhost")
	return base
}

func buildEtcdMergeScript(config *VClusterConfig) string {
	return fmt.Sprintf(`set -e
echo "Waiting for certificates to be ready..."

# Wait for CA cert
until kubectl get secret %s-etcd-ca -n %s 2>/dev/null; do
  echo "Waiting for CA certificate..."
  sleep 2
done

# Wait for server cert
until kubectl get secret %s-etcd-server -n %s 2>/dev/null; do
  echo "Waiting for server certificate..."
  sleep 2
done

# Wait for peer cert
until kubectl get secret %s-etcd-peer -n %s 2>/dev/null; do
  echo "Waiting for peer certificate..."
  sleep 2
done

echo "All certificates ready, merging..."

# Extract certs
CA_CRT=$(kubectl get secret %s-etcd-ca -n %s -o jsonpath='{.data.tls\.crt}')
SERVER_CRT=$(kubectl get secret %s-etcd-server -n %s -o jsonpath='{.data.tls\.crt}')
SERVER_KEY=$(kubectl get secret %s-etcd-server -n %s -o jsonpath='{.data.tls\.key}')
PEER_CRT=$(kubectl get secret %s-etcd-peer -n %s -o jsonpath='{.data.tls\.crt}')
PEER_KEY=$(kubectl get secret %s-etcd-peer -n %s -o jsonpath='{.data.tls\.key}')

# Create merged secret
kubectl create secret generic %s-certs -n %s \
  --from-literal=etcd-ca.crt="$(echo $CA_CRT | base64 -d)" \
  --from-literal=etcd-server.crt="$(echo $SERVER_CRT | base64 -d)" \
  --from-literal=etcd-server.key="$(echo $SERVER_KEY | base64 -d)" \
  --from-literal=etcd-peer.crt="$(echo $PEER_CRT | base64 -d)" \
  --from-literal=etcd-peer.key="$(echo $PEER_KEY | base64 -d)" \
  --dry-run=client -o yaml | kubectl apply -f -

echo "Certificate merge complete!"`,
		config.Name, config.TargetNamespace,
		config.Name, config.TargetNamespace,
		config.Name, config.TargetNamespace,
		config.Name, config.TargetNamespace,
		config.Name, config.TargetNamespace,
		config.Name, config.TargetNamespace,
		config.Name, config.TargetNamespace,
		config.Name, config.TargetNamespace,
		config.Name, config.TargetNamespace,
		config.Name, config.TargetNamespace,
	)
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

	outputs := map[string]interface{}{
		"resources/delete-argocd-project-request.yaml": deleteResource(
			"platform.integratn.tech/v1alpha1",
			"ArgoCDProject",
			config.ProjectName,
			config.Namespace,
		),
		"resources/delete-argocd-application-request.yaml": deleteResource(
			"platform.integratn.tech/v1alpha1",
			"ArgoCDApplication",
			fmt.Sprintf("vcluster-%s", config.Name),
			config.Namespace,
		),
		"resources/delete-coredns-configmap.yaml": deleteResource(
			"v1",
			"ConfigMap",
			fmt.Sprintf("vc-%s-coredns", config.Name),
			config.TargetNamespace,
		),
		"resources/delete-kubeconfig-external-secret.yaml": deleteResource(
			"external-secrets.io/v1beta1",
			"ExternalSecret",
			fmt.Sprintf("%s-kubeconfig", config.Name),
			config.TargetNamespace,
		),
		"resources/delete-argocd-cluster-external-secret.yaml": deleteResource(
			"external-secrets.io/v1beta1",
			"ExternalSecret",
			fmt.Sprintf("%s-argocd-cluster", config.Name),
			"argocd",
		),
		"resources/delete-onepassword-token-external-secret.yaml": deleteResource(
			"external-secrets.io/v1beta1",
			"ExternalSecret",
			fmt.Sprintf("%s-onepassword-token", config.Name),
			config.TargetNamespace,
		),
		"resources/delete-kubeconfig-sync-sa.yaml": deleteResource(
			"v1",
			"ServiceAccount",
			fmt.Sprintf("%s-kubeconfig-sync", config.Name),
			config.TargetNamespace,
		),
		"resources/delete-kubeconfig-sync-role.yaml": deleteResource(
			"rbac.authorization.k8s.io/v1",
			"Role",
			fmt.Sprintf("%s-kubeconfig-sync", config.Name),
			config.TargetNamespace,
		),
		"resources/delete-kubeconfig-sync-rolebinding.yaml": deleteResource(
			"rbac.authorization.k8s.io/v1",
			"RoleBinding",
			fmt.Sprintf("%s-kubeconfig-sync", config.Name),
			config.TargetNamespace,
		),
		"resources/delete-vcluster-clusterrole.yaml": deleteResource(
			"rbac.authorization.k8s.io/v1",
			"ClusterRole",
			fmt.Sprintf("vc-%s-v-%s", config.Name, config.TargetNamespace),
			"",
		),
		"resources/delete-vcluster-clusterrolebinding.yaml": deleteResource(
			"rbac.authorization.k8s.io/v1",
			"ClusterRoleBinding",
			fmt.Sprintf("vc-%s-v-%s", config.Name, config.TargetNamespace),
			"",
		),
		"resources/delete-kubeconfig-sync-job.yaml": deleteResource(
			"batch/v1",
			"Job",
			config.KubeconfigSyncJobName,
			config.TargetNamespace,
		),
	}

	if etcdEnabled(config) {
		outputs["resources/delete-etcd-ca-certificate.yaml"] = deleteResource(
			"cert-manager.io/v1",
			"Certificate",
			fmt.Sprintf("%s-etcd-ca", config.Name),
			config.TargetNamespace,
		)
		outputs["resources/delete-etcd-server-certificate.yaml"] = deleteResource(
			"cert-manager.io/v1",
			"Certificate",
			fmt.Sprintf("%s-etcd-server", config.Name),
			config.TargetNamespace,
		)
		outputs["resources/delete-etcd-peer-certificate.yaml"] = deleteResource(
			"cert-manager.io/v1",
			"Certificate",
			fmt.Sprintf("%s-etcd-peer", config.Name),
			config.TargetNamespace,
		)
		outputs["resources/delete-etcd-selfsigned-issuer.yaml"] = deleteResource(
			"cert-manager.io/v1",
			"Issuer",
			fmt.Sprintf("%s-etcd-selfsigned", config.Name),
			config.TargetNamespace,
		)
		outputs["resources/delete-etcd-ca-issuer.yaml"] = deleteResource(
			"cert-manager.io/v1",
			"Issuer",
			fmt.Sprintf("%s-etcd-ca", config.Name),
			config.TargetNamespace,
		)
		outputs["resources/delete-etcd-certs-job.yaml"] = deleteResource(
			"batch/v1",
			"Job",
			fmt.Sprintf("%s-etcd-certs-merge", config.Name),
			config.TargetNamespace,
		)
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
			fmt.Sprintf("%s-certs", config.Name),
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

func extractSpec(resource kratix.Resource, path string, target interface{}) error {
	val, err := resource.GetValue(path)
	if err != nil {
		return err
	}

	jsonBytes, err := json.Marshal(val)
	if err != nil {
		return fmt.Errorf("failed to marshal %s: %w", path, err)
	}

	if err := json.Unmarshal(jsonBytes, target); err != nil {
		return fmt.Errorf("failed to unmarshal %s: %w", path, err)
	}

	return nil
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
