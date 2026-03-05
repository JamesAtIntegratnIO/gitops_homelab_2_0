package main

import (
	u "github.com/jamesatintegratnio/gitops_homelab_2_0/promises/_shared/kratixutil"
)

// ApplicationSpec is the ArgoCD Application spec.
type ApplicationSpec struct {
	Project     string        `json:"project"`
	Source      u.AppSource   `json:"source"`
	Destination u.Destination `json:"destination"`
	SyncPolicy  interface{}   `json:"syncPolicy,omitempty"`
}
