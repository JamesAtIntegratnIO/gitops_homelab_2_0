package vcluster

import (
	"github.com/spf13/cobra"
)

// NewCmd returns the vcluster command group.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "vcluster",
		Aliases: []string{"vc"},
		Short:   "Manage vClusters",
		Long:    "Create, list, inspect, and manage vCluster instances on the platform.",
	}

	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newCreateCmd())
	cmd.AddCommand(newStatusCmd())
	cmd.AddCommand(newKubeconfigCmd())
	cmd.AddCommand(newDeleteCmd())
	cmd.AddCommand(newConnectCmd())
	cmd.AddCommand(newAppsCmd())
	cmd.AddCommand(newSyncCmd())

	return cmd
}
