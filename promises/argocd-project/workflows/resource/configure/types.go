package main

import (
	ku "github.com/jamesatintegratnio/gitops_homelab_2_0/promises/_shared/kratixutil"
)

// ProjectConfig holds configuration extracted from the ArgoCD Project resource request.
type ProjectConfig struct {
	Name                       string
	Namespace                  string
	Description                string
	Annotations                map[string]string
	Labels                     map[string]string
	SourceRepos                []string
	Destinations               []ku.ProjectDestination
	ClusterResourceWhitelist   []ku.ResourceFilter
	NamespaceResourceWhitelist []ku.ResourceFilter
}

// AppProjectSpec is the ArgoCD AppProject spec.
type AppProjectSpec struct {
	Description                string                  `json:"description,omitempty"`
	SourceRepos                []string                `json:"sourceRepos"`
	Destinations               []ku.ProjectDestination `json:"destinations"`
	ClusterResourceWhitelist   []ku.ResourceFilter     `json:"clusterResourceWhitelist,omitempty"`
	NamespaceResourceWhitelist []ku.ResourceFilter     `json:"namespaceResourceWhitelist,omitempty"`
}
