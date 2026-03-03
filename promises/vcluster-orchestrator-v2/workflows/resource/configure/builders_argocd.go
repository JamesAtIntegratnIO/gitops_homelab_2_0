package main

import (
	"fmt"

	u "github.com/jamesatintegratnio/gitops_homelab_2_0/promises/_shared/kratixutil"
)

func buildArgoCDProjectRequest(config *VClusterConfig) u.Resource {
	metadataLabels := u.MergeStringMap(map[string]string{
		"app.kubernetes.io/name": "argocd-project",
	}, u.BaseLabels(config.WorkflowContext.PromiseName, config.Name))

	specLabels := map[string]string{
		"app.kubernetes.io/managed-by":     "kratix",
		"argocd.argoproj.io/project-group": "appteam",
		"kratix.io/promise-name":           config.WorkflowContext.PromiseName,
		"kratix.io/resource-name":          config.Name,
	}

	return u.Resource{
		APIVersion: "platform.integratn.tech/v1alpha1",
		Kind:       "ArgoCDProject",
		Metadata: u.ResourceMeta(
			config.ProjectName,
			config.Namespace,
			metadataLabels,
			nil,
		),
		Spec: u.ArgoCDProjectSpec{
			Namespace:   "argocd",
			Name:        config.ProjectName,
			Description: fmt.Sprintf("VCluster project for %s", config.Name),
			Annotations: map[string]string{
				"argocd.argoproj.io/sync-wave": "-1",
			},
			Labels:      specLabels,
			SourceRepos: []string{"https://charts.loft.sh"},
			Destinations: []u.ProjectDestination{
				{
					Namespace: config.TargetNamespace,
					Server:    "https://kubernetes.default.svc",
				},
			},
			ClusterResourceWhitelist: []u.ResourceFilter{
				{Group: "*", Kind: "*"},
			},
			NamespaceResourceWhitelist: []u.ResourceFilter{
				{Group: "*", Kind: "*"},
			},
		},
	}
}

func buildArgoCDApplicationRequest(config *VClusterConfig) u.Resource {
	metadataLabels := u.MergeStringMap(map[string]string{
		"app.kubernetes.io/name": "argocd-application",
	}, u.BaseLabels(config.WorkflowContext.PromiseName, config.Name))

	spec := u.ArgoCDApplicationSpec{
		Name:      fmt.Sprintf("vcluster-%s", config.Name),
		Namespace: "argocd",
		Annotations: map[string]string{
			"argocd.argoproj.io/sync-wave": "0",
		},
		Finalizers: []string{"resources-finalizer.argocd.argoproj.io"},
		Project:    config.ProjectName,
		Destination: u.Destination{
			Server:    config.ArgoCDDestServer,
			Namespace: config.TargetNamespace,
		},
		Source: u.AppSource{
			RepoURL:        config.ArgoCDRepoURL,
			Chart:          config.ArgoCDChart,
			TargetRevision: config.ArgoCDTargetRevision,
			Helm: &u.HelmSource{
				ReleaseName:  config.Name,
				ValuesObject: config.ValuesObject,
			},
		},
		SyncPolicy: config.ArgoCDSyncPolicy,
	}

	return u.Resource{
		APIVersion: "platform.integratn.tech/v1alpha1",
		Kind:       "ArgoCDApplication",
		Metadata: u.ResourceMeta(
			fmt.Sprintf("vcluster-%s", config.Name),
			config.Namespace,
			metadataLabels,
			nil,
		),
		Spec: spec,
	}
}

func buildArgoCDClusterRegistrationRequest(config *VClusterConfig) u.Resource {
	metadataLabels := u.MergeStringMap(map[string]string{
		"app.kubernetes.io/name": "argocd-cluster-registration",
	}, u.BaseLabels(config.WorkflowContext.PromiseName, config.Name))

	spec := u.ArgoCDClusterRegistrationSpec{
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

	return u.Resource{
		APIVersion: "platform.integratn.tech/v1alpha1",
		Kind:       "ArgoCDClusterRegistration",
		Metadata: u.ResourceMeta(
			fmt.Sprintf("%s-cluster-registration", config.Name),
			config.Namespace,
			metadataLabels,
			nil,
		),
		Spec: spec,
	}
}
