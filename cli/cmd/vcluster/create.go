package vcluster

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jamesatintegratnio/hctl/internal/config"
	"github.com/jamesatintegratnio/hctl/internal/git"
	"github.com/jamesatintegratnio/hctl/internal/platform"
	"github.com/jamesatintegratnio/hctl/internal/tui"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	createPreset   string
	createReplicas int
	createHostname string
	createEnableNFS bool
	createEnvironment string
	createAutoCommit  bool
)

func newCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create [name]",
		Short: "Create a new vCluster",
		Long: `Create a new vCluster on the platform.

In interactive mode (default), a guided wizard walks through configuration.
Use flags for non-interactive/scripted usage.

Examples:
  hctl vcluster create my-dev --preset dev
  hctl vcluster create my-prod --preset prod --replicas 3 --hostname my-prod.cluster.integratn.tech --enable-nfs`,
		Args: cobra.MaximumNArgs(1),
		RunE: runCreate,
	}

	cmd.Flags().StringVar(&createPreset, "preset", "", "vCluster preset (dev, prod)")
	cmd.Flags().IntVar(&createReplicas, "replicas", 0, "number of replicas (overrides preset default)")
	cmd.Flags().StringVar(&createHostname, "hostname", "", "external hostname for the vCluster API")
	cmd.Flags().BoolVar(&createEnableNFS, "enable-nfs", false, "enable NFS egress network policy")
	cmd.Flags().StringVar(&createEnvironment, "environment", "production", "ArgoCD environment")
	cmd.Flags().BoolVar(&createAutoCommit, "auto-commit", false, "automatically commit and push (overrides gitMode)")

	return cmd
}

func runCreate(cmd *cobra.Command, args []string) error {
	cfg := config.Get()
	interactive := cfg.Interactive && !cmd.Flags().Changed("non-interactive")

	var name string
	if len(args) > 0 {
		name = args[0]
	}

	// Interactive wizard
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

	// Preset selection
	preset := createPreset
	if interactive && preset == "" {
		idx, err := tui.Select("Select preset", []string{
			"dev  — 1 replica, 512Mi, SQLite, no persistence",
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

	// Build spec
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

	// Override replicas if specified
	if createReplicas > 0 {
		spec.VCluster.Replicas = createReplicas
	}

	// Hostname
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
		APIPort:  443,
	}

	// NFS
	if interactive && !cmd.Flags().Changed("enable-nfs") {
		confirmed, _ := tui.Confirm("Enable NFS egress?")
		createEnableNFS = confirmed
	}
	spec.NetworkPolicies.EnableNFS = createEnableNFS

	// Environment
	if spec.Integrations.ArgoCD != nil {
		spec.Integrations.ArgoCD.Environment = createEnvironment
	}

	// Prod preset extras
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

	// Build resource
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
	fmt.Printf("\n%s Written to %s\n", tui.SuccessStyle.Render("✓"), relPath)

	// Git handling
	gitMode := cfg.GitMode
	if createAutoCommit {
		gitMode = "auto"
	}

	switch gitMode {
	case "auto":
		repo, err := git.DetectRepo(repoPath)
		if err != nil {
			return fmt.Errorf("git repo: %w", err)
		}
		msg := git.FormatCommitMessage("create vcluster", name, fmt.Sprintf("%s, %d replicas", preset, spec.VCluster.Replicas))
		if err := repo.CommitAndPush([]string{relPath}, msg); err != nil {
			return fmt.Errorf("git commit/push: %w", err)
		}
		fmt.Printf("%s Committed and pushed\n", tui.SuccessStyle.Render("✓"))
	case "prompt":
		if interactive {
			confirmed, _ := tui.Confirm("Commit and push?")
			if confirmed {
				repo, err := git.DetectRepo(repoPath)
				if err != nil {
					return fmt.Errorf("git repo: %w", err)
				}
				msg := git.FormatCommitMessage("create vcluster", name, fmt.Sprintf("%s, %d replicas", preset, spec.VCluster.Replicas))
				if err := repo.CommitAndPush([]string{relPath}, msg); err != nil {
					return fmt.Errorf("git commit/push: %w", err)
				}
				fmt.Printf("%s Committed and pushed\n", tui.SuccessStyle.Render("✓"))
			} else {
				fmt.Println(tui.DimStyle.Render("  Skipped git commit. Run manually: git add && git commit && git push"))
			}
		}
	default:
		fmt.Println(tui.DimStyle.Render("  Generated only. Commit and push when ready."))
	}

	fmt.Printf("\n%s\n", tui.DimStyle.Render("Next: ArgoCD will sync the resource and Kratix will provision the vCluster."))
	fmt.Printf("%s\n", tui.DimStyle.Render("Monitor with: hctl vcluster status "+name))

	return nil
}
