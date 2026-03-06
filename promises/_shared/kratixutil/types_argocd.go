package kratixutil

// ArgoCDApplicationSpec is the spec for a platform.integratn.tech/v1alpha1
// ArgoCDApplication sub-ResourceRequest. The argocd-application promise
// pipeline reads these fields to construct the actual argoproj.io/v1alpha1
// Application.
type ArgoCDApplicationSpec struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Finalizers  []string          `json:"finalizers,omitempty"`
	Project     string            `json:"project"`
	Source      AppSource         `json:"source"`
	Destination Destination       `json:"destination"`
	SyncPolicy  *SyncPolicy       `json:"syncPolicy,omitempty"`
}

// AppSource defines the source repository for an ArgoCD Application.
type AppSource struct {
	RepoURL        string      `json:"repoURL"`
	Chart          string      `json:"chart,omitempty"`
	TargetRevision string      `json:"targetRevision"`
	Helm           *HelmSource `json:"helm,omitempty"`
}

// HelmSource configures Helm-specific source options: release name and
// inline values.
type HelmSource struct {
	ReleaseName  string      `json:"releaseName,omitempty"`
	ValuesObject interface{} `json:"valuesObject,omitempty"`
}

// Destination identifies the target cluster and namespace for an ArgoCD
// Application.
type Destination struct {
	Server    string `json:"server"`
	Namespace string `json:"namespace"`
}

// SyncPolicy controls ArgoCD automated sync behavior and sync options.
type SyncPolicy struct {
	Automated   *AutomatedSync `json:"automated,omitempty"`
	SyncOptions []string       `json:"syncOptions,omitempty"`
	Retry       *RetryStrategy `json:"retry,omitempty"`
}

// AutomatedSync configures ArgoCD auto-sync self-heal and prune.
type AutomatedSync struct {
	SelfHeal bool `json:"selfHeal,omitempty"`
	Prune    bool `json:"prune,omitempty"`
}

// RetryStrategy controls ArgoCD sync retry behavior.
type RetryStrategy struct {
	Backoff *Backoff `json:"backoff,omitempty"`
}

// Backoff configures exponential backoff for sync retries.
type Backoff struct {
	Duration    string `json:"duration,omitempty"`
	MaxDuration string `json:"maxDuration,omitempty"`
	Factor      *int64 `json:"factor,omitempty"`
}

// ArgoCDProjectSpec is the spec for a platform.integratn.tech/v1alpha1
// ArgoCDProject sub-ResourceRequest.
type ArgoCDProjectSpec struct {
	Namespace                  string               `json:"namespace"`
	Name                       string               `json:"name"`
	Description                string               `json:"description"`
	Annotations                map[string]string     `json:"annotations,omitempty"`
	Labels                     map[string]string     `json:"labels,omitempty"`
	SourceRepos                []string              `json:"sourceRepos"`
	Destinations               []ProjectDestination  `json:"destinations"`
	ClusterResourceWhitelist   []ResourceFilter      `json:"clusterResourceWhitelist,omitempty"`
	NamespaceResourceWhitelist []ResourceFilter      `json:"namespaceResourceWhitelist,omitempty"`
}

// ProjectDestination pairs a namespace with a target cluster server URL
// for ArgoCD project scope control.
type ProjectDestination struct {
	Namespace string `json:"namespace"`
	Server    string `json:"server"`
}

// ResourceFilter restricts which resource types an ArgoCD project may manage.
type ResourceFilter struct {
	Group string `json:"group"`
	Kind  string `json:"kind"`
}

// ArgoCDClusterRegistrationSpec is the spec for a platform.integratn.tech/v1alpha1
// ArgoCDClusterRegistration sub-ResourceRequest.
type ArgoCDClusterRegistrationSpec struct {
	Name                string            `json:"name"`
	TargetNamespace     string            `json:"targetNamespace"`
	KubeconfigSecret    string            `json:"kubeconfigSecret"`
	ExternalServerURL   string            `json:"externalServerURL"`
	Environment         string            `json:"environment,omitempty"`
	BaseDomain          string            `json:"baseDomain,omitempty"`
	BaseDomainSanitized string            `json:"baseDomainSanitized,omitempty"`
	ClusterLabels       map[string]string `json:"clusterLabels,omitempty"`
	ClusterAnnotations  map[string]string `json:"clusterAnnotations,omitempty"`
	SyncJobName         string            `json:"syncJobName,omitempty"`
}
