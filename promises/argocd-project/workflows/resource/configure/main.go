package main

import (
	"fmt"
	"log"

	kratix "github.com/syntasso/kratix-go"
)

func main() {
	sdk := kratix.New()

	log.Printf("=== ArgoCD Project Promise Pipeline ===")
	log.Printf("Action: %s", sdk.WorkflowAction())

	resource, err := sdk.ReadResourceInput()
	if err != nil {
		log.Fatalf("ERROR: Failed to read resource input: %v", err)
	}

	log.Printf("Processing resource: %s/%s",
		resource.GetNamespace(), resource.GetName())

	if sdk.WorkflowAction() == "configure" {
		if err := handleConfigure(sdk, resource); err != nil {
			log.Fatalf("ERROR: Configure failed: %v", err)
		}
	} else if sdk.WorkflowAction() == "delete" {
		if err := handleDelete(sdk, resource); err != nil {
			log.Fatalf("ERROR: Delete failed: %v", err)
		}
	} else {
		log.Fatalf("ERROR: Unknown workflow action: %s", sdk.WorkflowAction())
	}

	log.Println("=== Pipeline completed successfully ===")
}

func handleConfigure(sdk *kratix.KratixSDK, resource kratix.Resource) error {
	name, err := getStringValue(resource, "spec.name")
	if err != nil {
		return fmt.Errorf("spec.name is required: %w", err)
	}

	namespace, _ := getStringValueWithDefault(resource, "spec.namespace", "argocd")
	description, _ := getStringValue(resource, "spec.description")

	annotations := extractStringMap(resource, "spec.annotations")
	labels := extractStringMap(resource, "spec.labels")
	sourceRepos := extractStringSlice(resource, "spec.sourceRepos")
	destinations := extractObjectSlice(resource, "spec.destinations")
	clusterResourceWhitelist := extractObjectSlice(resource, "spec.clusterResourceWhitelist")
	namespaceResourceWhitelist := extractObjectSlice(resource, "spec.namespaceResourceWhitelist")

	// Build the ArgoCD AppProject
	project := Resource{
		APIVersion: "argoproj.io/v1alpha1",
		Kind:       "AppProject",
		Metadata: ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: AppProjectSpec{
			Description:                description,
			SourceRepos:                sourceRepos,
			Destinations:               destinations,
			ClusterResourceWhitelist:   clusterResourceWhitelist,
			NamespaceResourceWhitelist: namespaceResourceWhitelist,
		},
	}

	if err := writeYAML(sdk, "resources/appproject.yaml", project); err != nil {
		return fmt.Errorf("write appproject: %w", err)
	}
	log.Printf("✓ Rendered ArgoCD AppProject: %s", name)

	status := kratix.NewStatus()
	status.Set("phase", "Configured")
	status.Set("message", fmt.Sprintf("AppProject %s configured", name))
	status.Set("projectName", name)
	status.Set("namespace", namespace)

	if err := sdk.WriteStatus(status); err != nil {
		return fmt.Errorf("failed to write status: %w", err)
	}

	return nil
}

func handleDelete(sdk *kratix.KratixSDK, resource kratix.Resource) error {
	name, err := getStringValue(resource, "spec.name")
	if err != nil {
		return fmt.Errorf("spec.name is required: %w", err)
	}

	namespace, _ := getStringValueWithDefault(resource, "spec.namespace", "argocd")

	deleteObj := Resource{
		APIVersion: "argoproj.io/v1alpha1",
		Kind:       "AppProject",
		Metadata: ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}

	if err := writeYAML(sdk, "resources/delete-appproject.yaml", deleteObj); err != nil {
		return fmt.Errorf("write delete appproject: %w", err)
	}
	log.Printf("✓ Delete scheduled for AppProject: %s", name)

	status := kratix.NewStatus()
	status.Set("phase", "Deleting")
	status.Set("message", fmt.Sprintf("AppProject %s scheduled for deletion", name))

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

func extractStringSlice(resource kratix.Resource, path string) []string {
	val, err := resource.GetValue(path)
	if err != nil {
		return nil
	}
	arr, ok := val.([]interface{})
	if !ok {
		return nil
	}
	result := make([]string, 0, len(arr))
	for _, v := range arr {
		if str, ok := v.(string); ok {
			result = append(result, str)
		}
	}
	return result
}

func extractObjectSlice(resource kratix.Resource, path string) []map[string]interface{} {
	val, err := resource.GetValue(path)
	if err != nil {
		return nil
	}
	arr, ok := val.([]interface{})
	if !ok {
		return nil
	}
	result := make([]map[string]interface{}, 0, len(arr))
	for _, v := range arr {
		if obj, ok := v.(map[string]interface{}); ok {
			result = append(result, obj)
		}
	}
	return result
}
