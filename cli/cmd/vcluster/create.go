package vcluster

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jamesatintegratnio/hctl/internal/config"
	"github.com/jamesatintegratnio/hctl/internal/git"
	"github.com/jamesatintegratnio/hctl/internal/kube"
	"github.com/jamesatintegratnio/hctl/internal/platform"
	"github.com/jamesatintegratnio/hctl/internal/tui"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
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

	var name string
	if len(args) > 0 {
		name = args[0]
	}

	// ── Name ──────────────────────────────────────────────────────────
	if interactive && name == "" {
		val, err := tui.Input("vCluster name", "e.g. my-project", "")
		if err != nil {
			return err
		}
		if val == "" {
			return fmt.Errorf("name is required")
		}
		name = val
	}
	if name == "" {
		return fmt.Errorf("name is required")
	}

	// ── Preset ────────────────────────────────────────────────────────
	preset := createPreset
	if interactive && preset == "" {
		idx, err := tui.Select("Select preset", []string{
			"dev  — 1 replica, 768Mi, SQLite, no persistence",
			"prod — 3 replicas, 2Gi, etcd HA, 10Gi persistence",
		})
		if err != nil {
			return err
		}
		if idx < 0 {
			return fmt.Errorf("cancelled")
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

	// ── Build base spec ───────────────────────────────────────────────
	spec := platform.VClusterSpec{
		Name:            name,
		TargetNamespace: name,
		ProjectName:     name,
		Integrations:    platform.DefaultIntegrations(),
		ArgocdApp:       platform.DefaultArgocdApp(),
	}

	if err := platform.ApplyPreset(&spec, preset); err != nil {
		return err
	}

	// ── K8s version ───────────────────────────────────────────────────
	if createK8sVersion != "" {
		spec.VCluster.K8sVersion = createK8sVersion
	}

	// ── Replicas ──────────────────────────────────────────────────────
	if createReplicas > 0 {
		spec.VCluster.Replicas = createReplicas
	}

	// ── Isolation mode ────────────────────────────────────────────────
	if createIsolationMode != "" {
		spec.VCluster.IsolationMode = createIsolationMode
	}

	// ── Hostname ──────────────────────────────────────────────────────
	hostname := createHostname
	if interactive && hostname == "" {
		defaultHost := fmt.Sprintf("%s.%s", name, cfg.Platform.Domain)
		val, err := tui.Input("External hostname", "e.g. "+defaultHost, defaultHost)
		if err != nil {
			return err
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

	// ── Subnet / VIP ─────────────────────────────────────────────────
	if createSubnet != "" {
		spec.Exposure.Subnet = createSubnet
	}
	if createVIP != "" {
		spec.Exposure.VIP = createVIP
	}

	// ── Persistence ──────────────────────────────────────────────────
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

	// ── CoreDNS replicas ─────────────────────────────────────────────
	if createCoreDNSReplicas > 0 {
		spec.VCluster.CoreDNS = &platform.CoreDNSConfig{Replicas: createCoreDNSReplicas}
	}

	// ── NFS ──────────────────────────────────────────────────────────
	if interactive && !cmd.Flags().Changed("enable-nfs") {
		confirmed, _ := tui.Confirm("Enable NFS egress?")
		createEnableNFS = confirmed
	}
	spec.NetworkPolicies.EnableNFS = createEnableNFS

	// ── Extra egress rules ───────────────────────────────────────────
	if len(createExtraEgress) > 0 {
		for _, rule := range createExtraEgress {
			eg, err := parseEgressRule(rule)
			if err != nil {
				return err
			}
			spec.NetworkPolicies.ExtraEgress = append(spec.NetworkPolicies.ExtraEgress, eg)
		}
	}

	// ── Environment ──────────────────────────────────────────────────
	if spec.Integrations.ArgoCD != nil {
		spec.Integrations.ArgoCD.Environment = createEnvironment
	}

	// ── Advanced settings gate ───────────────────────────────────────
	// Only prompt for k8s version, isolation, environment, persistence,
	// subnet/VIP, and CoreDNS if the user opts in. All of these can
	// still be set directly via flags without the gate.
	if interactive {
		advanced, _ := tui.Confirm("Customize advanced settings? (k8s version, isolation, environment, persistence, networking)")
		if advanced {
			// K8s version
			if !cmd.Flags().Changed("k8s-version") {
				idx, err := tui.Select("Kubernetes version", []string{
					"v1.34.3 (default)",
					"1.33",
					"1.32",
				})
				if err != nil {
					return err
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
					return err
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
					return err
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
						return err
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
					return err
				}
				if subnet != "" {
					spec.Exposure.Subnet = subnet
					if !cmd.Flags().Changed("vip") {
						vip, err := tui.Input("Static VIP (optional, auto-assigned from subnet if empty)", "e.g. 10.0.4.210", "")
						if err != nil {
							return err
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
					return err
				}
				if val != "" && val != fmt.Sprintf("%d", currentReplicas) {
					var r int
					if _, err := fmt.Sscanf(val, "%d", &r); err == nil && r > 0 {
						spec.VCluster.CoreDNS = &platform.CoreDNSConfig{Replicas: r}
					}
				}
			}
		}
	}

	// ── Cluster labels / annotations ─────────────────────────────────
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

	// ── Workload repo ────────────────────────────────────────────────
	hasWorkloadFlags := createWorkloadRepoURL != "" || createWorkloadRepoBasePath != "" ||
		createWorkloadRepoPath != "" || createWorkloadRepoRevision != ""

	if interactive && !hasWorkloadFlags {
		confirmed, _ := tui.Confirm("Use a custom workload repository? (default: workloads/ in this repo)")
		if confirmed {
			url, err := tui.Input("Workload repo URL", "e.g. https://github.com/myorg/my-workloads", "")
			if err != nil {
				return err
			}
			if url != "" {
				createWorkloadRepoURL = url
			}

			basePath, err := tui.Input("Base path in repo (optional)", "e.g. clusters/dev-team-1", "")
			if err != nil {
				return err
			}
			if basePath != "" {
				createWorkloadRepoBasePath = basePath
			}

			path, err := tui.Input("Workload path", "directory containing manifests", "workloads")
			if err != nil {
				return err
			}
			if path != "" {
				createWorkloadRepoPath = path
			}

			rev, err := tui.Input("Git revision (branch/tag)", "", "main")
			if err != nil {
				return err
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

	// ── Chart version override ───────────────────────────────────────
	if createChartVersion != "" {
		spec.ArgocdApp.TargetRevision = createChartVersion
	}

	// ── Prod preset extras (helmOverrides for etcd certs) ────────────
	if preset == "prod" {
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

	// ── Build resource ───────────────────────────────────────────────
	resource := platform.NewVClusterResource(spec, cfg.Platform.PlatformNamespace)

	data, err := yaml.Marshal(resource)
	if err != nil {
		return fmt.Errorf("marshaling resource: %w", err)
	}

	// Show preview
	fmt.Println(tui.TitleStyle.Render("Generated VClusterOrchestratorV2"))
	fmt.Println(tui.DimStyle.Render("---"))
	fmt.Println(string(data))

	// Write file
	repoPath := cfg.RepoPath
	if repoPath == "" {
		repo, err := git.DetectRepo("")
		if err != nil {
			return fmt.Errorf("cannot detect repo — run 'hctl init' first or set repoPath in config")
		}
		repoPath = repo.Root
	}

	outPath := filepath.Join(repoPath, "platform", "vclusters", name+".yaml")
	if _, err := os.Stat(outPath); err == nil {
		if interactive {
			confirmed, _ := tui.Confirm(fmt.Sprintf("File %s already exists. Overwrite?", outPath))
			if !confirmed {
				return fmt.Errorf("cancelled")
			}
		} else {
			return fmt.Errorf("file already exists: %s (use --auto-commit with caution)", outPath)
		}
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}
	if err := os.WriteFile(outPath, data, 0o644); err != nil {
		return fmt.Errorf("writing file: %w", err)
	}

	relPath, _ := filepath.Rel(repoPath, outPath)
	fmt.Printf("\n%s Written to %s\n", tui.SuccessStyle.Render(tui.IconCheck), relPath)

	// Git handling
	gitMode := cfg.GitMode
	if createAutoCommit {
		gitMode = "auto"
	}

	gitResult, err := git.HandleGitWorkflow(git.WorkflowOpts{
		RepoPath:    repoPath,
		Paths:       []string{relPath},
		Action:      "create vcluster",
		Resource:    name,
		Details:     fmt.Sprintf("%s, %d replicas", preset, spec.VCluster.Replicas),
		GitMode:     gitMode,
		Interactive: interactive,
	})
	if err != nil {
		return err
	}

	fmt.Printf("\n%s\n", tui.DimStyle.Render("Next: ArgoCD will sync the resource and Kratix will provision the vCluster."))
	fmt.Printf("%s\n", tui.DimStyle.Render("Monitor with: hctl vcluster status "+name))

	// ── Wait for provisioning ────────────────────────────────────────
	committed := gitResult == git.GitCommitted
	if createWait && committed {
		if err := watchProvisioning(cfg, name, hostname, spec); err != nil {
			// Non-fatal — the resource was already committed
			fmt.Printf("\n%s %s\n", tui.WarningStyle.Render(tui.IconWarn), err.Error())
			fmt.Printf("%s\n", tui.DimStyle.Render("The request was committed. Check status later: hctl vcluster status "+name))
		}
	}

	return nil
}

// watchProvisioning runs the animated provisioning wait sequence.
func watchProvisioning(cfg *config.Config, name, hostname string, spec platform.VClusterSpec) error {
	client, err := kube.NewClient(cfg.KubeContext)
	if err != nil {
		return fmt.Errorf("connecting to cluster: %w", err)
	}

	ns := cfg.Platform.PlatformNamespace
	timeout := time.Duration(createTimeout) * time.Second
	poll := 3 * time.Second

	steps := []tui.Step{
		{
			Title: "Request accepted",
			Run: func() (string, error) {
				ctx, cancel := context.WithTimeout(context.Background(), timeout)
				defer cancel()
				return platform.WaitForRequest(ctx, client, ns, name, poll)
			},
		},
		{
			Title: "Pipeline running",
			Run: func() (string, error) {
				ctx, cancel := context.WithTimeout(context.Background(), timeout)
				defer cancel()
				return platform.WaitForPipeline(ctx, client, ns, name, poll)
			},
		},
		{
			Title: "ArgoCD syncing",
			Run: func() (string, error) {
				ctx, cancel := context.WithTimeout(context.Background(), timeout)
				defer cancel()
				return platform.WaitForArgoSync(ctx, client, name, poll)
			},
		},
		{
			Title: "Cluster ready",
			Run: func() (string, error) {
				ctx, cancel := context.WithTimeout(context.Background(), timeout)
				defer cancel()
				return platform.WaitForClusterReady(ctx, client, name, poll)
			},
		},
	}

	fmt.Println()
	_, err = tui.RunSteps("Provisioning "+name, steps)
	if err != nil {
		return err
	}

	// Collect and display summary
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := platform.CollectProvisionResult(ctx, client, ns, name)
	if err != nil {
		// Non-fatal — provisioning succeeded but summary collection failed
		fmt.Printf("\n  %s %s is ready!\n", tui.SuccessStyle.Render(tui.IconCheck), name)
		fmt.Printf("  %s\n", tui.DimStyle.Render("Run 'hctl vcluster status "+name+"' for details"))
		return nil
	}

	fmt.Print(platform.FormatProvisionSummary(result, hostname))
	return nil
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
