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
func buildVClusterSpec(cmd *cobra.Command, cfg *config.Config, opts *CreateOptions, name, preset string, interactive bool) (platform.VClusterSpec, error) {
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
	applyFlagOverrides(cmd, opts, &spec)

	// Hostname
	hostname := opts.Core.Hostname
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
		APIPort:  opts.Exposure.APIPort,
	}
	if opts.Exposure.Subnet != "" {
		spec.Exposure.Subnet = opts.Exposure.Subnet
	}
	if opts.Exposure.VIP != "" {
		spec.Exposure.VIP = opts.Exposure.VIP
	}

	// Persistence overrides
	if cmd.Flags().Changed("persistence") || opts.Persistence.Size != "" || opts.Persistence.StorageClass != "" {
		if spec.VCluster.Persistence == nil {
			spec.VCluster.Persistence = &platform.PersistenceConfig{}
		}
		if cmd.Flags().Changed("persistence") {
			spec.VCluster.Persistence.Enabled = opts.Persistence.Enabled
		}
		if opts.Persistence.Size != "" {
			spec.VCluster.Persistence.Size = opts.Persistence.Size
		}
		if opts.Persistence.StorageClass != "" {
			spec.VCluster.Persistence.StorageClass = opts.Persistence.StorageClass
		}
	}

	// CoreDNS
	if opts.CoreDNSReplicas > 0 {
		spec.VCluster.CoreDNS = &platform.CoreDNSConfig{Replicas: opts.CoreDNSReplicas}
	}

	// NFS
	if interactive && !cmd.Flags().Changed("enable-nfs") {
		confirmed, confirmErr := tui.Confirm("Enable NFS egress?")
		if confirmErr != nil {
			return spec, fmt.Errorf("confirming NFS egress: %w", confirmErr)
		}
		opts.EnableNFS = confirmed
	}
	spec.NetworkPolicies.EnableNFS = opts.EnableNFS

	// Extra egress
	if len(opts.ExtraEgress) > 0 {
		for _, rule := range opts.ExtraEgress {
			eg, err := parseEgressRule(rule)
			if err != nil {
				return spec, err
			}
			spec.NetworkPolicies.ExtraEgress = append(spec.NetworkPolicies.ExtraEgress, eg)
		}
	}

	// ArgoCD environment
	if spec.Integrations.ArgoCD != nil {
		spec.Integrations.ArgoCD.Environment = opts.Core.Environment
	}

	// Interactive advanced settings
	if interactive {
		if err := collectAdvancedSettings(cmd, opts, &spec); err != nil {
			return spec, fmt.Errorf("collecting advanced settings: %w", err)
		}
	}

	// Cluster labels/annotations
	if err := applyClusterMetadata(cmd, opts, &spec); err != nil {
		return spec, err
	}

	// Workload repo
	if err := collectWorkloadRepo(cmd, opts, &spec, interactive); err != nil {
		return spec, fmt.Errorf("configuring workload repo: %w", err)
	}

	// Chart version override
	if opts.ChartVersion != "" {
		spec.ArgocdApp.TargetRevision = opts.ChartVersion
	}

	// Prod preset extras
	if preset == "prod" {
		applyProdPresetExtras(&spec, name)
	}

	return spec, nil
}

// applyFlagOverrides applies simple flag-based overrides to the spec.
func applyFlagOverrides(cmd *cobra.Command, opts *CreateOptions, spec *platform.VClusterSpec) {
	if opts.Core.K8sVersion != "" {
		spec.VCluster.K8sVersion = opts.Core.K8sVersion
	}
	if opts.Core.Replicas > 0 {
		spec.VCluster.Replicas = opts.Core.Replicas
	}
	if opts.Core.IsolationMode != "" {
		spec.VCluster.IsolationMode = opts.Core.IsolationMode
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
