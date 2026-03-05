// Package kratixutil provides shared types, helpers, and writers for Kratix
// promise pipelines. It eliminates code duplication across promise workflows
// by extracting common Kubernetes resource types, value-extraction helpers,
// YAML output writers, and resource-construction utilities.
package kratixutil

// ============================================================================
// Core Kubernetes Types
// ============================================================================

type Resource struct {
	APIVersion string      `json:"apiVersion"`
	Kind       string      `json:"kind"`
	Metadata   ObjectMeta  `json:"metadata"`
	Spec       interface{} `json:"spec,omitempty"`
	Data       interface{} `json:"data,omitempty"`
	Rules      interface{} `json:"rules,omitempty"`
	RoleRef    *RoleRef    `json:"roleRef,omitempty"`
	Subjects   []Subject   `json:"subjects,omitempty"`
}

type RoleRef struct {
	APIGroup string `json:"apiGroup"`
	Kind     string `json:"kind"`
	Name     string `json:"name"`
}

type Subject struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
}

type ObjectMeta struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	Finalizers  []string          `json:"finalizers,omitempty"`
}

// ============================================================================
// ArgoCD Kratix ResourceRequest Types
// ============================================================================

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
	SyncPolicy  interface{}       `json:"syncPolicy,omitempty"`
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

type SyncPolicy struct {
	Automated   *AutomatedSync `json:"automated,omitempty"`
	SyncOptions []string       `json:"syncOptions,omitempty"`
}

type AutomatedSync struct {
	SelfHeal bool `json:"selfHeal"`
	Prune    bool `json:"prune"`
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

type ProjectDestination struct {
	Namespace string `json:"namespace"`
	Server    string `json:"server"`
}

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

// ============================================================================
// Secret Types (1Password / ExternalSecrets)
// ============================================================================

type SecretRef struct {
	Name            string      `json:"name"`
	OnePasswordItem string      `json:"onePasswordItem"`
	Keys            []SecretKey `json:"keys"`
}

type SecretKey struct {
	SecretKey string `json:"secretKey"`
	Property  string `json:"property"`
}

// ============================================================================
// K8s Workload Types (Job, RBAC)
// ============================================================================

type PolicyRule struct {
	APIGroups     []string `json:"apiGroups"`
	Resources     []string `json:"resources"`
	Verbs         []string `json:"verbs"`
	ResourceNames []string `json:"resourceNames,omitempty"`
}

type JobSpec struct {
	BackoffLimit            int             `json:"backoffLimit,omitempty"`
	TTLSecondsAfterFinished int             `json:"ttlSecondsAfterFinished,omitempty"`
	Template                PodTemplateSpec `json:"template"`
}

type PodTemplateSpec struct {
	Metadata *ObjectMeta `json:"metadata,omitempty"`
	Spec     PodSpec     `json:"spec"`
}

type PodSpec struct {
	RestartPolicy      string      `json:"restartPolicy,omitempty"`
	ServiceAccountName string      `json:"serviceAccountName,omitempty"`
	InitContainers     []Container `json:"initContainers,omitempty"`
	Containers         []Container `json:"containers"`
	Volumes            []Volume    `json:"volumes,omitempty"`
}

type Container struct {
	Name         string        `json:"name"`
	Image        string        `json:"image"`
	Command      []string      `json:"command,omitempty"`
	Env          []EnvVar      `json:"env,omitempty"`
	VolumeMounts []VolumeMount `json:"volumeMounts,omitempty"`
}

type EnvVar struct {
	Name      string        `json:"name"`
	Value     string        `json:"value,omitempty"`
	ValueFrom *EnvVarSource `json:"valueFrom,omitempty"`
}

type EnvVarSource struct {
	SecretKeyRef *SecretKeySelector `json:"secretKeyRef,omitempty"`
}

type SecretKeySelector struct {
	Name string `json:"name"`
	Key  string `json:"key"`
}

type VolumeMount struct {
	Name      string `json:"name"`
	MountPath string `json:"mountPath"`
	ReadOnly  bool   `json:"readOnly,omitempty"`
}

type Volume struct {
	Name   string        `json:"name"`
	Secret *SecretVolume `json:"secret,omitempty"`
}

type SecretVolume struct {
	SecretName string `json:"secretName"`
	Optional   bool   `json:"optional,omitempty"`
}

// ============================================================================
// ExternalSecret Types
// ============================================================================

type ExternalSecretSpec struct {
	SecretStoreRef  SecretStoreRef           `json:"secretStoreRef"`
	Target          ExternalSecretTarget     `json:"target"`
	Data            []ExternalSecretData     `json:"data,omitempty"`
	DataFrom        []ExternalSecretDataFrom `json:"dataFrom,omitempty"`
	RefreshInterval string                   `json:"refreshInterval,omitempty"`
}

type SecretStoreRef struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
}

type ExternalSecretTarget struct {
	Name     string                  `json:"name,omitempty"`
	Template *ExternalSecretTemplate `json:"template,omitempty"`
}

type ExternalSecretTemplate struct {
	EngineVersion string            `json:"engineVersion,omitempty"`
	Type          string            `json:"type,omitempty"`
	Metadata      *TemplateMetadata `json:"metadata,omitempty"`
	Data          map[string]string `json:"data,omitempty"`
}

type TemplateMetadata struct {
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

type ExternalSecretData struct {
	SecretKey string    `json:"secretKey"`
	RemoteRef RemoteRef `json:"remoteRef"`
}

type RemoteRef struct {
	Key      string `json:"key"`
	Property string `json:"property,omitempty"`
}

type ExternalSecretDataFrom struct {
	Extract *ExternalSecretExtract `json:"extract"`
}

type ExternalSecretExtract struct {
	Key                string `json:"key"`
	ConversionStrategy string `json:"conversionStrategy,omitempty"`
	DecodingStrategy   string `json:"decodingStrategy,omitempty"`
}
