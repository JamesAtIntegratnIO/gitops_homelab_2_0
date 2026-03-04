package vcluster

import (
	"fmt"
	"strings"

	"github.com/jamesatintegratnio/hctl/internal/config"
	"github.com/jamesatintegratnio/hctl/internal/platform"
	"github.com/jamesatintegratnio/hctl/internal/tui"
	"github.com/spf13/cobra"
)

// buildVClusterSpec constructs a VClusterSpec from the preset, flags, and interactive wizard.
func buildVClusterSpec(cmd *cobra.Command, cfg *config.Config, name, preset string, interactive bool) (platform.VClusterSpec, error) {
	spec := platform.VClusterSpec{
		Name:            name,
		TargetNamespace: name,
		ProjectName:     name,
		Integrations:    platform.DefaultIntegrations(),
		ArgocdApp:       platform.DefaultArgocdApp(),
	}

	if err := platform.ApplyPreset(&spec, preset); err != nil {
		return spec, fmt.Errorf("applying preset %q: %w", preset, err)
	}

	// Apply flag overrides
	applyFlagOverrides(cmd, &spec)

	// Hostname
	hostname := createHostname
	if interactive && hostname == "" {
		defaultHost := fmt.Sprintf("%s.%s", name, cfg.Platform.Domain)
		val, err := tui.Input("External hostname", "e.g. "+defaultHost, defaultHost)
		if err != nil {
			return spec, fmt.Errorf("collecting hostname: %w", err)
		}
		hostname = val
	}
	if hostname == "" {
		hostname = fmt.Sprintf("%s.%s", name, cfg.Platform.Domain)
	}
	spec.Exposure = platform.ExposureConfig{
		Hostname: hostname,
		APIPort:  createAPIPort,
	}
	if createSubnet != "" {
		spec.Exposure.Subnet = createSubnet
	}
	if createVIP != "" {
		spec.Exposure.VIP = createVIP
	}

	// Persistence overrides
	if cmd.Flags().Changed("persistence") || createPersistenceSize != "" || createStorageClass != "" {
		if spec.VCluster.Persistence == nil {
			spec.VCluster.Persistence = &platform.PersistenceConfig{}
		}
		if cmd.Flags().Changed("persistence") {
			spec.VCluster.Persistence.Enabled = createPersistence
		}
		if createPersistenceSize != "" {
			spec.VCluster.Persistence.Size = createPersistenceSize
		}
		if createStorageClass != "" {
			spec.VCluster.Persistence.StorageClass = createStorageClass
		}
	}

	// CoreDNS
	if createCoreDNSReplicas > 0 {
		spec.VCluster.CoreDNS = &platform.CoreDNSConfig{Replicas: createCoreDNSReplicas}
	}

	// NFS
	if interactive && !cmd.Flags().Changed("enable-nfs") {
		confirmed, _ := tui.Confirm("Enable NFS egress?")
		createEnableNFS = confirmed
	}
	spec.NetworkPolicies.EnableNFS = createEnableNFS

	// Extra egress
	if len(createExtraEgress) > 0 {
		for _, rule := range createExtraEgress {
			eg, err := parseEgressRule(rule)
			if err != nil {
				return spec, err
			}
			spec.NetworkPolicies.ExtraEgress = append(spec.NetworkPolicies.ExtraEgress, eg)
		}
	}

	// ArgoCD environment
	if spec.Integrations.ArgoCD != nil {
		spec.Integrations.ArgoCD.Environment = createEnvironment
	}

	// Interactive advanced settings
	if interactive {
		if err := collectAdvancedSettings(cmd, &spec); err != nil {
			return spec, fmt.Errorf("collecting advanced settings: %w", err)
		}
	}

	// Cluster labels/annotations
	if err := applyClusterMetadata(cmd, &spec); err != nil {
		return spec, err
	}

	// Workload repo
	if err := collectWorkloadRepo(cmd, &spec, interactive); err != nil {
		return spec, fmt.Errorf("configuring workload repo: %w", err)
	}

	// Chart version override
	if createChartVersion != "" {
		spec.ArgocdApp.TargetRevision = createChartVersion
	}

	// Prod preset extras
	if preset == "prod" {
		applyProdPresetExtras(&spec, name)
	}

	return spec, nil
}

// applyFlagOverrides applies simple flag-based overrides to the spec.
func applyFlagOverrides(cmd *cobra.Command, spec *platform.VClusterSpec) {
	if createK8sVersion != "" {
		spec.VCluster.K8sVersion = createK8sVersion
	}
	if createReplicas > 0 {
		spec.VCluster.Replicas = createReplicas
	}
	if createIsolationMode != "" {
		spec.VCluster.IsolationMode = createIsolationMode
	}
}

// applyProdPresetExtras adds production-specific helm overrides (etcd certs, HA configuration).
func applyProdPresetExtras(spec *platform.VClusterSpec, name string) {
	spec.VCluster.HelmOverrides = map[string]interface{}{
		"controlPlane": map[string]interface{}{
			"statefulSet": map[string]interface{}{
				"persistence": map[string]interface{}{
					"addVolumes": []interface{}{
						map[string]interface{}{
							"name": "etcd-certs",
							"secret": map[string]interface{}{
								"secretName": name + "-etcd-certs",
							},
						},
					},
					"addVolumeMounts": []interface{}{
						map[string]interface{}{
							"name":      "etcd-certs",
							"mountPath": "/etcd-certs",
							"readOnly":  true,
						},
					},
				},
			},
			"backingStore": map[string]interface{}{
				"etcd": map[string]interface{}{
					"deploy": map[string]interface{}{
						"enabled": true,
						"statefulSet": map[string]interface{}{
							"extraArgs": []string{"--client-cert-auth=false"},
							"highAvailability": map[string]interface{}{
								"replicas": spec.VCluster.Replicas,
							},
						},
					},
				},
			},
			"ingress": map[string]interface{}{
				"enabled": false,
			},
		},
		"integrations": map[string]interface{}{
			"metricsServer": map[string]interface{}{
				"enabled": false,
			},
		},
	}
}

// parseEgressRule parses "name:cidr:port[:protocol]" into an EgressRule.
func parseEgressRule(s string) (platform.EgressRule, error) {
	parts := strings.SplitN(s, ":", 4)
	if len(parts) < 3 {
		return platform.EgressRule{}, fmt.Errorf("invalid egress rule %q: expected name:cidr:port[:protocol]", s)
	}
	port := 0
	if _, err := fmt.Sscanf(parts[2], "%d", &port); err != nil {
		return platform.EgressRule{}, fmt.Errorf("invalid port in egress rule %q: %w", s, err)
	}
	protocol := "TCP"
	if len(parts) == 4 && parts[3] != "" {
		protocol = strings.ToUpper(parts[3])
		if protocol != "TCP" && protocol != "UDP" {
			return platform.EgressRule{}, fmt.Errorf("invalid protocol %q in egress rule (must be TCP or UDP)", parts[3])
		}
	}
	return platform.EgressRule{
		Name:     parts[0],
		CIDR:     parts[1],
		Port:     port,
		Protocol: protocol,
	}, nil
}

// parseKeyValue parses "key=value" into separate key and value strings.
func parseKeyValue(kv string) (string, string, error) {
	parts := strings.SplitN(kv, "=", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("expected key=value format")
	}
	if parts[0] == "" {
		return "", "", fmt.Errorf("key cannot be empty")
	}
	return parts[0], parts[1], nil
}
