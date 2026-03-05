package platform

import (
	"context"
	"fmt"
	"time"

	hcerrors "github.com/jamesatintegratnio/hctl/internal/errors"
	unstr "github.com/jamesatintegratnio/hctl/internal/unstructured"
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
func WaitForRequest(ctx context.Context, client KubeClient, namespace, name string, pollInterval time.Duration) (string, error) {
	for {
		select {
		case <-ctx.Done():
			return "", hcerrors.NewTimeoutError("timed out waiting for request to appear (ArgoCD may not have synced yet)")
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
func WaitForPipeline(ctx context.Context, client KubeClient, namespace, name string, pollInterval time.Duration) (string, error) {
	for {
		select {
		case <-ctx.Done():
			return "", hcerrors.NewTimeoutError("timed out waiting for pipeline to complete")
		default:
		}

		jobs, err := client.ListJobs(ctx, namespace, fmt.Sprintf("kratix.io/resource-name=%s", name))
		if err != nil {
			time.Sleep(pollInterval)
			continue
		}

		if len(jobs) == 0 {
			time.Sleep(pollInterval)
			continue
		}

		latest := jobs[len(jobs)-1]
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
func WaitForArgoSync(ctx context.Context, client KubeClient, name string, pollInterval time.Duration) (string, error) {
	argoAppName := "vcluster-" + name

	for {
		select {
		case <-ctx.Done():
			return "", hcerrors.NewTimeoutError("timed out waiting for ArgoCD to sync")
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

		syncStatus := unstr.MustString(app.Object, "status", "sync", "status")
		healthStatus := unstr.MustString(app.Object, "status", "health", "status")

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
func WaitForClusterReady(ctx context.Context, client KubeClient, name string, pollInterval time.Duration) (string, error) {
	targetNs := name

	for {
		select {
		case <-ctx.Done():
			return "", hcerrors.NewTimeoutError("timed out waiting for cluster to become ready")
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
func CollectProvisionResult(ctx context.Context, client KubeClient, namespace, name string) (*ProvisionResult, error) {
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
