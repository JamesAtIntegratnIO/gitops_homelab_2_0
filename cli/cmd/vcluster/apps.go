package vcluster

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
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
					tui.DimStyle.Render("○"), name)
				return nil
			}

			fmt.Printf("\n%s\n\n", tui.TitleStyle.Render(fmt.Sprintf("ArgoCD Apps → %s (%d apps)", name, len(apps))))

			var rows [][]string
			var warnings []string

			for _, app := range apps {
				syncIcon := formatSyncIcon(app.SyncStatus)
				healthIcon := formatHealthIcon(app.HealthStatus)
				opInfo := formatOpInfo(app)

				rows = append(rows, []string{
					app.Name,
					syncIcon,
					healthIcon,
					opInfo,
				})

				// Collect warnings
				if !app.HasSelfHeal {
					warnings = append(warnings, fmt.Sprintf("  ⚠  %s: missing selfHeal in sync policy", app.Name))
				}
				if app.OpPhase == "Failed" || app.OpPhase == "Error" {
					if isStaleOperation(app.OpStartedAt) {
						warnings = append(warnings, fmt.Sprintf("  🔴 %s: stale %s operation (started %s, %d retries)",
							app.Name, app.OpPhase, app.OpStartedAt, app.RetryCount))
					}
					if strings.Contains(app.Message, "resource mapping not found") {
						warnings = append(warnings, fmt.Sprintf("  🔴 %s: CRD missing — %s",
							app.Name, truncateMessage(app.Message, 80)))
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
				colorCount(failed, tui.ErrorStyle),
				colorCount(noSelfHeal, tui.WarningStyle),
			)

			return nil
		},
	}
}

func formatSyncIcon(status string) string {
	switch status {
	case "Synced":
		return tui.SuccessStyle.Render("✓ Synced")
	case "OutOfSync":
		return tui.WarningStyle.Render("⟳ OutOfSync")
	case "Unknown":
		return tui.DimStyle.Render("? Unknown")
	default:
		return tui.DimStyle.Render(status)
	}
}

func formatHealthIcon(status string) string {
	switch status {
	case "Healthy":
		return tui.SuccessStyle.Render("♥ Healthy")
	case "Degraded":
		return tui.ErrorStyle.Render("✗ Degraded")
	case "Progressing":
		return tui.WarningStyle.Render("⟳ Progressing")
	case "Missing":
		return tui.ErrorStyle.Render("✗ Missing")
	case "Suspended":
		return tui.DimStyle.Render("⏸ Suspended")
	default:
		return tui.DimStyle.Render(status)
	}
}

func formatOpInfo(app kube.ArgoAppInfo) string {
	if app.OpPhase == "" {
		return tui.DimStyle.Render("—")
	}

	parts := []string{app.OpPhase}
	if app.RetryCount > 0 {
		parts = append(parts, fmt.Sprintf("retry:%d", app.RetryCount))
	}
	info := strings.Join(parts, " ")

	switch app.OpPhase {
	case "Succeeded":
		return tui.SuccessStyle.Render(info)
	case "Running":
		return tui.WarningStyle.Render(info)
	case "Failed", "Error":
		return tui.ErrorStyle.Render(info)
	default:
		return info
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

func colorCount(n int, style lipgloss.Style) string {
	s := fmt.Sprintf("%d", n)
	if n > 0 {
		return style.Render(s)
	}
	return s
}
