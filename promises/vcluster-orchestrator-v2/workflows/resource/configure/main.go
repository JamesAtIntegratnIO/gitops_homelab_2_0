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
	HelmOverrides       map[string]interface{}
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
	config.BaseDomain = "integratn.tech" // TODO: Extract from annotation if present
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

	return config, nil
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
