package platform

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jamesatintegratnio/hctl/internal/kube"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DiagnosticResult holds the results of a platform diagnostic check.
type DiagnosticResult struct {
	Steps []DiagnosticStep
}

// DiagnosticStep represents one step in the diagnostic chain.
type DiagnosticStep struct {
	Name    string
	Status  StepStatus
	Message string
	Details string
}

// StepStatus represents the health status of a diagnostic step.
type StepStatus int

const (
	StatusOK StepStatus = iota
	StatusWarning
	StatusError
	StatusUnknown
)

func (s StepStatus) String() string {
	switch s {
	case StatusOK:
		return "✅"
	case StatusWarning:
		return "⚠️"
	case StatusError:
		return "❌"
	default:
		return "❓"
	}
}

// DiagnoseVCluster runs the full diagnostic chain for a vCluster resource.
func DiagnoseVCluster(ctx context.Context, client *kube.Client, namespace, name string) (*DiagnosticResult, error) {
	result := &DiagnosticResult{}

	// Step 1: Check ResourceRequest exists
	vc, err := client.GetVCluster(ctx, namespace, name)
	if err != nil {
		result.Steps = append(result.Steps, DiagnosticStep{
			Name:    "ResourceRequest",
			Status:  StatusError,
			Message: fmt.Sprintf("Not found: %s", name),
			Details: err.Error(),
		})
		return result, nil
	}

	creationTime := vc.GetCreationTimestamp().Time
	age := time.Since(creationTime).Round(time.Second)
	result.Steps = append(result.Steps, DiagnosticStep{
		Name:    "ResourceRequest",
		Status:  StatusOK,
		Message: fmt.Sprintf("Found (age: %s)", age),
	})

	// Step 2: Check pipeline jobs
	jobs, err := client.Clientset.BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("kratix.io/resource-name=%s", name),
	})
	if err != nil {
		result.Steps = append(result.Steps, DiagnosticStep{
			Name:    "Pipeline Job",
			Status:  StatusWarning,
			Message: "Could not list jobs",
			Details: err.Error(),
		})
	} else if len(jobs.Items) == 0 {
		result.Steps = append(result.Steps, DiagnosticStep{
			Name:    "Pipeline Job",
			Status:  StatusWarning,
			Message: "No pipeline jobs found — pipeline may not have run yet",
		})
	} else {
		latest := jobs.Items[len(jobs.Items)-1]
		succeeded := false
		for _, cond := range latest.Status.Conditions {
			if cond.Type == "Complete" && cond.Status == "True" {
				succeeded = true
			}
			if cond.Type == "Failed" && cond.Status == "True" {
				result.Steps = append(result.Steps, DiagnosticStep{
					Name:    "Pipeline Job",
					Status:  StatusError,
					Message: fmt.Sprintf("Job %s failed", latest.Name),
					Details: cond.Message,
				})
				succeeded = false
				break
			}
		}
		if succeeded {
			result.Steps = append(result.Steps, DiagnosticStep{
				Name:    "Pipeline Job",
				Status:  StatusOK,
				Message: fmt.Sprintf("Completed (%s)", latest.Name),
			})
		} else if len(latest.Status.Conditions) == 0 {
			result.Steps = append(result.Steps, DiagnosticStep{
				Name:    "Pipeline Job",
				Status:  StatusWarning,
				Message: fmt.Sprintf("Job %s still running", latest.Name),
			})
		}
	}

	// Step 3: Check Work objects
	works, err := client.Dynamic.Resource(kube.WorkGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		result.Steps = append(result.Steps, DiagnosticStep{
			Name:    "Work",
			Status:  StatusWarning,
			Message: "Could not list Work resources",
			Details: err.Error(),
		})
	} else {
		found := false
		for _, w := range works.Items {
			if strings.Contains(w.GetName(), name) {
				found = true
				result.Steps = append(result.Steps, DiagnosticStep{
					Name:    "Work",
					Status:  StatusOK,
					Message: fmt.Sprintf("Found: %s", w.GetName()),
				})
				break
			}
		}
		if !found {
			result.Steps = append(result.Steps, DiagnosticStep{
				Name:    "Work",
				Status:  StatusWarning,
				Message: "No Work resource found for this vCluster",
			})
		}
	}

	// Step 4: Check WorkPlacement
	placements, err := client.Dynamic.Resource(kube.WorkPlacementGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		result.Steps = append(result.Steps, DiagnosticStep{
			Name:    "WorkPlacement",
			Status:  StatusWarning,
			Message: "Could not list WorkPlacements",
			Details: err.Error(),
		})
	} else {
		found := false
		for _, wp := range placements.Items {
			if strings.Contains(wp.GetName(), name) {
				found = true
				// Check for failing status
				conditions, _, _ := UnstructuredNestedSlice(wp.Object, "status", "conditions")
				failing := false
				for _, c := range conditions {
					if cm, ok := c.(map[string]interface{}); ok {
						if cm["type"] == "Failing" && cm["status"] == "True" {
							failing = true
						}
					}
				}
				if failing {
					result.Steps = append(result.Steps, DiagnosticStep{
						Name:    "WorkPlacement",
						Status:  StatusWarning,
						Message: fmt.Sprintf("WorkPlacement %s shows Failing (may be cosmetic)", wp.GetName()),
						Details: "Known issue: WorkPlacement can show Failing despite successful deployment. Verify git state repo.",
					})
				} else {
					result.Steps = append(result.Steps, DiagnosticStep{
						Name:    "WorkPlacement",
						Status:  StatusOK,
						Message: fmt.Sprintf("Synced: %s", wp.GetName()),
					})
				}
				break
			}
		}
		if !found {
			result.Steps = append(result.Steps, DiagnosticStep{
				Name:    "WorkPlacement",
				Status:  StatusWarning,
				Message: "No WorkPlacement found — Work may not have been scheduled",
			})
		}
	}

	// Step 5: Check ArgoCD Application
	targetNs := name
	if ns, ok, _ := UnstructuredNestedString(vc.Object, "spec", "targetNamespace"); ok && ns != "" {
		targetNs = ns
	}

	argoAppName := "vcluster-" + name
	argoApp, err := client.GetArgoApp(ctx, "argocd", argoAppName)
	if err != nil {
		// Fallback: try just the name in case naming convention differs
		argoApp, err = client.GetArgoApp(ctx, "argocd", name)
	}
	if err != nil {
		result.Steps = append(result.Steps, DiagnosticStep{
			Name:    "ArgoCD App",
			Status:  StatusWarning,
			Message: "ArgoCD application not found",
			Details: err.Error(),
		})
	} else {
		syncStatus, _, _ := UnstructuredNestedString(argoApp.Object, "status", "sync", "status")
		healthStatus, _, _ := UnstructuredNestedString(argoApp.Object, "status", "health", "status")
		status := StatusOK
		if syncStatus != "Synced" || healthStatus != "Healthy" {
			status = StatusWarning
		}
		result.Steps = append(result.Steps, DiagnosticStep{
			Name:    "ArgoCD App",
			Status:  status,
			Message: fmt.Sprintf("Sync: %s, Health: %s", syncStatus, healthStatus),
		})
	}

	// Step 6: Check pods in target namespace
	pods, err := client.ListPods(ctx, targetNs, "")
	if err != nil {
		result.Steps = append(result.Steps, DiagnosticStep{
			Name:    "Pods",
			Status:  StatusWarning,
			Message: "Could not list pods",
			Details: err.Error(),
		})
	} else {
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
		result.Steps = append(result.Steps, DiagnosticStep{
			Name:    "Pods",
			Status:  status,
			Message: fmt.Sprintf("%d/%d ready", running, total),
		})
	}

	// Step 7: Check pod resource allocation (memory)
	podResources, err := client.GetPodResourceInfo(ctx, targetNs, "app=vcluster")
	if err == nil && len(podResources) > 0 {
		for _, pr := range podResources {
			status := StatusOK
			details := ""
			if pr.Restarts > 0 {
				status = StatusWarning
				details = fmt.Sprintf("%d restart(s) — may indicate OOM or crash-loop", pr.Restarts)
			}
			result.Steps = append(result.Steps, DiagnosticStep{
				Name:    "Resources",
				Status:  status,
				Message: fmt.Sprintf("%s: mem req=%s lim=%s, cpu req=%s lim=%s, restarts=%d",
					pr.Name, pr.MemoryRequest, pr.MemoryLimit, pr.CPURequest, pr.CPULimit, pr.Restarts),
				Details: details,
			})
		}
	} else if err == nil && len(podResources) == 0 {
		// Try without label selector for broader match
		podResources, err = client.GetPodResourceInfo(ctx, targetNs, "")
		if err == nil {
			for _, pr := range podResources {
				if !strings.Contains(pr.Name, name) && !strings.Contains(pr.Name, "vcluster") {
					continue
				}
				status := StatusOK
				details := ""
				if pr.Restarts > 0 {
					status = StatusWarning
					details = fmt.Sprintf("%d restart(s) — may indicate OOM or crash-loop", pr.Restarts)
				}
				result.Steps = append(result.Steps, DiagnosticStep{
					Name:    "Resources",
					Status:  status,
					Message: fmt.Sprintf("%s: mem req=%s lim=%s, restarts=%d",
						pr.Name, pr.MemoryRequest, pr.MemoryLimit, pr.Restarts),
					Details: details,
				})
			}
		}
	}

	// Step 8: Check sub-app health (ArgoCD apps targeting this vcluster)
	subApps, err := client.ListArgoAppsForCluster(ctx, "argocd", name)
	if err == nil && len(subApps) > 0 {
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

		subAppStatus := StatusOK
		subAppMsg := fmt.Sprintf("%d/%d healthy", healthyApps, len(subApps))
		subAppDetails := ""

		if unhealthyApps > 0 {
			subAppStatus = StatusWarning
			subAppMsg += fmt.Sprintf(", %d unhealthy", unhealthyApps)
			subAppDetails = strings.Join(unhealthyDetails, "; ")
		}

		result.Steps = append(result.Steps, DiagnosticStep{
			Name:    "Sub-Apps",
			Status:  subAppStatus,
			Message: subAppMsg,
			Details: subAppDetails,
		})

		// Step 9: selfHeal policy check
		if noSelfHealApps > 0 {
			result.Steps = append(result.Steps, DiagnosticStep{
				Name:    "SelfHeal",
				Status:  StatusWarning,
				Message: fmt.Sprintf("%d/%d apps missing selfHeal policy", noSelfHealApps, len(subApps)),
				Details: "Apps without selfHeal won't auto-recover from sync failures: " + strings.Join(selfHealWarnings, ", "),
			})
		} else {
			result.Steps = append(result.Steps, DiagnosticStep{
				Name:    "SelfHeal",
				Status:  StatusOK,
				Message: fmt.Sprintf("All %d apps have selfHeal enabled", len(subApps)),
			})
		}
	}

	return result, nil
}

// UnstructuredNestedString extracts a string from a nested unstructured object.
func UnstructuredNestedString(obj map[string]interface{}, fields ...string) (string, bool, error) {
	val, found, err := nestedField(obj, fields...)
	if !found || err != nil {
		return "", found, err
	}
	s, ok := val.(string)
	return s, ok, nil
}

// UnstructuredNestedSlice extracts a slice from a nested unstructured object.
func UnstructuredNestedSlice(obj map[string]interface{}, fields ...string) ([]interface{}, bool, error) {
	val, found, err := nestedField(obj, fields...)
	if !found || err != nil {
		return nil, found, err
	}
	s, ok := val.([]interface{})
	return s, ok, nil
}

func nestedField(obj map[string]interface{}, fields ...string) (interface{}, bool, error) {
	var current interface{} = obj
	for _, f := range fields {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil, false, nil
		}
		current, ok = m[f]
		if !ok {
			return nil, false, nil
		}
	}
	return current, true, nil
}
