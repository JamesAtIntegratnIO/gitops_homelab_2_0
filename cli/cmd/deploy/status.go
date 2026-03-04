package deploy

import (
	"context"
	"fmt"
	"strings"
	"time"

	deploylib "github.com/jamesatintegratnio/hctl/internal/deploy"
	"github.com/jamesatintegratnio/hctl/internal/config"
	hcerrors "github.com/jamesatintegratnio/hctl/internal/errors"
	"github.com/jamesatintegratnio/hctl/internal/git"
	"github.com/jamesatintegratnio/hctl/internal/kube"
	"github.com/jamesatintegratnio/hctl/internal/platform"
	"github.com/jamesatintegratnio/hctl/internal/score"
	"github.com/jamesatintegratnio/hctl/internal/tui"
	"github.com/spf13/cobra"
)

func newDeployStatusCmd() *cobra.Command {
	var cluster string
	cmd := &cobra.Command{
		Use:   "status [workload]",
		Short: "Check deployment status of a workload",
		Long: `Shows the ArgoCD sync and health status of a deployed workload.

If no workload name is given, reads from score.yaml in the current directory.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Get()

			// Determine workload name
			var workloadName string
			if len(args) > 0 {
				workloadName = args[0]
			} else {
				w, err := score.LoadWorkload("score.yaml")
				if err != nil {
					return fmt.Errorf("no workload specified and no score.yaml found: %w", err)
				}
				workloadName = w.Metadata.Name
				if cluster == "" {
					cluster = w.TargetCluster()
				}
			}

			if cluster == "" {
				cluster = cfg.DefaultCluster
			}
			if cluster == "" {
				return hcerrors.NewUserError("no cluster specified — use --cluster or set defaultCluster")
			}

			client, err := kube.NewClient(cfg.KubeContext)
			if err != nil {
				return hcerrors.NewPlatformError("connecting to cluster: %v", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			// Check ArgoCD app (workload apps are usually in the vCluster's ArgoCD)
			app, err := client.GetArgoApp(ctx, "argocd", workloadName)
			if err != nil {
				// Try with cluster prefix
				app, err = client.GetArgoApp(ctx, "argocd", cluster+"-"+workloadName)
				if err != nil {
					return fmt.Errorf("ArgoCD application not found for %q", workloadName)
				}
			}

			syncStatus, _, _ := platform.UnstructuredNestedString(app.Object, "status", "sync", "status")
			healthStatus, _, _ := platform.UnstructuredNestedString(app.Object, "status", "health", "status")
			revision, _, _ := platform.UnstructuredNestedString(app.Object, "status", "sync", "revision")

			fmt.Printf("\n%s\n\n", tui.TitleStyle.Render(workloadName))
			fmt.Printf("  Cluster:  %s\n", cluster)

			statusStr := fmt.Sprintf("%s/%s", syncStatus, healthStatus)
			if syncStatus == "Synced" && healthStatus == "Healthy" {
				fmt.Printf("  Status:   %s\n", tui.SuccessStyle.Render(statusStr))
			} else {
				fmt.Printf("  Status:   %s\n", tui.WarningStyle.Render(statusStr))
			}
			if revision != "" {
				fmt.Printf("  Revision: %s\n", tui.DimStyle.Render(revision))
			}

			// Check pods
			namespace := cluster
			pods, err := client.ListPods(ctx, namespace, fmt.Sprintf("app.kubernetes.io/name=%s", workloadName))
			if err == nil && len(pods) > 0 {
				fmt.Printf("\n  Pods:\n")
				for _, p := range pods {
					status := tui.SuccessStyle.Render(p.Phase)
					if p.Phase != "Running" || p.ReadyContainers < p.TotalContainers {
						status = tui.WarningStyle.Render(p.Phase)
					}
					fmt.Printf("    %s  %d/%d  %s\n", p.Name, p.ReadyContainers, p.TotalContainers, status)
				}
			}

			fmt.Println()
			return nil
		},
	}
	cmd.Flags().StringVar(&cluster, "cluster", "", "target vCluster")
	return cmd
}

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
				ok, _ := tui.Confirm(fmt.Sprintf("Remove workload %q from cluster %q?", workloadName, cluster))
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

func newDeployListCmd() *cobra.Command {
	var cluster string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List deployed workloads for a cluster",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
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

			workloads, err := deploylib.ListWorkloads(cfg.RepoPath, cluster)
			if err != nil {
				return fmt.Errorf("reading workloads: %w", err)
			}

			if len(workloads) == 0 {
				fmt.Println(tui.DimStyle.Render("No workloads deployed to " + cluster))
				return nil
			}

			var rows [][]string
			for _, name := range workloads {
				rows = append(rows, []string{name, cluster})
			}

			_, err = tui.InteractiveTable(tui.InteractiveTableConfig{
				Title:   "Workloads (" + cluster + ")",
				Headers: []string{"WORKLOAD", "CLUSTER"},
				Rows:    rows,
				OnSelect: func(row []string, index int) string {
					if len(row) == 0 {
						return ""
					}
					workloadName := row[0]

					client, cErr := kube.NewClient(cfg.KubeContext)
					if cErr != nil {
						return tui.ErrorStyle.Render("Cannot connect: " + cErr.Error())
					}
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()

					var sb strings.Builder
					sb.WriteString(tui.HeaderStyle.Render("Workload: "+workloadName) + "\n\n")

					// ArgoCD status
					app, aErr := client.GetArgoApp(ctx, "argocd", workloadName)
					if aErr != nil {
						app, aErr = client.GetArgoApp(ctx, "argocd", cluster+"-"+workloadName)
					}
					if aErr == nil {
						syncStatus, _, _ := platform.UnstructuredNestedString(app.Object, "status", "sync", "status")
						healthStatus, _, _ := platform.UnstructuredNestedString(app.Object, "status", "health", "status")
						statusStr := syncStatus + "/" + healthStatus
						if syncStatus == "Synced" && healthStatus == "Healthy" {
							sb.WriteString("  Status: " + tui.SuccessStyle.Render(statusStr) + "\n")
						} else {
							sb.WriteString("  Status: " + tui.WarningStyle.Render(statusStr) + "\n")
						}
					} else {
						sb.WriteString("  Status: " + tui.DimStyle.Render("not found in ArgoCD") + "\n")
					}

					// Pods
					pods, pErr := client.ListPods(ctx, cluster, fmt.Sprintf("app.kubernetes.io/name=%s", workloadName))
					if pErr == nil && len(pods) > 0 {
						sb.WriteString("\n  Pods:\n")
						for _, p := range pods {
							status := tui.SuccessStyle.Render(p.Phase)
							if p.Phase != "Running" || p.ReadyContainers < p.TotalContainers {
								status = tui.WarningStyle.Render(p.Phase)
							}
							sb.WriteString(fmt.Sprintf("    %s  %d/%d  %s\n", p.Name, p.ReadyContainers, p.TotalContainers, status))
						}
					}

					return sb.String()
				},
			})
			return err
		},
	}
	cmd.Flags().StringVar(&cluster, "cluster", "", "target vCluster")
	return cmd
}
