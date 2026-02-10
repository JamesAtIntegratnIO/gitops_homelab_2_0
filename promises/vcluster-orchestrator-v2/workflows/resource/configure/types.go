package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

// toMap converts a struct to map[string]interface{} via JSON roundtrip.
// Used at the merge boundary where typed structs meet mergeMaps.
func toMap(v interface{}) (map[string]interface{}, error) {
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

// deleteFromResource creates a minimal delete resource from a typed Resource.
func deleteFromResource(r Resource) Resource {
	return Resource{
		APIVersion: r.APIVersion,
		Kind:       r.Kind,
		Metadata: ObjectMeta{
			Name:      r.Metadata.Name,
			Namespace: r.Metadata.Namespace,
		},
	}
}

// deleteOutputPathForResource computes the output file path for a delete resource.
func deleteOutputPathForResource(prefix string, r Resource) string {
	if prefix == "" {
		prefix = "resources/"
	}
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	return fmt.Sprintf("%sdelete-%s-%s.yaml", prefix, strings.ToLower(r.Kind), r.Metadata.Name)
}

// ============================================================================
// Core Kubernetes Types
// ============================================================================

// Resource is a generic Kubernetes resource with typed spec.
type Resource struct {
	APIVersion string      `json:"apiVersion"`
	Kind       string      `json:"kind"`
	Metadata   ObjectMeta  `json:"metadata"`
	Spec       interface{} `json:"spec,omitempty"`
	Data       interface{} `json:"data,omitempty"`
	Rules      interface{} `json:"rules,omitempty"`
}

// ObjectMeta is a lightweight Kubernetes metadata block.
type ObjectMeta struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

// ============================================================================
// RBAC Types (PolicyRule kept for VCluster RBAC config)
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
	Name      string       `json:"name"`
	Value     string       `json:"value,omitempty"`
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
// cert-manager Types
// ============================================================================

type CertificateSpec struct {
	IsCA           bool              `json:"isCA,omitempty"`
	CommonName     string            `json:"commonName"`
	SecretName     string            `json:"secretName"`
	DNSNames       []string          `json:"dnsNames,omitempty"`
	IPAddresses    []string          `json:"ipAddresses,omitempty"`
	Usages         []string          `json:"usages,omitempty"`
	PrivateKey     *PrivateKeySpec   `json:"privateKey,omitempty"`
	IssuerRef      IssuerRef         `json:"issuerRef"`
	SecretTemplate *SecretTemplate   `json:"secretTemplate,omitempty"`
}

type PrivateKeySpec struct {
	Algorithm string `json:"algorithm"`
	Size      int    `json:"size"`
}

type IssuerRef struct {
	Name  string `json:"name"`
	Kind  string `json:"kind"`
	Group string `json:"group"`
}

type SecretTemplate struct {
	Labels map[string]string `json:"labels,omitempty"`
}

type IssuerSpec struct {
	SelfSigned *SelfSignedIssuer `json:"selfSigned,omitempty"`
	CA         *CAIssuer         `json:"ca,omitempty"`
}

type SelfSignedIssuer struct{}

type CAIssuer struct {
	SecretName string `json:"secretName"`
}

// ============================================================================
// ArgoCD Kratix ResourceRequest Types
// ============================================================================

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

type ArgoCDApplicationSpec struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace"`
	Annotations map[string]string `json:"annotations,omitempty"`
	Finalizers  []string          `json:"finalizers,omitempty"`
	Project     string            `json:"project"`
	Destination Destination       `json:"destination"`
	Source      AppSource         `json:"source"`
	SyncPolicy  interface{}      `json:"syncPolicy,omitempty"`
}

type Destination struct {
	Server    string `json:"server"`
	Namespace string `json:"namespace"`
}

type AppSource struct {
	RepoURL        string     `json:"repoURL"`
	Chart          string     `json:"chart,omitempty"`
	TargetRevision string     `json:"targetRevision"`
	Helm           *HelmSource `json:"helm,omitempty"`
}

type HelmSource struct {
	ReleaseName  string      `json:"releaseName,omitempty"`
	ValuesObject interface{} `json:"valuesObject,omitempty"`
}

// SyncPolicy for ArgoCD applications.
type SyncPolicy struct {
	Automated   *AutomatedSync `json:"automated,omitempty"`
	SyncOptions []string       `json:"syncOptions,omitempty"`
}

type AutomatedSync struct {
	SelfHeal bool `json:"selfHeal"`
	Prune    bool `json:"prune"`
}

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
// VCluster Helm Values Types
// ============================================================================

type VClusterValues struct {
	ControlPlane     ControlPlane         `json:"controlPlane"`
	Deploy           DeployConfig         `json:"deploy,omitempty"`
	Integrations     Integrations         `json:"integrations"`
	Telemetry        EnabledFlag          `json:"telemetry"`
	Logging          LoggingConfig        `json:"logging"`
	Networking       NetworkingConfig     `json:"networking"`
	Sync             SyncConfig           `json:"sync"`
	RBAC             RBACConfig           `json:"rbac"`
	ExportKubeConfig interface{}          `json:"exportKubeConfig,omitempty"`
}

type EnabledFlag struct {
	Enabled bool `json:"enabled"`
}

type ControlPlane struct {
	Distro         DistroConfig       `json:"distro"`
	ServiceMonitor ServiceMonitor     `json:"serviceMonitor"`
	StatefulSet    StatefulSetConfig  `json:"statefulSet"`
	CoreDNS        CoreDNSConfig      `json:"coredns"`
	Ingress        EnabledFlag        `json:"ingress"`
	Advanced       AdvancedConfig     `json:"advanced"`
	Service        ServiceConfig      `json:"service"`
	BackingStore   interface{}        `json:"backingStore,omitempty"`
	Proxy          *ProxyConfig       `json:"proxy,omitempty"`
}

type DistroConfig struct {
	K8s K8sDistro `json:"k8s"`
}

type K8sDistro struct {
	Enabled bool   `json:"enabled"`
	Version string `json:"version"`
}

type ServiceMonitor struct {
	Enabled bool              `json:"enabled"`
	Labels  map[string]string `json:"labels,omitempty"`
}

type StatefulSetConfig struct {
	HighAvailability HAConfig          `json:"highAvailability"`
	Scheduling       SchedulingConfig  `json:"scheduling"`
	ImagePullPolicy  string            `json:"imagePullPolicy"`
	Image            ImageConfig       `json:"image"`
	Persistence      PersistenceConfig `json:"persistence"`
	Resources        ResourcesConfig   `json:"resources"`
}

type HAConfig struct {
	Replicas int `json:"replicas"`
}

type SchedulingConfig struct {
	PodManagementPolicy string `json:"podManagementPolicy"`
	PriorityClassName   string `json:"priorityClassName"`
}

type ImageConfig struct {
	Repository string `json:"repository"`
}

type PersistenceConfig struct {
	VolumeClaim VolumeClaimConfig `json:"volumeClaim"`
}

type VolumeClaimConfig struct {
	Enabled      bool   `json:"enabled"`
	Size         string `json:"size"`
	StorageClass string `json:"storageClass,omitempty"`
}

type ResourcesConfig struct {
	Requests ResourceValues `json:"requests"`
	Limits   ResourceValues `json:"limits"`
}

type ResourceValues struct {
	CPU    string `json:"cpu"`
	Memory string `json:"memory"`
}

type CoreDNSConfig struct {
	Enabled         bool             `json:"enabled"`
	Deployment      DeploymentConfig `json:"deployment"`
	OverwriteConfig string           `json:"overwriteConfig,omitempty"`
}

type DeploymentConfig struct {
	Replicas int `json:"replicas"`
}

type AdvancedConfig struct {
	PodDisruptionBudget PDBConfig `json:"podDisruptionBudget"`
}

type PDBConfig struct {
	Enabled      bool `json:"enabled"`
	MinAvailable int  `json:"minAvailable"`
}

type ServiceConfig struct {
	Enabled     bool              `json:"enabled"`
	Annotations map[string]string `json:"annotations,omitempty"`
	Spec        ServiceSpecConfig `json:"spec"`
}

type ServiceSpecConfig struct {
	Type           string       `json:"type"`
	Ports          []ServicePort `json:"ports"`
	LoadBalancerIP string       `json:"loadBalancerIP,omitempty"`
}

type ServicePort struct {
	Name       string `json:"name"`
	Port       int    `json:"port"`
	TargetPort int    `json:"targetPort"`
	Protocol   string `json:"protocol"`
}

type ProxyConfig struct {
	ExtraSANs []string `json:"extraSANs"`
}

type DeployConfig struct {
	MetalLB EnabledFlag `json:"metallb"`
}

type Integrations struct {
	ExternalSecrets IntegrationExternalSecrets `json:"externalSecrets"`
	MetricsServer   EnabledFlag               `json:"metricsServer"`
	CertManager     IntegrationCertManager    `json:"certManager"`
}

type IntegrationExternalSecrets struct {
	Enabled bool        `json:"enabled"`
	Webhook EnabledFlag `json:"webhook"`
	Sync    ESSyncConfig `json:"sync"`
}

type ESSyncConfig struct {
	FromHost ESFromHostConfig `json:"fromHost"`
}

type ESFromHostConfig struct {
	ClusterStores ClusterStoresConfig `json:"clusterStores"`
}

type ClusterStoresConfig struct {
	Enabled  bool            `json:"enabled"`
	Selector LabelSelector   `json:"selector"`
}

type LabelSelector struct {
	MatchLabels map[string]string `json:"matchLabels,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
}

type IntegrationCertManager struct {
	Enabled bool           `json:"enabled"`
	Sync    CMSyncConfig   `json:"sync"`
}

type CMSyncConfig struct {
	FromHost CMFromHostConfig `json:"fromHost"`
}

type CMFromHostConfig struct {
	ClusterIssuers ClusterIssuersConfig `json:"clusterIssuers"`
}

type ClusterIssuersConfig struct {
	Enabled  bool          `json:"enabled"`
	Selector LabelSelector `json:"selector"`
}

type LoggingConfig struct {
	Encoding string `json:"encoding"`
}

type NetworkingConfig struct {
	Advanced          NetworkAdvanced   `json:"advanced"`
	ReplicateServices ReplicateServices `json:"replicateServices"`
}

type NetworkAdvanced struct {
	ClusterDomain string `json:"clusterDomain"`
}

type ReplicateServices struct {
	FromHost []ServiceMapping `json:"fromHost"`
}

type ServiceMapping struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type SyncConfig struct {
	ToHost   SyncToHost   `json:"toHost"`
	FromHost SyncFromHost `json:"fromHost"`
}

type SyncToHost struct {
	Pods              EnabledFlag `json:"pods"`
	PersistentVolumes EnabledFlag `json:"persistentVolumes"`
	Ingresses         EnabledFlag `json:"ingresses"`
	NetworkPolicies   EnabledFlag `json:"networkPolicies"`
}

type SyncFromHost struct {
	StorageClasses EnabledFlag       `json:"storageClasses"`
	IngressClasses EnabledFlag       `json:"ingressClasses"`
	Secrets        SecretSyncConfig  `json:"secrets"`
}

type SecretSyncConfig struct {
	Enabled  bool              `json:"enabled"`
	Mappings SecretMappings    `json:"mappings"`
}

type SecretMappings struct {
	ByName map[string]string `json:"byName"`
}

type RBACConfig struct {
	ClusterRole ClusterRoleConfig `json:"clusterRole"`
}

type ClusterRoleConfig struct {
	Enabled    bool         `json:"enabled"`
	ExtraRules []PolicyRule `json:"extraRules"`
}

// ============================================================================
// Preset Types
// ============================================================================

type PresetDefaults struct {
	Replicas           int
	CPURequest         string
	MemoryRequest      string
	CPULimit           string
	MemoryLimit        string
	PersistenceEnabled bool
	PersistenceSize    string
	CorednsReplicas    int
}
