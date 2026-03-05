package main

import (
	"fmt"

	kratix "github.com/syntasso/kratix-go"

	ku "github.com/jamesatintegratnio/gitops_homelab_2_0/promises/_shared/kratixutil"
)

func main() {
	ku.RunPromiseWithConfig("ArgoCD Application", buildConfig, handleConfigure, handleDelete)
}

func buildConfig(_ *kratix.KratixSDK, resource kratix.Resource) (*AppConfig, error) {
	config := &AppConfig{}

	var err error
	config.Name, err = ku.GetStringValue(resource, "spec.name")
	if err != nil {
		return nil, fmt.Errorf("spec.name is required: %w", err)
	}

	config.Namespace = ku.GetStringValueWithDefault(resource, "spec.namespace", "argocd")

	config.Project, err = ku.GetStringValue(resource, "spec.project")
	if err != nil {
		return nil, fmt.Errorf("spec.project is required: %w", err)
	}

	config.Annotations = ku.ExtractStringMap(resource, "spec.annotations")
	config.Labels = ku.ExtractStringMap(resource, "spec.labels")
	config.Finalizers = ku.ExtractStringSlice(resource, "spec.finalizers")

	// Extract source
	repoURL, err := ku.GetStringValue(resource, "spec.source.repoURL")
	if err != nil {
		return nil, fmt.Errorf("spec.source.repoURL is required: %w", err)
	}
	chart, _ := ku.GetStringValue(resource, "spec.source.chart")
	targetRevision, err := ku.GetStringValue(resource, "spec.source.targetRevision")
	if err != nil {
		return nil, fmt.Errorf("spec.source.targetRevision is required: %w", err)
	}

	config.Source = ku.AppSource{
		RepoURL:        repoURL,
		Chart:          chart,
		TargetRevision: targetRevision,
	}

	// Extract helm config
	releaseName, _ := ku.GetStringValue(resource, "spec.source.helm.releaseName")
	valuesObject, _ := resource.GetValue("spec.source.helm.valuesObject")
	if releaseName != "" || valuesObject != nil {
		config.Source.Helm = &ku.HelmSource{
			ReleaseName:  releaseName,
			ValuesObject: valuesObject,
		}
	}

	// Extract destination
	destServer, err := ku.GetStringValue(resource, "spec.destination.server")
	if err != nil {
		return nil, fmt.Errorf("spec.destination.server is required: %w", err)
	}
	destNamespace, err := ku.GetStringValue(resource, "spec.destination.namespace")
	if err != nil {
		return nil, fmt.Errorf("spec.destination.namespace is required: %w", err)
	}

	config.Destination = ku.Destination{
		Server:    destServer,
		Namespace: destNamespace,
	}

	// Extract sync policy
	if raw, _ := resource.GetValue("spec.syncPolicy"); raw != nil {
		parsed, parseErr := ku.ParseSyncPolicyE(raw)
		if parseErr != nil {
			return nil, fmt.Errorf("spec.syncPolicy: %w", parseErr)
		}
		config.SyncPolicy = parsed
	}

	return config, nil
}

func handleConfigure(sdk *kratix.KratixSDK, config *AppConfig) error {
	// Build ArgoCD Application
	app := ku.Resource{
		APIVersion: "argoproj.io/v1alpha1",
		Kind:       "Application",
		Metadata: ku.ObjectMeta{
			Name:        config.Name,
			Namespace:   config.Namespace,
			Labels:      config.Labels,
			Annotations: config.Annotations,
			Finalizers:  config.Finalizers,
		},
		Spec: ApplicationSpec{
			Project:     config.Project,
			Source:      config.Source,
			Destination: config.Destination,
			SyncPolicy:  config.SyncPolicy,
		},
	}

	if err := ku.WriteYAML(sdk, "resources/application.yaml", app); err != nil {
		return fmt.Errorf("write application: %w", err)
	}

	if err := ku.WritePromiseStatus(sdk, "Configured",
		fmt.Sprintf("Application %s configured", config.Name),
		map[string]interface{}{"applicationName": config.Name, "namespace": config.Namespace, "project": config.Project}); err != nil {
		return fmt.Errorf("failed to write status: %w", err)
	}

	return nil
}

func handleDelete(sdk *kratix.KratixSDK, config *AppConfig) error {
	deleteObj := ku.DeleteFromResource(ku.Resource{
		APIVersion: "argoproj.io/v1alpha1",
		Kind:       "Application",
		Metadata: ku.ObjectMeta{
			Name:      config.Name,
			Namespace: config.Namespace,
		},
	})

	if err := ku.WriteYAML(sdk, "resources/delete-application.yaml", deleteObj); err != nil {
		return fmt.Errorf("write delete application: %w", err)
	}

	if err := ku.WritePromiseStatus(sdk, "Deleting",
		fmt.Sprintf("Application %s scheduled for deletion", config.Name), nil); err != nil {
		return fmt.Errorf("failed to write status: %w", err)
	}

	return nil
}
