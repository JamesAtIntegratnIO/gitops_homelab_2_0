package main

import (
	"fmt"
	"log"

	kratix "github.com/syntasso/kratix-go"

	ku "github.com/jamesatintegratnio/gitops_homelab_2_0/promises/_shared/kratixutil"
)

func main() {
	ku.RunPromiseWithConfig("External Secret", buildConfig, handleConfigure, handleDelete)
}

func buildConfig(_ *kratix.KratixSDK, resource kratix.Resource) (*ExternalSecretConfig, error) {
	config := &ExternalSecretConfig{
		SecretStoreName: ku.DefaultSecretStoreName,
		SecretStoreKind: ku.DefaultSecretStoreKind,
	}

	var err error
	config.Namespace, err = ku.GetStringValue(resource, "spec.namespace")
	if err != nil {
		return nil, fmt.Errorf("spec.namespace is required: %w", err)
	}

	// appName defaults to the resource name
	config.AppName = ku.GetStringValueWithDefault(resource, "spec.appName", resource.GetName())

	config.OwnerPromise = ku.GetStringValueWithDefault(resource, "spec.ownerPromise", "external-secret")

	if v, err := ku.GetStringValue(resource, "spec.secretStoreName"); err == nil && v != "" {
		config.SecretStoreName = v
	}
	if v, err := ku.GetStringValue(resource, "spec.secretStoreKind"); err == nil && v != "" {
		config.SecretStoreKind = v
	}

	config.Secrets = ku.ExtractSecrets(resource, "spec.secrets")
	if len(config.Secrets) == 0 {
		return nil, fmt.Errorf("spec.secrets must contain at least one entry")
	}

	return config, nil
}

func handleConfigure(sdk *kratix.KratixSDK, config *ExternalSecretConfig) error {
	externalSecrets := buildExternalSecrets(config)
	if err := ku.WriteYAMLDocuments(sdk, "resources/external-secrets.yaml", externalSecrets); err != nil {
		return fmt.Errorf("write ExternalSecrets: %w", err)
	}
	log.Printf("✓ Rendered %d ExternalSecret(s)", len(externalSecrets))

	if err := ku.WritePromiseStatus(sdk, "Configured",
		fmt.Sprintf("Rendered %d ExternalSecret(s) in namespace %s", len(config.Secrets), config.Namespace),
		map[string]interface{}{"namespace": config.Namespace, "secretCount": len(config.Secrets)}); err != nil {
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

		deleteObj := ku.DeleteResource(
			"external-secrets.io/v1beta1",
			"ExternalSecret",
			secretName,
			config.Namespace,
		)

		path := fmt.Sprintf("resources/delete-externalsecret-%s.yaml", secretName)
		if err := ku.WriteYAML(sdk, path, deleteObj); err != nil {
			return fmt.Errorf("write delete ExternalSecret %s: %w", secretName, err)
		}
	}

	if err := ku.WritePromiseStatus(sdk, "Deleting",
		fmt.Sprintf("ExternalSecrets in %s scheduled for deletion", config.Namespace), nil); err != nil {
		return fmt.Errorf("write status: %w", err)
	}

	return nil
}

func buildExternalSecrets(config *ExternalSecretConfig) []ku.Resource {
	var resources []ku.Resource

	for _, s := range config.Secrets {
		secretName := s.Name
		if secretName == "" {
			secretName = fmt.Sprintf("%s-%s", config.AppName, s.OnePasswordItem)
		}

		var data []ku.ExternalSecretData
		for _, k := range s.Keys {
			data = append(data, ku.ExternalSecretData{
				SecretKey: k.SecretKey,
				RemoteRef: ku.RemoteRef{
					Key:      s.OnePasswordItem,
					Property: k.Property,
				},
			})
		}

		es := ku.Resource{
			APIVersion: "external-secrets.io/v1beta1",
			Kind:       "ExternalSecret",
			Metadata: ku.ObjectMeta{
				Name:      secretName,
				Namespace: config.Namespace,
				Labels:    ku.BaseLabels(config.OwnerPromise, config.AppName),
			},
			Spec: ku.ExternalSecretSpec{
				SecretStoreRef: ku.SecretStoreRef{
					Name: config.SecretStoreName,
					Kind: config.SecretStoreKind,
				},
				Target: ku.ExternalSecretTarget{
					Name: secretName,
				},
				Data: data,
			},
		}

		resources = append(resources, es)
	}

	return resources
}
