package main

import (
	"fmt"
	"log"

	kratix "github.com/syntasso/kratix-go"

	u "github.com/jamesatintegratnio/gitops_homelab_2_0/promises/_shared/kratixutil"
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
	name, err := u.GetStringValue(resource, "spec.name")
	if err != nil {
		return fmt.Errorf("spec.name is required: %w", err)
	}

	namespace, _ := u.GetStringValueWithDefault(resource, "spec.namespace", "argocd")
	description, _ := u.GetStringValue(resource, "spec.description")

	annotations := u.ExtractStringMap(resource, "spec.annotations")
	labels := u.ExtractStringMap(resource, "spec.labels")
	sourceRepos := u.ExtractStringSlice(resource, "spec.sourceRepos")
	destinations := u.ExtractObjectSlice(resource, "spec.destinations")
	clusterResourceWhitelist := u.ExtractObjectSlice(resource, "spec.clusterResourceWhitelist")
	namespaceResourceWhitelist := u.ExtractObjectSlice(resource, "spec.namespaceResourceWhitelist")

	// Build the ArgoCD AppProject
	project := u.Resource{
		APIVersion: "argoproj.io/v1alpha1",
		Kind:       "AppProject",
		Metadata: u.ObjectMeta{
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

	if err := u.WriteYAML(sdk, "resources/appproject.yaml", project); err != nil {
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
	name, err := u.GetStringValue(resource, "spec.name")
	if err != nil {
		return fmt.Errorf("spec.name is required: %w", err)
	}

	namespace, _ := u.GetStringValueWithDefault(resource, "spec.namespace", "argocd")

	deleteObj := u.Resource{
		APIVersion: "argoproj.io/v1alpha1",
		Kind:       "AppProject",
		Metadata: u.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}

	if err := u.WriteYAML(sdk, "resources/delete-appproject.yaml", deleteObj); err != nil {
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
