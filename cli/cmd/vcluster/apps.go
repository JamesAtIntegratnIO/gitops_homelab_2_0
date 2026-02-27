package vcluster

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jamesatintegratnio/hctl/internal/config"
	"github.com/jamesatintegratnio/hctl/internal/kube"
	"github.com/jamesatintegratnio/hctl/internal/tui"
	"github.com/spf13/cobra"
)

func newAppsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "apps [name]",
		Short: "List ArgoCD apps targeting a vCluster",
		Long: `Lists all ArgoCD applications whose destination matches the vCluster cluster
registration. Shows sync status, health, operation state, and configuration warnings.

Highlights:
  - Stale operations (retrying for >1 hour)
  - SyncFailed with CRD-related errors
  - Missing selfHeal in sync policy`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			cfg := config.Get()

			client, err := kube.NewClient(cfg.KubeContext)
			if err != nil {
				return fmt.Errorf("connecting to cluster: %w", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			// ArgoCD cluster registrations typically use the vcluster name as the destination name
			clusterName := name
			apps, err := client.ListArgoAppsForCluster(ctx, "argocd", clusterName)
			if err != nil {
				return fmt.Errorf("listing apps for cluster %s: %w", clusterName, err)
			}

			if len(apps) == 0 {
				fmt.Printf("%s No ArgoCD applications found targeting cluster %q\n",
					tui.MutedStyle.Render(tui.IconPending), name)
				return nil
			}

			fmt.Printf("\n%s\n\n", tui.TitleStyle.Render(fmt.Sprintf("ArgoCD Apps %s %s (%d apps)", tui.IconArrow, name, len(apps))))

			var rows [][]string
			var warnings []string

			for _, app := range apps {
				syncIcon := tui.SyncBadge(app.SyncStatus)
				healthIcon := tui.HealthBadge(app.HealthStatus)
				opInfo := tui.OpBadge(app.OpPhase, app.RetryCount)

				rows = append(rows, []string{
					app.Name,
					syncIcon,
					healthIcon,
					opInfo,
				})

				// Collect warnings
				if !app.HasSelfHeal {
					warnings = append(warnings, fmt.Sprintf("  %s  %s: missing selfHeal in sync policy", tui.WarningStyle.Render(tui.IconWarn), app.Name))
				}
				if app.OpPhase == "Failed" || app.OpPhase == "Error" {
					if isStaleOperation(app.OpStartedAt) {
						warnings = append(warnings, fmt.Sprintf("  %s %s: stale %s operation (started %s, %d retries)",
							tui.ErrorStyle.Render(tui.IconCross), app.Name, app.OpPhase, app.OpStartedAt, app.RetryCount))
					}
					if strings.Contains(app.Message, "resource mapping not found") {
						warnings = append(warnings, fmt.Sprintf("  %s %s: CRD missing — %s",
							tui.ErrorStyle.Render(tui.IconCross), app.Name, truncateMessage(app.Message, 80)))
					}
				}
			}

			fmt.Println(tui.Table([]string{"APP", "SYNC", "HEALTH", "OPERATION"}, rows))

			if len(warnings) > 0 {
				fmt.Printf("\n%s\n", tui.WarningStyle.Render("Warnings:"))
				for _, w := range warnings {
					fmt.Println(w)
				}
			}

			// Summary
			synced, healthy, failed, noSelfHeal := 0, 0, 0, 0
			for _, app := range apps {
				if app.SyncStatus == "Synced" {
					synced++
				}
				if app.HealthStatus == "Healthy" {
					healthy++
				}
				if app.OpPhase == "Failed" || app.OpPhase == "Error" {
					failed++
				}
				if !app.HasSelfHeal {
					noSelfHeal++
				}
			}

			fmt.Printf("\n  Summary: %d synced, %d healthy, %s failed, %s missing selfHeal\n\n",
				synced, healthy,
				tui.StyledCount(failed, tui.ErrorStyle),
				tui.StyledCount(noSelfHeal, tui.WarningStyle),
			)

			return nil
		},
	}
}

func isStaleOperation(startedAt string) bool {
	if startedAt == "" {
		return false
	}
	t, err := time.Parse(time.RFC3339, startedAt)
	if err != nil {
		return false
	}
	return time.Since(t) > time.Hour
}

func truncateMessage(msg string, maxLen int) string {
	if len(msg) <= maxLen {
		return msg
	}
	return msg[:maxLen] + "..."
}
