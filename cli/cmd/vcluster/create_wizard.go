package vcluster

import (
	"fmt"

	hcerrors "github.com/jamesatintegratnio/hctl/internal/errors"
	"github.com/jamesatintegratnio/hctl/internal/platform"
	"github.com/jamesatintegratnio/hctl/internal/tui"
	"github.com/spf13/cobra"
)

// collectNameAndPreset resolves the vCluster name and preset from args, flags, or interactive prompts.
//
// Note: the wizard functions in this file have relatively deep call indirection because each
// interactive step (name → preset → advanced settings → workload repo → metadata) is a
// separate function that may prompt the user, validate, and bail out. This depth is inherent
// to the interactive-wizard pattern and is preferred over a single monolithic function.
func collectNameAndPreset(cmd *cobra.Command, args []string, opts *CreateOptions, interactive bool) (string, string, error) {
	var name string
	if len(args) > 0 {
		name = args[0]
	}

	// ── Name ──────────────────────────────────────────────────────────
	if interactive && name == "" {
		val, err := tui.Input("vCluster name", "e.g. my-project", "")
		if err != nil {
			return "", "", hcerrors.NewUserError("prompting vcluster name: %w", err)
		}
		if val == "" {
			return "", "", hcerrors.NewUserError("name is required")
		}
		name = val
	}
	if name == "" {
		return "", "", hcerrors.NewUserError("name is required")
	}

	// ── Preset ────────────────────────────────────────────────────────
	preset := opts.Core.Preset
	if interactive && preset == "" {
		idx, err := tui.Select("Select preset", []string{
			"dev  — 1 replica, 768Mi, SQLite, no persistence",
			"prod — 3 replicas, 2Gi, etcd HA, 10Gi persistence",
		})
		if err != nil {
			return "", "", hcerrors.NewUserError("selecting preset: %w", err)
		}
		if idx < 0 {
			return "", "", hcerrors.NewUserError("cancelled")
		}
		if idx == 0 {
			preset = "dev"
		} else {
			preset = "prod"
		}
	}
	if preset == "" {
		preset = "dev"
	}

	return name, preset, nil
}

// collectAdvancedSettings runs the interactive wizard for advanced vCluster settings.
func collectAdvancedSettings(cmd *cobra.Command, opts *CreateOptions, spec *platform.VClusterSpec) error {
	advanced, confirmErr := tui.Confirm("Customize advanced settings? (k8s version, isolation, environment, persistence, networking)")
	if confirmErr != nil {
		return hcerrors.NewUserError("confirming advanced settings: %w", confirmErr)
	}
	if !advanced {
		return nil
	}

	// K8s version
	if !cmd.Flags().Changed("k8s-version") {
		idx, err := tui.Select("Kubernetes version", []string{
			"v1.34.3 (default)",
			"1.33",
			"1.32",
		})
		if err != nil {
			return hcerrors.NewUserError("selecting k8s version: %w", err)
		}
		switch idx {
		case 1:
			spec.VCluster.K8sVersion = "1.33"
		case 2:
			spec.VCluster.K8sVersion = "1.32"
		}
	}

	// Isolation mode
	if !cmd.Flags().Changed("isolation") {
		idx, err := tui.Select("Isolation mode", []string{
			"standard — shared kernel, namespace isolation (default)",
			"strict   — resource quotas, limit ranges, network policies",
		})
		if err != nil {
			return hcerrors.NewUserError("selecting isolation mode: %w", err)
		}
		if idx == 1 {
			spec.VCluster.IsolationMode = "strict"
		}
	}

	// Environment
	if spec.Integrations.ArgoCD != nil && !cmd.Flags().Changed("environment") {
		idx, err := tui.Select("ArgoCD environment", []string{
			"production",
			"staging",
			"development",
		})
		if err != nil {
			return hcerrors.NewUserError("selecting environment: %w", err)
		}
		switch idx {
		case 0:
			spec.Integrations.ArgoCD.Environment = "production"
		case 1:
			spec.Integrations.ArgoCD.Environment = "staging"
		case 2:
			spec.Integrations.ArgoCD.Environment = "development"
		}
	}

	// Persistence
	if !cmd.Flags().Changed("persistence") && !cmd.Flags().Changed("persistence-size") {
		enablePersist, confirmErr := tui.Confirm(fmt.Sprintf("Enable persistence? (preset default: %v)",
			spec.VCluster.Persistence != nil && spec.VCluster.Persistence.Enabled))
		if confirmErr != nil {
			return hcerrors.NewUserError("confirming persistence: %w", confirmErr)
		}
		if enablePersist {
			if spec.VCluster.Persistence == nil {
				spec.VCluster.Persistence = &platform.PersistenceConfig{}
			}
			spec.VCluster.Persistence.Enabled = true
			size, err := tui.Input("Persistence size", "e.g. 10Gi", "10Gi")
			if err != nil {
				return hcerrors.NewUserError("collecting persistence size: %w", err)
			}
			if size != "" {
				spec.VCluster.Persistence.Size = size
			}
		}
	}

	// Subnet / VIP
	if !cmd.Flags().Changed("subnet") {
		subnet, err := tui.Input("VIP subnet (optional)", "e.g. 10.0.4.0/24", "")
		if err != nil {
			return hcerrors.NewUserError("collecting subnet: %w", err)
		}
		if subnet != "" {
			spec.Exposure.Subnet = subnet
			if !cmd.Flags().Changed("vip") {
				vip, err := tui.Input("Static VIP (optional, auto-assigned from subnet if empty)", "e.g. 10.0.4.210", "")
				if err != nil {
					return hcerrors.NewUserError("collecting VIP address: %w", err)
				}
				if vip != "" {
					spec.Exposure.VIP = vip
				}
			}
		}
	}

	// CoreDNS replicas
	if !cmd.Flags().Changed("coredns-replicas") {
		currentReplicas := 1
		if spec.VCluster.CoreDNS != nil {
			currentReplicas = spec.VCluster.CoreDNS.Replicas
		}
		val, err := tui.Input("CoreDNS replicas", "", fmt.Sprintf("%d", currentReplicas))
		if err != nil {
			return hcerrors.NewUserError("collecting coredns replicas: %w", err)
		}
		if val != "" && val != fmt.Sprintf("%d", currentReplicas) {
			var r int
			if _, err := fmt.Sscanf(val, "%d", &r); err == nil && r > 0 {
				spec.VCluster.CoreDNS = &platform.CoreDNSConfig{Replicas: r}
			}
		}
	}

	return nil
}

// applyClusterMetadata parses and applies cluster labels and annotations from flags.
func applyClusterMetadata(cmd *cobra.Command, opts *CreateOptions, spec *platform.VClusterSpec) error {
	if len(opts.ClusterLabels) > 0 && spec.Integrations.ArgoCD != nil {
		if spec.Integrations.ArgoCD.ClusterLabels == nil {
			spec.Integrations.ArgoCD.ClusterLabels = map[string]string{}
		}
		for _, kv := range opts.ClusterLabels {
			k, v, err := parseKeyValue(kv)
			if err != nil {
				return hcerrors.NewUserError("invalid --cluster-label %q: %w", kv, err)
			}
			spec.Integrations.ArgoCD.ClusterLabels[k] = v
		}
	}
	if len(opts.ClusterAnnotations) > 0 && spec.Integrations.ArgoCD != nil {
		if spec.Integrations.ArgoCD.ClusterAnnotations == nil {
			spec.Integrations.ArgoCD.ClusterAnnotations = map[string]string{}
		}
		for _, kv := range opts.ClusterAnnotations {
			k, v, err := parseKeyValue(kv)
			if err != nil {
				return hcerrors.NewUserError("invalid --cluster-annotation %q: %w", kv, err)
			}
			spec.Integrations.ArgoCD.ClusterAnnotations[k] = v
		}
	}
	return nil
}

// collectWorkloadRepo handles workload repo configuration from flags or interactive prompts.
func collectWorkloadRepo(cmd *cobra.Command, opts *CreateOptions, spec *platform.VClusterSpec, interactive bool) error {
	hasWorkloadFlags := opts.WorkloadRepo.URL != "" || opts.WorkloadRepo.BasePath != "" ||
		opts.WorkloadRepo.Path != "" || opts.WorkloadRepo.Revision != ""

	if interactive && !hasWorkloadFlags {
		confirmed, confirmErr := tui.Confirm("Use a custom workload repository? (default: workloads/ in this repo)")
		if confirmErr != nil {
			return hcerrors.NewUserError("confirming workload repo: %w", confirmErr)
		}
		if confirmed {
			url, err := tui.Input("Workload repo URL", "e.g. https://github.com/myorg/my-workloads", "")
			if err != nil {
				return hcerrors.NewUserError("collecting workload repo URL: %w", err)
			}
			if url != "" {
				opts.WorkloadRepo.URL = url
			}

			basePath, err := tui.Input("Base path in repo (optional)", "e.g. clusters/dev-team-1", "")
			if err != nil {
				return hcerrors.NewUserError("collecting workload repo base path: %w", err)
			}
			if basePath != "" {
				opts.WorkloadRepo.BasePath = basePath
			}

			path, err := tui.Input("Workload path", "directory containing manifests", "workloads")
			if err != nil {
				return hcerrors.NewUserError("collecting workload path: %w", err)
			}
			if path != "" {
				opts.WorkloadRepo.Path = path
			}

			rev, err := tui.Input("Git revision (branch/tag)", "", "main")
			if err != nil {
				return hcerrors.NewUserError("collecting git revision: %w", err)
			}
			if rev != "" {
				opts.WorkloadRepo.Revision = rev
			}

			hasWorkloadFlags = opts.WorkloadRepo.URL != "" || opts.WorkloadRepo.BasePath != "" ||
				opts.WorkloadRepo.Path != "" || opts.WorkloadRepo.Revision != ""
		}
	}

	if hasWorkloadFlags && spec.Integrations.ArgoCD != nil {
		spec.Integrations.ArgoCD.WorkloadRepo = &platform.WorkloadRepoConfig{}
		if opts.WorkloadRepo.URL != "" {
			spec.Integrations.ArgoCD.WorkloadRepo.URL = opts.WorkloadRepo.URL
		}
		if opts.WorkloadRepo.BasePath != "" {
			spec.Integrations.ArgoCD.WorkloadRepo.BasePath = opts.WorkloadRepo.BasePath
		}
		if opts.WorkloadRepo.Path != "" {
			spec.Integrations.ArgoCD.WorkloadRepo.Path = opts.WorkloadRepo.Path
		}
		if opts.WorkloadRepo.Revision != "" {
			spec.Integrations.ArgoCD.WorkloadRepo.Revision = opts.WorkloadRepo.Revision
		}
	}

	return nil
}
