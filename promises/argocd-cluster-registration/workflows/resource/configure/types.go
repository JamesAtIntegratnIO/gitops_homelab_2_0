package main

import (
	u "github.com/jamesatintegratnio/gitops_homelab_2_0/promises/_shared/kratixutil"
)

// ============================================================================
// RBAC Types
// ============================================================================

type PolicyRule struct {
	APIGroups     []string `json:"apiGroups"`
	Resources     []string `json:"resources"`
	Verbs         []string `json:"verbs"`
	ResourceNames []string `json:"resourceNames,omitempty"`
}

// ============================================================================
// Job Types
// ============================================================================

type JobSpec struct {
	BackoffLimit            int             `json:"backoffLimit,omitempty"`
	TTLSecondsAfterFinished int             `json:"ttlSecondsAfterFinished,omitempty"`
	Template                PodTemplateSpec `json:"template"`
}

type PodTemplateSpec struct {
	Metadata *u.ObjectMeta `json:"metadata,omitempty"`
	Spec     PodSpec       `json:"spec"`
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

// ============================================================================
// Config Type
// ============================================================================

// RegistrationConfig holds all configuration extracted from the ResourceRequest.
type RegistrationConfig struct {
	Name                   string
	TargetNamespace        string
	KubeconfigSecret       string
	KubeconfigKey          string
	ExternalServerURL      string
	OnePasswordItem        string
	OnePasswordConnectHost string
	Environment            string
	BaseDomain             string
	BaseDomainSanitized    string
	ClusterLabels          map[string]string
	ClusterAnnotations     map[string]string
	SyncJobName            string
	PromiseName            string
}
