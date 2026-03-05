package vcluster

import (
	"fmt"
	"strings"
	"time"

	"github.com/jamesatintegratnio/hctl/internal/platform"
	"github.com/jamesatintegratnio/hctl/internal/tui"
)

// FormatProvisionSummary formats the provisioning result for developer-friendly terminal output.
// Zero Kubernetes jargon — uses terms like "components", "endpoints", "access".
func FormatProvisionSummary(result *platform.ProvisionResult, hostname string) string {
	var sb strings.Builder

	// Status line
	if result.Healthy {
		sb.WriteString(fmt.Sprintf("\n  %s %s is ready!\n", tui.SuccessStyle.Render(tui.IconCheck), result.Name))
	} else {
		sb.WriteString(fmt.Sprintf("\n  %s %s is provisioning (may take a few more minutes)\n", tui.WarningStyle.Render(tui.IconWarn), result.Name))
	}

	// Endpoints
	hasEndpoints := result.Endpoints.API != "" || result.Endpoints.ArgoCD != "" || hostname != ""
	if hasEndpoints {
		sb.WriteString(tui.SectionHeader("Access") + "\n")
		if result.Endpoints.API != "" {
			sb.WriteString(tui.KeyValue("API Server", tui.CodeStyle.Render(result.Endpoints.API)) + "\n")
		} else if hostname != "" {
			sb.WriteString(tui.KeyValue("API Server", tui.CodeStyle.Render("https://"+hostname)) + "\n")
		}
		if result.Endpoints.ArgoCD != "" {
			sb.WriteString(tui.KeyValue("ArgoCD", tui.CodeStyle.Render(result.Endpoints.ArgoCD)) + "\n")
		}
	}

	// Health
	if result.Health.ComponentsTotal > 0 {
		sb.WriteString(tui.SectionHeader("Health") + "\n")
		compStr := fmt.Sprintf("%d/%d ready", result.Health.ComponentsReady, result.Health.ComponentsTotal)
		if result.Health.ComponentsReady == result.Health.ComponentsTotal {
			compStr = tui.SuccessStyle.Render(compStr)
		} else {
			compStr = tui.WarningStyle.Render(compStr)
		}
		sb.WriteString(tui.KeyValue("Components", compStr) + "\n")
		if result.Health.SubAppsTotal > 0 {
			appStr := fmt.Sprintf("%d/%d healthy", result.Health.SubAppsHealthy, result.Health.SubAppsTotal)
			if result.Health.SubAppsHealthy == result.Health.SubAppsTotal {
				appStr = tui.SuccessStyle.Render(appStr)
			} else {
				appStr = tui.WarningStyle.Render(appStr)
			}
			sb.WriteString(tui.KeyValue("Apps", appStr) + "\n")
		}
	}

	// Unhealthy items
	if len(result.Health.Unhealthy) > 0 {
		sb.WriteString(fmt.Sprintf("\n  %s Still converging:\n", tui.WarningStyle.Render(tui.IconWarn)))
		for _, u := range result.Health.Unhealthy {
			sb.WriteString(fmt.Sprintf("    %s %s\n", tui.MutedStyle.Render(tui.IconBullet), u))
		}
	}

	// Next steps
	sb.WriteString(tui.SectionHeader("Next Steps") + "\n")
	sb.WriteString(tui.KeyValue("Connect", tui.CodeStyle.Render(fmt.Sprintf("hctl vcluster connect %s", result.Name))) + "\n")
	sb.WriteString(tui.KeyValue("Status", tui.CodeStyle.Render(fmt.Sprintf("hctl vcluster status %s", result.Name))) + "\n")
	sb.WriteString(tui.KeyValue("Diagnose", tui.CodeStyle.Render(fmt.Sprintf("hctl vcluster status %s --diagnose", result.Name))) + "\n")

	return sb.String()
}

// FormatStatusContract formats the status contract for terminal display.
func FormatStatusContract(name string, sc *platform.StatusContract) string {
	var sb strings.Builder

	sb.WriteString(tui.HeadingStyle.Render("VCluster: "+name) + "\n\n")

	phaseLabel := tui.PhaseBadge(sc.Phase)
	sb.WriteString(tui.KeyValue("Phase", phaseLabel) + "\n")
	if sc.Message != "" {
		sb.WriteString(tui.KeyValue("Message", sc.Message) + "\n")
	}
	if sc.LastReconciled != "" {
		sb.WriteString(tui.KeyValue("Last Check", formatTimeAgo(sc.LastReconciled)) + "\n")
	}

	// Endpoints
	if sc.Endpoints.API != "" || sc.Endpoints.ArgoCD != "" {
		sb.WriteString(tui.SectionHeader("Endpoints") + "\n")
		if sc.Endpoints.API != "" {
			sb.WriteString(tui.KeyValue("API", tui.CodeStyle.Render(sc.Endpoints.API)) + "\n")
		}
		if sc.Endpoints.ArgoCD != "" {
			sb.WriteString(tui.KeyValue("ArgoCD", tui.CodeStyle.Render(sc.Endpoints.ArgoCD)) + "\n")
		}
	}

	// Credentials
	if sc.Credentials.KubeconfigSecret != "" || sc.Credentials.OnePasswordItem != "" {
		sb.WriteString(tui.SectionHeader("Credentials") + "\n")
		if sc.Credentials.KubeconfigSecret != "" {
			sb.WriteString(tui.KeyValue("Secret", sc.Credentials.KubeconfigSecret) + "\n")
		}
		if sc.Credentials.OnePasswordItem != "" {
			sb.WriteString(tui.KeyValue("1Password", sc.Credentials.OnePasswordItem) + "\n")
		}
	}

	// Health
	if sc.Health.ArgoCDSync != "" || sc.Health.PodsTotal > 0 {
		sb.WriteString(tui.SectionHeader("Health") + "\n")
		if sc.Health.ArgoCDSync != "" {
			healthStr := sc.Health.ArgoCDSync + " / " + sc.Health.ArgoCDHealth
			sb.WriteString(tui.KeyValue("ArgoCD", healthStr) + "\n")
		}
		podStr := fmt.Sprintf("%d/%d Ready", sc.Health.PodsReady, sc.Health.PodsTotal)
		if sc.Health.PodsReady == sc.Health.PodsTotal && sc.Health.PodsTotal > 0 {
			podStr = tui.SuccessStyle.Render(podStr)
		} else if sc.Health.PodsTotal > 0 {
			podStr = tui.WarningStyle.Render(podStr)
		}
		sb.WriteString(tui.KeyValue("Pods", podStr) + "\n")
		if sc.Health.SubAppsTotal > 0 {
			subStr := fmt.Sprintf("%d/%d Healthy", sc.Health.SubAppsHealthy, sc.Health.SubAppsTotal)
			if sc.Health.SubAppsHealthy == sc.Health.SubAppsTotal {
				subStr = tui.SuccessStyle.Render(subStr)
			} else {
				subStr = tui.WarningStyle.Render(subStr)
			}
			sb.WriteString(tui.KeyValue("Sub-Apps", subStr) + "\n")
			for _, app := range sc.Health.SubAppsUnhealthy {
				sb.WriteString(fmt.Sprintf("    %s %s\n", tui.ErrorStyle.Render(tui.IconCross), app))
			}
		}
	}

	// Conditions
	if len(sc.Conditions) > 0 {
		sb.WriteString(tui.SectionHeader("Conditions") + "\n")
		for _, c := range sc.Conditions {
			icon := tui.SuccessStyle.Render(tui.IconCheck)
			if c.Status != "True" {
				icon = tui.ErrorStyle.Render(tui.IconCross)
			}
			ago := formatTimeAgo(c.LastTransitionTime)
			sb.WriteString(fmt.Sprintf("  %s %-22s %s\n", icon, c.Type, tui.MutedStyle.Render(fmt.Sprintf("(%s, %s)", c.Reason, ago))))
		}
	}

	return tui.Box(sb.String())
}

func formatTimeAgo(timestamp string) string {
	if timestamp == "" {
		return "unknown"
	}
	t, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return timestamp
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}
