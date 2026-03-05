package main

import (
	"fmt"
	"log"

	kratix "github.com/syntasso/kratix-go"

	u "github.com/jamesatintegratnio/gitops_homelab_2_0/promises/_shared/kratixutil"
)

func main() {
	u.RunPromise("ArgoCD Project Promise Pipeline", handleConfigure, handleDelete)
}

func handleConfigure(sdk *kratix.KratixSDK, resource kratix.Resource) error {
	name, err := u.GetStringValue(resource, "spec.name")
	if err != nil {
		return fmt.Errorf("spec.name is required: %w", err)
	}

	namespace := u.GetStringValueWithDefault(resource, "spec.namespace", "argocd")
	description, _ := u.GetStringValue(resource, "spec.description")

	annotations := u.ExtractStringMap(resource, "spec.annotations")
	labels := u.ExtractStringMap(resource, "spec.labels")
	sourceRepos := u.ExtractStringSlice(resource, "spec.sourceRepos")
	destinations := toProjectDestinations(u.ExtractObjectSlice(resource, "spec.destinations"))
	clusterResourceWhitelist := toResourceFilters(u.ExtractObjectSlice(resource, "spec.clusterResourceWhitelist"))
	namespaceResourceWhitelist := toResourceFilters(u.ExtractObjectSlice(resource, "spec.namespaceResourceWhitelist"))

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

	namespace := u.GetStringValueWithDefault(resource, "spec.namespace", "argocd")

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

// toProjectDestinations converts untyped maps to typed ProjectDestination values.
func toProjectDestinations(raw []map[string]interface{}) []u.ProjectDestination {
	if raw == nil {
		return nil
	}
	result := make([]u.ProjectDestination, 0, len(raw))
	for _, m := range raw {
		d := u.ProjectDestination{}
		if v, ok := m["namespace"].(string); ok {
			d.Namespace = v
		}
		if v, ok := m["server"].(string); ok {
			d.Server = v
		}
		result = append(result, d)
	}
	return result
}

// toResourceFilters converts untyped maps to typed ResourceFilter values.
func toResourceFilters(raw []map[string]interface{}) []u.ResourceFilter {
	if raw == nil {
		return nil
	}
	result := make([]u.ResourceFilter, 0, len(raw))
	for _, m := range raw {
		f := u.ResourceFilter{}
		if v, ok := m["group"].(string); ok {
			f.Group = v
		}
		if v, ok := m["kind"].(string); ok {
			f.Kind = v
		}
		result = append(result, f)
	}
	return result
}
