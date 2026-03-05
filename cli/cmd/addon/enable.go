package addon

import (
	"fmt"
	"os"
	"path/filepath"

	hcerrors "github.com/jamesatintegratnio/hctl/internal/errors"
	"github.com/jamesatintegratnio/hctl/internal/tui"
	"github.com/spf13/cobra"
)

func newAddonEnableCmd() *cobra.Command {
	var (
		env         string
		cluster     string
		clusterRole string
		namespace   string
		chartRepo   string
		chartName   string
		version     string
		layer       string
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
			ns := namespace
			if ns == "" {
				ns = addonName
			}

			err := addonModify(addonName, addonModifyOpts{
				Env:         env,
				Layer:       layer,
				ClusterRole: clusterRole,
				Cluster:     cluster,
				AllowCreate: true,
			}, func(entries map[string]map[string]interface{}, addonsPath, valuesDir string) (*addonMutateResult, error) {
				mr := &addonMutateResult{Action: "enable addon"}

				if existing, ok := entries[addonName]; ok {
					existing["enabled"] = true
					entries[addonName] = existing
					fmt.Printf("%s Enabled %s in %s\n", tui.SuccessStyle.Render(tui.IconCheck), addonName, addonsPath)
				} else {
					entry := map[string]interface{}{
						"enabled":         true,
						"namespace":       ns,
						"chartRepository": chartRepo,
						"chartName":       chartName,
						"defaultVersion":  version,
					}
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

				// Scaffold values directory
				if err := os.MkdirAll(valuesDir, 0o755); err != nil {
					return nil, hcerrors.NewPlatformError("creating values directory: %w", err)
				}
				valuesFile := filepath.Join(valuesDir, "values.yaml")
				if _, err := os.Stat(valuesFile); os.IsNotExist(err) {
					scaffold := fmt.Sprintf("# %s values\n# Layer: %s\n# See: https://github.com/stakater/application\n", addonName, layer)
					if err := os.WriteFile(valuesFile, []byte(scaffold), 0o644); err != nil {
						return nil, hcerrors.NewPlatformError("writing values scaffold: %w", err)
					}
					mr.ExtraPaths = append(mr.ExtraPaths, valuesFile)
					fmt.Printf("%s Scaffolded %s\n", tui.SuccessStyle.Render(tui.IconCheck), valuesFile)
				}

				return mr, nil
			})
			if err != nil {
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
