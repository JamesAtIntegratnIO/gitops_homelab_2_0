package main

import (
	ku "github.com/jamesatintegratnio/gitops_homelab_2_0/promises/_shared/kratixutil"
)

// AppConfig holds configuration extracted from the ArgoCD Application resource request.
type AppConfig struct {
	Name        string
	Namespace   string
	Project     string
	Annotations map[string]string
	Labels      map[string]string
	Finalizers  []string
	Source      ku.AppSource
	Destination ku.Destination
	SyncPolicy  *ku.SyncPolicy
}

// ApplicationSpec is the ArgoCD Application spec.
type ApplicationSpec struct {
	Project     string         `json:"project"`
	Source      ku.AppSource   `json:"source"`
	Destination ku.Destination `json:"destination"`
	SyncPolicy  *ku.SyncPolicy `json:"syncPolicy,omitempty"`
}
