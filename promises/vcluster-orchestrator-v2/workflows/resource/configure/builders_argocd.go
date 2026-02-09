package main

import "fmt"

func buildArgoCDProjectRequest(config *VClusterConfig) map[string]interface{} {
	metadataLabels := mergeStringMap(map[string]string{
		"app.kubernetes.io/name": "argocd-project",
	}, baseLabels(config, config.Name))

	specLabels := map[string]string{
		"app.kubernetes.io/managed-by":     "kratix",
		"argocd.argoproj.io/project-group": "appteam",
		"kratix.io/promise-name":           config.WorkflowContext.PromiseName,
		"kratix.io/resource-name":          config.Name,
	}

	return map[string]interface{}{
		"apiVersion": "platform.integratn.tech/v1alpha1",
		"kind":       "ArgoCDProject",
		"metadata": resourceMeta(
			config.ProjectName,
			config.Namespace,
			metadataLabels,
			nil,
		),
		"spec": map[string]interface{}{
			"namespace":   "argocd",
			"name":        config.ProjectName,
			"description": fmt.Sprintf("VCluster project for %s", config.Name),
			"annotations": map[string]string{
				"argocd.argoproj.io/sync-wave": "-1",
			},
			"labels": specLabels,
			"sourceRepos": []string{
				"https://charts.loft.sh",
			},
			"destinations": []map[string]interface{}{
				{
					"namespace": config.TargetNamespace,
					"server":    "https://kubernetes.default.svc",
				},
			},
			"clusterResourceWhitelist": []map[string]interface{}{
				{
					"group": "*",
					"kind":  "*",
				},
			},
			"namespaceResourceWhitelist": []map[string]interface{}{
				{
					"group": "*",
					"kind":  "*",
				},
			},
		},
	}
}

func buildArgoCDApplicationRequest(config *VClusterConfig) map[string]interface{} {
	metadataLabels := mergeStringMap(map[string]string{
		"app.kubernetes.io/name": "argocd-application",
	}, baseLabels(config, config.Name))

	spec := map[string]interface{}{
		"name":      fmt.Sprintf("vcluster-%s", config.Name),
		"namespace": "argocd",
		"annotations": map[string]string{
			"argocd.argoproj.io/sync-wave": "0",
		},
		"finalizers": []string{"resources-finalizer.argocd.argoproj.io"},
		"project":    config.ProjectName,
		"destination": map[string]interface{}{
			"server":    config.ArgoCDDestServer,
			"namespace": config.TargetNamespace,
		},
		"source": map[string]interface{}{
			"repoURL":        config.ArgoCDRepoURL,
			"chart":          config.ArgoCDChart,
			"targetRevision": config.ArgoCDTargetRevision,
			"helm": map[string]interface{}{
				"releaseName":  config.Name,
				"valuesObject": config.ValuesObject,
			},
		},
	}
	if config.ArgoCDSyncPolicy != nil {
		spec["syncPolicy"] = config.ArgoCDSyncPolicy
	}

	return map[string]interface{}{
		"apiVersion": "platform.integratn.tech/v1alpha1",
		"kind":       "ArgoCDApplication",
		"metadata": resourceMeta(
			fmt.Sprintf("vcluster-%s", config.Name),
			config.Namespace,
			metadataLabels,
			nil,
		),
		"spec": spec,
	}
}

func buildArgoCDClusterExternalSecret(config *VClusterConfig) map[string]interface{} {
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

	metadata := resourceMeta(
		fmt.Sprintf("%s-argocd-cluster", config.Name),
		"argocd",
		labels,
		metadataAnnotations,
	)

	targetMetadata := map[string]interface{}{
		"labels": targetLabels,
	}
	if len(targetAnnotations) > 0 {
		targetMetadata["annotations"] = targetAnnotations
	}

	return map[string]interface{}{
		"apiVersion": "external-secrets.io/v1beta1",
		"kind":       "ExternalSecret",
		"metadata":   metadata,
		"spec": map[string]interface{}{
			"secretStoreRef": map[string]interface{}{
				"name": "onepassword-store",
				"kind": "ClusterSecretStore",
			},
			"target": map[string]interface{}{
				"name": fmt.Sprintf("vcluster-%s", config.Name),
				"template": map[string]interface{}{
					"engineVersion": "v2",
					"type":         "Opaque",
					"metadata":     targetMetadata,
					"data": map[string]string{
						"name":   "{{ index . \"argocd-name\" }}",
						"server": "{{ index . \"argocd-server\" }}",
						"config": "{{ index . \"argocd-config\" }}",
					},
				},
			},
			"dataFrom": []map[string]interface{}{
				{
					"extract": map[string]interface{}{
						"key":                config.OnePasswordItem,
						"conversionStrategy": "Default",
						"decodingStrategy":   "None",
					},
				},
			},
			"refreshInterval": "15m",
		},
	}
}
