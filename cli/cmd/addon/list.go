package addon

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jamesatintegratnio/hctl/internal/config"
	"github.com/jamesatintegratnio/hctl/internal/kube"
	"github.com/jamesatintegratnio/hctl/internal/platform"
	"github.com/jamesatintegratnio/hctl/internal/tui"
	"github.com/spf13/cobra"
)

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
				return fmt.Errorf("reading addons config: %w", err)
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
