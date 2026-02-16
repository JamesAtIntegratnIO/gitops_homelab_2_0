package platform

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// VClusterSpec represents the spec of a VClusterOrchestratorV2 resource request.
type VClusterSpec struct {
	Name            string           `yaml:"name"`
	TargetNamespace string           `yaml:"targetNamespace"`
	ProjectName     string           `yaml:"projectName"`
	VCluster        VClusterConfig   `yaml:"vcluster"`
	Exposure        ExposureConfig   `yaml:"exposure"`
	Integrations    IntegrationsCfg  `yaml:"integrations"`
	ArgocdApp       ArgocdAppConfig  `yaml:"argocdApplication"`
	NetworkPolicies NetworkPolConfig `yaml:"networkPolicies"`
}

// VClusterConfig holds vCluster-specific settings.
type VClusterConfig struct {
	Preset        string                 `yaml:"preset"`
	Replicas      int                    `yaml:"replicas,omitempty"`
	K8sVersion    string                 `yaml:"k8sVersion,omitempty"`
	HelmOverrides map[string]interface{} `yaml:"helmOverrides,omitempty"`
	Resources     *ResourceRequirements  `yaml:"resources,omitempty"`
	BackingStore  map[string]interface{} `yaml:"backingStore,omitempty"`
}

// ResourceRequirements holds resource requests and limits.
type ResourceRequirements struct {
	Requests map[string]string `yaml:"requests,omitempty"`
	Limits   map[string]string `yaml:"limits,omitempty"`
}

// ExposureConfig holds network exposure settings.
type ExposureConfig struct {
	Hostname string `yaml:"hostname"`
	APIPort  int    `yaml:"apiPort,omitempty"`
}

// IntegrationsCfg holds platform integration settings.
type IntegrationsCfg struct {
	CertManager     *CertManagerCfg     `yaml:"certManager,omitempty"`
	ExternalSecrets *ExternalSecretsCfg `yaml:"externalSecrets,omitempty"`
	ArgoCD          *ArgoCDIntegration  `yaml:"argocd,omitempty"`
}

// CertManagerCfg holds cert-manager integration config.
type CertManagerCfg struct {
	ClusterIssuerSelectorLabels map[string]string `yaml:"clusterIssuerSelectorLabels,omitempty"`
}

// ExternalSecretsCfg holds external-secrets integration config.
type ExternalSecretsCfg struct {
	ClusterStoreSelectorLabels map[string]string `yaml:"clusterStoreSelectorLabels,omitempty"`
}

// ArgoCDIntegration holds ArgoCD-specific integration settings.
type ArgoCDIntegration struct {
	Environment        string            `yaml:"environment"`
	ClusterLabels      map[string]string `yaml:"clusterLabels,omitempty"`
	ClusterAnnotations map[string]string `yaml:"clusterAnnotations,omitempty"`
}

// ArgocdAppConfig holds ArgoCD Application deployment config.
type ArgocdAppConfig struct {
	RepoURL        string                 `yaml:"repoURL"`
	Chart          string                 `yaml:"chart"`
	TargetRevision string                 `yaml:"targetRevision"`
	SyncPolicy     map[string]interface{} `yaml:"syncPolicy,omitempty"`
}

// NetworkPolConfig holds network policy settings.
type NetworkPolConfig struct {
	EnableNFS  bool          `yaml:"enableNFS"`
	ExtraEgress []EgressRule `yaml:"extraEgress,omitempty"`
}

// EgressRule defines a custom egress network policy rule.
type EgressRule struct {
	Name     string `yaml:"name"`
	CIDR     string `yaml:"cidr"`
	Port     int    `yaml:"port"`
	Protocol string `yaml:"protocol"`
}

// VClusterResource builds the full Kubernetes resource YAML for a VClusterOrchestratorV2.
type VClusterResource struct {
	APIVersion string            `yaml:"apiVersion"`
	Kind       string            `yaml:"kind"`
	Metadata   ResourceMetadata  `yaml:"metadata"`
	Spec       VClusterSpec      `yaml:"spec"`
}

// ResourceMetadata holds standard Kubernetes metadata.
type ResourceMetadata struct {
	Name        string            `yaml:"name"`
	Namespace   string            `yaml:"namespace"`
	Annotations map[string]string `yaml:"annotations,omitempty"`
}

// Presets defines the preset configurations for vClusters.
var Presets = map[string]PresetConfig{
	"dev": {
		Replicas: 1,
		Memory:   "512Mi",
		BackingStore: nil,
		Resources: &ResourceRequirements{
			Requests: map[string]string{"memory": "512Mi"},
			Limits:   map[string]string{"memory": "512Mi"},
		},
	},
	"prod": {
		Replicas: 3,
		Memory:   "2Gi",
		BackingStore: map[string]interface{}{
			"etcd": map[string]interface{}{
				"deploy": map[string]interface{}{
					"enabled": true,
					"statefulSet": map[string]interface{}{
						"highAvailability": map[string]interface{}{
							"replicas": 3,
						},
					},
				},
			},
		},
		Resources: &ResourceRequirements{
			Requests: map[string]string{"memory": "2Gi"},
			Limits:   map[string]string{"memory": "2Gi"},
		},
	},
}

// PresetConfig defines default values for a vCluster preset.
type PresetConfig struct {
	Replicas     int
	Memory       string
	BackingStore map[string]interface{}
	Resources    *ResourceRequirements
}

// NewVClusterResource creates a VClusterOrchestratorV2 resource from the given spec.
func NewVClusterResource(spec VClusterSpec, namespace string) *VClusterResource {
	if spec.TargetNamespace == "" {
		spec.TargetNamespace = spec.Name
	}
	if spec.ProjectName == "" {
		spec.ProjectName = spec.Name
	}

	return &VClusterResource{
		APIVersion: "platform.integratn.tech/v1alpha1",
		Kind:       "VClusterOrchestratorV2",
		Metadata: ResourceMetadata{
			Name:      spec.Name,
			Namespace: namespace,
		},
		Spec: spec,
	}
}

// ApplyPreset applies preset defaults to a VClusterSpec.
func ApplyPreset(spec *VClusterSpec, preset string) error {
	p, ok := Presets[preset]
	if !ok {
		return fmt.Errorf("unknown preset: %s (available: %s)", preset, strings.Join(PresetNames(), ", "))
	}

	spec.VCluster.Preset = preset
	if spec.VCluster.Replicas == 0 {
		spec.VCluster.Replicas = p.Replicas
	}
	if spec.VCluster.Resources == nil {
		spec.VCluster.Resources = p.Resources
	}
	if spec.VCluster.BackingStore == nil && p.BackingStore != nil {
		spec.VCluster.BackingStore = p.BackingStore
	}

	return nil
}

// PresetNames returns the available preset names.
func PresetNames() []string {
	names := make([]string, 0, len(Presets))
	for k := range Presets {
		names = append(names, k)
	}
	return names
}

// DefaultIntegrations returns the standard platform integration config.
func DefaultIntegrations() IntegrationsCfg {
	return IntegrationsCfg{
		CertManager: &CertManagerCfg{
			ClusterIssuerSelectorLabels: map[string]string{
				"integratn.tech/cluster-issuer": "letsencrypt-prod",
			},
		},
		ExternalSecrets: &ExternalSecretsCfg{
			ClusterStoreSelectorLabels: map[string]string{
				"integratn.tech/cluster-secret-store": "onepassword-store",
			},
		},
		ArgoCD: &ArgoCDIntegration{
			Environment: "production",
		},
	}
}

// DefaultArgocdApp returns the standard ArgoCD Application config.
func DefaultArgocdApp() ArgocdAppConfig {
	return ArgocdAppConfig{
		RepoURL:        "https://charts.loft.sh",
		Chart:          "vcluster",
		TargetRevision: "0.31.0",
		SyncPolicy: map[string]interface{}{
			"automated": map[string]interface{}{
				"selfHeal": true,
				"prune":    true,
			},
			"syncOptions": []string{
				"CreateNamespace=true",
			},
		},
	}
}

// Marshal serializes the resource to YAML.
func (r *VClusterResource) Marshal() ([]byte, error) {
	return yaml.Marshal(r)
}
