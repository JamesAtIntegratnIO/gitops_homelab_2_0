package main

import "fmt"

func buildArgoCDProjectRequest(config *VClusterConfig) Resource {
	metadataLabels := mergeStringMap(map[string]string{
		"app.kubernetes.io/name": "argocd-project",
	}, baseLabels(config, config.Name))

	specLabels := map[string]string{
		"app.kubernetes.io/managed-by":     "kratix",
		"argocd.argoproj.io/project-group": "appteam",
		"kratix.io/promise-name":           config.WorkflowContext.PromiseName,
		"kratix.io/resource-name":          config.Name,
	}

	return Resource{
		APIVersion: "platform.integratn.tech/v1alpha1",
		Kind:       "ArgoCDProject",
		Metadata: resourceMeta(
			config.ProjectName,
			config.Namespace,
			metadataLabels,
			nil,
		),
		Spec: ArgoCDProjectSpec{
			Namespace:   "argocd",
			Name:        config.ProjectName,
			Description: fmt.Sprintf("VCluster project for %s", config.Name),
			Annotations: map[string]string{
				"argocd.argoproj.io/sync-wave": "-1",
			},
			Labels:      specLabels,
			SourceRepos: []string{"https://charts.loft.sh"},
			Destinations: []ProjectDestination{
				{
					Namespace: config.TargetNamespace,
					Server:    "https://kubernetes.default.svc",
				},
			},
			ClusterResourceWhitelist: []ResourceFilter{
				{Group: "*", Kind: "*"},
			},
			NamespaceResourceWhitelist: []ResourceFilter{
				{Group: "*", Kind: "*"},
			},
		},
	}
}

func buildArgoCDApplicationRequest(config *VClusterConfig) Resource {
	metadataLabels := mergeStringMap(map[string]string{
		"app.kubernetes.io/name": "argocd-application",
	}, baseLabels(config, config.Name))

	spec := ArgoCDApplicationSpec{
		Name:      fmt.Sprintf("vcluster-%s", config.Name),
		Namespace: "argocd",
		Annotations: map[string]string{
			"argocd.argoproj.io/sync-wave": "0",
		},
		Finalizers: []string{"resources-finalizer.argocd.argoproj.io"},
		Project:    config.ProjectName,
		Destination: Destination{
			Server:    config.ArgoCDDestServer,
			Namespace: config.TargetNamespace,
		},
		Source: AppSource{
			RepoURL:        config.ArgoCDRepoURL,
			Chart:          config.ArgoCDChart,
			TargetRevision: config.ArgoCDTargetRevision,
			Helm: &HelmSource{
				ReleaseName:  config.Name,
				ValuesObject: config.ValuesObject,
			},
		},
		SyncPolicy: config.ArgoCDSyncPolicy,
	}

	return Resource{
		APIVersion: "platform.integratn.tech/v1alpha1",
		Kind:       "ArgoCDApplication",
		Metadata: resourceMeta(
			fmt.Sprintf("vcluster-%s", config.Name),
			config.Namespace,
			metadataLabels,
			nil,
		),
		Spec: spec,
	}
}

func buildArgoCDClusterExternalSecret(config *VClusterConfig) Resource {
	labels := mergeStringMap(map[string]string{
		"app.kubernetes.io/name":         "external-secret",
		"app.kubernetes.io/component":    "argocd-cluster",
		"argocd.argoproj.io/secret-type": "cluster",
	}, baseLabels(config, config.Name))
	labels = mergeStringMap(labels, config.ArgoCDClusterLabels)

	metadataAnnotations := map[string]string{}
	if len(config.ArgoCDClusterAnnotations) > 0 {
		metadataAnnotations = mergeStringMap(metadataAnnotations, config.ArgoCDClusterAnnotations)
	}

	targetLabels := mergeStringMap(map[string]string{
		"argocd.argoproj.io/secret-type": "cluster",
		"integratn.tech/vcluster-name":  config.Name,
		"integratn.tech/environment":    config.ArgoCDEnvironment,
	}, config.ArgoCDClusterLabels)

	targetAnnotations := map[string]string{}
	if len(config.ArgoCDClusterAnnotations) > 0 {
		targetAnnotations = mergeStringMap(targetAnnotations, config.ArgoCDClusterAnnotations)
	}

	tmplMeta := &TemplateMetadata{
		Labels: targetLabels,
	}
	if len(targetAnnotations) > 0 {
		tmplMeta.Annotations = targetAnnotations
	}

	return Resource{
		APIVersion: "external-secrets.io/v1beta1",
		Kind:       "ExternalSecret",
		Metadata: resourceMeta(
			fmt.Sprintf("%s-argocd-cluster", config.Name),
			"argocd",
			labels,
			metadataAnnotations,
		),
		Spec: ExternalSecretSpec{
			SecretStoreRef: SecretStoreRef{
				Name: "onepassword-store",
				Kind: "ClusterSecretStore",
			},
			Target: ExternalSecretTarget{
				Name: fmt.Sprintf("vcluster-%s", config.Name),
				Template: &ExternalSecretTemplate{
					EngineVersion: "v2",
					Type:          "Opaque",
					Metadata:      tmplMeta,
					Data: map[string]string{
						"name":   "{{ index . \"argocd-name\" }}",
						"server": "{{ index . \"argocd-server\" }}",
						"config": "{{ index . \"argocd-config\" }}",
					},
				},
			},
			DataFrom: []ExternalSecretDataFrom{
				{
					Extract: &ExternalSecretExtract{
						Key:                config.OnePasswordItem,
						ConversionStrategy: "Default",
						DecodingStrategy:   "None",
					},
				},
			},
			RefreshInterval: "15m",
		},
	}
}
