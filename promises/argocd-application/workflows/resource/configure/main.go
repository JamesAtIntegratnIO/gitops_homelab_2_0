package main

import (
	"fmt"
	"log"

	kratix "github.com/syntasso/kratix-go"

	u "github.com/jamesatintegratnio/gitops_homelab_2_0/promises/_shared/kratixutil"
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
	name, err := u.GetStringValue(resource, "spec.name")
	if err != nil {
		return fmt.Errorf("spec.name is required: %w", err)
	}

	namespace, _ := u.GetStringValueWithDefault(resource, "spec.namespace", "argocd")
	project, err := u.GetStringValue(resource, "spec.project")
	if err != nil {
		return fmt.Errorf("spec.project is required: %w", err)
	}

	annotations := u.ExtractStringMap(resource, "spec.annotations")
	labels := u.ExtractStringMap(resource, "spec.labels")
	finalizers := u.ExtractStringSlice(resource, "spec.finalizers")

	// Extract source
	repoURL, err := u.GetStringValue(resource, "spec.source.repoURL")
	if err != nil {
		return fmt.Errorf("spec.source.repoURL is required: %w", err)
	}
	chart, _ := u.GetStringValue(resource, "spec.source.chart")
	targetRevision, err := u.GetStringValue(resource, "spec.source.targetRevision")
	if err != nil {
		return fmt.Errorf("spec.source.targetRevision is required: %w", err)
	}

	source := AppSource{
		RepoURL:        repoURL,
		Chart:          chart,
		TargetRevision: targetRevision,
	}

	// Extract helm config
	releaseName, _ := u.GetStringValue(resource, "spec.source.helm.releaseName")
	valuesObject, _ := resource.GetValue("spec.source.helm.valuesObject")
	if releaseName != "" || valuesObject != nil {
		source.Helm = &HelmSource{
			ReleaseName:  releaseName,
			ValuesObject: valuesObject,
		}
	}

	// Extract destination
	destServer, err := u.GetStringValue(resource, "spec.destination.server")
	if err != nil {
		return fmt.Errorf("spec.destination.server is required: %w", err)
	}
	destNamespace, err := u.GetStringValue(resource, "spec.destination.namespace")
	if err != nil {
		return fmt.Errorf("spec.destination.namespace is required: %w", err)
	}

	// Extract sync policy
	syncPolicy, _ := resource.GetValue("spec.syncPolicy")

	// Build ArgoCD Application
	app := u.Resource{
		APIVersion: "argoproj.io/v1alpha1",
		Kind:       "Application",
		Metadata: u.ObjectMeta{
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

	if err := u.WriteYAML(sdk, "resources/application.yaml", app); err != nil {
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
	name, err := u.GetStringValue(resource, "spec.name")
	if err != nil {
		return fmt.Errorf("spec.name is required: %w", err)
	}

	namespace, _ := u.GetStringValueWithDefault(resource, "spec.namespace", "argocd")

	deleteObj := u.Resource{
		APIVersion: "argoproj.io/v1alpha1",
		Kind:       "Application",
		Metadata: u.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}

	if err := u.WriteYAML(sdk, "resources/delete-application.yaml", deleteObj); err != nil {
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
