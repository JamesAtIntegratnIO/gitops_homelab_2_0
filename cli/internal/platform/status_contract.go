package platform

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jamesatintegratnio/hctl/internal/kube"
	"github.com/jamesatintegratnio/hctl/internal/tui"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// StatusContract represents the status contract from a VClusterOrchestratorV2 resource.
type StatusContract struct {
	Phase          string
	Message        string
	LastReconciled string

	Endpoints   StatusEndpoints
	Credentials StatusCredentials
	Health      StatusHealth
	Conditions  []StatusCondition
}

// StatusEndpoints holds discoverable URLs.
type StatusEndpoints struct {
	API    string
	ArgoCD string
}

// StatusCredentials holds credential references.
type StatusCredentials struct {
	KubeconfigSecret string
	OnePasswordItem  string
}

// StatusHealth aggregates health data.
type StatusHealth struct {
	ArgoCDSync   string
	ArgoCDHealth string
	PodsReady    int64
	PodsTotal    int64
	SubAppsHealthy int64
	SubAppsTotal   int64
	SubAppsUnhealthy []string
}

// StatusCondition represents a Kubernetes-style condition.
type StatusCondition struct {
	Type               string
	Status             string
	Reason             string
	Message            string
	LastTransitionTime string
}

// GetStatusContract reads the .status contract from a VClusterOrchestratorV2 resource.
func GetStatusContract(ctx context.Context, client *kube.Client, namespace, name string) (*StatusContract, error) {
	vc, err := client.GetVCluster(ctx, namespace, name)
	if err != nil {
		return nil, err
	}

	return parseStatusContract(vc)
}

func parseStatusContract(vc *unstructured.Unstructured) (*StatusContract, error) {
	sc := &StatusContract{}

	// Top-level fields
	sc.Phase, _, _ = unstructured.NestedString(vc.Object, "status", "phase")
	sc.Message, _, _ = unstructured.NestedString(vc.Object, "status", "message")
	sc.LastReconciled, _, _ = unstructured.NestedString(vc.Object, "status", "lastReconciled")

	// Endpoints
	if endpoints, found, _ := unstructured.NestedStringMap(vc.Object, "status", "endpoints"); found {
		sc.Endpoints.API = endpoints["api"]
		sc.Endpoints.ArgoCD = endpoints["argocd"]
	}

	// Credentials
	if creds, found, _ := unstructured.NestedStringMap(vc.Object, "status", "credentials"); found {
		sc.Credentials.KubeconfigSecret = creds["kubeconfigSecret"]
		sc.Credentials.OnePasswordItem = creds["onePasswordItem"]
	}

	// Health
	sc.Health.ArgoCDSync, _, _ = unstructured.NestedString(vc.Object, "status", "health", "argocd", "syncStatus")
	sc.Health.ArgoCDHealth, _, _ = unstructured.NestedString(vc.Object, "status", "health", "argocd", "healthStatus")
	sc.Health.PodsReady, _, _ = unstructured.NestedInt64(vc.Object, "status", "health", "workloads", "ready")
	sc.Health.PodsTotal, _, _ = unstructured.NestedInt64(vc.Object, "status", "health", "workloads", "total")
	sc.Health.SubAppsHealthy, _, _ = unstructured.NestedInt64(vc.Object, "status", "health", "subApps", "healthy")
	sc.Health.SubAppsTotal, _, _ = unstructured.NestedInt64(vc.Object, "status", "health", "subApps", "total")

	if unhealthy, found, _ := unstructured.NestedStringSlice(vc.Object, "status", "health", "subApps", "unhealthy"); found {
		sc.Health.SubAppsUnhealthy = unhealthy
	}

	// Conditions
	if condSlice, found, _ := unstructured.NestedSlice(vc.Object, "status", "conditions"); found {
		for _, c := range condSlice {
			if condMap, ok := c.(map[string]interface{}); ok {
				cond := StatusCondition{}
				if v, ok := condMap["type"].(string); ok {
					cond.Type = v
				}
				if v, ok := condMap["status"].(string); ok {
					cond.Status = v
				}
				if v, ok := condMap["reason"].(string); ok {
					cond.Reason = v
				}
				if v, ok := condMap["message"].(string); ok {
					cond.Message = v
				}
				if v, ok := condMap["lastTransitionTime"].(string); ok {
					cond.LastTransitionTime = v
				}
				sc.Conditions = append(sc.Conditions, cond)
			}
		}
	}

	return sc, nil
}

// FormatStatusContract formats the status contract for terminal display.
func FormatStatusContract(name string, sc *StatusContract) string {
	var sb strings.Builder

	sb.WriteString(tui.HeadingStyle.Render("VCluster: "+name) + "\n\n")

	phaseIcon := phaseStyledIcon(sc.Phase)
	sb.WriteString(tui.KeyValue("Phase", sc.Phase+" "+phaseIcon) + "\n")
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

func phaseStyledIcon(phase string) string {
	switch phase {
	case "Ready":
		return tui.SuccessStyle.Render(tui.IconCheck)
	case "Progressing":
		return tui.WarningStyle.Render(tui.IconSync)
	case "Degraded":
		return tui.WarningStyle.Render(tui.IconWarn)
	case "Failed":
		return tui.ErrorStyle.Render(tui.IconCross)
	case "Scheduled":
		return tui.MutedStyle.Render(tui.IconPending)
	case "Deleting":
		return tui.WarningStyle.Render(tui.IconCross)
	default:
		return tui.MutedStyle.Render("?")
	}
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
