package deploy

import (
	"context"
	"fmt"
	"strings"
	"time"

	deploylib "github.com/jamesatintegratnio/hctl/internal/deploy"
	"github.com/jamesatintegratnio/hctl/internal/config"
	hcerrors "github.com/jamesatintegratnio/hctl/internal/errors"
	"github.com/jamesatintegratnio/hctl/internal/kube"
	"github.com/jamesatintegratnio/hctl/internal/tui"
	unstr "github.com/jamesatintegratnio/hctl/internal/unstructured"
	"github.com/spf13/cobra"
)

func newDeployListCmd() *cobra.Command {
	var cluster string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List deployed workloads for a cluster",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Get()
			if cfg.RepoPath == "" {
				return hcerrors.NewUserError("repo path not set \u2014 run 'hctl init'")
			}

			if cluster == "" {
				cluster = cfg.DefaultCluster
			}
			if cluster == "" {
				return hcerrors.NewUserError("no cluster specified \u2014 use --cluster or set defaultCluster")
			}

			workloads, err := deploylib.ListWorkloads(cfg.RepoPath, cluster)
			if err != nil {
				return hcerrors.NewPlatformError("reading workloads: %w", err)
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

					client, cErr := kube.Shared()
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
						syncStatus := unstr.MustString(app.Object, "status", "sync", "status")
						healthStatus := unstr.MustString(app.Object, "status", "health", "status")
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
