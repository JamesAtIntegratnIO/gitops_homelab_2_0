package main

import (
	"fmt"

	kratix "github.com/syntasso/kratix-go"

	ku "github.com/jamesatintegratnio/gitops_homelab_2_0/promises/_shared/kratixutil"
)

func buildValuesObject(config *VClusterConfig) (map[string]interface{}, error) {
	cp := ControlPlane{
		Distro: DistroConfig{
			K8s: K8sDistro{
				Enabled: true,
				Version: config.K8sVersion,
			},
		},
		ServiceMonitor: ServiceMonitor{
			Enabled: true,
			Labels: map[string]string{
				"vcluster_name":      config.Name,
				"vcluster_namespace": config.TargetNamespace,
				"environment":        config.ArgoCDEnvironment,
				"cluster_role":       "vcluster",
			},
		},
		StatefulSet: StatefulSetConfig{
			HighAvailability: HAConfig{Replicas: config.Replicas},
			Scheduling: SchedulingConfig{
				PodManagementPolicy: "Parallel",
				PriorityClassName:   "system-cluster-critical",
			},
			ImagePullPolicy: "Always",
			Image:           ImageConfig{Repository: "loft-sh/vcluster-oss"},
			Persistence: PersistenceConfig{
				VolumeClaim: VolumeClaimConfig{
					Enabled: config.PersistenceEnabled,
					Size:    config.PersistenceSize,
				},
			},
			Resources: ResourcesConfig{
				Requests: ResourceValues{CPU: config.CPURequest, Memory: config.MemoryRequest},
				Limits:   ResourceValues{CPU: config.CPULimit, Memory: config.MemoryLimit},
			},
		},
		CoreDNS: CoreDNSConfig{
			Enabled:         true,
			Deployment:      DeploymentConfig{Replicas: config.CorednsReplicas},
			OverwriteConfig: helmCorefileOverwrite(config.ClusterDomain),
		},
		Ingress: EnabledFlag{Enabled: false},
		Advanced: AdvancedConfig{
			PodDisruptionBudget: PDBConfig{Enabled: true, MinAvailable: 1},
		},
		Service: ServiceConfig{
			Enabled: true,
			Annotations: map[string]string{
				"external-dns.alpha.kubernetes.io/hostname": config.Hostname,
			},
			Spec: ServiceSpecConfig{
				Type: "LoadBalancer",
				Ports: []ServicePort{
					{
						Name:       "https",
						Port:       config.APIPort,
						TargetPort: 8443,
						Protocol:   "TCP",
					},
				},
			},
		},
	}

	if config.PersistenceClass != "" {
		cp.StatefulSet.Persistence.VolumeClaim.StorageClass = config.PersistenceClass
	}

	if config.BackingStore != nil {
		cp.BackingStore = config.BackingStore
	}

	if len(config.ProxyExtraSANs) > 0 {
		cp.Proxy = &ProxyConfig{ExtraSANs: config.ProxyExtraSANs}
	}

	if config.VIP != "" {
		cp.Service.Spec.LoadBalancerIP = config.VIP
	}

	if config.APIPort != 443 {
		cp.Service.Spec.Ports = append(cp.Service.Spec.Ports, ServicePort{
			Name:       "https-internal",
			Port:       443,
			TargetPort: 8443,
			Protocol:   "TCP",
		})
	}

	values := VClusterValues{
		ControlPlane: cp,
		Deploy: DeployConfig{
			MetalLB: EnabledFlag{Enabled: true},
		},
		Integrations: Integrations{
			ExternalSecrets: IntegrationExternalSecrets{
				Enabled: true,
				Webhook: EnabledFlag{Enabled: true},
				Sync: ESSyncConfig{
					FromHost: ESFromHostConfig{
						ClusterStores: ClusterStoresConfig{
							Enabled: true,
							Selector: LabelSelector{
								MatchLabels: config.ExternalSecretsStoreLabels,
							},
						},
					},
				},
			},
			MetricsServer: EnabledFlag{Enabled: true},
			CertManager: IntegrationCertManager{
				Enabled: true,
				Sync: CMSyncConfig{
					FromHost: CMFromHostConfig{
						ClusterIssuers: ClusterIssuersConfig{
							Enabled: true,
							Selector: LabelSelector{
								Labels: config.CertManagerIssuerLabels,
							},
						},
					},
				},
			},
		},
		Telemetry: EnabledFlag{Enabled: false},
		Logging:   LoggingConfig{Encoding: "json"},
		Networking: NetworkingConfig{
			Advanced: NetworkAdvanced{ClusterDomain: config.ClusterDomain},
			ReplicateServices: ReplicateServices{
				FromHost: []ServiceMapping{
					{From: "default/kubernetes", To: "default/kubernetes"},
				},
			},
		},
		Sync: SyncConfig{
			ToHost: SyncToHost{
				Pods:              EnabledFlag{Enabled: true},
				PersistentVolumes: EnabledFlag{Enabled: true},
				Ingresses:         EnabledFlag{Enabled: true},
				NetworkPolicies:   EnabledFlag{Enabled: true},
			},
			FromHost: SyncFromHost{
				StorageClasses: EnabledFlag{Enabled: true},
				IngressClasses: EnabledFlag{Enabled: true},
				Secrets: SecretSyncConfig{
					Enabled: true,
					Mappings: SecretMappings{
						ByName: map[string]string{
							"external-secrets/eso-onepassword-token": "external-secrets/eso-onepassword-token",
						},
					},
				},
			},
		},
		RBAC: RBACConfig{
			ClusterRole: ClusterRoleConfig{
				Enabled: true,
				ExtraRules: []ku.PolicyRule{
					{
						APIGroups:     []string{""},
						Resources:     []string{"secrets"},
						Verbs:         []string{"get", "list", "watch"},
						ResourceNames: []string{"eso-onepassword-token"},
					},
				},
			},
		},
	}

	if len(config.ExportKubeConfig) > 0 {
		values.ExportKubeConfig = config.ExportKubeConfig
	}

	valuesMap, err := ku.ToMap(values)
	if err != nil {
		return nil, fmt.Errorf("failed to convert values to map: %w", err)
	}

	return ku.DeepMerge(valuesMap, config.HelmOverrides), nil
}

func applyPresetDefaults(config *VClusterConfig, resource kratix.Resource) error {
	presetDefaults := map[string]PresetDefaults{
		"dev": {
			Replicas:           1,
			CPURequest:         "200m",
			MemoryRequest:      "768Mi",
			CPULimit:           "1000m",
			MemoryLimit:        "1536Mi",
			PersistenceEnabled: false,
			PersistenceSize:    "5Gi",
			CorednsReplicas:    1,
		},
		"prod": {
			Replicas:           3,
			CPURequest:         "500m",
			MemoryRequest:      "1Gi",
			CPULimit:           "2",
			MemoryLimit:        "2Gi",
			PersistenceEnabled: true,
			PersistenceSize:    "10Gi",
			CorednsReplicas:    2,
		},
	}

	defaults, ok := presetDefaults[config.Preset]
	if !ok {
		defaults = presetDefaults["dev"]
	}

	// Apply replicas
	if val, err := ku.GetOptionalIntValue(resource, "spec.vcluster.replicas"); err != nil {
		return fmt.Errorf("spec.vcluster.replicas: %w", err)
	} else if val > 0 {
		config.Replicas = val
	} else {
		config.Replicas = defaults.Replicas
	}

	// Apply resource requests/limits
	config.CPURequest = ku.GetStringValueWithDefault(resource, "spec.vcluster.resources.requests.cpu", defaults.CPURequest)
	config.MemoryRequest = ku.GetStringValueWithDefault(resource, "spec.vcluster.resources.requests.memory", defaults.MemoryRequest)
	config.CPULimit = ku.GetStringValueWithDefault(resource, "spec.vcluster.resources.limits.cpu", defaults.CPULimit)
	config.MemoryLimit = ku.GetStringValueWithDefault(resource, "spec.vcluster.resources.limits.memory", defaults.MemoryLimit)

	// Apply persistence
	if val, err := ku.GetOptionalBoolValue(resource, "spec.vcluster.persistence.enabled"); err != nil {
		return fmt.Errorf("spec.vcluster.persistence.enabled: %w", err)
	} else if val {
		config.PersistenceEnabled = val
	} else {
		config.PersistenceEnabled = defaults.PersistenceEnabled
	}

	config.PersistenceSize = ku.GetStringValueWithDefault(resource, "spec.vcluster.persistence.size", defaults.PersistenceSize)

	// Apply coredns replicas
	if val, err := ku.GetOptionalIntValue(resource, "spec.vcluster.coredns.replicas"); err != nil {
		return fmt.Errorf("spec.vcluster.coredns.replicas: %w", err)
	} else if val > 0 {
		config.CorednsReplicas = val
	} else {
		config.CorednsReplicas = defaults.CorednsReplicas
	}

	return nil
}
