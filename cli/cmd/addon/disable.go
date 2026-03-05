package addon

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jamesatintegratnio/hctl/internal/config"
	hcerrors "github.com/jamesatintegratnio/hctl/internal/errors"
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

			err := addonModify(addonName, addonModifyOpts{
				Env:         env,
				Layer:       layer,
				ClusterRole: clusterRole,
				Cluster:     cluster,
			}, func(entries map[string]map[string]interface{}, addonsPath, valuesDir string) (*addonMutateResult, error) {
				if _, ok := entries[addonName]; !ok {
					return nil, hcerrors.NewUserError("addon %q not found in %s", addonName, addonsPath)
				}

				cfg := config.Get()
				if cfg.Interactive {
					action := "disable"
					if remove {
						action = "remove"
					}
					label := strings.ToUpper(action[:1]) + action[1:]
					ok, confirmErr := tui.Confirm(fmt.Sprintf("%s addon %q from %s?", label, addonName, filepath.Base(filepath.Dir(addonsPath))))
					if confirmErr != nil {
						return nil, hcerrors.NewUserError("confirming operation: %w", confirmErr)
					}
					if !ok {
						fmt.Println(tui.DimStyle.Render("Cancelled"))
						return &addonMutateResult{Action: "disable addon"}, nil
					}
				}

				action := "disable addon"
				mr := &addonMutateResult{Action: action}

				if remove {
					delete(entries, addonName)
					fmt.Printf("%s Removed %s from addons.yaml\n", tui.SuccessStyle.Render(tui.IconCheck), addonName)

					if _, err := os.Stat(valuesDir); err == nil {
						if err := os.RemoveAll(valuesDir); err != nil {
							fmt.Printf("%s Could not remove values directory: %v\n", tui.WarningStyle.Render(tui.IconWarn), err)
						} else {
							fmt.Printf("%s Removed %s\n", tui.SuccessStyle.Render(tui.IconCheck), valuesDir)
							mr.ExtraPaths = append(mr.ExtraPaths, valuesDir)
						}
					}
					mr.Action = "remove addon"
				} else {
					entries[addonName]["enabled"] = false
					fmt.Printf("%s Disabled %s\n", tui.SuccessStyle.Render(tui.IconCheck), addonName)
				}

				return mr, nil
			})
			if err != nil {
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
