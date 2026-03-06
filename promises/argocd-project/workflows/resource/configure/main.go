package main

import (
	"fmt"
	"log"

	kratix "github.com/syntasso/kratix-go"

	ku "github.com/jamesatintegratnio/gitops_homelab_2_0/promises/_shared/kratixutil"
)

func main() {
	ku.RunPromiseWithConfig("ArgoCD Project", buildConfig, handleConfigure, handleDelete)
}

func buildConfig(_ *kratix.KratixSDK, resource kratix.Resource) (*ProjectConfig, error) {
	config := &ProjectConfig{}

	var err error
	config.Name, err = ku.GetStringValue(resource, "spec.name")
	if err != nil {
		return nil, fmt.Errorf("spec.name is required: %w", err)
	}

	config.Namespace = ku.GetStringValueWithDefault(resource, "spec.namespace", "argocd")
	config.Description, err = ku.GetOptionalStringValue(resource, "spec.description")
	if err != nil {
		return nil, err
	}
	config.Annotations, err = ku.ExtractStringMapFromResource(resource, "spec.annotations")
	if err != nil {
		return nil, err
	}
	config.Labels, err = ku.ExtractStringMapFromResource(resource, "spec.labels")
	if err != nil {
		return nil, err
	}
	config.SourceRepos, err = ku.ExtractStringSliceFromResource(resource, "spec.sourceRepos")
	if err != nil {
		return nil, err
	}
	rawDestinations, err := ku.ExtractObjectSliceFromResource(resource, "spec.destinations")
	if err != nil {
		return nil, err
	}
	config.Destinations = toProjectDestinations(rawDestinations)
	rawClusterResourceWhitelist, err := ku.ExtractObjectSliceFromResource(resource, "spec.clusterResourceWhitelist")
	if err != nil {
		return nil, err
	}
	config.ClusterResourceWhitelist = toResourceFilters(rawClusterResourceWhitelist)
	rawNamespaceResourceWhitelist, err := ku.ExtractObjectSliceFromResource(resource, "spec.namespaceResourceWhitelist")
	if err != nil {
		return nil, err
	}
	config.NamespaceResourceWhitelist = toResourceFilters(rawNamespaceResourceWhitelist)

	return config, nil
}

func handleConfigure(sdk *kratix.KratixSDK, config *ProjectConfig) error {
	// Build the ArgoCD AppProject
	project := ku.Resource{
		APIVersion: "argoproj.io/v1alpha1",
		Kind:       "AppProject",
		Metadata: ku.ObjectMeta{
			Name:        config.Name,
			Namespace:   config.Namespace,
			Labels:      config.Labels,
			Annotations: config.Annotations,
		},
		Spec: AppProjectSpec{
			Description:                config.Description,
			SourceRepos:                config.SourceRepos,
			Destinations:               config.Destinations,
			ClusterResourceWhitelist:   config.ClusterResourceWhitelist,
			NamespaceResourceWhitelist: config.NamespaceResourceWhitelist,
		},
	}

	if err := ku.WriteYAML(sdk, "resources/appproject.yaml", project); err != nil {
		return fmt.Errorf("write appproject: %w", err)
	}

	if err := ku.WritePromiseStatus(sdk, ku.PhaseConfigured,
		fmt.Sprintf("AppProject %s configured", config.Name),
		map[string]interface{}{"projectName": config.Name, "namespace": config.Namespace}); err != nil {
		return fmt.Errorf("failed to write status: %w", err)
	}

	return nil
}

func handleDelete(sdk *kratix.KratixSDK, config *ProjectConfig) error {
	deleteObj := ku.DeleteFromResource(ku.Resource{
		APIVersion: "argoproj.io/v1alpha1",
		Kind:       "AppProject",
		Metadata: ku.ObjectMeta{
			Name:      config.Name,
			Namespace: config.Namespace,
		},
	})

	if err := ku.WriteYAML(sdk, "resources/delete-appproject.yaml", deleteObj); err != nil {
		return fmt.Errorf("write delete appproject: %w", err)
	}

	if err := ku.WritePromiseStatus(sdk, ku.PhaseDeleting,
		fmt.Sprintf("AppProject %s scheduled for deletion", config.Name), nil); err != nil {
		return fmt.Errorf("failed to write status: %w", err)
	}

	return nil
}

// toProjectDestinations converts untyped maps to typed ProjectDestination values.
func toProjectDestinations(raw []map[string]interface{}) []ku.ProjectDestination {
	if raw == nil {
		return nil
	}
	result := make([]ku.ProjectDestination, 0, len(raw))
	for i, m := range raw {
		d := ku.ProjectDestination{}
		if v, ok := m["namespace"]; ok {
			if s, sOk := v.(string); sOk {
				d.Namespace = s
			} else {
				log.Printf("WARNING: spec.destinations[%d].namespace: expected string, got %T — skipping field", i, v)
			}
		}
		if v, ok := m["server"]; ok {
			if s, sOk := v.(string); sOk {
				d.Server = s
			} else {
				log.Printf("WARNING: spec.destinations[%d].server: expected string, got %T — skipping field", i, v)
			}
		}
		result = append(result, d)
	}
	return result
}

// toResourceFilters converts untyped maps to typed ResourceFilter values.
func toResourceFilters(raw []map[string]interface{}) []ku.ResourceFilter {
	if raw == nil {
		return nil
	}
	result := make([]ku.ResourceFilter, 0, len(raw))
	for i, m := range raw {
		f := ku.ResourceFilter{}
		if v, ok := m["group"]; ok {
			if s, sOk := v.(string); sOk {
				f.Group = s
			} else {
				log.Printf("WARNING: resourceFilter[%d].group: expected string, got %T — skipping field", i, v)
			}
		}
		if v, ok := m["kind"]; ok {
			if s, sOk := v.(string); sOk {
				f.Kind = s
			} else {
				log.Printf("WARNING: resourceFilter[%d].kind: expected string, got %T — skipping field", i, v)
			}
		}
		result = append(result, f)
	}
	return result
}
