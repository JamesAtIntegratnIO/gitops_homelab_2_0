package main

import (
	"fmt"
	"log"

	kratix "github.com/syntasso/kratix-go"
)

func main() {
	sdk := kratix.New()

	log.Printf("=== ArgoCD Application Promise Pipeline ===")
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
	project, err := getStringValue(resource, "spec.project")
	if err != nil {
		return fmt.Errorf("spec.project is required: %w", err)
	}

	annotations := extractStringMap(resource, "spec.annotations")
	labels := extractStringMap(resource, "spec.labels")
	finalizers := extractStringSlice(resource, "spec.finalizers")

	// Extract source
	repoURL, err := getStringValue(resource, "spec.source.repoURL")
	if err != nil {
		return fmt.Errorf("spec.source.repoURL is required: %w", err)
	}
	chart, _ := getStringValue(resource, "spec.source.chart")
	targetRevision, err := getStringValue(resource, "spec.source.targetRevision")
	if err != nil {
		return fmt.Errorf("spec.source.targetRevision is required: %w", err)
	}

	source := AppSource{
		RepoURL:        repoURL,
		Chart:          chart,
		TargetRevision: targetRevision,
	}

	// Extract helm config
	releaseName, _ := getStringValue(resource, "spec.source.helm.releaseName")
	valuesObject, _ := resource.GetValue("spec.source.helm.valuesObject")
	if releaseName != "" || valuesObject != nil {
		source.Helm = &HelmSource{
			ReleaseName:  releaseName,
			ValuesObject: valuesObject,
		}
	}

	// Extract destination
	destServer, err := getStringValue(resource, "spec.destination.server")
	if err != nil {
		return fmt.Errorf("spec.destination.server is required: %w", err)
	}
	destNamespace, err := getStringValue(resource, "spec.destination.namespace")
	if err != nil {
		return fmt.Errorf("spec.destination.namespace is required: %w", err)
	}

	// Extract sync policy
	syncPolicy, _ := resource.GetValue("spec.syncPolicy")

	// Build ArgoCD Application
	app := Resource{
		APIVersion: "argoproj.io/v1alpha1",
		Kind:       "Application",
		Metadata: ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Labels:      labels,
			Annotations: annotations,
			Finalizers:  finalizers,
		},
		Spec: ApplicationSpec{
			Project: project,
			Source:  source,
			Destination: Destination{
				Server:    destServer,
				Namespace: destNamespace,
			},
			SyncPolicy: syncPolicy,
		},
	}

	if err := writeYAML(sdk, "resources/application.yaml", app); err != nil {
		return fmt.Errorf("write application: %w", err)
	}
	log.Printf("✓ Rendered ArgoCD Application: %s", name)

	status := kratix.NewStatus()
	status.Set("phase", "Configured")
	status.Set("message", fmt.Sprintf("Application %s configured", name))
	status.Set("applicationName", name)
	status.Set("namespace", namespace)
	status.Set("project", project)

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
		Kind:       "Application",
		Metadata: ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}

	if err := writeYAML(sdk, "resources/delete-application.yaml", deleteObj); err != nil {
		return fmt.Errorf("write delete application: %w", err)
	}
	log.Printf("✓ Delete scheduled for Application: %s", name)

	status := kratix.NewStatus()
	status.Set("phase", "Deleting")
	status.Set("message", fmt.Sprintf("Application %s scheduled for deletion", name))

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
