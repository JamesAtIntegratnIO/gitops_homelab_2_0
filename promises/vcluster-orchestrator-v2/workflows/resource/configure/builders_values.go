package main

import (
	"fmt"

	kratix "github.com/syntasso/kratix-go"

	u "github.com/jamesatintegratnio/gitops_homelab_2_0/promises/_shared/kratixutil"
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
			Enabled: true,
			Deployment: DeploymentConfig{Replicas: config.CorednsReplicas},
			OverwriteConfig: fmt.Sprintf(`.:1053 {
  errors
  health
  ready
  kubernetes %s in-addr.arpa ip6.arpa {
    pods insecure
    fallthrough in-addr.arpa ip6.arpa
    ttl 30
  }
  prometheus 0.0.0.0:9153
  forward . /etc/resolv.conf
  cache 30
  loop
  reload
  loadbalance
}`,
				config.ClusterDomain,
			),
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
				ExtraRules: []u.PolicyRule{
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

	valuesMap, err := u.ToMap(values)
	if err != nil {
		return nil, fmt.Errorf("failed to convert values to map: %w", err)
	}

	return u.DeepMerge(valuesMap, config.HelmOverrides), nil
}

func applyPresetDefaults(config *VClusterConfig, resource kratix.Resource) {
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
	if val, err := u.GetIntValue(resource, "spec.vcluster.replicas"); err == nil && val > 0 {
		config.Replicas = val
	} else {
		config.Replicas = defaults.Replicas
	}

	// Apply resource requests/limits
	config.CPURequest = u.GetStringValueWithDefault(resource, "spec.vcluster.resources.requests.cpu", defaults.CPURequest)
	config.MemoryRequest = u.GetStringValueWithDefault(resource, "spec.vcluster.resources.requests.memory", defaults.MemoryRequest)
	config.CPULimit = u.GetStringValueWithDefault(resource, "spec.vcluster.resources.limits.cpu", defaults.CPULimit)
	config.MemoryLimit = u.GetStringValueWithDefault(resource, "spec.vcluster.resources.limits.memory", defaults.MemoryLimit)

	// Apply persistence
	if val, err := u.GetBoolValue(resource, "spec.vcluster.persistence.enabled"); err == nil {
		config.PersistenceEnabled = val
	} else {
		config.PersistenceEnabled = defaults.PersistenceEnabled
	}

	config.PersistenceSize = u.GetStringValueWithDefault(resource, "spec.vcluster.persistence.size", defaults.PersistenceSize)

	// Apply coredns replicas
	if val, err := u.GetIntValue(resource, "spec.vcluster.coredns.replicas"); err == nil && val > 0 {
		config.CorednsReplicas = val
	} else {
		config.CorednsReplicas = defaults.CorednsReplicas
	}
}
