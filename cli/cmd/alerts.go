package cmd

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jamesatintegratnio/hctl/internal/config"
	"github.com/jamesatintegratnio/hctl/internal/kube"
	"github.com/jamesatintegratnio/hctl/internal/tui"
	"github.com/spf13/cobra"
)

var (
	alertsShowAll bool
)

var alertsCmd = &cobra.Command{
	Use:   "alerts",
	Short: "Show firing alerts from Prometheus",
	Long: `Queries Prometheus for currently firing alerts and displays them in a
developer-friendly summary. Watchdog and InfoInhibitor (noise) alerts are
hidden by default — use --all to include them.`,
	RunE: runAlerts,
}

func init() {
	alertsCmd.Flags().BoolVar(&alertsShowAll, "all", false, "include noise alerts (Watchdog, InfoInhibitor)")
}

// noiseAlerts are meta-alerts that don't indicate real problems.
var noiseAlerts = map[string]bool{
	"Watchdog":      true,
	"InfoInhibitor": true,
}

func runAlerts(cmd *cobra.Command, args []string) error {
	cfg := config.Get()

	var alerts []kube.PrometheusAlert

	_, err := tui.Spin("Querying Prometheus", func() (string, error) {
		client, err := kube.NewClient(cfg.KubeContext)
		if err != nil {
			return "", fmt.Errorf("connecting to cluster: %w", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		alerts, err = client.QueryFiringAlerts(ctx,
			"monitoring",
			"kube-prometheus-stack-prometheus",
			9090,
		)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%d alerts", len(alerts)), nil
	})
	if err != nil {
		return err
	}

	// Filter noise
	var filtered []kube.PrometheusAlert
	noiseCount := 0
	for _, a := range alerts {
		if !alertsShowAll && noiseAlerts[a.AlertName] {
			noiseCount++
			continue
		}
		filtered = append(filtered, a)
	}

	// Sort by severity then alertname
	severityOrder := map[string]int{"critical": 0, "warning": 1, "info": 2, "none": 3, "": 4}
	sort.Slice(filtered, func(i, j int) bool {
		si := severityOrder[filtered[i].Severity]
		sj := severityOrder[filtered[j].Severity]
		if si != sj {
			return si < sj
		}
		return filtered[i].AlertName < filtered[j].AlertName
	})

	fmt.Println()

	if len(filtered) == 0 {
		fmt.Printf("  %s No firing alerts\n", tui.SuccessStyle.Render(tui.IconCheck))
		if noiseCount > 0 {
			fmt.Printf("  %s\n", tui.MutedStyle.Render(fmt.Sprintf("(%d noise alerts hidden — use --all to show)", noiseCount)))
		}
		fmt.Println()
		return nil
	}

	// Summary counts
	critical, warning, info := 0, 0, 0
	for _, a := range filtered {
		switch a.Severity {
		case "critical":
			critical++
		case "warning":
			warning++
		default:
			info++
		}
	}

	fmt.Printf("  %s  Firing alerts: ", tui.TitleStyle.Render(tui.IconBell+" Alerts"))
	parts := []string{}
	if critical > 0 {
		parts = append(parts, tui.ErrorStyle.Render(fmt.Sprintf("%d critical", critical)))
	}
	if warning > 0 {
		parts = append(parts, tui.WarningStyle.Render(fmt.Sprintf("%d warning", warning)))
	}
	if info > 0 {
		parts = append(parts, tui.MutedStyle.Render(fmt.Sprintf("%d info", info)))
	}
	fmt.Println(strings.Join(parts, "  "))
	fmt.Println()

	// Table
	var rows [][]string
	for _, a := range filtered {
		sev := tui.SeverityBadge(a.Severity)
		ns := a.Namespace
		if ns == "" {
			ns = "cluster"
		}
		detail := a.Controller
		if detail == "" && a.Pod != "" {
			detail = a.Pod
		}
		rows = append(rows, []string{sev, a.AlertName, ns, detail})
	}

	fmt.Println(tui.Table([]string{"SEV", "ALERT", "NAMESPACE", "DETAIL"}, rows))

	if noiseCount > 0 {
		fmt.Printf("\n  %s\n", tui.MutedStyle.Render(fmt.Sprintf("(%d noise alerts hidden — use --all to show)", noiseCount)))
	}
	fmt.Println()

	return nil
}
