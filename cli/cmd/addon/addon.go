package addon

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

// NewCmd returns the addon command group.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "addon",
		Short: "Manage platform addons",
		Long: `List, enable, disable, and check status of platform addons.

Addons are deployed via ArgoCD ApplicationSets with layered value files
(environment → cluster-role → cluster-specific).

Addon values are resolved in three layers (last wins):
  1. environments/<env>/addons/<addon>/values.yaml
  2. cluster-roles/<role>/addons/<addon>/values.yaml
  3. clusters/<cluster>/addons/<addon>/values.yaml`,
	}

	cmd.AddCommand(newAddonListCmd())
	cmd.AddCommand(newAddonStatusCmd())
	cmd.AddCommand(newAddonEnableCmd())
	cmd.AddCommand(newAddonDisableCmd())

	return cmd
}

func newAddonListCmd() *cobra.Command {
	var env string
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List available addons",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Get()
			repoPath := cfg.RepoPath
			if repoPath == "" {
				return fmt.Errorf("repo path not set — run 'hctl init'")
			}

			if env == "" {
				env = "production"
			}

			entries, err := readAddonsYAML(filepath.Join(repoPath, "addons", "environments", env, "addons", "addons.yaml"))
			if err != nil {
				return err
			}

			if len(entries) == 0 {
				fmt.Println(tui.DimStyle.Render("No addons defined"))
				return nil
			}

			// Try to get ArgoCD app status
			var appStatus map[string]string
			client, err := kube.NewClient(cfg.KubeContext)
			if err == nil {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				argoApps, err := client.ListArgoApps(ctx, "argocd")
				if err == nil {
					appStatus = make(map[string]string)
					for _, app := range argoApps {
						syncStatus, _, _ := platform.UnstructuredNestedString(app.Object, "status", "sync", "status")
						healthStatus, _, _ := platform.UnstructuredNestedString(app.Object, "status", "health", "status")
						appStatus[app.GetName()] = fmt.Sprintf("%s/%s", syncStatus, healthStatus)
					}
				}
			}

			var rows [][]string
			for name, entry := range entries {
				enabled := "yes"
				if e, ok := entry["enabled"]; ok {
					if b, ok := e.(bool); ok && !b {
						enabled = tui.DimStyle.Render("no")
					}
				}

				status := tui.DimStyle.Render("—")
				if appStatus != nil {
					if s, ok := appStatus[name]; ok {
						if s == "Synced/Healthy" {
							status = tui.SuccessStyle.Render(s)
						} else {
							status = tui.WarningStyle.Render(s)
						}
					}
				}
				rows = append(rows, []string{name, enabled, env, status})
			}

			_, err = tui.InteractiveTable(tui.InteractiveTableConfig{
				Title:   "Addons (" + env + ")",
				Headers: []string{"ADDON", "ENABLED", "ENVIRONMENT", "STATUS"},
				Rows:    rows,
				OnSelect: func(row []string, index int) string {
					if len(row) == 0 {
						return ""
					}
					addonName := row[0]
					var sb strings.Builder
					sb.WriteString(tui.HeaderStyle.Render("Addon: "+addonName) + "\n\n")

					// Resolve the values folder name the same way the ApplicationSet does:
					// valuesFolderName → chartName → addonName (normalized)
					folderName := addonName
					if entry, ok := entries[addonName]; ok {
						if vfn, ok := entry["valuesFolderName"].(string); ok && vfn != "" {
							folderName = vfn
						} else if cn, ok := entry["chartName"].(string); ok && cn != "" {
							folderName = cn
						}
					}

					// Show value file layers matching the ApplicationSet valueFiles order:
					//   environments/<env>/addons/<folder>/values.yaml
					//   cluster-roles/<role>/addons/<folder>/values.yaml  (all roles)
					//   clusters/<cluster>/addons/<folder>/values.yaml    (all clusters)
					sb.WriteString("  Value layers:\n")

					type valueLayer struct {
						label string
						path  string
					}
					var layers []valueLayer

					// 1. Environment layer
					layers = append(layers, valueLayer{
						label: fmt.Sprintf("environment/%s", env),
						path:  filepath.Join(repoPath, "addons", "environments", env, "addons", folderName, "values.yaml"),
					})

					// 2. Cluster-role layers (discover all roles)
					rolesDir := filepath.Join(repoPath, "addons", "cluster-roles")
					if roleDirs, err := os.ReadDir(rolesDir); err == nil {
						for _, d := range roleDirs {
							if d.IsDir() {
								layers = append(layers, valueLayer{
									label: fmt.Sprintf("cluster-role/%s", d.Name()),
									path:  filepath.Join(rolesDir, d.Name(), "addons", folderName, "values.yaml"),
								})
							}
						}
					}

					// 3. Cluster layers (discover all clusters)
					clustersDir := filepath.Join(repoPath, "addons", "clusters")
					if clusterDirs, err := os.ReadDir(clustersDir); err == nil {
						for _, d := range clusterDirs {
							if d.IsDir() {
								layers = append(layers, valueLayer{
									label: fmt.Sprintf("cluster/%s", d.Name()),
									path:  filepath.Join(clustersDir, d.Name(), "addons", folderName, "values.yaml"),
								})
							}
						}
					}

					for _, l := range layers {
						if _, err := os.Stat(l.path); err == nil {
							sb.WriteString(fmt.Sprintf("    ✓ %s: %s\n", l.label, l.path))
						} else {
							sb.WriteString(fmt.Sprintf("    ○ %s: %s\n", l.label, tui.DimStyle.Render("not found")))
						}
					}

					// Show ArgoCD status if available
					if appStatus != nil {
						if s, ok := appStatus[addonName]; ok {
							sb.WriteString(fmt.Sprintf("\n  ArgoCD: %s\n", s))
						}
					}
					return sb.String()
				},
			})
			return err
		},
	}

	cmd.Flags().StringVar(&env, "environment", "", "environment to list addons for (default: production)")
	return cmd
}

func newAddonStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status [addon]",
		Short: "Check addon health and sync status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			addonName := args[0]
			cfg := config.Get()

			client, err := kube.NewClient(cfg.KubeContext)
			if err != nil {
				return fmt.Errorf("connecting to cluster: %w", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			app, err := client.GetArgoApp(ctx, "argocd", addonName)
			if err != nil {
				return fmt.Errorf("addon %q not found as ArgoCD application: %w", addonName, err)
			}

			syncStatus, _, _ := platform.UnstructuredNestedString(app.Object, "status", "sync", "status")
			healthStatus, _, _ := platform.UnstructuredNestedString(app.Object, "status", "health", "status")
			revision, _, _ := platform.UnstructuredNestedString(app.Object, "status", "sync", "revision")

			fmt.Printf("\n%s\n\n", tui.TitleStyle.Render(addonName))
			fmt.Printf("  Sync:     %s\n", syncStatus)
			fmt.Printf("  Health:   %s\n", healthStatus)
			fmt.Printf("  Revision: %s\n", revision)
			fmt.Println()

			return nil
		},
	}
}

func newAddonEnableCmd() *cobra.Command {
	var (
		env        string
		cluster    string
		clusterRole string
		namespace  string
		chartRepo  string
		chartName  string
		version    string
		layer      string
	)
	cmd := &cobra.Command{
		Use:   "enable [addon]",
		Short: "Enable an addon",
		Long: `Enable an addon by adding it to addons.yaml and scaffolding value directories.

Addons can be enabled at different layers:
  --layer environment   (default) — affects all clusters in the environment
  --layer cluster-role  — affects all clusters with a specific role
  --layer cluster       — affects a single cluster

If the addon already exists in addons.yaml, its 'enabled' field is set to true.
If it doesn't exist, a new entry is created with Stakater Application chart defaults.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			addonName := args[0]
			cfg := config.Get()
			if cfg.RepoPath == "" {
				return fmt.Errorf("repo path not set — run 'hctl init'")
			}

			if env == "" {
				env = "production"
			}
			if namespace == "" {
				namespace = addonName
			}
			if layer == "" {
				layer = "environment"
			}

			// Determine addons.yaml path based on layer
			addonsPath, valuesDir, err := resolveLayerPaths(cfg.RepoPath, layer, env, clusterRole, cluster, addonName)
			if err != nil {
				return err
			}

			// Read or create addons.yaml
			entries, err := readAddonsYAML(addonsPath)
			if err != nil && !os.IsNotExist(err) {
				return err
			}
			if entries == nil {
				entries = make(map[string]map[string]interface{})
			}

			var changedPaths []string

			if existing, ok := entries[addonName]; ok {
				// Addon exists — set enabled: true
				existing["enabled"] = true
				entries[addonName] = existing
				fmt.Printf("%s Enabled %s in %s\n", tui.SuccessStyle.Render(tui.IconCheck), addonName, addonsPath)
			} else {
				// Create new addon entry
				entry := map[string]interface{}{
					"enabled":         true,
					"namespace":       namespace,
					"chartRepository": chartRepo,
					"chartName":       chartName,
					"defaultVersion":  version,
				}

				// Clean up empty defaults
				if chartRepo == "" {
					entry["chartRepository"] = "https://stakater.github.io/stakater-charts"
				}
				if chartName == "" {
					entry["chartName"] = "application"
				}
				if version == "" {
					entry["defaultVersion"] = "6.14.0"
				}

				entries[addonName] = entry
				fmt.Printf("%s Added %s to %s\n", tui.SuccessStyle.Render(tui.IconCheck), addonName, filepath.Base(filepath.Dir(addonsPath)))
			}

			// Write addons.yaml
			if err := writeAddonsYAML(addonsPath, entries); err != nil {
				return err
			}
			changedPaths = append(changedPaths, addonsPath)

			// Scaffold values directory
			if err := os.MkdirAll(valuesDir, 0o755); err != nil {
				return fmt.Errorf("creating values directory: %w", err)
			}

			valuesFile := filepath.Join(valuesDir, "values.yaml")
			if _, err := os.Stat(valuesFile); os.IsNotExist(err) {
				scaffold := fmt.Sprintf("# %s values\n# Layer: %s\n# See: https://github.com/stakater/application\n", addonName, layer)
				if err := os.WriteFile(valuesFile, []byte(scaffold), 0o644); err != nil {
					return fmt.Errorf("writing values scaffold: %w", err)
				}
				changedPaths = append(changedPaths, valuesFile)
				fmt.Printf("%s Scaffolded %s\n", tui.SuccessStyle.Render(tui.IconCheck), valuesFile)
			}

			// Git operations
			repo, err := git.DetectRepo(cfg.RepoPath)
			if err != nil {
				return nil
			}

			// Convert to relative paths
			var relPaths []string
			for _, p := range changedPaths {
				rp, err := repo.RelPath(p)
				if err == nil {
					relPaths = append(relPaths, rp)
				}
			}

			if _, err := git.HandleGitWorkflow(git.WorkflowOpts{
				RepoPath:    cfg.RepoPath,
				Paths:       relPaths,
				Action:      "enable addon",
				Resource:    addonName,
				Details:     layer + "/" + env,
				GitMode:     cfg.GitMode,
				Interactive: cfg.Interactive,
			}); err != nil {
				return err
			}

			fmt.Printf("\n%s\n", tui.DimStyle.Render("ArgoCD will sync the addon on next reconciliation."))
			return nil
		},
	}

	cmd.Flags().StringVar(&env, "environment", "", "target environment (default: production)")
	cmd.Flags().StringVar(&cluster, "cluster", "", "target cluster (for --layer cluster)")
	cmd.Flags().StringVar(&clusterRole, "cluster-role", "", "cluster role (for --layer cluster-role)")
	cmd.Flags().StringVar(&namespace, "namespace", "", "namespace for the addon (default: addon name)")
	cmd.Flags().StringVar(&chartRepo, "chart-repo", "", "Helm chart repository URL")
	cmd.Flags().StringVar(&chartName, "chart-name", "", "Helm chart name")
	cmd.Flags().StringVar(&version, "version", "", "chart version")
	cmd.Flags().StringVar(&layer, "layer", "environment", "config layer: environment, cluster-role, or cluster")
	return cmd
}

func newAddonDisableCmd() *cobra.Command {
	var (
		env        string
		cluster    string
		clusterRole string
		layer      string
		remove     bool
	)
	cmd := &cobra.Command{
		Use:   "disable [addon]",
		Short: "Disable an addon",
		Long: `Disable an addon by setting enabled: false in addons.yaml.

With --remove, the addon entry and its values directory are deleted entirely.
Without --remove, the entry remains but is marked disabled.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			addonName := args[0]
			cfg := config.Get()
			if cfg.RepoPath == "" {
				return fmt.Errorf("repo path not set — run 'hctl init'")
			}

			if env == "" {
				env = "production"
			}
			if layer == "" {
				layer = "environment"
			}

			addonsPath, valuesDir, err := resolveLayerPaths(cfg.RepoPath, layer, env, clusterRole, cluster, addonName)
			if err != nil {
				return err
			}

			entries, err := readAddonsYAML(addonsPath)
			if err != nil {
				return fmt.Errorf("reading addons.yaml: %w", err)
			}

			if _, ok := entries[addonName]; !ok {
				return fmt.Errorf("addon %q not found in %s", addonName, addonsPath)
			}

			if cfg.Interactive {
				action := "disable"
				if remove {
					action = "remove"
				}
				label := strings.ToUpper(action[:1]) + action[1:]
				ok, _ := tui.Confirm(fmt.Sprintf("%s addon %q from %s?", label, addonName, filepath.Base(filepath.Dir(addonsPath))))
				if !ok {
					fmt.Println(tui.DimStyle.Render("Cancelled"))
					return nil
				}
			}

			var changedPaths []string

			if remove {
				delete(entries, addonName)
				fmt.Printf("%s Removed %s from addons.yaml\n", tui.SuccessStyle.Render(tui.IconCheck), addonName)

				// Remove values directory
				if _, err := os.Stat(valuesDir); err == nil {
					if err := os.RemoveAll(valuesDir); err != nil {
						fmt.Printf("%s Could not remove values directory: %v\n", tui.WarningStyle.Render(tui.IconWarn), err)
					} else {
						fmt.Printf("%s Removed %s\n", tui.SuccessStyle.Render(tui.IconCheck), valuesDir)
						changedPaths = append(changedPaths, valuesDir)
					}
				}
			} else {
				entries[addonName]["enabled"] = false
				fmt.Printf("%s Disabled %s\n", tui.SuccessStyle.Render(tui.IconCheck), addonName)
			}

			if err := writeAddonsYAML(addonsPath, entries); err != nil {
				return err
			}
			changedPaths = append(changedPaths, addonsPath)

			// Git operations
			repo, err := git.DetectRepo(cfg.RepoPath)
			if err != nil {
				return nil
			}

			var relPaths []string
			for _, p := range changedPaths {
				rp, err := repo.RelPath(p)
				if err == nil {
					relPaths = append(relPaths, rp)
				}
			}

			action := "disable addon"
			if remove {
				action = "remove addon"
			}

			if _, err := git.HandleGitWorkflow(git.WorkflowOpts{
				RepoPath:    cfg.RepoPath,
				Paths:       relPaths,
				Action:      action,
				Resource:    addonName,
				Details:     layer + "/" + env,
				GitMode:     cfg.GitMode,
				Interactive: cfg.Interactive,
			}); err != nil {
				return err
			}

			fmt.Printf("\n%s\n", tui.DimStyle.Render("ArgoCD will reflect the change on next sync."))
			return nil
		},
	}

	cmd.Flags().StringVar(&env, "environment", "", "target environment (default: production)")
	cmd.Flags().StringVar(&cluster, "cluster", "", "target cluster (for --layer cluster)")
	cmd.Flags().StringVar(&clusterRole, "cluster-role", "", "cluster role (for --layer cluster-role)")
	cmd.Flags().StringVar(&layer, "layer", "environment", "config layer: environment, cluster-role, or cluster")
	cmd.Flags().BoolVar(&remove, "remove", false, "completely remove the addon entry and values")
	return cmd
}

// --- Helpers ---

// resolveLayerPaths returns the addons.yaml path and values directory for a given layer.
func resolveLayerPaths(repoPath, layer, env, clusterRole, cluster, addonName string) (string, string, error) {
	var base string
	switch layer {
	case "environment":
		base = filepath.Join(repoPath, "addons", "environments", env, "addons")
	case "cluster-role":
		if clusterRole == "" {
			return "", "", fmt.Errorf("--cluster-role is required for layer 'cluster-role'")
		}
		base = filepath.Join(repoPath, "addons", "cluster-roles", clusterRole, "addons")
	case "cluster":
		if cluster == "" {
			return "", "", fmt.Errorf("--cluster is required for layer 'cluster'")
		}
		base = filepath.Join(repoPath, "addons", "clusters", cluster, "addons")
	default:
		return "", "", fmt.Errorf("invalid layer %q (must be environment, cluster-role, or cluster)", layer)
	}

	addonsFile := filepath.Join(base, "addons.yaml")
	valuesDir := filepath.Join(base, addonName)
	return addonsFile, valuesDir, nil
}

// readAddonsYAML reads and parses an addons.yaml file into a map of addon entries.
func readAddonsYAML(path string) (map[string]map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing addons.yaml: %w", err)
	}

	entries := make(map[string]map[string]interface{})
	for name, val := range raw {
		if m, ok := val.(map[string]interface{}); ok {
			entries[name] = m
		}
	}
	return entries, nil
}

// writeAddonsYAML writes addon entries back to addons.yaml.
func writeAddonsYAML(path string, entries map[string]map[string]interface{}) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating addons directory: %w", err)
	}

	data, err := yaml.Marshal(entries)
	if err != nil {
		return fmt.Errorf("marshaling addons.yaml: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing addons.yaml: %w", err)
	}
	return nil
}
