package main

import (
	ku "github.com/jamesatintegratnio/gitops_homelab_2_0/promises/_shared/kratixutil"
)

// AppProjectSpec is the ArgoCD AppProject spec.
type AppProjectSpec struct {
	Description                string                 `json:"description,omitempty"`
	SourceRepos                []string               `json:"sourceRepos"`
	Destinations               []ku.ProjectDestination `json:"destinations"`
	ClusterResourceWhitelist   []ku.ResourceFilter     `json:"clusterResourceWhitelist,omitempty"`
	NamespaceResourceWhitelist []ku.ResourceFilter     `json:"namespaceResourceWhitelist,omitempty"`
}
