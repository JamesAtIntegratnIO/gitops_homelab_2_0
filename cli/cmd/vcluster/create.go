package vcluster

import (
	"fmt"

	"github.com/jamesatintegratnio/hctl/internal/config"
	"github.com/jamesatintegratnio/hctl/internal/tui"
	"github.com/spf13/cobra"
)

var (
	// Core
	createPreset      string
	createReplicas    int
	createHostname    string
	createEnvironment string
	createAutoCommit  bool

	// vCluster settings
	createK8sVersion    string
	createIsolationMode string

	// Exposure
	createSubnet  string
	createVIP     string
	createAPIPort int

	// Persistence
	createPersistence     bool
	createPersistenceSize string
	createStorageClass    string

	// Networking
	createEnableNFS    bool
	createExtraEgress  []string // "name:cidr:port[:protocol]"
	createCoreDNSReplicas int

	// Workload repo
	createWorkloadRepoURL      string
	createWorkloadRepoBasePath string
	createWorkloadRepoPath     string
	createWorkloadRepoRevision string

	// ArgoCD cluster
	createClusterLabels      []string // "key=value"
	createClusterAnnotations []string // "key=value"

	// ArgoCD app overrides
	createChartVersion string

	// Provisioning wait
	createWait    bool
	createTimeout int // seconds
)

func newCreateCmd() *cobra.Command {
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
		RunE: runCreate,
	}

	// Core flags
	cmd.Flags().StringVar(&createPreset, "preset", "", "vCluster preset (dev, prod)")
	cmd.Flags().IntVar(&createReplicas, "replicas", 0, "number of replicas (overrides preset default)")
	cmd.Flags().StringVar(&createHostname, "hostname", "", "external hostname for the vCluster API")
	cmd.Flags().StringVar(&createEnvironment, "environment", "production", "ArgoCD environment label")
	cmd.Flags().BoolVar(&createAutoCommit, "auto-commit", false, "automatically commit and push (overrides gitMode)")

	// vCluster settings
	cmd.Flags().StringVar(&createK8sVersion, "k8s-version", "", "Kubernetes version (v1.34.3, 1.34, 1.33, 1.32)")
	cmd.Flags().StringVar(&createIsolationMode, "isolation", "", "workload isolation mode (standard, strict)")

	// Exposure
	cmd.Flags().StringVar(&createSubnet, "subnet", "", "CIDR subnet for VIP allocation (e.g. 10.0.4.0/24)")
	cmd.Flags().StringVar(&createVIP, "vip", "", "static VIP for the vCluster API (e.g. 10.0.4.210)")
	cmd.Flags().IntVar(&createAPIPort, "api-port", 443, "API port exposed by the vCluster service")

	// Persistence
	cmd.Flags().BoolVar(&createPersistence, "persistence", false, "enable control plane persistence (prod preset enables by default)")
	cmd.Flags().StringVar(&createPersistenceSize, "persistence-size", "", "persistence volume size (e.g. 10Gi)")
	cmd.Flags().StringVar(&createStorageClass, "storage-class", "", "storage class for persistence volume")

	// Networking
	cmd.Flags().BoolVar(&createEnableNFS, "enable-nfs", false, "enable NFS egress network policy")
	cmd.Flags().StringSliceVar(&createExtraEgress, "extra-egress", nil, "extra egress rule as name:cidr:port[:protocol] (repeatable)")
	cmd.Flags().IntVar(&createCoreDNSReplicas, "coredns-replicas", 0, "CoreDNS replica count (overrides preset default)")

	// Workload repo
	cmd.Flags().StringVar(&createWorkloadRepoURL, "workload-repo-url", "", "Git URL for workload definitions (default: same repo)")
	cmd.Flags().StringVar(&createWorkloadRepoBasePath, "workload-repo-base-path", "", "base path prefix in workload repo")
	cmd.Flags().StringVar(&createWorkloadRepoPath, "workload-repo-path", "", "path within repo to workload manifests (default: workloads)")
	cmd.Flags().StringVar(&createWorkloadRepoRevision, "workload-repo-revision", "", "Git branch/tag for workload repo (default: main)")

	// ArgoCD cluster metadata
	cmd.Flags().StringSliceVar(&createClusterLabels, "cluster-label", nil, "additional ArgoCD cluster label as key=value (repeatable)")
	cmd.Flags().StringSliceVar(&createClusterAnnotations, "cluster-annotation", nil, "additional ArgoCD cluster annotation as key=value (repeatable)")

	// ArgoCD app overrides
	cmd.Flags().StringVar(&createChartVersion, "chart-version", "", "vCluster Helm chart version (default: platform default)")

	// Provisioning wait
	cmd.Flags().BoolVar(&createWait, "wait", true, "wait for provisioning to complete after commit")
	cmd.Flags().IntVar(&createTimeout, "timeout", 300, "timeout in seconds when using --wait (default: 300)")

	return cmd
}

func runCreate(cmd *cobra.Command, args []string) error {
	cfg := config.Get()
	interactive := cfg.Interactive && !cmd.Flags().Changed("non-interactive")

	// Phase 1: Collect name and preset
	name, preset, err := collectNameAndPreset(cmd, args, interactive)
	if err != nil {
		return err
	}

	// Phase 2: Build and configure spec
	spec, err := buildVClusterSpec(cmd, cfg, name, preset, interactive)
	if err != nil {
		return err
	}

	// Phase 3: Marshal, write, and commit
	committed, err := writeAndCommitVCluster(cfg, name, preset, spec, interactive)
	if err != nil {
		return err
	}

	// Phase 4: Optionally watch provisioning
	hostname := spec.Exposure.Hostname
	if createWait && committed {
		if err := watchProvisioning(cfg, name, hostname, spec); err != nil {
			// Non-fatal — the resource was already committed
			fmt.Printf("\n%s %s\n", tui.WarningStyle.Render(tui.IconWarn), err.Error())
			fmt.Printf("%s\n", tui.DimStyle.Render("The request was committed. Check status later: hctl vcluster status "+name))
		}
	}

	return nil
}

