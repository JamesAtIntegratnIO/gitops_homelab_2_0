package addon

import (
	"github.com/spf13/cobra"
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

