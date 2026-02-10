package main

// Resource is a generic Kubernetes resource.
type Resource struct {
	APIVersion string      `json:"apiVersion"`
	Kind       string      `json:"kind"`
	Metadata   ObjectMeta  `json:"metadata"`
	Spec       interface{} `json:"spec,omitempty"`
}

// ObjectMeta is a lightweight Kubernetes metadata block.
type ObjectMeta struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

// AppProjectSpec is the ArgoCD AppProject spec.
type AppProjectSpec struct {
	Description                string                   `json:"description,omitempty"`
	SourceRepos                []string                 `json:"sourceRepos"`
	Destinations               []map[string]interface{} `json:"destinations"`
	ClusterResourceWhitelist   []map[string]interface{} `json:"clusterResourceWhitelist,omitempty"`
	NamespaceResourceWhitelist []map[string]interface{} `json:"namespaceResourceWhitelist,omitempty"`
}
