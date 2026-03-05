package main

import (
	u "github.com/jamesatintegratnio/gitops_homelab_2_0/promises/_shared/kratixutil"
)

// ============================================================================
// cert-manager Types
// ============================================================================

type CertificateSpec struct {
	IsCA        bool             `json:"isCA,omitempty"`
	CommonName  string           `json:"commonName"`
	SecretName  string           `json:"secretName"`
	DNSNames    []string         `json:"dnsNames,omitempty"`
	IPAddresses []string         `json:"ipAddresses,omitempty"`
	Usages      []string         `json:"usages,omitempty"`
	PrivateKey  *PrivateKeySpec  `json:"privateKey,omitempty"`
	IssuerRef   IssuerRef        `json:"issuerRef"`
	SecretTemplate *SecretTemplate `json:"secretTemplate,omitempty"`
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
// VCluster Helm Values Types
// ============================================================================

type VClusterValues struct {
	ControlPlane     ControlPlane    `json:"controlPlane"`
	Deploy           DeployConfig    `json:"deploy,omitempty"`
	Integrations     Integrations    `json:"integrations"`
	Telemetry        EnabledFlag     `json:"telemetry"`
	Logging          LoggingConfig   `json:"logging"`
	Networking       NetworkingConfig `json:"networking"`
	Sync             SyncConfig      `json:"sync"`
	RBAC             RBACConfig      `json:"rbac"`
	ExportKubeConfig interface{}     `json:"exportKubeConfig,omitempty"`
}

type EnabledFlag struct {
	Enabled bool `json:"enabled"`
}

type ControlPlane struct {
	Distro         DistroConfig      `json:"distro"`
	ServiceMonitor ServiceMonitor    `json:"serviceMonitor"`
	StatefulSet    StatefulSetConfig `json:"statefulSet"`
	CoreDNS        CoreDNSConfig     `json:"coredns"`
	Ingress        EnabledFlag       `json:"ingress"`
	Advanced       AdvancedConfig    `json:"advanced"`
	Service        ServiceConfig     `json:"service"`
	BackingStore   interface{}       `json:"backingStore,omitempty"`
	Proxy          *ProxyConfig      `json:"proxy,omitempty"`
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
	Type           string        `json:"type"`
	Ports          []ServicePort `json:"ports"`
	LoadBalancerIP string        `json:"loadBalancerIP,omitempty"`
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
	Enabled  bool          `json:"enabled"`
	Selector LabelSelector `json:"selector"`
}

type LabelSelector struct {
	MatchLabels map[string]string `json:"matchLabels,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
}

type IntegrationCertManager struct {
	Enabled bool         `json:"enabled"`
	Sync    CMSyncConfig `json:"sync"`
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
	StorageClasses EnabledFlag      `json:"storageClasses"`
	IngressClasses EnabledFlag      `json:"ingressClasses"`
	Secrets        SecretSyncConfig `json:"secrets"`
}

type SecretSyncConfig struct {
	Enabled  bool           `json:"enabled"`
	Mappings SecretMappings `json:"mappings"`
}

type SecretMappings struct {
	ByName map[string]string `json:"byName"`
}

type RBACConfig struct {
	ClusterRole ClusterRoleConfig `json:"clusterRole"`
}

type ClusterRoleConfig struct {
	Enabled    bool           `json:"enabled"`
	ExtraRules []u.PolicyRule `json:"extraRules"`
}

// ============================================================================
// Network Policy Types
// ============================================================================

// ExtraEgressRule defines a custom egress rule for the vcluster namespace.
type ExtraEgressRule struct {
	Name     string `json:"name"`
	CIDR     string `json:"cidr"`
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
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
