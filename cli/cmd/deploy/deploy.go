package deploy

import (
	"github.com/spf13/cobra"
)

// NewCmd returns the deploy command group.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy workloads via Score",
		Long: `Deploy workloads to vClusters using Score specification.

Score (score.dev/v1b1) is a platform-agnostic workload spec. hctl translates
Score workloads into platform-native resources (Stakater Application chart,
ExternalSecrets, HTTPRoutes, Certificates).

Workflow:
  1. hctl deploy init          — scaffold a score.yaml
  2. hctl deploy run           — translate and deploy to the target vCluster
  3. hctl deploy render        — preview rendered manifests
  4. hctl deploy diff          — compare rendered vs on-disk
  5. hctl deploy status        — check deployment status
  6. hctl deploy remove        — tear down the workload`,
	}

	cmd.AddCommand(newDeployInitCmd())
	cmd.AddCommand(newDeployRunCmd())
	cmd.AddCommand(newDeployRenderCmd())
	cmd.AddCommand(newDeployDiffCmd())
	cmd.AddCommand(newDeployStatusCmd())
	cmd.AddCommand(newDeployRemoveCmd())
	cmd.AddCommand(newDeployListCmd())

	return cmd
}
