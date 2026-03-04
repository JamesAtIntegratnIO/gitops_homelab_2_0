package deploy

import (
	"fmt"

	deploylib "github.com/jamesatintegratnio/hctl/internal/deploy"
	"github.com/jamesatintegratnio/hctl/internal/config"
	"github.com/jamesatintegratnio/hctl/internal/git"
	"github.com/jamesatintegratnio/hctl/internal/tui"
	"github.com/spf13/cobra"
)

func newDeployRemoveCmd() *cobra.Command {
	var cluster string
	cmd := &cobra.Command{
		Use:   "remove [workload]",
		Short: "Remove a deployed workload",
		Long: `Removes a workload from the platform by deleting its entry from addons.yaml
and removing its values directory. ArgoCD will clean up the resources on next sync.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			workloadName := args[0]
			cfg := config.Get()
			if cfg.RepoPath == "" {
				return fmt.Errorf("repo path not set — run 'hctl init'")
			}

			if cluster == "" {
				cluster = cfg.DefaultCluster
			}
			if cluster == "" {
				return fmt.Errorf("no cluster specified — use --cluster or set defaultCluster")
			}

			// Confirm removal
			if cfg.Interactive {
				ok, confirmErr := tui.Confirm(fmt.Sprintf("Remove workload %q from cluster %q?", workloadName, cluster))
				if confirmErr != nil {
					return fmt.Errorf("confirming operation: %w", confirmErr)
				}
				if !ok {
					fmt.Println(tui.DimStyle.Render("Cancelled"))
					return nil
				}
			}

			removedPaths, err := deploylib.RemoveWorkload(cfg.RepoPath, cluster, workloadName)
			if err != nil {
				return fmt.Errorf("removing workload files: %w", err)
			}

			fmt.Printf("%s Removed workload %s from %s\n",
				tui.SuccessStyle.Render(tui.IconCheck), workloadName, cluster)

			// Git operations
			if _, err := git.HandleGitWorkflow(git.WorkflowOpts{
				RepoPath:      cfg.RepoPath,
				Paths:         removedPaths,
				Action:        "remove",
				Resource:      workloadName,
				Details:       cluster,
				GitMode:       cfg.GitMode,
				Interactive:   cfg.Interactive,
				ConfirmPrompt: "Commit and push removal?",
			}); err != nil {
				return fmt.Errorf("committing workload removal: %w", err)
			}

			fmt.Printf("\n%s\n", tui.DimStyle.Render("ArgoCD will remove the workload on next sync."))
			return nil
		},
	}
	cmd.Flags().StringVar(&cluster, "cluster", "", "target vCluster")
	return cmd
}
