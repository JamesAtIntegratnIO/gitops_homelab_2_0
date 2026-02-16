package vcluster

import (
	"context"
	"fmt"
	"time"

	"github.com/jamesatintegratnio/hctl/internal/config"
	"github.com/jamesatintegratnio/hctl/internal/kube"
	"github.com/jamesatintegratnio/hctl/internal/platform"
	"github.com/jamesatintegratnio/hctl/internal/tui"
	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all vClusters",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Get()
			client, err := kube.NewClient(cfg.KubeContext)
			if err != nil {
				return fmt.Errorf("connecting to cluster: %w", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			vclusters, err := client.ListVClusters(ctx, cfg.Platform.PlatformNamespace)
			if err != nil {
				return fmt.Errorf("listing vclusters: %w", err)
			}

			if len(vclusters) == 0 {
				fmt.Println(tui.DimStyle.Render("No vClusters found"))
				return nil
			}

			var rows [][]string
			for _, vc := range vclusters {
				name := vc.GetName()
				preset, _, _ := platform.UnstructuredNestedString(vc.Object, "spec", "vcluster", "preset")
				hostname, _, _ := platform.UnstructuredNestedString(vc.Object, "spec", "exposure", "hostname")
				age := time.Since(vc.GetCreationTimestamp().Time).Round(time.Minute)

				// Check ArgoCD app health
				argoApp, err := client.GetArgoApp(ctx, "argocd", name)
				health := tui.DimStyle.Render("unknown")
				if err == nil {
					syncStatus, _, _ := platform.UnstructuredNestedString(argoApp.Object, "status", "sync", "status")
					healthStatus, _, _ := platform.UnstructuredNestedString(argoApp.Object, "status", "health", "status")
					if syncStatus == "Synced" && healthStatus == "Healthy" {
						health = tui.SuccessStyle.Render("Healthy")
					} else {
						health = tui.WarningStyle.Render(fmt.Sprintf("%s/%s", syncStatus, healthStatus))
					}
				}

				rows = append(rows, []string{name, preset, hostname, health, formatAge(age)})
			}

			fmt.Println(tui.Table([]string{"NAME", "PRESET", "HOSTNAME", "STATUS", "AGE"}, rows))
			return nil
		},
	}
}

func formatAge(d time.Duration) string {
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}
