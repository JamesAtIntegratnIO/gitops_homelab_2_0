package main

import (
	"context"
	"fmt"
	"log"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ArgoAppStatus holds the extracted status of an ArgoCD Application.
type ArgoAppStatus struct {
	Name         string `json:"name"`
	AddonName    string `json:"addonName"`
	ClusterName  string `json:"clusterName"`
	Environment  string `json:"environment"`
	Namespace    string `json:"namespace"`
	SyncStatus   string `json:"syncStatus"`
	HealthStatus string `json:"healthStatus"`
	Phase        string `json:"phase"`
}

// ReconcileWorkloads discovers ArgoCD Applications targeting vClusters (workloads)
// and emits Prometheus metrics for each.
func (r *Reconciler) ReconcileWorkloads(ctx context.Context, vclusterNames []string) {
	log.Println("Reconciling workloads")

	for _, vcName := range vclusterNames {
		apps := r.listAddonApps(ctx, fmt.Sprintf("addon=true,clusterName=%s", vcName))
		for _, app := range apps {
			status := extractAppStatus(app)
			updateWorkloadMetrics(status)
		}
		log.Printf("  workloads for %s: %d apps", vcName, len(apps))
	}
}

// ReconcileAddons discovers infrastructure addon ArgoCD Applications
// (those targeting the host cluster, not vClusters) and emits Prometheus metrics.
func (r *Reconciler) ReconcileAddons(ctx context.Context, vclusterNames []string) {
	log.Println("Reconciling addons")

	// Build set of vcluster names for exclusion
	vcSet := make(map[string]bool, len(vclusterNames))
	for _, n := range vclusterNames {
		vcSet[n] = true
	}

	apps := r.listAddonApps(ctx, "addon=true")
	addonCount := 0
	for _, app := range apps {
		status := extractAppStatus(app)
		// Skip apps targeting vClusters — those are workloads, not addons
		if vcSet[status.ClusterName] {
			continue
		}
		updateAddonMetrics(status)
		addonCount++
	}
	log.Printf("  infrastructure addons: %d apps", addonCount)
}

// listAddonApps lists ArgoCD Applications matching a label selector.
func (r *Reconciler) listAddonApps(ctx context.Context, labelSelector string) []unstructured.Unstructured {
	list, err := r.dynClient.Resource(argoAppGVR).Namespace("argocd").List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		log.Printf("WARN: listing ArgoCD apps with selector %q: %v", labelSelector, err)
		return nil
	}
	return list.Items
}

// extractAppStatus reads sync/health/labels from an ArgoCD Application.
func extractAppStatus(app unstructured.Unstructured) ArgoAppStatus {
	labels := app.GetLabels()
	syncStatus, _, _ := unstructured.NestedString(app.Object, "status", "sync", "status")
	healthStatus, _, _ := unstructured.NestedString(app.Object, "status", "health", "status")
	destNS, _, _ := unstructured.NestedString(app.Object, "spec", "destination", "namespace")

	if syncStatus == "" {
		syncStatus = "Unknown"
	}
	if healthStatus == "" {
		healthStatus = "Unknown"
	}

	return ArgoAppStatus{
		Name:         app.GetName(),
		AddonName:    labels["addonName"],
		ClusterName:  labels["clusterName"],
		Environment:  labels["environment"],
		Namespace:    destNS,
		SyncStatus:   syncStatus,
		HealthStatus: healthStatus,
		Phase:        phaseFromArgoCD(syncStatus, healthStatus),
	}
}

// phaseFromArgoCD derives a simple phase from ArgoCD sync+health.
func phaseFromArgoCD(syncStatus, healthStatus string) string {
	switch {
	case syncStatus == "Synced" && healthStatus == "Healthy":
		return "Ready"
	case healthStatus == "Degraded":
		return "Degraded"
	case healthStatus == "Missing" || syncStatus == "Unknown":
		return "Unknown"
	case healthStatus == "Progressing" || syncStatus == "OutOfSync":
		return "Progressing"
	case healthStatus == "Suspended":
		return "Suspended"
	default:
		return "Progressing"
	}
}
