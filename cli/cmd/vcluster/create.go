package vcluster

import (
	"fmt"

	"github.com/jamesatintegratnio/hctl/internal/config"
	"github.com/jamesatintegratnio/hctl/internal/tui"
	"github.com/spf13/cobra"
)

// CreateOptions holds all flag values for the vcluster create command.
type CreateOptions struct {
	// Core
	Preset      string
	Replicas    int
	Hostname    string
	Environment string
	AutoCommit  bool

	// vCluster settings
	K8sVersion    string
	IsolationMode string

	// Exposure
	Subnet  string
	VIP     string
	APIPort int

	// Persistence
	Persistence     bool
	PersistenceSize string
	StorageClass    string

	// Networking
	EnableNFS       bool
	ExtraEgress     []string // "name:cidr:port[:protocol]"
	CoreDNSReplicas int

	// Workload repo
	WorkloadRepoURL      string
	WorkloadRepoBasePath string
	WorkloadRepoPath     string
	WorkloadRepoRevision string

	// ArgoCD cluster
	ClusterLabels      []string // "key=value"
	ClusterAnnotations []string // "key=value"

	// ArgoCD app overrides
	ChartVersion string

	// Provisioning wait
	Wait    bool
	Timeout int // seconds
}

func newCreateCmd() *cobra.Command {
	opts := &CreateOptions{}

	cmd := &cobra.Command{
		Use:   "create [name]",
		Short: "Create a new vCluster",
		Long: `Create a new vCluster on the platform.

In interactive mode (default), a guided wizard walks through configuration.
Use flags for non-interactive/scripted usage.

Examples:
  # Quick dev cluster
  hctl vcluster create my-dev --preset dev --auto-commit

  # Production HA cluster with custom hostname
  hctl vcluster create my-prod --preset prod --replicas 3 \
    --hostname my-prod.cluster.integratn.tech --enable-nfs --auto-commit

  # Workloads in a separate repo
  hctl vcluster create team-api --preset dev \
    --workload-repo-url https://github.com/myorg/team-api-workloads \
    --workload-repo-path deploy/k8s --workload-repo-revision main

  # Custom egress rules for database access
  hctl vcluster create data-team --preset dev \
    --extra-egress postgres:10.0.1.50/32:5432 \
    --extra-egress redis:10.0.1.60/32:6379:TCP

  # Interactive wizard (walks through all options)
  hctl vcluster create`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCreate(cmd, args, opts)
		},
	}

	// Core flags
	cmd.Flags().StringVar(&opts.Preset, "preset", "", "vCluster preset (dev, prod)")
	cmd.Flags().IntVar(&opts.Replicas, "replicas", 0, "number of replicas (overrides preset default)")
	cmd.Flags().StringVar(&opts.Hostname, "hostname", "", "external hostname for the vCluster API")
	cmd.Flags().StringVar(&opts.Environment, "environment", "production", "ArgoCD environment label")
	cmd.Flags().BoolVar(&opts.AutoCommit, "auto-commit", false, "automatically commit and push (overrides gitMode)")

	// vCluster settings
	cmd.Flags().StringVar(&opts.K8sVersion, "k8s-version", "", "Kubernetes version (v1.34.3, 1.34, 1.33, 1.32)")
	cmd.Flags().StringVar(&opts.IsolationMode, "isolation", "", "workload isolation mode (standard, strict)")

	// Exposure
	cmd.Flags().StringVar(&opts.Subnet, "subnet", "", "CIDR subnet for VIP allocation (e.g. 10.0.4.0/24)")
	cmd.Flags().StringVar(&opts.VIP, "vip", "", "static VIP for the vCluster API (e.g. 10.0.4.210)")
	cmd.Flags().IntVar(&opts.APIPort, "api-port", 443, "API port exposed by the vCluster service")

	// Persistence
	cmd.Flags().BoolVar(&opts.Persistence, "persistence", false, "enable control plane persistence (prod preset enables by default)")
	cmd.Flags().StringVar(&opts.PersistenceSize, "persistence-size", "", "persistence volume size (e.g. 10Gi)")
	cmd.Flags().StringVar(&opts.StorageClass, "storage-class", "", "storage class for persistence volume")

	// Networking
	cmd.Flags().BoolVar(&opts.EnableNFS, "enable-nfs", false, "enable NFS egress network policy")
	cmd.Flags().StringSliceVar(&opts.ExtraEgress, "extra-egress", nil, "extra egress rule as name:cidr:port[:protocol] (repeatable)")
	cmd.Flags().IntVar(&opts.CoreDNSReplicas, "coredns-replicas", 0, "CoreDNS replica count (overrides preset default)")

	// Workload repo
	cmd.Flags().StringVar(&opts.WorkloadRepoURL, "workload-repo-url", "", "Git URL for workload definitions (default: same repo)")
	cmd.Flags().StringVar(&opts.WorkloadRepoBasePath, "workload-repo-base-path", "", "base path prefix in workload repo")
	cmd.Flags().StringVar(&opts.WorkloadRepoPath, "workload-repo-path", "", "path within repo to workload manifests (default: workloads)")
	cmd.Flags().StringVar(&opts.WorkloadRepoRevision, "workload-repo-revision", "", "Git branch/tag for workload repo (default: main)")

	// ArgoCD cluster metadata
	cmd.Flags().StringSliceVar(&opts.ClusterLabels, "cluster-label", nil, "additional ArgoCD cluster label as key=value (repeatable)")
	cmd.Flags().StringSliceVar(&opts.ClusterAnnotations, "cluster-annotation", nil, "additional ArgoCD cluster annotation as key=value (repeatable)")

	// ArgoCD app overrides
	cmd.Flags().StringVar(&opts.ChartVersion, "chart-version", "", "vCluster Helm chart version (default: platform default)")

	// Provisioning wait
	cmd.Flags().BoolVar(&opts.Wait, "wait", true, "wait for provisioning to complete after commit")
	cmd.Flags().IntVar(&opts.Timeout, "timeout", 300, "timeout in seconds when using --wait (default: 300)")

	return cmd
}

func runCreate(cmd *cobra.Command, args []string, opts *CreateOptions) error {
	cfg := config.Get()
	interactive := cfg.Interactive && !cmd.Flags().Changed("non-interactive")

	// Phase 1: Collect name and preset
	name, preset, err := collectNameAndPreset(cmd, args, opts, interactive)
	if err != nil {
		return fmt.Errorf("collecting parameters: %w", err)
	}

	// Phase 2: Build and configure spec
	spec, err := buildVClusterSpec(cmd, cfg, opts, name, preset, interactive)
	if err != nil {
		return fmt.Errorf("building vcluster spec: %w", err)
	}

	// Phase 3: Marshal, write, and commit
	committed, err := writeAndCommitVCluster(cfg, opts, name, preset, spec, interactive)
	if err != nil {
		return fmt.Errorf("writing vcluster: %w", err)
	}

	// Phase 4: Optionally watch provisioning
	hostname := spec.Exposure.Hostname
	if opts.Wait && committed {
		if err := watchProvisioning(cfg, opts, name, hostname, spec); err != nil {
			// Non-fatal — the resource was already committed
			fmt.Printf("\n%s %s\n", tui.WarningStyle.Render(tui.IconWarn), err.Error())
			fmt.Printf("%s\n", tui.DimStyle.Render("The request was committed. Check status later: hctl vcluster status "+name))
		}
	}

	return nil
}

