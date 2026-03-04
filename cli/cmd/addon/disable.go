package addon

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jamesatintegratnio/hctl/internal/config"
	hcerrors "github.com/jamesatintegratnio/hctl/internal/errors"
	"github.com/jamesatintegratnio/hctl/internal/git"
	"github.com/jamesatintegratnio/hctl/internal/tui"
	"github.com/spf13/cobra"
)

func newAddonDisableCmd() *cobra.Command {
	var (
		env         string
		cluster     string
		clusterRole string
		layer       string
		remove      bool
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
				return hcerrors.NewUserError("repo path not set \u2014 run 'hctl init'")
			}

			if env == "" {
				env = "production"
			}
			if layer == "" {
				layer = "environment"
			}

			addonsPath, valuesDir, err := resolveLayerPaths(cfg.RepoPath, layer, env, clusterRole, cluster, addonName)
			if err != nil {
				return hcerrors.NewUserError("resolving addon paths: %w", err)
			}

			entries, err := readAddonsYAML(addonsPath)
			if err != nil {
				return hcerrors.NewPlatformError("reading addons.yaml: %w", err)
			}

			if _, ok := entries[addonName]; !ok {
				return hcerrors.NewUserError("addon %q not found in %s", addonName, addonsPath)
			}

			if cfg.Interactive {
				action := "disable"
				if remove {
					action = "remove"
				}
				label := strings.ToUpper(action[:1]) + action[1:]
				ok, confirmErr := tui.Confirm(fmt.Sprintf("%s addon %q from %s?", label, addonName, filepath.Base(filepath.Dir(addonsPath))))
				if confirmErr != nil {
					return hcerrors.NewUserError("confirming operation: %w", confirmErr)
				}
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
				return hcerrors.NewPlatformError("writing addons config: %w", err)
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
				UI:          tui.GitUIAdapter{},
			}); err != nil {
				return hcerrors.NewPlatformError("committing addon changes: %w", err)
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
