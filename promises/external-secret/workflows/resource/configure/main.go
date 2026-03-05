package main

import (
	"fmt"
	"log"

	kratix "github.com/syntasso/kratix-go"

	u "github.com/jamesatintegratnio/gitops_homelab_2_0/promises/_shared/kratixutil"
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
	u.RunPromiseWithConfig("External Secret", buildConfig, handleConfigure, handleDelete)
}

func buildConfig(_ *kratix.KratixSDK, resource kratix.Resource) (*ExternalSecretConfig, error) {
	config := &ExternalSecretConfig{
		SecretStoreName: u.DefaultSecretStoreName,
		SecretStoreKind: u.DefaultSecretStoreKind,
	}

	var err error
	config.Namespace, err = u.GetStringValue(resource, "spec.namespace")
	if err != nil {
		return nil, fmt.Errorf("spec.namespace is required: %w", err)
	}

	// appName defaults to the resource name
	config.AppName = u.GetStringValueWithDefault(resource, "spec.appName", resource.GetName())

	config.OwnerPromise = u.GetStringValueWithDefault(resource, "spec.ownerPromise", "external-secret")

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

		var data []u.ExternalSecretData
		for _, k := range s.Keys {
			data = append(data, u.ExternalSecretData{
				SecretKey: k.SecretKey,
				RemoteRef: u.RemoteRef{
					Key:      s.OnePasswordItem,
					Property: k.Property,
				},
			})
		}

		es := u.Resource{
			APIVersion: "external-secrets.io/v1beta1",
			Kind:       "ExternalSecret",
			Metadata: u.ObjectMeta{
				Name:      secretName,
				Namespace: config.Namespace,
				Labels:    u.BaseLabels(config.OwnerPromise, config.AppName),
			},
			Spec: u.ExternalSecretSpec{
				SecretStoreRef: u.SecretStoreRef{
					Name: config.SecretStoreName,
					Kind: config.SecretStoreKind,
				},
				Target: u.ExternalSecretTarget{
					Name: secretName,
				},
				Data: data,
			},
		}

		resources = append(resources, es)
	}

	return resources
}
