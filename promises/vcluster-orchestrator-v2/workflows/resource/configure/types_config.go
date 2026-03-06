package main

import (
	ku "github.com/jamesatintegratnio/gitops_homelab_2_0/promises/_shared/kratixutil"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// KubeClientFactory abstracts Kubernetes client creation for testability.
type KubeClientFactory interface {
	NewClient() (kubernetes.Interface, error)
}

// InClusterClientFactory creates clients using in-cluster config.
type InClusterClientFactory struct{}

func (f InClusterClientFactory) NewClient() (kubernetes.Interface, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(config)
}

// VClusterConfig holds all configuration for template rendering.
type VClusterConfig struct {
	// Basic identity
	Name            string
	Namespace       string
	ProjectName     string
	TargetNamespace string

	// Vcluster configuration
	K8sVersion       string
	Preset           string
	ClusterDomain    string
	BackingStore     map[string]interface{}
	ExportKubeConfig map[string]interface{}
	HelmOverrides    map[string]interface{}
	ValuesObject     map[string]interface{}

	VClusterResourceConfig    // Replicas, CPU/Memory, Persistence, CorednsReplicas
	ExposureConfig            // Hostname, VIP, Subnet, APIPort, ExternalServerURL, ProxyExtraSANs
	VClusterIntegrationConfig // CertManager/ExternalSecrets labels, ArgoCD env/labels, WorkloadRepo
	ArgoCDAppConfig           // RepoURL, Chart, TargetRevision, DestServer, SyncPolicy

	// Network policy configuration
	EnableNFS   bool
	ExtraEgress []ExtraEgressRule

	// Derived values
	OnePasswordItem       string
	KubeconfigSyncJobName string
	BaseDomain            string
	BaseDomainSanitized   string

	// EtcdEnabled is set during buildConfig via etcdEnabledE validation.
	// Post-validation callers use this field directly, avoiding error-discarding convenience wrappers.
	EtcdEnabled bool

	// Client factory for direct Kubernetes API calls (delete pipeline)
	KubeClient KubeClientFactory

	PromiseName string
}

// VClusterResourceConfig groups compute and storage resource settings.
type VClusterResourceConfig struct {
	Replicas           int
	CPURequest         string
	MemoryRequest      string
	CPULimit           string
	MemoryLimit        string
	PersistenceEnabled bool
	PersistenceSize    string
	PersistenceClass   string
	CorednsReplicas    int
}

// ExposureConfig groups vcluster network exposure settings.
type ExposureConfig struct {
	Hostname          string
	VIP               string
	Subnet            string
	APIPort           int
	ExternalServerURL string
	ProxyExtraSANs    []string
}

// VClusterIntegrationConfig groups platform integration settings.
type VClusterIntegrationConfig struct {
	CertManagerIssuerLabels    map[string]string
	ExternalSecretsStoreLabels map[string]string
	ArgoCDEnvironment          string
	ArgoCDClusterLabels        map[string]string
	ArgoCDClusterAnnotations   map[string]string
	WorkloadRepoURL            string
	WorkloadRepoBasePath       string
	WorkloadRepoPath           string
	WorkloadRepoRevision       string
}

// ArgoCDAppConfig groups ArgoCD application source settings.
type ArgoCDAppConfig struct {
	ArgoCDRepoURL        string
	ArgoCDChart          string
	ArgoCDTargetRevision string
	ArgoCDDestServer     string
	ArgoCDSyncPolicy     *ku.SyncPolicy
}

// ExtraEgressRule defines a custom egress rule for the vcluster namespace.
type ExtraEgressRule struct {
	Name     string `json:"name"`
	CIDR     string `json:"cidr"`
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
}

// PresetDefaults defines resource defaults for each vcluster preset tier.
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
