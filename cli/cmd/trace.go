package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jamesatintegratnio/hctl/internal/config"
	"github.com/jamesatintegratnio/hctl/internal/kube"
	"github.com/jamesatintegratnio/hctl/internal/platform"
	"github.com/jamesatintegratnio/hctl/internal/tui"
	"github.com/spf13/cobra"
)

var traceCmd = &cobra.Command{
	Use:   "trace [resource]",
	Short: "Trace a resource through the platform lifecycle",
	Long: `Follows a resource (vCluster, workload, addon) through each stage of the
platform lifecycle and shows its current state at each hop.

Lifecycle chain:
  1. ResourceRequest (Kratix CR)
  2. Pipeline execution (Job)
  3. Work placement (GitStateStore)
  4. ArgoCD Application sync
  5. Runtime resources (Pods, Services)

This gives a complete picture of where a resource is in the delivery pipeline.`,
	Args: cobra.ExactArgs(1),
	RunE: runTrace,
}

type traceHop struct {
	Stage   string `json:"stage"`
	Status  string `json:"status"`
	Details string `json:"details,omitempty"`
}

func runTrace(cmd *cobra.Command, args []string) error {
	name := args[0]
	cfg := config.Get()

	client, err := kube.NewClient(cfg.KubeContext)
	if err != nil {
		return fmt.Errorf("connecting to cluster: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var hops []traceHop

	// Stage 1: Kratix ResourceRequest (VClusterOrchestratorV2)
	vc, vcErr := client.GetVCluster(ctx, cfg.Platform.PlatformNamespace, name)
	if vcErr == nil {
		phase, _, _ := platform.UnstructuredNestedString(vc.Object, "status", "phase")
		if phase == "" {
			phase = "Unknown"
		}
		hops = append(hops, traceHop{
			Stage:   "ResourceRequest",
			Status:  phase,
			Details: fmt.Sprintf("platform.integratn.tech/v1alpha1 VClusterOrchestratorV2 in %s", cfg.Platform.PlatformNamespace),
		})

		// Stage 2: Pipeline Job
		pipelineMsg, _, _ := platform.UnstructuredNestedString(vc.Object, "status", "message")
		conditions, _, _ := platform.UnstructuredNestedSlice(vc.Object, "status", "conditions")
		pipelineStatus := "Unknown"
		if len(conditions) > 0 {
			pipelineStatus = "Completed"
			for _, c := range conditions {
				if cm, ok := c.(map[string]interface{}); ok {
					if s, ok := cm["status"].(string); ok && s != "True" {
						pipelineStatus = "InProgress"
					}
				}
			}
		}
		hop := traceHop{Stage: "Pipeline", Status: pipelineStatus}
		if pipelineMsg != "" {
			hop.Details = pipelineMsg
		}
		hops = append(hops, hop)
	} else {
		hops = append(hops, traceHop{
			Stage:   "ResourceRequest",
			Status:  "NotFound",
			Details: fmt.Sprintf("No VClusterOrchestratorV2 %q in %s — checking ArgoCD directly", name, cfg.Platform.PlatformNamespace),
		})
	}

	// Stage 3: ArgoCD Application
	app, aErr := client.GetArgoApp(ctx, "argocd", name)
	if aErr != nil {
		// Try with common prefixes
		for _, prefix := range []string{"vcluster-", cfg.DefaultCluster + "-"} {
			app, aErr = client.GetArgoApp(ctx, "argocd", prefix+name)
			if aErr == nil {
				break
			}
		}
	}
	if aErr == nil {
		syncStatus, _, _ := platform.UnstructuredNestedString(app.Object, "status", "sync", "status")
		healthStatus, _, _ := platform.UnstructuredNestedString(app.Object, "status", "health", "status")
		revision, _, _ := platform.UnstructuredNestedString(app.Object, "status", "sync", "revision")
		argoStatus := fmt.Sprintf("%s/%s", syncStatus, healthStatus)
		details := ""
		if revision != "" {
			details = "revision=" + revision[:min(7, len(revision))]
		}
		hops = append(hops, traceHop{
			Stage:   "ArgoCD",
			Status:  argoStatus,
			Details: details,
		})

		// Stage 4: Sub-applications (for vClusters)
		subApps, subErr := client.ListArgoAppsForCluster(ctx, "argocd", name)
		if subErr == nil && len(subApps) > 0 {
			synced, healthy := 0, 0
			for _, sa := range subApps {
				if sa.SyncStatus == "Synced" {
					synced++
				}
				if sa.HealthStatus == "Healthy" {
					healthy++
				}
			}
			hops = append(hops, traceHop{
				Stage:   "SubApplications",
				Status:  fmt.Sprintf("%d/%d synced, %d/%d healthy", synced, len(subApps), healthy, len(subApps)),
				Details: fmt.Sprintf("%d total sub-apps", len(subApps)),
			})
		}
	} else {
		hops = append(hops, traceHop{
			Stage:   "ArgoCD",
			Status:  "NotFound",
			Details: "No ArgoCD Application found",
		})
	}

	// Stage 5: Runtime (Pods)
	namespace := name // default assumption: namespace matches name
	pods, podErr := client.ListPods(ctx, namespace, fmt.Sprintf("app.kubernetes.io/name=%s", name))
	if podErr != nil || len(pods) == 0 {
		pods, podErr = client.ListPods(ctx, namespace, "")
	}
	if podErr == nil && len(pods) > 0 {
		running, total := 0, len(pods)
		for _, p := range pods {
			if p.Phase == "Running" && p.ReadyContainers == p.TotalContainers {
				running++
			}
		}
		hops = append(hops, traceHop{
			Stage:   "Runtime",
			Status:  fmt.Sprintf("%d/%d pods ready", running, total),
		})
	}

	// Render output
	if tui.IsStructured() {
		return tui.RenderOutput(map[string]interface{}{
			"resource": name,
			"chain":    hops,
		}, "")
	}

	fmt.Printf("\n  %s\n\n", tui.TitleStyle.Render("Trace: "+name))
	for i, hop := range hops {
		isLast := i == len(hops)-1
		statusIcon := tui.SuccessStyle.Render(tui.IconCheck)

		status := strings.ToLower(hop.Status)
		if strings.Contains(status, "notfound") || strings.Contains(status, "failed") || strings.Contains(status, "degraded") {
			statusIcon = tui.ErrorStyle.Render(tui.IconCross)
		} else if strings.Contains(status, "progress") || strings.Contains(status, "outofsync") || strings.Contains(status, "unknown") {
			statusIcon = tui.WarningStyle.Render(tui.IconPending)
		}

		connector := "├─"
		indent := "│ "
		if isLast {
			connector = "└─"
			indent = "  "
		}

		fmt.Printf("  %s %s %s  %s\n", connector, statusIcon, hop.Stage, tui.InfoStyle.Render(hop.Status))
		if hop.Details != "" {
			fmt.Printf("  %s   %s\n", indent, tui.DimStyle.Render(hop.Details))
		}
	}
	fmt.Println()

	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
