// Package kratixutil provides shared types, helpers, and writers for Kratix
// promise pipelines. It eliminates code duplication across promise workflows
// by extracting common Kubernetes resource types, value-extraction helpers,
// YAML output writers, and resource-construction utilities.
package kratixutil

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ============================================================================
// Core Kubernetes Types
// ============================================================================

// Resource is a generic Kubernetes resource suitable for any API object.
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

// RoleRef references a Role or ClusterRole for a RoleBinding.
type RoleRef struct {
	APIGroup string `json:"apiGroup"`
	Kind     string `json:"kind"`
	Name     string `json:"name"`
}

// Subject identifies the entity bound by a RoleBinding.
type Subject struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
}

// ObjectMeta is a lightweight Kubernetes metadata block.
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

// AppSource defines the Helm chart or git source for an ArgoCD Application.
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

// SyncPolicy for ArgoCD applications.
type SyncPolicy struct {
	Automated   *AutomatedSync `json:"automated,omitempty"`
	SyncOptions []string       `json:"syncOptions,omitempty"`
}

// AutomatedSync configures automatic syncing for ArgoCD.
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

// ProjectDestination defines an ArgoCD project destination.
type ProjectDestination struct {
	Namespace string `json:"namespace"`
	Server    string `json:"server"`
}

// ResourceFilter matches a Kubernetes resource by API group and kind.
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

// SecretRef describes a 1Password-backed ExternalSecret.
type SecretRef struct {
	Name            string      `json:"name"`
	OnePasswordItem string      `json:"onePasswordItem"`
	Keys            []SecretKey `json:"keys"`
}

// SecretKey maps a 1Password property to a Kubernetes Secret key.
type SecretKey struct {
	SecretKey string `json:"secretKey"`
	Property  string `json:"property"`
}

// ============================================================================
// Type Conversion Utilities
// ============================================================================

// ToMap converts a struct to map[string]interface{} via JSON roundtrip.
// Useful at the merge boundary where typed structs meet DeepMerge.
func ToMap(v interface{}) (map[string]interface{}, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("toMap marshal: %w", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("toMap unmarshal: %w", err)
	}
	return m, nil
}

// DeleteFromResource creates a minimal delete resource (only apiVersion, kind,
// name, namespace) from a fully-populated Resource.
func DeleteFromResource(r Resource) Resource {
	return Resource{
		APIVersion: r.APIVersion,
		Kind:       r.Kind,
		Metadata: ObjectMeta{
			Name:      r.Metadata.Name,
			Namespace: r.Metadata.Namespace,
		},
	}
}

// DeleteOutputPathForResource computes the output file path for a delete
// resource in the standard "resources/delete-{kind}-{name}.yaml" pattern.
func DeleteOutputPathForResource(prefix string, r Resource) string {
	if prefix == "" {
		prefix = "resources/"
	}
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	return fmt.Sprintf("%sdelete-%s-%s.yaml", prefix, strings.ToLower(r.Kind), r.Metadata.Name)
}

// ============================================================================
// K8s Workload Types (Job, RBAC)
// ============================================================================

// PolicyRule defines a Kubernetes RBAC policy rule.
type PolicyRule struct {
	APIGroups     []string `json:"apiGroups"`
	Resources     []string `json:"resources"`
	Verbs         []string `json:"verbs"`
	ResourceNames []string `json:"resourceNames,omitempty"`
}

// JobSpec defines the spec for a Kubernetes batch/v1 Job.
type JobSpec struct {
	BackoffLimit            int             `json:"backoffLimit,omitempty"`
	TTLSecondsAfterFinished int             `json:"ttlSecondsAfterFinished,omitempty"`
	Template                PodTemplateSpec `json:"template"`
}

// PodTemplateSpec holds a pod template for a Job.
type PodTemplateSpec struct {
	Metadata *ObjectMeta `json:"metadata,omitempty"`
	Spec     PodSpec     `json:"spec"`
}

// PodSpec defines the pod spec within a Job template.
type PodSpec struct {
	RestartPolicy      string      `json:"restartPolicy,omitempty"`
	ServiceAccountName string      `json:"serviceAccountName,omitempty"`
	InitContainers     []Container `json:"initContainers,omitempty"`
	Containers         []Container `json:"containers"`
	Volumes            []Volume    `json:"volumes,omitempty"`
}

// Container defines a container within a pod.
type Container struct {
	Name         string        `json:"name"`
	Image        string        `json:"image"`
	Command      []string      `json:"command,omitempty"`
	Env          []EnvVar      `json:"env,omitempty"`
	VolumeMounts []VolumeMount `json:"volumeMounts,omitempty"`
}

// EnvVar defines an environment variable for a container.
type EnvVar struct {
	Name      string        `json:"name"`
	Value     string        `json:"value,omitempty"`
	ValueFrom *EnvVarSource `json:"valueFrom,omitempty"`
}

// EnvVarSource defines the source for an environment variable value.
type EnvVarSource struct {
	SecretKeyRef *SecretKeySelector `json:"secretKeyRef,omitempty"`
}

// SecretKeySelector selects a key from a Kubernetes Secret.
type SecretKeySelector struct {
	Name string `json:"name"`
	Key  string `json:"key"`
}

// VolumeMount describes a volume mount within a container.
type VolumeMount struct {
	Name      string `json:"name"`
	MountPath string `json:"mountPath"`
	ReadOnly  bool   `json:"readOnly,omitempty"`
}

// Volume defines a volume available to containers in a pod.
type Volume struct {
	Name   string        `json:"name"`
	Secret *SecretVolume `json:"secret,omitempty"`
}

// SecretVolume projects a Kubernetes Secret into a volume.
type SecretVolume struct {
	SecretName string `json:"secretName"`
	Optional   bool   `json:"optional,omitempty"`
}

// ============================================================================
// ExternalSecret Types
// ============================================================================

// ExternalSecretSpec defines the spec for an external-secrets.io ExternalSecret.
type ExternalSecretSpec struct {
	SecretStoreRef  SecretStoreRef           `json:"secretStoreRef"`
	Target          ExternalSecretTarget     `json:"target"`
	Data            []ExternalSecretData     `json:"data,omitempty"`
	DataFrom        []ExternalSecretDataFrom `json:"dataFrom,omitempty"`
	RefreshInterval string                   `json:"refreshInterval,omitempty"`
}

// SecretStoreRef references a SecretStore or ClusterSecretStore.
type SecretStoreRef struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
}

// ExternalSecretTarget defines the target Secret for an ExternalSecret.
type ExternalSecretTarget struct {
	Name     string                  `json:"name,omitempty"`
	Template *ExternalSecretTemplate `json:"template,omitempty"`
}

// ExternalSecretTemplate defines the template for the target Secret.
type ExternalSecretTemplate struct {
	EngineVersion string            `json:"engineVersion,omitempty"`
	Type          string            `json:"type,omitempty"`
	Metadata      *TemplateMetadata `json:"metadata,omitempty"`
	Data          map[string]string `json:"data,omitempty"`
}

// TemplateMetadata holds metadata for an ExternalSecret template.
type TemplateMetadata struct {
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

// ExternalSecretData maps a remote secret key to a local Secret key.
type ExternalSecretData struct {
	SecretKey string    `json:"secretKey"`
	RemoteRef RemoteRef `json:"remoteRef"`
}

// RemoteRef references a value in an external secret store.
type RemoteRef struct {
	Key      string `json:"key"`
	Property string `json:"property,omitempty"`
}

// ExternalSecretDataFrom extracts multiple keys from an external secret.
type ExternalSecretDataFrom struct {
	Extract *ExternalSecretExtract `json:"extract"`
}

// ExternalSecretExtract configures extraction from an external secret store.
type ExternalSecretExtract struct {
	Key                string `json:"key"`
	ConversionStrategy string `json:"conversionStrategy,omitempty"`
	DecodingStrategy   string `json:"decodingStrategy,omitempty"`
}
