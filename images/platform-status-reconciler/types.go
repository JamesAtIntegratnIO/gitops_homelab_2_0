package main

import (
	"time"
)

// StatusResult holds the computed status for a single vcluster.
type StatusResult struct {
	Phase          string      `json:"phase"`
	Message        string      `json:"message"`
	LastReconciled string      `json:"lastReconciled"`
	Endpoints      Endpoints   `json:"endpoints,omitempty"`
	Credentials    Credentials `json:"credentials,omitempty"`
	Health         Health      `json:"health"`
	Conditions     []Condition `json:"conditions"`
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
	ArgoCD    ArgoCDHealth   `json:"argocd"`
	Workloads WorkloadHealth `json:"workloads"`
	SubApps   SubAppHealth   `json:"subApps"`
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
	Status             string `json:"status"` // "True", "False", "Unknown"
	Reason             string `json:"reason"`
	Message            string `json:"message"`
	LastTransitionTime string `json:"lastTransitionTime"`
}

// NewCondition creates a Condition stamped with the current time.
//
// Callers that are refreshing an existing condition must run the result through
// CarryTransitionTime, or the timestamp churns on every pass -- see the comment
// there for why that matters.
func NewCondition(condType, status, reason, message string) Condition {
	return Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: time.Now().UTC().Format(time.RFC3339),
	}
}

// CarryTransitionTime restores lastTransitionTime from the matching existing
// condition whenever the status has not actually transitioned.
//
// lastTransitionTime means "when this condition last changed state", not "when
// we last looked". Re-stamping it every pass made the reconciler rewrite .status
// on every cycle even when nothing had changed, which:
//
//   - woke Kratix's ResourceRequestController on its watch, once per cycle;
//   - and, because each side then rewrote the conditions array with its own
//     timestamps, the two controllers never converged.
//
// On 2026-08-21 that loop was running at ~9.5 writes/second against a single
// object and had produced 202,435 ReconcileStarted events -- 97% of every event
// in the cluster -- against etcd already struggling with 178ms p99 fsync.
func CarryTransitionTime(fresh []Condition, existing []Condition) []Condition {
	prev := make(map[string]Condition, len(existing))
	for _, c := range existing {
		prev[c.Type] = c
	}
	out := make([]Condition, 0, len(fresh))
	for _, c := range fresh {
		if old, ok := prev[c.Type]; ok && old.Status == c.Status && old.LastTransitionTime != "" {
			c.LastTransitionTime = old.LastTransitionTime
		}
		out = append(out, c)
	}
	return out
}

// MergeForeignConditions appends conditions owned by other controllers, which a
// merge patch on the conditions array would otherwise drop.
//
// The reconciler patches .status.conditions as a whole list, and a JSON merge
// patch replaces a list rather than merging it. Kratix sets its own
// WorksSucceeded condition on the same resource, so every write deleted it and
// Kratix immediately put it back -- the other half of the write loop.
func MergeForeignConditions(ours []Condition, existing []Condition) []Condition {
	owned := make(map[string]bool, len(ours))
	for _, c := range ours {
		owned[c.Type] = true
	}
	out := append([]Condition{}, ours...)
	for _, c := range existing {
		if !owned[c.Type] {
			out = append(out, c)
		}
	}
	return out
}
