package platform

import (
	"context"
	"fmt"
	"time"

	"github.com/jamesatintegratnio/hctl/internal/kube"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// CollectPlatformStatus gathers status for all vclusters, workloads, and addons.
func CollectPlatformStatus(ctx context.Context, client *kube.Client, platformNS string) (*PlatformStatus, error) {
	ps := &PlatformStatus{}

	// vClusters from CRs
	vclusters, err := CollectVClusterStatus(ctx, client, platformNS)
	if err == nil {
		ps.VClusters = vclusters
	}

	// All ArgoCD apps (single query, classify by labels)
	apps, err := client.ListArgoApps(ctx, "argocd")
	if err != nil {
		return ps, fmt.Errorf("listing argocd apps: %w", err)
	}

	// Build a set of known vcluster names for workload classification
	vclusterNames := make(map[string]bool)
	for _, vc := range ps.VClusters {
		vclusterNames[vc.Name] = true
	}

	for _, app := range apps {
		labels := app.GetLabels()
		if labels["addon"] != "true" {
			continue
		}

		rs := argoAppToResourceStatus(app)
		clusterName := labels["clusterName"]

		// Classify: if clusterName matches a known vcluster → workload, else → addon
		if vclusterNames[clusterName] {
			rs.Kind = KindWorkload
			ps.Workloads = append(ps.Workloads, rs)
		} else {
			rs.Kind = KindAddon
			ps.Addons = append(ps.Addons, rs)
		}
	}

	return ps, nil
}

// CollectVClusterStatus gathers status for all VClusterOrchestratorV2 CRs.
func CollectVClusterStatus(ctx context.Context, client *kube.Client, namespace string) ([]ResourceStatus, error) {
	vclusters, err := client.ListVClusters(ctx, namespace)
	if err != nil {
		return nil, err
	}

	var result []ResourceStatus
	now := time.Now()

	for _, vc := range vclusters {
		name := vc.GetName()
		rs := ResourceStatus{
			Kind:        KindVCluster,
			Name:        name,
			Namespace:   vc.GetNamespace(),
			LastChecked: now,
		}

		// Try to read from status contract (set by reconciler)
		phase, _, _ := unstructured.NestedString(vc.Object, "status", "phase")
		message, _, _ := unstructured.NestedString(vc.Object, "status", "message")
		lastReconciled, _, _ := unstructured.NestedString(vc.Object, "status", "lastReconciled")

		if phase != "" {
			rs.Phase = phase
			rs.Message = message
			if t, err := time.Parse(time.RFC3339, lastReconciled); err == nil {
				rs.LastChecked = t
			}
		}

		// Read endpoints from status
		if endpoints, found, _ := unstructured.NestedStringMap(vc.Object, "status", "endpoints"); found {
			if api := endpoints["api"]; api != "" {
				rs.Endpoints = append(rs.Endpoints, Endpoint{Name: "API", URL: api})
			}
			if argocd := endpoints["argocd"]; argocd != "" {
				rs.Endpoints = append(rs.Endpoints, Endpoint{Name: "ArgoCD", URL: argocd})
			}
		}

		// Read ArgoCD health from status
		syncStatus, _, _ := unstructured.NestedString(vc.Object, "status", "health", "argocd", "syncStatus")
		healthStatus, _, _ := unstructured.NestedString(vc.Object, "status", "health", "argocd", "healthStatus")
		rs.ArgoCD = ArgoCDInfo{
			AppName:      fmt.Sprintf("vcluster-%s", name),
			SyncStatus:   syncStatus,
			HealthStatus: healthStatus,
		}

		// Read pod info
		podsReady, _, _ := unstructured.NestedInt64(vc.Object, "status", "health", "workloads", "ready")
		podsTotal, _, _ := unstructured.NestedInt64(vc.Object, "status", "health", "workloads", "total")
		rs.Pods = PodInfo{
			Ready: int(podsReady),
			Total: int(podsTotal),
		}

		// If no reconciler phase, derive from spec
		if rs.Phase == "" {
			preset, _, _ := unstructured.NestedString(vc.Object, "spec", "vcluster", "preset")
			rs.Phase = "Unknown"
			rs.Message = fmt.Sprintf("preset=%s (reconciler not reporting)", preset)
		}

		result = append(result, rs)
	}

	return result, nil
}

// CollectWorkloadStatus gathers status for workload ArgoCD apps targeting vclusters.
func CollectWorkloadStatus(ctx context.Context, client *kube.Client, vclusterName string) ([]ResourceStatus, error) {
	apps, err := client.Dynamic.Resource(kube.ArgoCDApplicationGVR).Namespace("argocd").List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("addon=true,clusterName=%s", vclusterName),
	})
	if err != nil {
		return nil, fmt.Errorf("listing workload apps: %w", err)
	}

	var result []ResourceStatus
	for _, app := range apps.Items {
		rs := argoAppToResourceStatus(app)
		rs.Kind = KindWorkload
		result = append(result, rs)
	}
	return result, nil
}

// CollectAddonStatus gathers status for infrastructure addon ArgoCD apps.
func CollectAddonStatus(ctx context.Context, client *kube.Client, clusterName string) ([]ResourceStatus, error) {
	selector := "addon=true"
	if clusterName != "" {
		selector += ",clusterName=" + clusterName
	}
	apps, err := client.Dynamic.Resource(kube.ArgoCDApplicationGVR).Namespace("argocd").List(ctx, metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		return nil, fmt.Errorf("listing addon apps: %w", err)
	}

	var result []ResourceStatus
	for _, app := range apps.Items {
		rs := argoAppToResourceStatus(app)
		rs.Kind = KindAddon
		result = append(result, rs)
	}
	return result, nil
}

// argoAppToResourceStatus converts an ArgoCD Application unstructured object to ResourceStatus.
func argoAppToResourceStatus(app unstructured.Unstructured) ResourceStatus {
	syncStatus, _, _ := unstructured.NestedString(app.Object, "status", "sync", "status")
	healthStatus, _, _ := unstructured.NestedString(app.Object, "status", "health", "status")

	if syncStatus == "" {
		syncStatus = "Unknown"
	}
	if healthStatus == "" {
		healthStatus = "Unknown"
	}

	labels := app.GetLabels()
	project, _, _ := unstructured.NestedString(app.Object, "spec", "project")
	destNS, _, _ := unstructured.NestedString(app.Object, "spec", "destination", "namespace")

	return ResourceStatus{
		Name:      labels["addonName"],
		Namespace: destNS,
		Phase:     PhaseFromArgoCD(syncStatus, healthStatus),
		ArgoCD: ArgoCDInfo{
			AppName:      app.GetName(),
			SyncStatus:   syncStatus,
			HealthStatus: healthStatus,
			Project:      project,
		},
		Labels:      labels,
		LastChecked: time.Now(),
	}
}
