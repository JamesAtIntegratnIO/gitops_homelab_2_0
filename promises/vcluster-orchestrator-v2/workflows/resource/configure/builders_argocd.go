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

func buildArgoCDClusterRegistrationRequest(config *VClusterConfig) Resource {
	metadataLabels := mergeStringMap(map[string]string{
		"app.kubernetes.io/name": "argocd-cluster-registration",
	}, baseLabels(config, config.Name))

	spec := ArgoCDClusterRegistrationSpec{
		Name:              config.Name,
		TargetNamespace:   config.TargetNamespace,
		KubeconfigSecret:  fmt.Sprintf("vc-%s", config.Name),
		ExternalServerURL: config.ExternalServerURL,
		Environment:       config.ArgoCDEnvironment,
		BaseDomain:        config.BaseDomain,
		BaseDomainSanitized: config.BaseDomainSanitized,
		ClusterLabels:     config.ArgoCDClusterLabels,
		ClusterAnnotations: config.ArgoCDClusterAnnotations,
		SyncJobName:       config.KubeconfigSyncJobName,
	}

	return Resource{
		APIVersion: "platform.integratn.tech/v1alpha1",
		Kind:       "ArgoCDClusterRegistration",
		Metadata: resourceMeta(
			fmt.Sprintf("%s-cluster-registration", config.Name),
			config.Namespace,
			metadataLabels,
			nil,
		),
		Spec: spec,
	}
}
