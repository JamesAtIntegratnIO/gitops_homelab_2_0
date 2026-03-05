package deploy

import (
	"context"
	"fmt"
	"time"

	"github.com/jamesatintegratnio/hctl/internal/config"
	hcerrors "github.com/jamesatintegratnio/hctl/internal/errors"
	"github.com/jamesatintegratnio/hctl/internal/kube"
	"github.com/jamesatintegratnio/hctl/internal/score"
	"github.com/jamesatintegratnio/hctl/internal/tui"
	unstr "github.com/jamesatintegratnio/hctl/internal/unstructured"
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
					return hcerrors.NewUserError("no workload specified and no score.yaml found: %w", err)
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

			client, err := kube.SharedWithConfig(config.Get().KubeContext)
			if err != nil {
				return hcerrors.NewPlatformError("connecting to cluster: %w", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			// Check ArgoCD app (workload apps are usually in the vCluster's ArgoCD)
			app, err := client.GetArgoApp(ctx, "argocd", workloadName)
			if err != nil {
				// Try with cluster prefix
				app, err = client.GetArgoApp(ctx, "argocd", cluster+"-"+workloadName)
				if err != nil {
					return hcerrors.NewPlatformError("ArgoCD application not found for %q", workloadName)
				}
			}

			syncStatus := unstr.MustString(app.Object, "status", "sync", "status")
			healthStatus := unstr.MustString(app.Object, "status", "health", "status")
			revision := unstr.MustString(app.Object, "status", "sync", "revision")

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
