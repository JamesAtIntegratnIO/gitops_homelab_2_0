package vcluster

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jamesatintegratnio/hctl/internal/config"
	"github.com/jamesatintegratnio/hctl/internal/git"
	"github.com/jamesatintegratnio/hctl/internal/tui"
	"github.com/spf13/cobra"
)

func newDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete [name]",
		Short: "Delete a vCluster",
		Long: `Delete a vCluster by removing its resource request file from git.

This removes the YAML file from platform/vclusters/ and commits the change.
ArgoCD will then remove the resource, triggering Kratix cleanup.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			cfg := config.Get()

			repoPath := cfg.RepoPath
			if repoPath == "" {
				repo, err := git.DetectRepo("")
				if err != nil {
					return fmt.Errorf("cannot detect repo — run 'hctl init' first")
				}
				repoPath = repo.Root
			}

			filePath := filepath.Join(repoPath, "platform", "vclusters", name+".yaml")
			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				return fmt.Errorf("vCluster file not found: %s", filePath)
			}

			// Confirm deletion
			confirmed, _ := tui.Confirm(fmt.Sprintf("Delete vCluster %q? This will remove %s and trigger cleanup.", name, filePath))
			if !confirmed {
				fmt.Println("Cancelled")
				return nil
			}

			if err := os.Remove(filePath); err != nil {
				return fmt.Errorf("removing file: %w", err)
			}

			relPath, _ := filepath.Rel(repoPath, filePath)
			fmt.Printf("%s Removed %s\n", tui.SuccessStyle.Render(tui.IconCheck), relPath)

			// Git handling
			if _, err := git.HandleGitWorkflow(git.WorkflowOpts{
				RepoPath:    repoPath,
				Paths:       []string{relPath},
				Action:      "delete vcluster",
				Resource:    name,
				GitMode:     cfg.GitMode,
				Interactive: cfg.Interactive,
			}); err != nil {
				return err
			}

			fmt.Printf("\n%s\n", tui.DimStyle.Render("ArgoCD will remove the resource and Kratix will clean up."))
			return nil
		},
	}
}
