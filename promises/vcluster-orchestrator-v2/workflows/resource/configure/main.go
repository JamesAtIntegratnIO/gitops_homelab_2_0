package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"text/template"

	kratix "github.com/syntasso/kratix-go"
	"sigs.k8s.io/yaml"
)

//go:embed templates/*.yaml.tmpl
var templatesFS embed.FS

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

	// Templates for ResourceRequests (keeps ArgoCD promises separate for reusability)
	rrTemplates := []string{
		"argocd-project-request.yaml.tmpl",
		"argocd-application-request.yaml.tmpl",
	}

	// Direct resource templates (consolidates vcluster-specific sub-promises)
	resourceTemplates := []string{
		"namespace.yaml.tmpl",
		"etcd-certificates.yaml.tmpl",
		"coredns-configmap.yaml.tmpl",
		"kubeconfig-sync-rbac.yaml.tmpl",
		"kubeconfig-sync-job.yaml.tmpl",
		"kubeconfig-external-secret.yaml.tmpl",
		"argocd-cluster-external-secret.yaml.tmpl",
	}

	allTemplates := append(rrTemplates, resourceTemplates...)

	for _, tmplName := range allTemplates {
		if err := renderTemplate(sdk, tmplName, config); err != nil {
			return fmt.Errorf("failed to render %s: %w", tmplName, err)
		}
		log.Printf("✓ Rendered: %s", tmplName)
	}

	status := kratix.NewStatus()
	status.Set("phase", "Scheduled")
	status.Set("message", "VCluster resources scheduled for creation")
	status.Set("resourceRequestsGenerated", len(rrTemplates))
	status.Set("directResourcesGenerated", len(resourceTemplates))
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

func renderTemplate(sdk *kratix.KratixSDK, tmplName string, config *VClusterConfig) error {
	content, err := templatesFS.ReadFile("templates/" + tmplName)
	if err != nil {
		return fmt.Errorf("read template: %w", err)
	}

	funcMap := template.FuncMap{
		"toYaml": func(v interface{}) string {
			data, _ := yaml.Marshal(v)
			return strings.TrimSuffix(string(data), "\n")
		},
		"indent": func(spaces int, s string) string {
			pad := strings.Repeat(" ", spaces)
			return pad + strings.ReplaceAll(s, "\n", "\n"+pad)
		},
		"toBool": func(v bool) string {
			if v {
				return "true"
			}
			return "false"
		},
		"quote": func(s string) string {
			return fmt.Sprintf("%q", s)
		},
	}

	tmpl, err := template.New(tmplName).Funcs(funcMap).Parse(string(content))
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, config); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}

	// Validate YAML
	var yamlCheck interface{}
	if err := yaml.Unmarshal(buf.Bytes(), &yamlCheck); err != nil {
		return fmt.Errorf("invalid YAML generated: %w", err)
	}

	// Strip .tmpl extension from output filename
	outputName := strings.TrimSuffix(tmplName, ".tmpl")
	outputPath := "resources/" + outputName
	if err := sdk.WriteOutput(outputPath, buf.Bytes()); err != nil {
		return fmt.Errorf("write output: %w", err)
	}

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

	outputs := map[string]string{
		"resources/delete-argocd-project-request.yaml": fmt.Sprintf(
			"apiVersion: platform.integratn.tech/v1alpha1\nkind: ArgoCDProject\nmetadata:\n  name: %s\n  namespace: %s\n",
			config.ProjectName,
			config.Namespace,
		),
		"resources/delete-argocd-application-request.yaml": fmt.Sprintf(
			"apiVersion: platform.integratn.tech/v1alpha1\nkind: ArgoCDApplication\nmetadata:\n  name: %s\n  namespace: %s\n",
			fmt.Sprintf("vcluster-%s", config.Name),
			config.Namespace,
		),
		"resources/delete-coredns-configmap.yaml": fmt.Sprintf(
			"apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: vc-%s-coredns\n  namespace: %s\n",
			config.Name,
			config.TargetNamespace,
		),
		"resources/delete-kubeconfig-external-secret.yaml": fmt.Sprintf(
			"apiVersion: external-secrets.io/v1beta1\nkind: ExternalSecret\nmetadata:\n  name: %s-kubeconfig\n  namespace: %s\n",
			config.Name,
			config.TargetNamespace,
		),
		"resources/delete-argocd-cluster-external-secret.yaml": fmt.Sprintf(
			"apiVersion: external-secrets.io/v1beta1\nkind: ExternalSecret\nmetadata:\n  name: %s-argocd-cluster\n  namespace: argocd\n",
			config.Name,
		),
		"resources/delete-onepassword-token-external-secret.yaml": fmt.Sprintf(
			"apiVersion: external-secrets.io/v1beta1\nkind: ExternalSecret\nmetadata:\n  name: %s-onepassword-token\n  namespace: %s\n",
			config.Name,
			config.TargetNamespace,
		),
		"resources/delete-kubeconfig-sync-sa.yaml": fmt.Sprintf(
			"apiVersion: v1\nkind: ServiceAccount\nmetadata:\n  name: %s-kubeconfig-sync\n  namespace: %s\n",
			config.Name,
			config.TargetNamespace,
		),
		"resources/delete-kubeconfig-sync-role.yaml": fmt.Sprintf(
			"apiVersion: rbac.authorization.k8s.io/v1\nkind: Role\nmetadata:\n  name: %s-kubeconfig-sync\n  namespace: %s\n",
			config.Name,
			config.TargetNamespace,
		),
		"resources/delete-kubeconfig-sync-rolebinding.yaml": fmt.Sprintf(
			"apiVersion: rbac.authorization.k8s.io/v1\nkind: RoleBinding\nmetadata:\n  name: %s-kubeconfig-sync\n  namespace: %s\n",
			config.Name,
			config.TargetNamespace,
		),
		"resources/delete-kubeconfig-sync-job.yaml": fmt.Sprintf(
			"apiVersion: batch/v1\nkind: Job\nmetadata:\n  name: %s\n  namespace: %s\n",
			config.KubeconfigSyncJobName,
			config.TargetNamespace,
		),
		"resources/delete-namespace.yaml": fmt.Sprintf(
			"apiVersion: v1\nkind: Namespace\nmetadata:\n  name: %s\n",
			config.TargetNamespace,
		),
	}

	if config.BackingStore != nil {
		if etcd, ok := config.BackingStore["etcd"].(map[string]interface{}); ok {
			if deploy, ok := etcd["deploy"].(map[string]interface{}); ok {
				if enabled, ok := deploy["enabled"].(bool); ok && enabled {
					outputs["resources/delete-etcd-ca-certificate.yaml"] = fmt.Sprintf(
						"apiVersion: cert-manager.io/v1\nkind: Certificate\nmetadata:\n  name: %s-etcd-ca\n  namespace: %s\n",
						config.Name,
						config.TargetNamespace,
					)
					outputs["resources/delete-etcd-server-certificate.yaml"] = fmt.Sprintf(
						"apiVersion: cert-manager.io/v1\nkind: Certificate\nmetadata:\n  name: %s-etcd-server\n  namespace: %s\n",
						config.Name,
						config.TargetNamespace,
					)
					outputs["resources/delete-etcd-peer-certificate.yaml"] = fmt.Sprintf(
						"apiVersion: cert-manager.io/v1\nkind: Certificate\nmetadata:\n  name: %s-etcd-peer\n  namespace: %s\n",
						config.Name,
						config.TargetNamespace,
					)
					outputs["resources/delete-etcd-selfsigned-issuer.yaml"] = fmt.Sprintf(
						"apiVersion: cert-manager.io/v1\nkind: Issuer\nmetadata:\n  name: %s-etcd-selfsigned\n  namespace: %s\n",
						config.Name,
						config.TargetNamespace,
					)
					outputs["resources/delete-etcd-ca-issuer.yaml"] = fmt.Sprintf(
						"apiVersion: cert-manager.io/v1\nkind: Issuer\nmetadata:\n  name: %s-etcd-ca\n  namespace: %s\n",
						config.Name,
						config.TargetNamespace,
					)
					outputs["resources/delete-etcd-certs-job.yaml"] = fmt.Sprintf(
						"apiVersion: batch/v1\nkind: Job\nmetadata:\n  name: %s-etcd-certs-merge\n  namespace: %s\n",
						config.Name,
						config.TargetNamespace,
					)
					outputs["resources/delete-etcd-ca-secret.yaml"] = fmt.Sprintf(
						"apiVersion: v1\nkind: Secret\nmetadata:\n  name: %s-etcd-ca\n  namespace: %s\n",
						config.Name,
						config.TargetNamespace,
					)
					outputs["resources/delete-etcd-server-secret.yaml"] = fmt.Sprintf(
						"apiVersion: v1\nkind: Secret\nmetadata:\n  name: %s-etcd-server\n  namespace: %s\n",
						config.Name,
						config.TargetNamespace,
					)
					outputs["resources/delete-etcd-peer-secret.yaml"] = fmt.Sprintf(
						"apiVersion: v1\nkind: Secret\nmetadata:\n  name: %s-etcd-peer\n  namespace: %s\n",
						config.Name,
						config.TargetNamespace,
					)
					outputs["resources/delete-etcd-merged-secret.yaml"] = fmt.Sprintf(
						"apiVersion: v1\nkind: Secret\nmetadata:\n  name: %s-certs\n  namespace: %s\n",
						config.Name,
						config.TargetNamespace,
					)
				}
			}
		}
	}

	for path, content := range outputs {
		if err := sdk.WriteOutput(path, []byte(content)); err != nil {
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
