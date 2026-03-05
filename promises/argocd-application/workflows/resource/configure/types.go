package main

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
