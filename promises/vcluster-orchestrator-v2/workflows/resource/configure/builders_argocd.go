package main

import (
	"fmt"

	ku "github.com/jamesatintegratnio/gitops_homelab_2_0/promises/_shared/kratixutil"
)

func buildArgoCDProjectRequest(config *VClusterConfig) ku.Resource {
	metadataLabels := ku.MergeStringMap(map[string]string{
		"app.kubernetes.io/name": "argocd-project",
	}, ku.BaseLabels(config.WorkflowContext.PromiseName, config.Name))

	specLabels := map[string]string{
		"app.kubernetes.io/managed-by":     "kratix",
		"argocd.argoproj.io/project-group": "appteam",
		"kratix.io/promise-name":           config.WorkflowContext.PromiseName,
		"kratix.io/resource-name":          config.Name,
	}

	return ku.Resource{
		APIVersion: "platform.integratn.tech/v1alpha1",
		Kind:       "ArgoCDProject",
		Metadata: ku.ResourceMeta(
			config.ProjectName,
			config.Namespace,
			metadataLabels,
			nil,
		),
		Spec: ku.ArgoCDProjectSpec{
			Namespace:   "argocd",
			Name:        config.ProjectName,
			Description: fmt.Sprintf("VCluster project for %s", config.Name),
			Annotations: map[string]string{
				"argocd.argoproj.io/sync-wave": "-1",
			},
			Labels:      specLabels,
			SourceRepos: []string{"https://charts.loft.sh"},
			Destinations: []ku.ProjectDestination{
				{
					Namespace: config.TargetNamespace,
					Server:    "https://kubernetes.default.svc",
				},
			},
			ClusterResourceWhitelist: []ku.ResourceFilter{
				{Group: "*", Kind: "*"},
			},
			NamespaceResourceWhitelist: []ku.ResourceFilter{
				{Group: "*", Kind: "*"},
			},
		},
	}
}

func buildArgoCDApplicationRequest(config *VClusterConfig) ku.Resource {
	metadataLabels := ku.MergeStringMap(map[string]string{
		"app.kubernetes.io/name": "argocd-application",
	}, ku.BaseLabels(config.WorkflowContext.PromiseName, config.Name))

	spec := ku.ArgoCDApplicationSpec{
		Name:      fmt.Sprintf("vcluster-%s", config.Name),
		Namespace: "argocd",
		Annotations: map[string]string{
			"argocd.argoproj.io/sync-wave": "0",
		},
		Finalizers: []string{"resources-finalizer.argocd.argoproj.io"},
		Project:    config.ProjectName,
		Destination: ku.Destination{
			Server:    config.ArgoCDDestServer,
			Namespace: config.TargetNamespace,
		},
		Source: ku.AppSource{
			RepoURL:        config.ArgoCDRepoURL,
			Chart:          config.ArgoCDChart,
			TargetRevision: config.ArgoCDTargetRevision,
			Helm: &ku.HelmSource{
				ReleaseName:  config.Name,
				ValuesObject: config.ValuesObject,
			},
		},
		SyncPolicy: config.ArgoCDSyncPolicy,
	}

	return ku.Resource{
		APIVersion: "platform.integratn.tech/v1alpha1",
		Kind:       "ArgoCDApplication",
		Metadata: ku.ResourceMeta(
			fmt.Sprintf("vcluster-%s", config.Name),
			config.Namespace,
			metadataLabels,
			nil,
		),
		Spec: spec,
	}
}

func buildArgoCDClusterRegistrationRequest(config *VClusterConfig) ku.Resource {
	metadataLabels := ku.MergeStringMap(map[string]string{
		"app.kubernetes.io/name": "argocd-cluster-registration",
	}, ku.BaseLabels(config.WorkflowContext.PromiseName, config.Name))

	spec := ku.ArgoCDClusterRegistrationSpec{
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

	return ku.Resource{
		APIVersion: "platform.integratn.tech/v1alpha1",
		Kind:       "ArgoCDClusterRegistration",
		Metadata: ku.ResourceMeta(
			fmt.Sprintf("%s-cluster-registration", config.Name),
			config.Namespace,
			metadataLabels,
			nil,
		),
		Spec: spec,
	}
}
