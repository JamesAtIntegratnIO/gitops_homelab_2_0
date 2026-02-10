package main

import (
	"fmt"
	"log"
	"strings"

	kratix "github.com/syntasso/kratix-go"
)

func main() {
	sdk := kratix.New()

	log.Printf("=== ArgoCD Cluster Registration Promise Pipeline ===")
	log.Printf("Action: %s", sdk.WorkflowAction())

	resource, err := sdk.ReadResourceInput()
	if err != nil {
		log.Fatalf("ERROR: Failed to read resource input: %v", err)
	}

	log.Printf("Processing resource: %s/%s",
		resource.GetNamespace(), resource.GetName())

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

func buildConfig(sdk *kratix.KratixSDK, resource kratix.Resource) (*RegistrationConfig, error) {
	name, err := getStringValue(resource, "spec.name")
	if err != nil {
		return nil, fmt.Errorf("spec.name is required: %w", err)
	}

	targetNamespace, err := getStringValue(resource, "spec.targetNamespace")
	if err != nil {
		return nil, fmt.Errorf("spec.targetNamespace is required: %w", err)
	}

	kubeconfigSecret, err := getStringValue(resource, "spec.kubeconfigSecret")
	if err != nil {
		return nil, fmt.Errorf("spec.kubeconfigSecret is required: %w", err)
	}

	externalServerURL, err := getStringValue(resource, "spec.externalServerURL")
	if err != nil {
		return nil, fmt.Errorf("spec.externalServerURL is required: %w", err)
	}

	kubeconfigKey, _ := getStringValueWithDefault(resource, "spec.kubeconfigKey", "config")
	onePasswordItem, _ := getStringValueWithDefault(resource, "spec.onePasswordItem", fmt.Sprintf("%s-kubeconfig", name))
	onePasswordConnectHost, _ := getStringValueWithDefault(resource, "spec.onePasswordConnectHost", "https://connect.integratn.tech")
	environment, _ := getStringValueWithDefault(resource, "spec.environment", "development")
	baseDomain, _ := getStringValueWithDefault(resource, "spec.baseDomain", "integratn.tech")

	baseDomainSanitized, _ := getStringValue(resource, "spec.baseDomainSanitized")
	if baseDomainSanitized == "" {
		baseDomainSanitized = strings.ReplaceAll(baseDomain, ".", "-")
	}

	syncJobName, _ := getStringValue(resource, "spec.syncJobName")
	if syncJobName == "" {
		syncJobName = fmt.Sprintf("%s-kubeconfig-sync", name)
	}

	clusterLabels := extractStringMap(resource, "spec.clusterLabels")
	clusterAnnotations := extractStringMap(resource, "spec.clusterAnnotations")

	return &RegistrationConfig{
		Name:                   name,
		TargetNamespace:        targetNamespace,
		KubeconfigSecret:       kubeconfigSecret,
		KubeconfigKey:          kubeconfigKey,
		ExternalServerURL:      externalServerURL,
		OnePasswordItem:        onePasswordItem,
		OnePasswordConnectHost: onePasswordConnectHost,
		Environment:            environment,
		BaseDomain:             baseDomain,
		BaseDomainSanitized:    baseDomainSanitized,
		ClusterLabels:          clusterLabels,
		ClusterAnnotations:     clusterAnnotations,
		SyncJobName:            syncJobName,
		PromiseName:            sdk.PromiseName(),
	}, nil
}

func handleConfigure(sdk *kratix.KratixSDK, config *RegistrationConfig) error {
	log.Println("--- Rendering cluster registration resources ---")

	// 1. Kubeconfig sync RBAC (ExternalSecret for 1Password token, SA, Role, RoleBinding)
	rbacResources := buildKubeconfigSyncRBAC(config)
	if err := writeYAMLDocuments(sdk, "resources/kubeconfig-sync-rbac.yaml", rbacResources); err != nil {
		return fmt.Errorf("write kubeconfig sync rbac: %w", err)
	}
	log.Printf("✓ Rendered: kubeconfig-sync-rbac.yaml (%d resources)", len(rbacResources))

	// 2. Kubeconfig sync Job
	syncJob := buildKubeconfigSyncJob(config)
	if err := writeYAML(sdk, "resources/kubeconfig-sync-job.yaml", syncJob); err != nil {
		return fmt.Errorf("write kubeconfig sync job: %w", err)
	}
	log.Printf("✓ Rendered: kubeconfig-sync-job.yaml")

	// 3. Kubeconfig ExternalSecret (reads kubeconfig from 1Password)
	kubeconfigES := buildKubeconfigExternalSecret(config)
	if err := writeYAML(sdk, "resources/kubeconfig-external-secret.yaml", kubeconfigES); err != nil {
		return fmt.Errorf("write kubeconfig external secret: %w", err)
	}
	log.Printf("✓ Rendered: kubeconfig-external-secret.yaml")

	// 4. ArgoCD Cluster ExternalSecret (creates ArgoCD cluster secret from 1Password)
	clusterES := buildArgoCDClusterExternalSecret(config)
	if err := writeYAML(sdk, "resources/argocd-cluster-external-secret.yaml", clusterES); err != nil {
		return fmt.Errorf("write argocd cluster external secret: %w", err)
	}
	log.Printf("✓ Rendered: argocd-cluster-external-secret.yaml")

	status := kratix.NewStatus()
	status.Set("phase", "Configured")
	status.Set("message", fmt.Sprintf("Cluster %s registration resources configured", config.Name))
	status.Set("clusterName", config.Name)
	status.Set("targetNamespace", config.TargetNamespace)
	status.Set("externalServerURL", config.ExternalServerURL)
	status.Set("environment", config.Environment)

	if err := sdk.WriteStatus(status); err != nil {
		return fmt.Errorf("failed to write status: %w", err)
	}

	log.Println("✓ Status updated")
	return nil
}

func handleDelete(sdk *kratix.KratixSDK, config *RegistrationConfig) error {
	log.Printf("--- Handling delete for cluster registration: %s ---", config.Name)

	status := kratix.NewStatus()
	status.Set("phase", "Deleting")
	status.Set("message", fmt.Sprintf("Cluster %s registration resources scheduled for deletion", config.Name))
	status.Set("clusterName", config.Name)

	if err := sdk.WriteStatus(status); err != nil {
		return fmt.Errorf("failed to write status: %w", err)
	}

	outputs := map[string]Resource{}

	// Delete all created resources
	allResources := []Resource{
		buildKubeconfigExternalSecret(config),
		buildArgoCDClusterExternalSecret(config),
		buildKubeconfigSyncJob(config),
	}
	for _, r := range allResources {
		outputs[deleteOutputPath("resources", r)] = deleteFromResource(r)
	}

	for _, r := range buildKubeconfigSyncRBAC(config) {
		outputs[deleteOutputPath("resources", r)] = deleteFromResource(r)
	}

	for path, obj := range outputs {
		if err := writeYAML(sdk, path, obj); err != nil {
			return fmt.Errorf("write delete output %s: %w", path, err)
		}
	}

	return nil
}

// ============================================================================
// Helper Functions
// ============================================================================

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
	if err != nil || val == "" {
		return defaultValue, nil
	}
	return val, nil
}

func extractStringMap(resource kratix.Resource, path string) map[string]string {
	val, err := resource.GetValue(path)
	if err != nil {
		return nil
	}
	m, ok := val.(map[string]interface{})
	if !ok {
		return nil
	}
	result := make(map[string]string)
	for k, v := range m {
		if str, ok := v.(string); ok {
			result[k] = str
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
