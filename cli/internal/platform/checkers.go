package platform

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jamesatintegratnio/hctl/internal/kube"
	unstr "github.com/jamesatintegratnio/hctl/internal/unstructured"
)

// ResourceRequestChecker verifies the vCluster ResourceRequest exists and
// populates shared diagnostic state (VCluster, TargetNS).
type ResourceRequestChecker struct{}

func (c *ResourceRequestChecker) Check(ctx context.Context, client KubeClient, state *DiagnosticState) ([]DiagnosticStep, bool) {
	vc, err := client.GetVCluster(ctx, state.Namespace, state.Name)
	if err != nil {
		return []DiagnosticStep{{
			Name:    "ResourceRequest",
			Status:  StatusError,
			Message: fmt.Sprintf("Not found: %s", state.Name),
			Details: err.Error(),
		}}, true
	}

	state.VCluster = vc
	if ns, ok, _ := unstr.NestedString(vc.Object, "spec", "targetNamespace"); ok && ns != "" {
		state.TargetNS = ns
	}

	age := time.Since(vc.GetCreationTimestamp().Time).Round(time.Second)
	return []DiagnosticStep{{
		Name:    "ResourceRequest",
		Status:  StatusOK,
		Message: fmt.Sprintf("Found (age: %s)", age),
	}}, false
}

// PipelineJobChecker verifies Kratix pipeline jobs for the resource.
type PipelineJobChecker struct{}

func (c *PipelineJobChecker) Check(ctx context.Context, client KubeClient, state *DiagnosticState) ([]DiagnosticStep, bool) {
	jobs, err := client.ListJobs(ctx, state.Namespace, fmt.Sprintf("kratix.io/resource-name=%s", state.Name))
	if err != nil {
		return []DiagnosticStep{{
			Name:    "Pipeline Job",
			Status:  StatusWarning,
			Message: "Could not list jobs",
			Details: err.Error(),
		}}, false
	}
	if len(jobs) == 0 {
		return []DiagnosticStep{{
			Name:    "Pipeline Job",
			Status:  StatusWarning,
			Message: "No pipeline jobs found — pipeline may not have run yet",
		}}, false
	}

	latest := jobs[len(jobs)-1]
	succeeded := false
	for _, cond := range latest.Status.Conditions {
		if cond.Type == "Complete" && cond.Status == "True" {
			succeeded = true
		}
		if cond.Type == "Failed" && cond.Status == "True" {
			return []DiagnosticStep{{
				Name:    "Pipeline Job",
				Status:  StatusError,
				Message: fmt.Sprintf("Job %s failed", latest.Name),
				Details: cond.Message,
			}}, false
		}
	}
	if succeeded {
		return []DiagnosticStep{{
			Name:    "Pipeline Job",
			Status:  StatusOK,
			Message: fmt.Sprintf("Completed (%s)", latest.Name),
		}}, false
	}
	if len(latest.Status.Conditions) == 0 {
		return []DiagnosticStep{{
			Name:    "Pipeline Job",
			Status:  StatusWarning,
			Message: fmt.Sprintf("Job %s still running", latest.Name),
		}}, false
	}
	return nil, false
}

// WorkChecker verifies Kratix Work resources exist for the vCluster.
type WorkChecker struct{}

func (c *WorkChecker) Check(ctx context.Context, client KubeClient, state *DiagnosticState) ([]DiagnosticStep, bool) {
	works, err := client.ListWorks(ctx, state.Namespace)
	if err != nil {
		return []DiagnosticStep{{
			Name:    "Work",
			Status:  StatusWarning,
			Message: "Could not list Work resources",
			Details: err.Error(),
		}}, false
	}

	for _, w := range works {
		if strings.Contains(w.GetName(), state.Name) {
			return []DiagnosticStep{{
				Name:    "Work",
				Status:  StatusOK,
				Message: fmt.Sprintf("Found: %s", w.GetName()),
			}}, false
		}
	}

	return []DiagnosticStep{{
		Name:    "Work",
		Status:  StatusWarning,
		Message: "No Work resource found for this vCluster",
	}}, false
}

// WorkPlacementChecker verifies Kratix WorkPlacement scheduling.
type WorkPlacementChecker struct{}

func (c *WorkPlacementChecker) Check(ctx context.Context, client KubeClient, state *DiagnosticState) ([]DiagnosticStep, bool) {
	placements, err := client.ListWorkPlacements(ctx, state.Namespace)
	if err != nil {
		return []DiagnosticStep{{
			Name:    "WorkPlacement",
			Status:  StatusWarning,
			Message: "Could not list WorkPlacements",
			Details: err.Error(),
		}}, false
	}

	for _, wp := range placements {
		if strings.Contains(wp.GetName(), state.Name) {
			conditions := unstr.MustSlice(wp.Object, "status", "conditions")
			failing := false
			for _, cond := range conditions {
				if cm, ok := cond.(map[string]interface{}); ok {
					if cm["type"] == "Failing" && cm["status"] == "True" {
						failing = true
					}
				}
			}
			if failing {
				return []DiagnosticStep{{
					Name:    "WorkPlacement",
					Status:  StatusWarning,
					Message: fmt.Sprintf("WorkPlacement %s shows Failing (may be cosmetic)", wp.GetName()),
					Details: "Known issue: WorkPlacement can show Failing despite successful deployment. Verify git state repo.",
				}}, false
			}
			return []DiagnosticStep{{
				Name:    "WorkPlacement",
				Status:  StatusOK,
				Message: fmt.Sprintf("Synced: %s", wp.GetName()),
			}}, false
		}
	}

	return []DiagnosticStep{{
		Name:    "WorkPlacement",
		Status:  StatusWarning,
		Message: "No WorkPlacement found — Work may not have been scheduled",
	}}, false
}

// ArgoCDAppChecker verifies the ArgoCD application for the vCluster.
type ArgoCDAppChecker struct{}

func (c *ArgoCDAppChecker) Check(ctx context.Context, client KubeClient, state *DiagnosticState) ([]DiagnosticStep, bool) {
	argoAppName := "vcluster-" + state.Name
	argoApp, err := client.GetArgoApp(ctx, "argocd", argoAppName)
	if err != nil {
		// Fallback: try just the name in case naming convention differs
		argoApp, err = client.GetArgoApp(ctx, "argocd", state.Name)
	}
	if err != nil {
		return []DiagnosticStep{{
			Name:    "ArgoCD App",
			Status:  StatusWarning,
			Message: "ArgoCD application not found",
			Details: err.Error(),
		}}, false
	}

	syncStatus := unstr.MustString(argoApp.Object, "status", "sync", "status")
	healthStatus := unstr.MustString(argoApp.Object, "status", "health", "status")
	status := StatusOK
	if syncStatus != "Synced" || healthStatus != "Healthy" {
		status = StatusWarning
	}
	return []DiagnosticStep{{
		Name:    "ArgoCD App",
		Status:  status,
		Message: fmt.Sprintf("Sync: %s, Health: %s", syncStatus, healthStatus),
	}}, false
}

// PodChecker verifies pods are running in the target namespace.
type PodChecker struct{}

func (c *PodChecker) Check(ctx context.Context, client KubeClient, state *DiagnosticState) ([]DiagnosticStep, bool) {
	pods, err := client.ListPods(ctx, state.TargetNS, "")
	if err != nil {
		return []DiagnosticStep{{
			Name:    "Pods",
			Status:  StatusWarning,
			Message: "Could not list pods",
			Details: err.Error(),
		}}, false
	}

	running := 0
	total := len(pods)
	for _, p := range pods {
		if p.Phase == "Running" && p.ReadyContainers == p.TotalContainers {
			running++
		}
	}
	status := StatusOK
	if running < total {
		status = StatusWarning
	}
	if total == 0 {
		status = StatusWarning
	}
	return []DiagnosticStep{{
		Name:    "Pods",
		Status:  status,
		Message: fmt.Sprintf("%d/%d ready", running, total),
	}}, false
}

// PodResourceChecker verifies pod resource allocation and restart counts.
type PodResourceChecker struct{}

func (c *PodResourceChecker) Check(ctx context.Context, client KubeClient, state *DiagnosticState) ([]DiagnosticStep, bool) {
	podResources, err := client.GetPodResourceInfo(ctx, state.TargetNS, "app=vcluster")
	if err == nil && len(podResources) > 0 {
		return c.buildSteps(podResources, false, state.Name), false
	}
	if err == nil {
		// No pods with app=vcluster label; try broader match
		podResources, err = client.GetPodResourceInfo(ctx, state.TargetNS, "")
		if err == nil {
			return c.buildSteps(podResources, true, state.Name), false
		}
	}
	return nil, false
}

func (c *PodResourceChecker) buildSteps(podResources []kube.PodResourceInfo, filterByName bool, name string) []DiagnosticStep {
	var steps []DiagnosticStep
	for _, pr := range podResources {
		if filterByName && !strings.Contains(pr.Name, name) && !strings.Contains(pr.Name, "vcluster") {
			continue
		}
		status := StatusOK
		details := ""
		if pr.Restarts > 0 {
			status = StatusWarning
			details = fmt.Sprintf("%d restart(s) — may indicate OOM or crash-loop", pr.Restarts)
		}
		var msg string
		if filterByName {
			msg = fmt.Sprintf("%s: mem req=%s lim=%s, restarts=%d",
				pr.Name, pr.MemoryRequest, pr.MemoryLimit, pr.Restarts)
		} else {
			msg = fmt.Sprintf("%s: mem req=%s lim=%s, cpu req=%s lim=%s, restarts=%d",
				pr.Name, pr.MemoryRequest, pr.MemoryLimit, pr.CPURequest, pr.CPULimit, pr.Restarts)
		}
		steps = append(steps, DiagnosticStep{
			Name:    "Resources",
			Status:  status,
			Message: msg,
			Details: details,
		})
	}
	return steps
}

// SubAppHealthChecker verifies ArgoCD sub-application health and selfHeal policies.
type SubAppHealthChecker struct{}

func (c *SubAppHealthChecker) Check(ctx context.Context, client KubeClient, state *DiagnosticState) ([]DiagnosticStep, bool) {
	subApps, err := client.ListArgoAppsForCluster(ctx, "argocd", state.Name)
	if err != nil || len(subApps) == 0 {
		return nil, false
	}

	healthyApps, unhealthyApps, noSelfHealApps := 0, 0, 0
	var unhealthyDetails []string
	var selfHealWarnings []string

	for _, app := range subApps {
		if app.SyncStatus == "Synced" && app.HealthStatus == "Healthy" {
			healthyApps++
		} else {
			unhealthyApps++
			detail := fmt.Sprintf("%s: %s/%s", app.Name, app.SyncStatus, app.HealthStatus)
			if app.OpPhase == "Failed" || app.OpPhase == "Error" {
				detail += fmt.Sprintf(" (%s, retries:%d)", app.OpPhase, app.RetryCount)
			}
			unhealthyDetails = append(unhealthyDetails, detail)
		}
		if !app.HasSelfHeal {
			noSelfHealApps++
			selfHealWarnings = append(selfHealWarnings, app.Name)
		}
	}

	var steps []DiagnosticStep

	subAppStatus := StatusOK
	subAppMsg := fmt.Sprintf("%d/%d healthy", healthyApps, len(subApps))
	subAppDetails := ""
	if unhealthyApps > 0 {
		subAppStatus = StatusWarning
		subAppMsg += fmt.Sprintf(", %d unhealthy", unhealthyApps)
		subAppDetails = strings.Join(unhealthyDetails, "; ")
	}
	steps = append(steps, DiagnosticStep{
		Name:    "Sub-Apps",
		Status:  subAppStatus,
		Message: subAppMsg,
		Details: subAppDetails,
	})

	if noSelfHealApps > 0 {
		steps = append(steps, DiagnosticStep{
			Name:    "SelfHeal",
			Status:  StatusWarning,
			Message: fmt.Sprintf("%d/%d apps missing selfHeal policy", noSelfHealApps, len(subApps)),
			Details: "Apps without selfHeal won't auto-recover from sync failures: " + strings.Join(selfHealWarnings, ", "),
		})
	} else {
		steps = append(steps, DiagnosticStep{
			Name:    "SelfHeal",
			Status:  StatusOK,
			Message: fmt.Sprintf("All %d apps have selfHeal enabled", len(subApps)),
		})
	}

	return steps, false
}
