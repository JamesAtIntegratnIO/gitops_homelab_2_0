package main

import (
	"time"
)

// StatusResult holds the computed status for a single vcluster.
type StatusResult struct {
	Phase          string         `json:"phase"`
	Message        string         `json:"message"`
	LastReconciled string         `json:"lastReconciled"`
	Endpoints      Endpoints      `json:"endpoints,omitempty"`
	Credentials    Credentials    `json:"credentials,omitempty"`
	Health         Health         `json:"health"`
	Conditions     []Condition    `json:"conditions"`
}

// Endpoints holds discoverable URLs for the vcluster.
type Endpoints struct {
	API    string `json:"api,omitempty"`
	ArgoCD string `json:"argocd,omitempty"`
}

// Credentials holds references (not values) for vcluster credentials.
type Credentials struct {
	KubeconfigSecret string `json:"kubeconfigSecret,omitempty"`
	OnePasswordItem  string `json:"onePasswordItem,omitempty"`
}

// Health aggregates health checks across the lifecycle chain.
type Health struct {
	ArgoCD    ArgoCDHealth    `json:"argocd"`
	Workloads WorkloadHealth  `json:"workloads"`
	SubApps   SubAppHealth    `json:"subApps"`
}

// ArgoCDHealth reflects the parent ArgoCD Application status.
type ArgoCDHealth struct {
	SyncStatus   string `json:"syncStatus"`
	HealthStatus string `json:"healthStatus"`
}

// WorkloadHealth reflects pod readiness in the vcluster namespace.
type WorkloadHealth struct {
	Ready int `json:"ready"`
	Total int `json:"total"`
}

// SubAppHealth reflects the health of child ArgoCD Applications.
type SubAppHealth struct {
	Healthy   int      `json:"healthy"`
	Total     int      `json:"total"`
	Unhealthy []string `json:"unhealthy,omitempty"`
}

// Condition follows the Kubernetes metav1.Condition convention.
type Condition struct {
	Type               string `json:"type"`
	Status             string `json:"status"`     // "True", "False", "Unknown"
	Reason             string `json:"reason"`
	Message            string `json:"message"`
	LastTransitionTime string `json:"lastTransitionTime"`
}

// NewCondition creates a Condition with the current timestamp.
func NewCondition(condType, status, reason, message string) Condition {
	return Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: time.Now().UTC().Format(time.RFC3339),
	}
}
