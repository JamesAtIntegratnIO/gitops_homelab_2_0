package platform

import "time"

// ResourceKind identifies the category of platform resource.
type ResourceKind string

const (
	// KindVCluster is a Kratix-managed virtual cluster.
	KindVCluster ResourceKind = "vcluster"
	// KindWorkload is a Score-deployed application running inside a vCluster.
	KindWorkload ResourceKind = "workload"
	// KindAddon is an infrastructure addon managed by ArgoCD ApplicationSets.
	KindAddon ResourceKind = "addon"
)

// ResourceStatus is a unified status representation for any platform resource.
// It provides enough detail for `hctl status`, `--output json`, and dashboards.
type ResourceStatus struct {
	Kind      ResourceKind `json:"kind" yaml:"kind"`
	Name      string       `json:"name" yaml:"name"`
	Namespace string       `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Phase     string       `json:"phase" yaml:"phase"`
	Message   string       `json:"message,omitempty" yaml:"message,omitempty"`

	// ArgoCD Application info (applies to all kinds).
	ArgoCD ArgoCDInfo `json:"argocd" yaml:"argocd"`

	// Pods shows workload readiness (Ready/Total).
	Pods PodInfo `json:"pods,omitempty" yaml:"pods,omitempty"`

	// Endpoints are discoverable URLs (API, UI, etc.).
	Endpoints []Endpoint `json:"endpoints,omitempty" yaml:"endpoints,omitempty"`

	// LastChecked is when this status was gathered.
	LastChecked time.Time `json:"lastChecked" yaml:"lastChecked"`

	// Labels preserves useful ArgoCD labels for grouping.
	Labels map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
}

// ArgoCDInfo captures ArgoCD Application sync and health.
type ArgoCDInfo struct {
	AppName      string `json:"appName" yaml:"appName"`
	SyncStatus   string `json:"syncStatus" yaml:"syncStatus"`
	HealthStatus string `json:"healthStatus" yaml:"healthStatus"`
	Project      string `json:"project,omitempty" yaml:"project,omitempty"`
}

// PodInfo reflects pod readiness counts.
type PodInfo struct {
	Ready int `json:"ready" yaml:"ready"`
	Total int `json:"total" yaml:"total"`
}

// Endpoint is a named URL for a resource.
type Endpoint struct {
	Name string `json:"name" yaml:"name"`
	URL  string `json:"url" yaml:"url"`
}

// PlatformStatus is the aggregate status returned by `hctl status --output json`.
type PlatformStatus struct {
	VClusters []ResourceStatus `json:"vclusters" yaml:"vclusters"`
	Workloads []ResourceStatus `json:"workloads" yaml:"workloads"`
	Addons    []ResourceStatus `json:"addons" yaml:"addons"`
}

// PhaseFromArgoCD derives a simple phase string from ArgoCD sync+health status.
func PhaseFromArgoCD(syncStatus, healthStatus string) string {
	switch {
	case syncStatus == "Synced" && healthStatus == "Healthy":
		return "Ready"
	case healthStatus == "Degraded":
		return "Degraded"
	case healthStatus == "Missing" || syncStatus == "Unknown":
		return "Unknown"
	case healthStatus == "Progressing" || syncStatus == "OutOfSync":
		return "Progressing"
	case healthStatus == "Suspended":
		return "Suspended"
	default:
		return "Progressing"
	}
}
