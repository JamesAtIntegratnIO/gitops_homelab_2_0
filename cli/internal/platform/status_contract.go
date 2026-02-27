package platform

import (
	"context"
	"fmt"
	"time"

	"github.com/jamesatintegratnio/hctl/internal/kube"
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
	var b []byte

	phaseIcon := phaseStatusIcon(sc.Phase)
	header := fmt.Sprintf("┌─ VCluster: %s ─────────────────────────\n", name)
	b = append(b, header...)
	b = append(b, fmt.Sprintf("│ Phase:     %s %s\n", sc.Phase, phaseIcon)...)
	if sc.Message != "" {
		b = append(b, fmt.Sprintf("│ Message:   %s\n", sc.Message)...)
	}
	if sc.LastReconciled != "" {
		ago := formatTimeAgo(sc.LastReconciled)
		b = append(b, fmt.Sprintf("│ Last Check: %s\n", ago)...)
	}

	// Endpoints
	if sc.Endpoints.API != "" || sc.Endpoints.ArgoCD != "" {
		b = append(b, "│\n│ Endpoints:\n"...)
		if sc.Endpoints.API != "" {
			b = append(b, fmt.Sprintf("│   API:     %s\n", sc.Endpoints.API)...)
		}
		if sc.Endpoints.ArgoCD != "" {
			b = append(b, fmt.Sprintf("│   ArgoCD:  %s\n", sc.Endpoints.ArgoCD)...)
		}
	}

	// Credentials
	if sc.Credentials.KubeconfigSecret != "" || sc.Credentials.OnePasswordItem != "" {
		b = append(b, "│\n│ Credentials:\n"...)
		if sc.Credentials.KubeconfigSecret != "" {
			b = append(b, fmt.Sprintf("│   Secret:     %s\n", sc.Credentials.KubeconfigSecret)...)
		}
		if sc.Credentials.OnePasswordItem != "" {
			b = append(b, fmt.Sprintf("│   1Password:  %s\n", sc.Credentials.OnePasswordItem)...)
		}
	}

	// Health
	if sc.Health.ArgoCDSync != "" || sc.Health.PodsTotal > 0 {
		b = append(b, "│\n│ Health:\n"...)
		if sc.Health.ArgoCDSync != "" {
			b = append(b, fmt.Sprintf("│   ArgoCD:   %s / %s\n", sc.Health.ArgoCDSync, sc.Health.ArgoCDHealth)...)
		}
		b = append(b, fmt.Sprintf("│   Pods:     %d/%d Ready\n", sc.Health.PodsReady, sc.Health.PodsTotal)...)
		if sc.Health.SubAppsTotal > 0 {
			b = append(b, fmt.Sprintf("│   Sub-Apps: %d/%d Healthy\n", sc.Health.SubAppsHealthy, sc.Health.SubAppsTotal)...)
			for _, app := range sc.Health.SubAppsUnhealthy {
				b = append(b, fmt.Sprintf("│     ✗ %s\n", app)...)
			}
		}
	}

	// Conditions
	if len(sc.Conditions) > 0 {
		b = append(b, "│\n│ Conditions:\n"...)
		for _, c := range sc.Conditions {
			icon := "✓"
			if c.Status != "True" {
				icon = "✗"
			}
			ago := formatTimeAgo(c.LastTransitionTime)
			b = append(b, fmt.Sprintf("│   %s %-22s (%s, %s)\n", icon, c.Type, c.Reason, ago)...)
		}
	}

	b = append(b, "└────────────────────────────────────────────\n"...)

	return string(b)
}

func phaseStatusIcon(phase string) string {
	switch phase {
	case "Ready":
		return "✓"
	case "Progressing":
		return "⟳"
	case "Degraded":
		return "⚠"
	case "Failed":
		return "✗"
	case "Scheduled":
		return "⏳"
	case "Deleting":
		return "🗑"
	default:
		return "?"
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
