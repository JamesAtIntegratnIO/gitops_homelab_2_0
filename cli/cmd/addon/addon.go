package addon

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jamesatintegratnio/hctl/internal/config"
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
(environment → cluster-role → cluster-specific).`,
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
		Use:   "list",
		Short: "List available addons",
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

			// Read addons.yaml
			addonsFile := filepath.Join(repoPath, "addons", "environments", env, "addons", "addons.yaml")
			data, err := os.ReadFile(addonsFile)
			if err != nil {
				return fmt.Errorf("reading addons.yaml: %w", err)
			}

			var addonsConfig map[string]interface{}
			if err := yaml.Unmarshal(data, &addonsConfig); err != nil {
				return fmt.Errorf("parsing addons.yaml: %w", err)
			}

			// Extract addon entries
			addons, ok := addonsConfig["addons"]
			if !ok {
				fmt.Println(tui.DimStyle.Render("No addons defined"))
				return nil
			}

			addonMap, ok := addons.(map[string]interface{})
			if !ok {
				return fmt.Errorf("unexpected addons format")
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
			for name := range addonMap {
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
				rows = append(rows, []string{name, env, status})
			}

			fmt.Println(tui.Table([]string{"ADDON", "ENVIRONMENT", "STATUS"}, rows))
			return nil
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
	return &cobra.Command{
		Use:   "enable [addon]",
		Short: "Enable an addon",
		Long:  "Add an addon entry to addons.yaml and scaffold value directories.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println(tui.WarningStyle.Render("⚠  addon enable is not yet implemented"))
			fmt.Println(tui.DimStyle.Render("  Will add addon to addons.yaml and scaffold value file directories"))
			return nil
		},
	}
}

func newAddonDisableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "disable [addon]",
		Short: "Disable an addon",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println(tui.WarningStyle.Render("⚠  addon disable is not yet implemented"))
			return nil
		},
	}
}
