package platform

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jamesatintegratnio/hctl/internal/kube"
	"github.com/jamesatintegratnio/hctl/internal/tui"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ProvisionPhase represents a phase in the provisioning lifecycle.
type ProvisionPhase int

const (
	PhaseRequestAccepted ProvisionPhase = iota
	PhasePipelineRunning
	PhaseArgoSyncing
	PhaseClusterReady
)

// ProvisionResult holds the final outcome of waiting for provisioning.
type ProvisionResult struct {
	Name     string
	Phase    string
	Healthy  bool
	Endpoints ProvisionEndpoints
	Health   ProvisionHealth
	Error    string
}

// ProvisionEndpoints holds the discovered endpoints.
type ProvisionEndpoints struct {
	API    string
	ArgoCD string
}

// ProvisionHealth holds the health summary.
type ProvisionHealth struct {
	ComponentsReady int
	ComponentsTotal int
	SubAppsHealthy  int
	SubAppsTotal    int
	Unhealthy       []string
}

// WaitForRequest polls until the ResourceRequest exists in the cluster.
// This is the first phase after git push — ArgoCD must sync the request.
func WaitForRequest(ctx context.Context, client *kube.Client, namespace, name string, pollInterval time.Duration) (string, error) {
	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("timed out waiting for request to appear (ArgoCD may not have synced yet)")
		default:
		}

		vc, err := client.GetVCluster(ctx, namespace, name)
		if err == nil && vc != nil {
			age := time.Since(vc.GetCreationTimestamp().Time).Round(time.Second)
			return fmt.Sprintf("created %s ago", age), nil
		}

		time.Sleep(pollInterval)
	}
}

// WaitForPipeline polls until the Kratix pipeline job completes.
func WaitForPipeline(ctx context.Context, client *kube.Client, namespace, name string, pollInterval time.Duration) (string, error) {
	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("timed out waiting for pipeline to complete")
		default:
		}

		jobs, err := client.Clientset.BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("kratix.io/resource-name=%s", name),
		})
		if err != nil {
			time.Sleep(pollInterval)
			continue
		}

		if len(jobs.Items) == 0 {
			time.Sleep(pollInterval)
			continue
		}

		latest := jobs.Items[len(jobs.Items)-1]
		for _, cond := range latest.Status.Conditions {
			if cond.Type == "Complete" && cond.Status == "True" {
				return fmt.Sprintf("job %s completed", latest.Name), nil
			}
			if cond.Type == "Failed" && cond.Status == "True" {
				return "", fmt.Errorf("pipeline failed: %s", cond.Message)
			}
		}

		time.Sleep(pollInterval)
	}
}

// WaitForArgoSync polls until the ArgoCD application is synced and healthy.
func WaitForArgoSync(ctx context.Context, client *kube.Client, name string, pollInterval time.Duration) (string, error) {
	argoAppName := "vcluster-" + name

	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("timed out waiting for ArgoCD to sync")
		default:
		}

		app, err := client.GetArgoApp(ctx, "argocd", argoAppName)
		if err != nil {
			// Try without vcluster- prefix
			app, err = client.GetArgoApp(ctx, "argocd", name)
		}
		if err != nil {
			time.Sleep(pollInterval)
			continue
		}

		syncStatus, _, _ := UnstructuredNestedString(app.Object, "status", "sync", "status")
		healthStatus, _, _ := UnstructuredNestedString(app.Object, "status", "health", "status")

		if syncStatus == "Synced" && healthStatus == "Healthy" {
			return "synced and healthy", nil
		}

		if healthStatus == "Degraded" {
			return "", fmt.Errorf("application is degraded — run 'hctl vcluster status %s --diagnose' for details", name)
		}

		time.Sleep(pollInterval)
	}
}

// WaitForClusterReady polls until the vCluster pods are running in the target namespace.
func WaitForClusterReady(ctx context.Context, client *kube.Client, name string, pollInterval time.Duration) (string, error) {
	targetNs := name

	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("timed out waiting for cluster to become ready")
		default:
		}

		pods, err := client.ListPods(ctx, targetNs, "")
		if err != nil {
			time.Sleep(pollInterval)
			continue
		}

		if len(pods) == 0 {
			time.Sleep(pollInterval)
			continue
		}

		running := 0
		for _, p := range pods {
			if p.Phase == "Running" && p.ReadyContainers == p.TotalContainers {
				running++
			}
		}

		if running > 0 && running == len(pods) {
			return fmt.Sprintf("%d/%d components running", running, len(pods)), nil
		}

		time.Sleep(pollInterval)
	}
}

// CollectProvisionResult gathers the final result after provisioning completes.
func CollectProvisionResult(ctx context.Context, client *kube.Client, namespace, name string) (*ProvisionResult, error) {
	result := &ProvisionResult{Name: name}

	// Try to get status contract first (most complete source)
	sc, err := GetStatusContract(ctx, client, namespace, name)
	if err == nil && sc.Phase != "" {
		result.Phase = sc.Phase
		result.Endpoints.API = sc.Endpoints.API
		result.Endpoints.ArgoCD = sc.Endpoints.ArgoCD
		result.Health.ComponentsReady = int(sc.Health.PodsReady)
		result.Health.ComponentsTotal = int(sc.Health.PodsTotal)
		result.Health.SubAppsHealthy = int(sc.Health.SubAppsHealthy)
		result.Health.SubAppsTotal = int(sc.Health.SubAppsTotal)
		result.Health.Unhealthy = sc.Health.SubAppsUnhealthy
		result.Healthy = sc.Phase == "Ready"
		return result, nil
	}

	// Fallback: assemble from individual queries
	result.Phase = "Ready"
	result.Healthy = true

	// Pods
	pods, err := client.ListPods(ctx, name, "")
	if err == nil {
		result.Health.ComponentsTotal = len(pods)
		for _, p := range pods {
			if p.Phase == "Running" && p.ReadyContainers == p.TotalContainers {
				result.Health.ComponentsReady++
			}
		}
		if result.Health.ComponentsReady < result.Health.ComponentsTotal {
			result.Healthy = false
			result.Phase = "Progressing"
		}
	}

	// Sub-apps
	subApps, err := client.ListArgoAppsForCluster(ctx, "argocd", name)
	if err == nil && len(subApps) > 0 {
		result.Health.SubAppsTotal = len(subApps)
		for _, app := range subApps {
			if app.SyncStatus == "Synced" && app.HealthStatus == "Healthy" {
				result.Health.SubAppsHealthy++
			} else {
				result.Health.Unhealthy = append(result.Health.Unhealthy, app.Name)
			}
		}
	}

	return result, nil
}

// FormatProvisionSummary formats the provisioning result for developer-friendly terminal output.
// Zero Kubernetes jargon — uses terms like "components", "endpoints", "access".
func FormatProvisionSummary(result *ProvisionResult, hostname string) string {
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
