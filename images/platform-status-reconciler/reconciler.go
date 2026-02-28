package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

// GVR definitions
var (
	vclusterGVR = schema.GroupVersionResource{
		Group:    "platform.integratn.tech",
		Version:  "v1alpha1",
		Resource: "vclusterorchestratorv2s",
	}

	argoAppGVR = schema.GroupVersionResource{
		Group:    "argoproj.io",
		Version:  "v1alpha1",
		Resource: "applications",
	}

	kratixWorkGVR = schema.GroupVersionResource{
		Group:    "platform.kratix.io",
		Version:  "v1alpha1",
		Resource: "works",
	}
)

// Reconciler continuously reconciles .status on VClusterOrchestratorV2 CRs.
type Reconciler struct {
	clientset kubernetes.Interface
	dynClient dynamic.Interface
}

// NewReconciler creates a reconciler with the given clients.
func NewReconciler(clientset kubernetes.Interface, dynClient dynamic.Interface) *Reconciler {
	return &Reconciler{
		clientset: clientset,
		dynClient: dynClient,
	}
}

// ReconcileAll lists all VClusterOrchestratorV2 resources and reconciles each.
func (r *Reconciler) ReconcileAll(ctx context.Context) {
	reconcileTotal.Inc()
	log.Println("Starting reconcile cycle")

	list, err := r.dynClient.Resource(vclusterGVR).Namespace("").List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Printf("ERROR: Failed to list VClusterOrchestratorV2: %v", err)
		return
	}

	log.Printf("Found %d VClusterOrchestratorV2 resources", len(list.Items))

	// Collect vcluster names for workload/addon classification
	var vclusterNames []string

	for i := range list.Items {
		vcr := &list.Items[i]
		name := vcr.GetName()
		ns := vcr.GetNamespace()
		vclusterNames = append(vclusterNames, name)

		start := time.Now()
		result, err := r.reconcileOne(ctx, vcr)
		duration := time.Since(start).Seconds()
		reconcileDuration.WithLabelValues(name).Observe(duration)

		if err != nil {
			log.Printf("ERROR: Reconcile %s/%s failed: %v", ns, name, err)
			reconcileErrors.WithLabelValues(name).Inc()
			continue
		}

		// Update Prometheus metrics
		updateMetrics(name, ns, result)

		// Patch .status on the CR
		if err := r.patchStatus(ctx, vcr, result); err != nil {
			log.Printf("ERROR: Failed to patch status for %s/%s: %v", ns, name, err)
			reconcileErrors.WithLabelValues(name).Inc()
			continue
		}

		log.Printf("Reconciled %s/%s: phase=%s pods=%d/%d argocd=%s/%s",
			ns, name, result.Phase,
			result.Health.Workloads.Ready, result.Health.Workloads.Total,
			result.Health.ArgoCD.SyncStatus, result.Health.ArgoCD.HealthStatus)
	}

	// Reconcile workload and addon ArgoCD Applications
	r.ReconcileWorkloads(ctx, vclusterNames)
	r.ReconcileAddons(ctx, vclusterNames)

	log.Println("Reconcile cycle complete")
}

// reconcileOne gathers health data and computes status for a single VClusterOrchestratorV2 resource.
func (r *Reconciler) reconcileOne(ctx context.Context, vcr *unstructured.Unstructured) (*StatusResult, error) {
	name := vcr.GetName()
	ns := vcr.GetNamespace()

	// Determine target namespace from spec or fall back to CR namespace
	targetNS, _, _ := unstructured.NestedString(vcr.Object, "spec", "targetNamespace")
	if targetNS == "" {
		targetNS = ns
	}

	result := &StatusResult{
		Phase:          "Unknown",
		LastReconciled: time.Now().UTC().Format(time.RFC3339),
	}

	// Preserve static fields set by pipeline (endpoints, credentials)
	if endpoints, found, _ := unstructured.NestedStringMap(vcr.Object, "status", "endpoints"); found {
		result.Endpoints = Endpoints{
			API:    endpoints["api"],
			ArgoCD: endpoints["argocd"],
		}
	}
	if creds, found, _ := unstructured.NestedStringMap(vcr.Object, "status", "credentials"); found {
		result.Credentials = Credentials{
			KubeconfigSecret: creds["kubeconfigSecret"],
			OnePasswordItem:  creds["onePasswordItem"],
		}
	}

	// 1. Check ArgoCD Application for the vcluster
	argoAppName := fmt.Sprintf("vcluster-%s", name)
	argoHealth := r.checkArgoCDApp(ctx, argoAppName, "argocd")
	result.Health.ArgoCD = argoHealth

	// 2. Check pod readiness in the target namespace
	result.Health.Workloads = r.checkPodReadiness(ctx, targetNS)

	// 3. Check sub-apps (ArgoCD apps registered to the vcluster's server)
	result.Health.SubApps = r.checkSubApps(ctx, name)

	// 4. Check kubeconfig secret existence
	kubeconfigExists := r.secretExists(ctx, targetNS, fmt.Sprintf("vc-%s", name))

	// 5. Compute phase from all health signals
	result.Phase = computePhase(result, vcr, kubeconfigExists)
	result.Message = phaseMessage(result.Phase, name)

	// 6. Build conditions
	result.Conditions = buildConditions(result, kubeconfigExists)

	return result, nil
}

// checkArgoCDApp retrieves health/sync status from an ArgoCD Application.
func (r *Reconciler) checkArgoCDApp(ctx context.Context, name, namespace string) ArgoCDHealth {
	app, err := r.dynClient.Resource(argoAppGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return ArgoCDHealth{SyncStatus: "Unknown", HealthStatus: "Missing"}
	}

	syncStatus, _, _ := unstructured.NestedString(app.Object, "status", "sync", "status")
	healthStatus, _, _ := unstructured.NestedString(app.Object, "status", "health", "status")

	if syncStatus == "" {
		syncStatus = "Unknown"
	}
	if healthStatus == "" {
		healthStatus = "Unknown"
	}

	return ArgoCDHealth{
		SyncStatus:   syncStatus,
		HealthStatus: healthStatus,
	}
}

// checkPodReadiness counts ready/total pods in a namespace.
func (r *Reconciler) checkPodReadiness(ctx context.Context, namespace string) WorkloadHealth {
	pods, err := r.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Printf("WARN: Failed to list pods in %s: %v", namespace, err)
		return WorkloadHealth{}
	}

	total := 0
	ready := 0
	for _, pod := range pods.Items {
		// Skip completed/evicted pods
		if pod.Status.Phase == "Succeeded" || pod.Status.Phase == "Failed" {
			continue
		}
		total++
		podReady := false
		for _, cond := range pod.Status.Conditions {
			if cond.Type == "Ready" && cond.Status == "True" {
				podReady = true
				break
			}
		}
		if podReady {
			ready++
		}
	}

	return WorkloadHealth{Ready: ready, Total: total}
}

// checkSubApps finds ArgoCD Applications that reference the vcluster's server and aggregates their health.
func (r *Reconciler) checkSubApps(ctx context.Context, vclusterName string) SubAppHealth {
	// Sub-apps are ArgoCD apps whose destination server matches the vcluster's external URL
	// Convention: vcluster apps have labels or destination matching the vcluster name
	apps, err := r.dynClient.Resource(argoAppGVR).Namespace("argocd").List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("argocd.argoproj.io/instance=vcluster-%s", vclusterName),
	})
	if err != nil || apps == nil {
		// Fall back: look for apps whose server URL contains the vcluster name
		return r.checkSubAppsByServerURL(ctx, vclusterName)
	}

	if len(apps.Items) == 0 {
		return r.checkSubAppsByServerURL(ctx, vclusterName)
	}

	return aggregateAppHealth(apps.Items)
}

// checkSubAppsByServerURL finds sub-apps by matching destination server URL pattern.
func (r *Reconciler) checkSubAppsByServerURL(ctx context.Context, vclusterName string) SubAppHealth {
	allApps, err := r.dynClient.Resource(argoAppGVR).Namespace("argocd").List(ctx, metav1.ListOptions{})
	if err != nil {
		return SubAppHealth{}
	}

	var matchingApps []unstructured.Unstructured
	for _, app := range allApps.Items {
		server, _, _ := unstructured.NestedString(app.Object, "spec", "destination", "server")
		// Match by vcluster server URL pattern: https://<name>.<domain>
		appName := app.GetName()
		if server != "" && (contains(server, vclusterName+".") || contains(server, "vcluster-"+vclusterName)) {
			// Don't include the parent vcluster app itself
			if appName != fmt.Sprintf("vcluster-%s", vclusterName) {
				matchingApps = append(matchingApps, app)
			}
		}
	}

	return aggregateAppHealth(matchingApps)
}

// aggregateAppHealth computes health summary from a list of ArgoCD applications.
func aggregateAppHealth(apps []unstructured.Unstructured) SubAppHealth {
	result := SubAppHealth{Total: len(apps)}
	for _, app := range apps {
		healthStatus, _, _ := unstructured.NestedString(app.Object, "status", "health", "status")
		if healthStatus == "Healthy" {
			result.Healthy++
		} else {
			result.Unhealthy = append(result.Unhealthy, app.GetName())
		}
	}
	return result
}

// secretExists checks if a Kubernetes secret exists.
func (r *Reconciler) secretExists(ctx context.Context, namespace, name string) bool {
	_, err := r.clientset.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	return err == nil
}

// computePhase determines the aggregate phase from all health signals.
func computePhase(result *StatusResult, vcr *unstructured.Unstructured, kubeconfigExists bool) string {
	// Check if currently in Deleting state
	currentPhase, _, _ := unstructured.NestedString(vcr.Object, "status", "phase")
	if currentPhase == "Deleting" {
		return "Deleting"
	}

	argoHealthy := result.Health.ArgoCD.HealthStatus == "Healthy"
	argoSynced := result.Health.ArgoCD.SyncStatus == "Synced"
	allPodsReady := result.Health.Workloads.Ready == result.Health.Workloads.Total && result.Health.Workloads.Total > 0

	// Sub-apps are optional — if none exist, that's fine
	subAppsOK := result.Health.SubApps.Total == 0 ||
		(result.Health.SubApps.Healthy == result.Health.SubApps.Total)

	// Fully healthy
	if argoHealthy && argoSynced && allPodsReady && subAppsOK && kubeconfigExists {
		return "Ready"
	}

	// ArgoCD hasn't picked it up yet
	if result.Health.ArgoCD.HealthStatus == "Missing" {
		return "Scheduled"
	}

	// Calculate age for timeout-based transitions
	age := time.Since(vcr.GetCreationTimestamp().Time)

	// Check for degradation signals
	podsDown := result.Health.Workloads.Total > 0 &&
		float64(result.Health.Workloads.Ready)/float64(result.Health.Workloads.Total) < 0.5
	argoFailed := result.Health.ArgoCD.HealthStatus == "Degraded"

	if argoFailed || podsDown {
		if age > 15*time.Minute {
			return "Failed"
		}
		return "Degraded"
	}

	// Not fully ready but not degraded — still progressing
	return "Progressing"
}

// phaseMessage returns a human-readable message for the phase.
func phaseMessage(phase, name string) string {
	switch phase {
	case "Ready":
		return fmt.Sprintf("VCluster %s is fully operational", name)
	case "Scheduled":
		return fmt.Sprintf("VCluster %s resources have been scheduled, waiting for ArgoCD to sync", name)
	case "Progressing":
		return fmt.Sprintf("VCluster %s is being provisioned", name)
	case "Degraded":
		return fmt.Sprintf("VCluster %s is running but some components are unhealthy", name)
	case "Failed":
		return fmt.Sprintf("VCluster %s has failed — components are unhealthy for an extended period", name)
	case "Deleting":
		return fmt.Sprintf("VCluster %s is being deleted", name)
	default:
		return fmt.Sprintf("VCluster %s is in an unknown state", name)
	}
}

// buildConditions creates standard Kubernetes-style conditions from the health status.
func buildConditions(result *StatusResult, kubeconfigExists bool) []Condition {
	conditions := []Condition{}

	// Ready condition (aggregate)
	if result.Phase == "Ready" {
		conditions = append(conditions, NewCondition("Ready", "True", "AllHealthy", "All components healthy"))
	} else {
		conditions = append(conditions, NewCondition("Ready", "False", result.Phase, result.Message))
	}

	// ArgoSynced condition
	if result.Health.ArgoCD.SyncStatus == "Synced" {
		conditions = append(conditions, NewCondition("ArgoSynced", "True", "Synced", "ArgoCD application is synced"))
	} else {
		conditions = append(conditions, NewCondition("ArgoSynced", "False", result.Health.ArgoCD.SyncStatus,
			fmt.Sprintf("ArgoCD sync status: %s", result.Health.ArgoCD.SyncStatus)))
	}

	// PodsReady condition
	if result.Health.Workloads.Ready == result.Health.Workloads.Total && result.Health.Workloads.Total > 0 {
		conditions = append(conditions, NewCondition("PodsReady", "True", "AllPodsRunning",
			fmt.Sprintf("All %d pods are ready", result.Health.Workloads.Total)))
	} else {
		conditions = append(conditions, NewCondition("PodsReady", "False", "PodsNotReady",
			fmt.Sprintf("%d/%d pods ready", result.Health.Workloads.Ready, result.Health.Workloads.Total)))
	}

	// KubeconfigAvailable condition
	if kubeconfigExists {
		conditions = append(conditions, NewCondition("KubeconfigAvailable", "True", "SecretExists", "Kubeconfig secret is available"))
	} else {
		conditions = append(conditions, NewCondition("KubeconfigAvailable", "False", "SecretMissing", "Kubeconfig secret not found"))
	}

	return conditions
}

// patchStatus applies a strategic merge patch to the CR's .status subresource.
func (r *Reconciler) patchStatus(ctx context.Context, vcr *unstructured.Unstructured, result *StatusResult) error {
	// Build status patch preserving existing status fields from the pipeline
	statusMap := map[string]interface{}{
		"phase":          result.Phase,
		"message":        result.Message,
		"lastReconciled": result.LastReconciled,
	}

	// Endpoints (preserve from pipeline if reconciler didn't find them)
	if result.Endpoints.API != "" || result.Endpoints.ArgoCD != "" {
		statusMap["endpoints"] = map[string]interface{}{
			"api":    result.Endpoints.API,
			"argocd": result.Endpoints.ArgoCD,
		}
	}

	// Credentials (preserve from pipeline)
	if result.Credentials.KubeconfigSecret != "" || result.Credentials.OnePasswordItem != "" {
		statusMap["credentials"] = map[string]interface{}{
			"kubeconfigSecret": result.Credentials.KubeconfigSecret,
			"onePasswordItem":  result.Credentials.OnePasswordItem,
		}
	}

	// Health (always updated by reconciler)
	statusMap["health"] = map[string]interface{}{
		"argocd": map[string]interface{}{
			"syncStatus":   result.Health.ArgoCD.SyncStatus,
			"healthStatus": result.Health.ArgoCD.HealthStatus,
		},
		"workloads": map[string]interface{}{
			"ready": result.Health.Workloads.Ready,
			"total": result.Health.Workloads.Total,
		},
		"subApps": map[string]interface{}{
			"healthy":   result.Health.SubApps.Healthy,
			"total":     result.Health.SubApps.Total,
			"unhealthy": result.Health.SubApps.Unhealthy,
		},
	}

	// Conditions
	condList := []interface{}{}
	for _, c := range result.Conditions {
		condList = append(condList, map[string]interface{}{
			"type":               c.Type,
			"status":             c.Status,
			"reason":             c.Reason,
			"message":            c.Message,
			"lastTransitionTime": c.LastTransitionTime,
		})
	}
	statusMap["conditions"] = condList

	patch := map[string]interface{}{
		"status": statusMap,
	}

	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("failed to marshal status patch: %w", err)
	}

	_, err = r.dynClient.Resource(vclusterGVR).Namespace(vcr.GetNamespace()).Patch(
		ctx,
		vcr.GetName(),
		types.MergePatchType,
		patchBytes,
		metav1.PatchOptions{},
		"status",
	)
	if err != nil {
		return fmt.Errorf("failed to patch status: %w", err)
	}

	return nil
}

// contains checks if s contains substr.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
