package vcluster

import (
	"context"
	"fmt"
	"strings"
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

			// Interactive table: enter to show diagnostics
			action, err := tui.InteractiveTable(tui.InteractiveTableConfig{
				Title:   "vClusters",
				Headers: []string{"NAME", "PRESET", "HOSTNAME", "STATUS", "AGE"},
				Rows:    rows,
				OnSelect: func(row []string, index int) string {
					vcName := row[0]
					ctx2, cancel2 := context.WithTimeout(context.Background(), 15*time.Second)
					defer cancel2()

					result, err := platform.DiagnoseVCluster(ctx2, client, cfg.Platform.PlatformNamespace, vcName)
					if err != nil {
						return tui.ErrorStyle.Render("Error: " + err.Error())
					}

					var sb strings.Builder
					sb.WriteString(tui.TitleStyle.Render("Diagnostics: "+vcName) + "\n\n")
					for i, step := range result.Steps {
						isLast := i == len(result.Steps)-1
						sb.WriteString(tui.TreeNode(
							fmt.Sprintf("%-15s", step.Name),
							step.Status.String(),
							step.Message,
							isLast,
						) + "\n")
						if step.Details != "" {
							indent := "  │   "
							if isLast {
								indent = "      "
							}
							sb.WriteString(indent + tui.DimStyle.Render(step.Details) + "\n")
						}
					}
					return sb.String()
				},
			})
			_ = action
			return err
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
