package main

import (
	"fmt"
	"log"
	"strings"

	kratix "github.com/syntasso/kratix-go"

	ku "github.com/jamesatintegratnio/gitops_homelab_2_0/promises/_shared/kratixutil"
)

func main() {
	ku.RunPromiseWithConfig("ArgoCD Cluster Registration", buildConfig, handleConfigure, handleDelete)
}

func buildConfig(sdk *kratix.KratixSDK, resource kratix.Resource) (*RegistrationConfig, error) {
	name, err := ku.GetStringValue(resource, "spec.name")
	if err != nil {
		return nil, fmt.Errorf("spec.name is required: %w", err)
	}

	targetNamespace, err := ku.GetStringValue(resource, "spec.targetNamespace")
	if err != nil {
		return nil, fmt.Errorf("spec.targetNamespace is required: %w", err)
	}

	kubeconfigSecret, err := ku.GetStringValue(resource, "spec.kubeconfigSecret")
	if err != nil {
		return nil, fmt.Errorf("spec.kubeconfigSecret is required: %w", err)
	}

	externalServerURL, err := ku.GetStringValue(resource, "spec.externalServerURL")
	if err != nil {
		return nil, fmt.Errorf("spec.externalServerURL is required: %w", err)
	}

	kubeconfigKey := ku.GetStringValueWithDefault(resource, "spec.kubeconfigKey", "config")
	onePasswordItem := ku.GetStringValueWithDefault(resource, "spec.onePasswordItem", fmt.Sprintf("%s-kubeconfig", name))
	onePasswordConnectHost := ku.GetStringValueWithDefault(resource, "spec.onePasswordConnectHost", "https://connect.integratn.tech")
	environment := ku.GetStringValueWithDefault(resource, "spec.environment", "development")
	baseDomain := ku.GetStringValueWithDefault(resource, "spec.baseDomain", "integratn.tech")

	baseDomainSanitized, _ := ku.GetStringValue(resource, "spec.baseDomainSanitized")
	if baseDomainSanitized == "" {
		baseDomainSanitized = strings.ReplaceAll(baseDomain, ".", "-")
	}

	syncJobName, _ := ku.GetStringValue(resource, "spec.syncJobName")
	if syncJobName == "" {
		syncJobName = fmt.Sprintf("%s-kubeconfig-sync", name)
	}

	clusterLabels, err := ku.ExtractStringMapFromResource(resource, "spec.clusterLabels")
	if err != nil {
		return nil, err
	}
	clusterAnnotations, err := ku.ExtractStringMapFromResource(resource, "spec.clusterAnnotations")
	if err != nil {
		return nil, err
	}

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
	if err := ku.WriteYAMLDocuments(sdk, "resources/kubeconfig-sync-rbac.yaml", rbacResources); err != nil {
		return fmt.Errorf("write kubeconfig sync rbac: %w", err)
	}

	// 2. Kubeconfig sync Job
	syncJob := buildKubeconfigSyncJob(config)
	if err := ku.WriteYAML(sdk, "resources/kubeconfig-sync-job.yaml", syncJob); err != nil {
		return fmt.Errorf("write kubeconfig sync job: %w", err)
	}

	// 3. Kubeconfig ExternalSecret (reads kubeconfig from 1Password)
	kubeconfigES := buildKubeconfigExternalSecret(config)
	if err := ku.WriteYAML(sdk, "resources/kubeconfig-external-secret.yaml", kubeconfigES); err != nil {
		return fmt.Errorf("write kubeconfig external secret: %w", err)
	}

	// 4. ArgoCD Cluster ExternalSecret (creates ArgoCD cluster secret from 1Password)
	clusterES := buildArgoCDClusterExternalSecret(config)
	if err := ku.WriteYAML(sdk, "resources/argocd-cluster-external-secret.yaml", clusterES); err != nil {
		return fmt.Errorf("write argocd cluster external secret: %w", err)
	}

	if err := ku.WritePromiseStatus(sdk, ku.PhaseConfigured,
		fmt.Sprintf("Cluster %s registration resources configured", config.Name),
		map[string]interface{}{
			"clusterName":       config.Name,
			"targetNamespace":   config.TargetNamespace,
			"externalServerURL": config.ExternalServerURL,
			"environment":       config.Environment,
		}); err != nil {
		return fmt.Errorf("failed to write status: %w", err)
	}

	return nil
}

func handleDelete(sdk *kratix.KratixSDK, config *RegistrationConfig) error {
	log.Printf("--- Handling delete for cluster registration: %s ---", config.Name)

	if err := ku.WritePromiseStatus(sdk, ku.PhaseDeleting,
		fmt.Sprintf("Cluster %s registration resources scheduled for deletion", config.Name),
		map[string]interface{}{"clusterName": config.Name}); err != nil {
		return fmt.Errorf("failed to write status: %w", err)
	}

	outputs := map[string]ku.Resource{}

	// Delete all created resources
	allResources := []ku.Resource{
		buildKubeconfigExternalSecret(config),
		buildArgoCDClusterExternalSecret(config),
		buildKubeconfigSyncJob(config),
	}
	for _, r := range allResources {
		outputs[ku.DeleteOutputPathForResource("resources", r)] = ku.DeleteFromResource(r)
	}

	for _, r := range buildKubeconfigSyncRBAC(config) {
		outputs[ku.DeleteOutputPathForResource("resources", r)] = ku.DeleteFromResource(r)
	}

	if err := ku.WriteOrderedResources(sdk, outputs); err != nil {
		return fmt.Errorf("write delete outputs: %w", err)
	}

	return nil
}

