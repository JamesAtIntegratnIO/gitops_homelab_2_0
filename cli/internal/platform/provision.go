package platform

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jamesatintegratnio/hctl/internal/kube"
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
		sb.WriteString(fmt.Sprintf("\n  ✅ %s is ready!\n", result.Name))
	} else {
		sb.WriteString(fmt.Sprintf("\n  ⚠️  %s is provisioning (may take a few more minutes)\n", result.Name))
	}

	// Endpoints
	hasEndpoints := result.Endpoints.API != "" || result.Endpoints.ArgoCD != "" || hostname != ""
	if hasEndpoints {
		sb.WriteString("\n  🔗 Access:\n")
		if result.Endpoints.API != "" {
			sb.WriteString(fmt.Sprintf("     API Server:  %s\n", result.Endpoints.API))
		} else if hostname != "" {
			sb.WriteString(fmt.Sprintf("     API Server:  https://%s\n", hostname))
		}
		if result.Endpoints.ArgoCD != "" {
			sb.WriteString(fmt.Sprintf("     ArgoCD:      %s\n", result.Endpoints.ArgoCD))
		}
	}

	// Health
	if result.Health.ComponentsTotal > 0 {
		sb.WriteString(fmt.Sprintf("\n  📊 Health: %d/%d components ready",
			result.Health.ComponentsReady, result.Health.ComponentsTotal))
		if result.Health.SubAppsTotal > 0 {
			sb.WriteString(fmt.Sprintf(", %d/%d apps healthy",
				result.Health.SubAppsHealthy, result.Health.SubAppsTotal))
		}
		sb.WriteString("\n")
	}

	// Unhealthy items (developer-friendly)
	if len(result.Health.Unhealthy) > 0 {
		sb.WriteString("\n  ⚠️  Still converging:\n")
		for _, u := range result.Health.Unhealthy {
			sb.WriteString(fmt.Sprintf("     • %s\n", u))
		}
	}

	// Next steps
	sb.WriteString(fmt.Sprintf("\n  📋 Next steps:\n"))
	sb.WriteString(fmt.Sprintf("     Connect:    hctl vcluster connect %s\n", result.Name))
	sb.WriteString(fmt.Sprintf("     Status:     hctl vcluster status %s\n", result.Name))
	sb.WriteString(fmt.Sprintf("     Diagnose:   hctl vcluster status %s --diagnose\n", result.Name))

	return sb.String()
}
