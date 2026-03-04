package vcluster

import (
	"fmt"

	"github.com/jamesatintegratnio/hctl/internal/platform"
	"github.com/jamesatintegratnio/hctl/internal/tui"
	"github.com/spf13/cobra"
)

// collectNameAndPreset resolves the vCluster name and preset from args, flags, or interactive prompts.
func collectNameAndPreset(cmd *cobra.Command, args []string, interactive bool) (string, string, error) {
	var name string
	if len(args) > 0 {
		name = args[0]
	}

	// ── Name ──────────────────────────────────────────────────────────
	if interactive && name == "" {
		val, err := tui.Input("vCluster name", "e.g. my-project", "")
		if err != nil {
			return "", "", fmt.Errorf("prompting vcluster name: %w", err)
		}
		if val == "" {
			return "", "", fmt.Errorf("name is required")
		}
		name = val
	}
	if name == "" {
		return "", "", fmt.Errorf("name is required")
	}

	// ── Preset ────────────────────────────────────────────────────────
	preset := createPreset
	if interactive && preset == "" {
		idx, err := tui.Select("Select preset", []string{
			"dev  — 1 replica, 768Mi, SQLite, no persistence",
			"prod — 3 replicas, 2Gi, etcd HA, 10Gi persistence",
		})
		if err != nil {
			return "", "", fmt.Errorf("selecting preset: %w", err)
		}
		if idx < 0 {
			return "", "", fmt.Errorf("cancelled")
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
func collectAdvancedSettings(cmd *cobra.Command, spec *platform.VClusterSpec) error {
	advanced, _ := tui.Confirm("Customize advanced settings? (k8s version, isolation, environment, persistence, networking)")
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
			return fmt.Errorf("selecting k8s version: %w", err)
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
			return fmt.Errorf("selecting isolation mode: %w", err)
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
			return fmt.Errorf("selecting environment: %w", err)
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
		enablePersist, _ := tui.Confirm(fmt.Sprintf("Enable persistence? (preset default: %v)",
			spec.VCluster.Persistence != nil && spec.VCluster.Persistence.Enabled))
		if enablePersist {
			if spec.VCluster.Persistence == nil {
				spec.VCluster.Persistence = &platform.PersistenceConfig{}
			}
			spec.VCluster.Persistence.Enabled = true
			size, err := tui.Input("Persistence size", "e.g. 10Gi", "10Gi")
			if err != nil {
				return fmt.Errorf("collecting persistence size: %w", err)
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
			return fmt.Errorf("collecting subnet: %w", err)
		}
		if subnet != "" {
			spec.Exposure.Subnet = subnet
			if !cmd.Flags().Changed("vip") {
				vip, err := tui.Input("Static VIP (optional, auto-assigned from subnet if empty)", "e.g. 10.0.4.210", "")
				if err != nil {
					return fmt.Errorf("collecting VIP address: %w", err)
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
			return fmt.Errorf("collecting coredns replicas: %w", err)
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
func applyClusterMetadata(cmd *cobra.Command, spec *platform.VClusterSpec) error {
	if len(createClusterLabels) > 0 && spec.Integrations.ArgoCD != nil {
		if spec.Integrations.ArgoCD.ClusterLabels == nil {
			spec.Integrations.ArgoCD.ClusterLabels = map[string]string{}
		}
		for _, kv := range createClusterLabels {
			k, v, err := parseKeyValue(kv)
			if err != nil {
				return fmt.Errorf("invalid --cluster-label %q: %w", kv, err)
			}
			spec.Integrations.ArgoCD.ClusterLabels[k] = v
		}
	}
	if len(createClusterAnnotations) > 0 && spec.Integrations.ArgoCD != nil {
		if spec.Integrations.ArgoCD.ClusterAnnotations == nil {
			spec.Integrations.ArgoCD.ClusterAnnotations = map[string]string{}
		}
		for _, kv := range createClusterAnnotations {
			k, v, err := parseKeyValue(kv)
			if err != nil {
				return fmt.Errorf("invalid --cluster-annotation %q: %w", kv, err)
			}
			spec.Integrations.ArgoCD.ClusterAnnotations[k] = v
		}
	}
	return nil
}

// collectWorkloadRepo handles workload repo configuration from flags or interactive prompts.
func collectWorkloadRepo(cmd *cobra.Command, spec *platform.VClusterSpec, interactive bool) error {
	hasWorkloadFlags := createWorkloadRepoURL != "" || createWorkloadRepoBasePath != "" ||
		createWorkloadRepoPath != "" || createWorkloadRepoRevision != ""

	if interactive && !hasWorkloadFlags {
		confirmed, _ := tui.Confirm("Use a custom workload repository? (default: workloads/ in this repo)")
		if confirmed {
			url, err := tui.Input("Workload repo URL", "e.g. https://github.com/myorg/my-workloads", "")
			if err != nil {
				return fmt.Errorf("collecting workload repo URL: %w", err)
			}
			if url != "" {
				createWorkloadRepoURL = url
			}

			basePath, err := tui.Input("Base path in repo (optional)", "e.g. clusters/dev-team-1", "")
			if err != nil {
				return fmt.Errorf("collecting workload repo base path: %w", err)
			}
			if basePath != "" {
				createWorkloadRepoBasePath = basePath
			}

			path, err := tui.Input("Workload path", "directory containing manifests", "workloads")
			if err != nil {
				return fmt.Errorf("collecting workload path: %w", err)
			}
			if path != "" {
				createWorkloadRepoPath = path
			}

			rev, err := tui.Input("Git revision (branch/tag)", "", "main")
			if err != nil {
				return fmt.Errorf("collecting git revision: %w", err)
			}
			if rev != "" {
				createWorkloadRepoRevision = rev
			}

			hasWorkloadFlags = createWorkloadRepoURL != "" || createWorkloadRepoBasePath != "" ||
				createWorkloadRepoPath != "" || createWorkloadRepoRevision != ""
		}
	}

	if hasWorkloadFlags && spec.Integrations.ArgoCD != nil {
		spec.Integrations.ArgoCD.WorkloadRepo = &platform.WorkloadRepoConfig{}
		if createWorkloadRepoURL != "" {
			spec.Integrations.ArgoCD.WorkloadRepo.URL = createWorkloadRepoURL
		}
		if createWorkloadRepoBasePath != "" {
			spec.Integrations.ArgoCD.WorkloadRepo.BasePath = createWorkloadRepoBasePath
		}
		if createWorkloadRepoPath != "" {
			spec.Integrations.ArgoCD.WorkloadRepo.Path = createWorkloadRepoPath
		}
		if createWorkloadRepoRevision != "" {
			spec.Integrations.ArgoCD.WorkloadRepo.Revision = createWorkloadRepoRevision
		}
	}

	return nil
}
