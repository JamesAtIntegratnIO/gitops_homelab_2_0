package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jamesatintegratnio/hctl/internal/kube"
	"github.com/jamesatintegratnio/hctl/internal/platform"
	"github.com/jamesatintegratnio/hctl/internal/tui"
	unstr "github.com/jamesatintegratnio/hctl/internal/unstructured"
)

// loadNodesSection fetches cluster nodes and renders a table.
func loadNodesSection(client *kube.Client) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	nodes, err := client.ListNodes(ctx)
	if err != nil {
		return "", err
	}
	var rows [][]string
	for _, n := range nodes {
		status := tui.StatusIcon(n.Ready)
		rows = append(rows, []string{n.Name, status, n.IP, strings.Join(n.Roles, ","), n.CPU, n.Memory})
	}
	return tui.Table([]string{"NAME", "READY", "IP", "ROLES", "CPU", "MEMORY"}, rows), nil
}

// loadArgoCDSection fetches ArgoCD applications and renders a summary with unhealthy apps.
func loadArgoCDSection(client *kube.Client) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	apps, err := client.ListArgoApps(ctx, "argocd")
	if err != nil {
		return "", err
	}
	synced, outOfSync, degraded, healthy := 0, 0, 0, 0
	var unhealthyRows [][]string
	for _, app := range apps {
		syncStatus := unstr.MustString(app.Object, "status", "sync", "status")
		healthStatus := unstr.MustString(app.Object, "status", "health", "status")
		if syncStatus == "Synced" {
			synced++
		} else {
			outOfSync++
		}
		if healthStatus == "Healthy" {
			healthy++
		} else if healthStatus == "Degraded" {
			degraded++
		}
		if syncStatus != "Synced" || healthStatus != "Healthy" {
			unhealthyRows = append(unhealthyRows, []string{
				app.GetName(),
				syncStatus,
				healthStatus,
			})
		}
	}

	var sb strings.Builder
	summary := fmt.Sprintf("  Total: %d  │  Synced: %s  │  OutOfSync: %s  │  Healthy: %s  │  Degraded: %s\n",
		len(apps),
		tui.SuccessStyle.Render(fmt.Sprintf("%d", synced)),
		tui.WarningStyle.Render(fmt.Sprintf("%d", outOfSync)),
		tui.SuccessStyle.Render(fmt.Sprintf("%d", healthy)),
		tui.ErrorStyle.Render(fmt.Sprintf("%d", degraded)),
	)
	sb.WriteString(summary)

	if len(unhealthyRows) > 0 {
		sb.WriteString("\n" + tui.WarningStyle.Render("  Unhealthy Applications:") + "\n")
		sb.WriteString(tui.Table([]string{"NAME", "SYNC", "HEALTH"}, unhealthyRows))
	}
	return sb.String(), nil
}

// loadPromisesSection fetches Kratix promises and renders their availability.
func loadPromisesSection(client *kube.Client) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	promises, err := client.ListPromises(ctx)
	if err != nil {
		return "", err
	}
	var rows [][]string
	for _, p := range promises {
		status := "Unknown"
		conditions := unstr.MustSlice(p.Object, "status", "conditions")
		for _, c := range conditions {
			if cm, ok := c.(map[string]interface{}); ok {
				if cm["type"] == "Available" {
					if cm["status"] == "True" {
						status = tui.SuccessStyle.Render("Available")
					} else {
						status = tui.ErrorStyle.Render("Unavailable")
					}
				}
			}
		}
		rows = append(rows, []string{p.GetName(), status})
	}
	return tui.Table([]string{"PROMISE", "STATUS"}, rows), nil
}

// loadVClustersSection fetches vclusters and their ArgoCD health status.
func loadVClustersSection(client *kube.Client, platformNS string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	vclusters, err := client.ListVClusters(ctx, platformNS)
	if err != nil {
		return "", err
	}
	if len(vclusters) == 0 {
		return tui.DimStyle.Render("  (no vclusters)"), nil
	}
	var rows [][]string
	for _, vc := range vclusters {
		name := vc.GetName()
		preset := unstr.MustString(vc.Object, "spec", "vcluster", "preset")
		hostname := unstr.MustString(vc.Object, "spec", "exposure", "hostname")

		argoApp, err := client.GetArgoApp(ctx, "argocd", name)
		health := tui.DimStyle.Render("unknown")
		if err == nil {
			syncStatus := unstr.MustString(argoApp.Object, "status", "sync", "status")
			healthStatus := unstr.MustString(argoApp.Object, "status", "health", "status")
			if syncStatus == "Synced" && healthStatus == "Healthy" {
				health = tui.SuccessStyle.Render("Healthy")
			} else {
				health = tui.WarningStyle.Render(syncStatus + "/" + healthStatus)
			}
		}
		rows = append(rows, []string{name, preset, hostname, health})
	}
	return tui.Table([]string{"NAME", "PRESET", "HOSTNAME", "STATUS"}, rows), nil
}

// loadWorkloadsSection fetches deployed workloads and renders their status.
func loadWorkloadsSection(client *kube.Client, platformNS string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ps, err := platform.CollectPlatformStatus(ctx, client, platformNS)
	if err != nil {
		return "", err
	}
	if len(ps.Workloads) == 0 {
		return tui.DimStyle.Render("  (no workloads deployed)"), nil
	}

	var rows [][]string
	for _, w := range ps.Workloads {
		cluster := w.Labels["clusterName"]
		phase := tui.PhaseBadge(w.Phase)
		rows = append(rows, []string{w.Name, cluster, w.Namespace, w.ArgoCD.SyncStatus, phase})
	}
	return tui.Table([]string{"NAME", "CLUSTER", "NAMESPACE", "SYNC", "STATUS"}, rows), nil
}

// loadAddonsSection fetches addons grouped by environment and renders them.
func loadAddonsSection(client *kube.Client, platformNS string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ps, err := platform.CollectPlatformStatus(ctx, client, platformNS)
	if err != nil {
		return "", err
	}
	if len(ps.Addons) == 0 {
		return tui.DimStyle.Render("  (no addons)"), nil
	}

	// Group by environment
	envGroups := make(map[string][]platform.ResourceStatus)
	for _, a := range ps.Addons {
		env := a.Labels["environment"]
		if env == "" {
			env = "(unset)"
		}
		envGroups[env] = append(envGroups[env], a)
	}

	var sb strings.Builder
	for env, addons := range envGroups {
		sb.WriteString(fmt.Sprintf("\n  %s\n", tui.TitleStyle.Render(env)))
		var rows [][]string
		for _, a := range addons {
			phase := tui.PhaseBadge(a.Phase)
			rows = append(rows, []string{a.Name, a.Namespace, a.ArgoCD.SyncStatus, phase})
		}
		sb.WriteString(tui.Table([]string{"NAME", "NAMESPACE", "SYNC", "STATUS"}, rows))
	}
	return sb.String(), nil
}
