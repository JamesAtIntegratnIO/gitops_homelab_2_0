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
	Finalizers  []string          `json:"finalizers,omitempty"`
}

// ApplicationSpec is the ArgoCD Application spec.
type ApplicationSpec struct {
	Project     string      `json:"project"`
	Source      AppSource   `json:"source"`
	Destination Destination `json:"destination"`
	SyncPolicy  interface{} `json:"syncPolicy,omitempty"`
}

type AppSource struct {
	RepoURL        string      `json:"repoURL"`
	Chart          string      `json:"chart,omitempty"`
	TargetRevision string      `json:"targetRevision"`
	Helm           *HelmSource `json:"helm,omitempty"`
}

type HelmSource struct {
	ReleaseName  string      `json:"releaseName,omitempty"`
	ValuesObject interface{} `json:"valuesObject,omitempty"`
}

type Destination struct {
	Server    string `json:"server"`
	Namespace string `json:"namespace"`
}
