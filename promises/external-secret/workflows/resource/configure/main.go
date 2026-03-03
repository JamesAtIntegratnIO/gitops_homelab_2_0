package main

import (
	"fmt"
	"log"

	kratix "github.com/syntasso/kratix-go"

	u "github.com/jamesatintegratnio/gitops_homelab_2_0/promises/_shared/kratixutil"
)

const (
	defaultSecretStore     = "onepassword-store"
	defaultSecretStoreKind = "ClusterSecretStore"
)

// ExternalSecretConfig holds the resolved configuration from the CR.
type ExternalSecretConfig struct {
	AppName         string
	Namespace       string
	OwnerPromise    string
	SecretStoreName string
	SecretStoreKind string
	Secrets         []u.SecretRef
}

func main() {
	sdk := kratix.New()

	log.Printf("=== External Secret Promise Pipeline ===")
	log.Printf("Action: %s", sdk.WorkflowAction())

	resource, err := sdk.ReadResourceInput()
	if err != nil {
		log.Fatalf("ERROR: Failed to read resource input: %v", err)
	}

	log.Printf("Processing resource: %s/%s",
		resource.GetNamespace(), resource.GetName())

	config, err := buildConfig(resource)
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

func buildConfig(resource kratix.Resource) (*ExternalSecretConfig, error) {
	config := &ExternalSecretConfig{
		SecretStoreName: defaultSecretStore,
		SecretStoreKind: defaultSecretStoreKind,
	}

	var err error
	config.Namespace, err = u.GetStringValue(resource, "spec.namespace")
	if err != nil {
		return nil, fmt.Errorf("spec.namespace is required: %w", err)
	}

	// appName defaults to the resource name
	config.AppName, _ = u.GetStringValueWithDefault(resource, "spec.appName", resource.GetName())

	config.OwnerPromise, _ = u.GetStringValueWithDefault(resource, "spec.ownerPromise", "external-secret")

	if v, err := u.GetStringValue(resource, "spec.secretStoreName"); err == nil && v != "" {
		config.SecretStoreName = v
	}
	if v, err := u.GetStringValue(resource, "spec.secretStoreKind"); err == nil && v != "" {
		config.SecretStoreKind = v
	}

	config.Secrets = u.ExtractSecrets(resource, "spec.secrets")
	if len(config.Secrets) == 0 {
		return nil, fmt.Errorf("spec.secrets must contain at least one entry")
	}

	return config, nil
}

func handleConfigure(sdk *kratix.KratixSDK, config *ExternalSecretConfig) error {
	externalSecrets := buildExternalSecrets(config)
	if err := u.WriteYAMLDocuments(sdk, "resources/external-secrets.yaml", externalSecrets); err != nil {
		return fmt.Errorf("write ExternalSecrets: %w", err)
	}
	log.Printf("✓ Rendered %d ExternalSecret(s)", len(externalSecrets))

	// Write status
	status := kratix.NewStatus()
	status.Set("phase", "Configured")
	status.Set("message", fmt.Sprintf("Rendered %d ExternalSecret(s) in namespace %s", len(config.Secrets), config.Namespace))
	status.Set("namespace", config.Namespace)
	status.Set("secretCount", len(config.Secrets))

	if err := sdk.WriteStatus(status); err != nil {
		return fmt.Errorf("write status: %w", err)
	}

	return nil
}

func handleDelete(sdk *kratix.KratixSDK, config *ExternalSecretConfig) error {
	// Emit minimal resources for Kratix to know what to clean up
	for _, s := range config.Secrets {
		secretName := s.Name
		if secretName == "" {
			secretName = fmt.Sprintf("%s-%s", config.AppName, s.OnePasswordItem)
		}

		deleteObj := u.DeleteResource(
			"external-secrets.io/v1beta1",
			"ExternalSecret",
			secretName,
			config.Namespace,
		)

		path := fmt.Sprintf("resources/delete-externalsecret-%s.yaml", secretName)
		if err := u.WriteYAML(sdk, path, deleteObj); err != nil {
			return fmt.Errorf("write delete ExternalSecret %s: %w", secretName, err)
		}
	}

	status := kratix.NewStatus()
	status.Set("phase", "Deleting")
	status.Set("message", fmt.Sprintf("ExternalSecrets in %s scheduled for deletion", config.Namespace))

	if err := sdk.WriteStatus(status); err != nil {
		return fmt.Errorf("write status: %w", err)
	}

	return nil
}

func buildExternalSecrets(config *ExternalSecretConfig) []u.Resource {
	var resources []u.Resource

	for _, s := range config.Secrets {
		secretName := s.Name
		if secretName == "" {
			secretName = fmt.Sprintf("%s-%s", config.AppName, s.OnePasswordItem)
		}

		data := []map[string]interface{}{}
		for _, k := range s.Keys {
			data = append(data, map[string]interface{}{
				"secretKey": k.SecretKey,
				"remoteRef": map[string]interface{}{
					"key":      s.OnePasswordItem,
					"property": k.Property,
				},
			})
		}

		es := u.Resource{
			APIVersion: "external-secrets.io/v1beta1",
			Kind:       "ExternalSecret",
			Metadata: u.ObjectMeta{
				Name:      secretName,
				Namespace: config.Namespace,
				Labels: map[string]string{
					"app.kubernetes.io/managed-by": "kratix",
					"kratix.io/promise-name":       config.OwnerPromise,
					"app.kubernetes.io/part-of":    config.AppName,
				},
			},
			Spec: map[string]interface{}{
				"secretStoreRef": map[string]interface{}{
					"name": config.SecretStoreName,
					"kind": config.SecretStoreKind,
				},
				"target": map[string]interface{}{
					"name": secretName,
				},
				"data": data,
			},
		}

		resources = append(resources, es)
	}

	return resources
}
