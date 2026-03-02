package main

// Resource is a generic Kubernetes resource.
type Resource struct {
	APIVersion string      `json:"apiVersion"`
	Kind       string      `json:"kind"`
	Metadata   ObjectMeta  `json:"metadata"`
	Spec       interface{} `json:"spec,omitempty"`
}

// ObjectMeta represents Kubernetes object metadata.
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

// AppSource defines the Helm chart source.
type AppSource struct {
	RepoURL        string      `json:"repoURL"`
	Chart          string      `json:"chart,omitempty"`
	TargetRevision string      `json:"targetRevision"`
	Helm           *HelmSource `json:"helm,omitempty"`
}

// HelmSource holds Helm-specific source config.
type HelmSource struct {
	ReleaseName  string      `json:"releaseName,omitempty"`
	ValuesObject interface{} `json:"valuesObject,omitempty"`
}

// Destination is the ArgoCD deployment target.
type Destination struct {
	Server    string `json:"server"`
	Namespace string `json:"namespace"`
}

// SecretRef describes a 1Password-backed ExternalSecret.
type SecretRef struct {
	Name             string      `json:"name"`
	OnePasswordItem  string      `json:"onePasswordItem"`
	Keys             []SecretKey `json:"keys"`
}

// SecretKey maps a 1Password property to a K8s Secret key.
type SecretKey struct {
	SecretKey string `json:"secretKey"`
	Property  string `json:"property"`
}

// HTTPServiceConfig holds the fully resolved config from the CR.
type HTTPServiceConfig struct {
	Name      string
	Namespace string
	Team      string

	// Image
	ImageRepository string
	ImageTag        string
	ImagePullPolicy string
	Command         []string
	Args            []string

	// Scaling
	Replicas       int
	CPURequest     string
	MemoryRequest  string
	CPULimit       string
	MemoryLimit    string

	// Networking
	Port            int
	IngressEnabled  bool
	IngressHostname string
	IngressPath     string

	// Secrets
	Secrets []SecretRef

	// Environment
	Env            map[string]string
	EnvFromSecrets []string

	// Health checks
	HealthCheckPath string
	HealthCheckPort int

	// Monitoring
	MonitoringEnabled  bool
	MonitoringPath     string
	MonitoringInterval string

	// Storage
	PersistenceEnabled bool
	PersistenceSize    string
	PersistenceClass   string
	PersistenceMountPath string

	// Security
	RunAsNonRoot          *bool
	ReadOnlyRootFilesystem *bool
	RunAsUser             *int64
	RunAsGroup            *int64

	// Escape hatch
	HelmOverrides map[string]interface{}

	// Platform defaults
	BaseDomain    string
	GatewayName   string
	GatewayNS     string
	SecretStoreName string
	SecretStoreKind string
}
