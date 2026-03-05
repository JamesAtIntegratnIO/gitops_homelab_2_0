package main

// AppProjectSpec is the ArgoCD AppProject spec.
type AppProjectSpec struct {
	Description                string                   `json:"description,omitempty"`
	SourceRepos                []string                 `json:"sourceRepos"`
	Destinations               []map[string]interface{} `json:"destinations"`
	ClusterResourceWhitelist   []map[string]interface{} `json:"clusterResourceWhitelist,omitempty"`
	NamespaceResourceWhitelist []map[string]interface{} `json:"namespaceResourceWhitelist,omitempty"`
}
