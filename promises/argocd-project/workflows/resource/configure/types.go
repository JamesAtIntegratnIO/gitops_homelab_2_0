package main

import (
	u "github.com/jamesatintegratnio/gitops_homelab_2_0/promises/_shared/kratixutil"
)

// AppProjectSpec is the ArgoCD AppProject spec.
type AppProjectSpec struct {
	Description                string                 `json:"description,omitempty"`
	SourceRepos                []string               `json:"sourceRepos"`
	Destinations               []u.ProjectDestination `json:"destinations"`
	ClusterResourceWhitelist   []u.ResourceFilter     `json:"clusterResourceWhitelist,omitempty"`
	NamespaceResourceWhitelist []u.ResourceFilter     `json:"namespaceResourceWhitelist,omitempty"`
}
