package main

import (
	"fmt"
	"log"

	kratix "github.com/syntasso/kratix-go"

	ku "github.com/jamesatintegratnio/gitops_homelab_2_0/promises/_shared/kratixutil"
)

func main() {
	ku.RunPromise("ArgoCD Application Promise Pipeline", handleConfigure, handleDelete)
}

func handleConfigure(sdk *kratix.KratixSDK, resource kratix.Resource) error {
	name, err := ku.GetStringValue(resource, "spec.name")
	if err != nil {
		return fmt.Errorf("spec.name is required: %w", err)
	}

	namespace := ku.GetStringValueWithDefault(resource, "spec.namespace", "argocd")
	project, err := ku.GetStringValue(resource, "spec.project")
	if err != nil {
		return fmt.Errorf("spec.project is required: %w", err)
	}

	annotations := ku.ExtractStringMap(resource, "spec.annotations")
	labels := ku.ExtractStringMap(resource, "spec.labels")
	finalizers := ku.ExtractStringSlice(resource, "spec.finalizers")

	// Extract source
	repoURL, err := ku.GetStringValue(resource, "spec.source.repoURL")
	if err != nil {
		return fmt.Errorf("spec.source.repoURL is required: %w", err)
	}
	chart, _ := ku.GetStringValue(resource, "spec.source.chart")
	targetRevision, err := ku.GetStringValue(resource, "spec.source.targetRevision")
	if err != nil {
		return fmt.Errorf("spec.source.targetRevision is required: %w", err)
	}

	source := ku.AppSource{
		RepoURL:        repoURL,
		Chart:          chart,
		TargetRevision: targetRevision,
	}

	// Extract helm config
	releaseName, _ := ku.GetStringValue(resource, "spec.source.helm.releaseName")
	valuesObject, _ := resource.GetValue("spec.source.helm.valuesObject")
	if releaseName != "" || valuesObject != nil {
		source.Helm = &ku.HelmSource{
			ReleaseName:  releaseName,
			ValuesObject: valuesObject,
		}
	}

	// Extract destination
	destServer, err := ku.GetStringValue(resource, "spec.destination.server")
	if err != nil {
		return fmt.Errorf("spec.destination.server is required: %w", err)
	}
	destNamespace, err := ku.GetStringValue(resource, "spec.destination.namespace")
	if err != nil {
		return fmt.Errorf("spec.destination.namespace is required: %w", err)
	}

	// Extract sync policy
	var syncPolicy *ku.SyncPolicy
	if raw, _ := resource.GetValue("spec.syncPolicy"); raw != nil {
		syncPolicy = parseSyncPolicy(raw)
	}

	// Build ArgoCD Application
	app := ku.Resource{
		APIVersion: "argoproj.io/v1alpha1",
		Kind:       "Application",
		Metadata: ku.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Labels:      labels,
			Annotations: annotations,
			Finalizers:  finalizers,
		},
		Spec: ApplicationSpec{
			Project: project,
			Source:  source,
			Destination: ku.Destination{
				Server:    destServer,
				Namespace: destNamespace,
			},
			SyncPolicy: syncPolicy,
		},
	}

	if err := ku.WriteYAML(sdk, "resources/application.yaml", app); err != nil {
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

// parseSyncPolicy converts an untyped map (from resource.GetValue) into a typed SyncPolicy.
func parseSyncPolicy(raw interface{}) *ku.SyncPolicy {
	m, ok := raw.(map[string]interface{})
	if !ok {
		return nil
	}
	sp := &ku.SyncPolicy{}
	if automated, ok := m["automated"].(map[string]interface{}); ok {
		sp.Automated = &ku.AutomatedSync{}
		if v, ok := automated["selfHeal"].(bool); ok {
			sp.Automated.SelfHeal = v
		}
		if v, ok := automated["prune"].(bool); ok {
			sp.Automated.Prune = v
		}
	}
	if opts, ok := m["syncOptions"].([]interface{}); ok {
		for _, o := range opts {
			if s, ok := o.(string); ok {
				sp.SyncOptions = append(sp.SyncOptions, s)
			}
		}
	}
	return sp
}

func handleDelete(sdk *kratix.KratixSDK, resource kratix.Resource) error {
	name, err := ku.GetStringValue(resource, "spec.name")
	if err != nil {
		return fmt.Errorf("spec.name is required: %w", err)
	}

	namespace := ku.GetStringValueWithDefault(resource, "spec.namespace", "argocd")

	deleteObj := ku.Resource{
		APIVersion: "argoproj.io/v1alpha1",
		Kind:       "Application",
		Metadata: ku.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}

	if err := ku.WriteYAML(sdk, "resources/delete-application.yaml", deleteObj); err != nil {
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
