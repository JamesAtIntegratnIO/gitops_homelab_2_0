package main

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	vclusterPhase = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "platform",
		Subsystem: "vcluster",
		Name:      "phase_info",
		Help:      "Current phase of vcluster (1=active for the labeled phase)",
	}, []string{"name", "namespace", "phase"})

	vclusterReady = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "platform",
		Subsystem: "vcluster",
		Name:      "ready",
		Help:      "Whether vcluster is in Ready phase (1=ready, 0=not)",
	}, []string{"name", "namespace"})

	vclusterPodsReady = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "platform",
		Subsystem: "vcluster",
		Name:      "pods_ready",
		Help:      "Number of ready pods in vcluster namespace",
	}, []string{"name", "namespace"})

	vclusterPodsTotal = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "platform",
		Subsystem: "vcluster",
		Name:      "pods_total",
		Help:      "Total pods in vcluster namespace",
	}, []string{"name", "namespace"})

	vclusterArgoSynced = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "platform",
		Subsystem: "vcluster",
		Name:      "argocd_synced",
		Help:      "Whether ArgoCD app is synced (1=synced, 0=not)",
	}, []string{"name", "namespace"})

	vclusterArgoHealthy = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "platform",
		Subsystem: "vcluster",
		Name:      "argocd_healthy",
		Help:      "Whether ArgoCD app is healthy (1=healthy, 0=not)",
	}, []string{"name", "namespace"})

	vclusterSubAppsHealthy = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "platform",
		Subsystem: "vcluster",
		Name:      "subapps_healthy",
		Help:      "Number of healthy sub-apps for vcluster",
	}, []string{"name", "namespace"})

	vclusterSubAppsTotal = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "platform",
		Subsystem: "vcluster",
		Name:      "subapps_total",
		Help:      "Total sub-apps for vcluster",
	}, []string{"name", "namespace"})

	reconcileDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "platform",
		Subsystem: "status_reconciler",
		Name:      "reconcile_duration_seconds",
		Help:      "Time taken to reconcile a single vcluster status",
		Buckets:   prometheus.DefBuckets,
	}, []string{"name"})

	reconcileErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "platform",
		Subsystem: "status_reconciler",
		Name:      "errors_total",
		Help:      "Total reconciliation errors by vcluster",
	}, []string{"name"})

	reconcileTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "platform",
		Subsystem: "status_reconciler",
		Name:      "reconciles_total",
		Help:      "Total number of reconcile cycles completed",
	})

	// --- Workload metrics ---

	workloadPhase = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "platform",
		Subsystem: "workload",
		Name:      "phase_info",
		Help:      "Current phase of a workload ArgoCD app (1=active for the labeled phase)",
	}, []string{"name", "cluster", "namespace", "phase"})

	workloadArgoSynced = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "platform",
		Subsystem: "workload",
		Name:      "argocd_synced",
		Help:      "Whether workload ArgoCD app is synced (1=synced, 0=not)",
	}, []string{"name", "cluster", "namespace"})

	workloadArgoHealthy = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "platform",
		Subsystem: "workload",
		Name:      "argocd_healthy",
		Help:      "Whether workload ArgoCD app is healthy (1=healthy, 0=not)",
	}, []string{"name", "cluster", "namespace"})

	// --- Addon metrics ---

	addonPhase = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "platform",
		Subsystem: "addon",
		Name:      "phase_info",
		Help:      "Current phase of an addon ArgoCD app (1=active for the labeled phase)",
	}, []string{"name", "cluster", "environment", "namespace", "phase"})

	addonArgoSynced = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "platform",
		Subsystem: "addon",
		Name:      "argocd_synced",
		Help:      "Whether addon ArgoCD app is synced (1=synced, 0=not)",
	}, []string{"name", "cluster", "environment", "namespace"})

	addonArgoHealthy = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "platform",
		Subsystem: "addon",
		Name:      "argocd_healthy",
		Help:      "Whether addon ArgoCD app is healthy (1=healthy, 0=not)",
	}, []string{"name", "cluster", "environment", "namespace"})
)

// RegisterMetrics registers all Prometheus metrics.
func RegisterMetrics() {
	prometheus.MustRegister(
		vclusterPhase,
		vclusterReady,
		vclusterPodsReady,
		vclusterPodsTotal,
		vclusterArgoSynced,
		vclusterArgoHealthy,
		vclusterSubAppsHealthy,
		vclusterSubAppsTotal,
		reconcileDuration,
		reconcileErrors,
		reconcileTotal,
		workloadPhase,
		workloadArgoSynced,
		workloadArgoHealthy,
		addonPhase,
		addonArgoSynced,
		addonArgoHealthy,
	)
}

// allPhases used for resetting phase gauge (only one phase should be 1 at a time).
var allPhases = []string{"Scheduled", "Progressing", "Ready", "Degraded", "Failed", "Deleting", "Unknown"}

// updateMetrics sets Prometheus gauges for a reconciled vcluster.
func updateMetrics(name, namespace string, result *StatusResult) {
	// Phase — set active phase to 1, all others to 0
	for _, p := range allPhases {
		val := float64(0)
		if p == result.Phase {
			val = 1
		}
		vclusterPhase.WithLabelValues(name, namespace, p).Set(val)
	}

	// Ready boolean
	ready := float64(0)
	if result.Phase == "Ready" {
		ready = 1
	}
	vclusterReady.WithLabelValues(name, namespace).Set(ready)

	// Pod counts
	vclusterPodsReady.WithLabelValues(name, namespace).Set(float64(result.Health.Workloads.Ready))
	vclusterPodsTotal.WithLabelValues(name, namespace).Set(float64(result.Health.Workloads.Total))

	// ArgoCD status
	synced := float64(0)
	if result.Health.ArgoCD.SyncStatus == "Synced" {
		synced = 1
	}
	vclusterArgoSynced.WithLabelValues(name, namespace).Set(synced)

	healthy := float64(0)
	if result.Health.ArgoCD.HealthStatus == "Healthy" {
		healthy = 1
	}
	vclusterArgoHealthy.WithLabelValues(name, namespace).Set(healthy)

	// Sub-app counts
	vclusterSubAppsHealthy.WithLabelValues(name, namespace).Set(float64(result.Health.SubApps.Healthy))
	vclusterSubAppsTotal.WithLabelValues(name, namespace).Set(float64(result.Health.SubApps.Total))
}

// allArgoPhases used for resetting workload/addon phase gauges.
var allArgoPhases = []string{"Ready", "Progressing", "Degraded", "Suspended", "Unknown"}

// updateWorkloadMetrics sets Prometheus gauges for a workload ArgoCD app.
func updateWorkloadMetrics(status ArgoAppStatus) {
	name := status.AddonName
	if name == "" {
		name = status.Name
	}
	cluster := status.ClusterName
	ns := status.Namespace

	for _, p := range allArgoPhases {
		val := float64(0)
		if p == status.Phase {
			val = 1
		}
		workloadPhase.WithLabelValues(name, cluster, ns, p).Set(val)
	}

	synced := float64(0)
	if status.SyncStatus == "Synced" {
		synced = 1
	}
	workloadArgoSynced.WithLabelValues(name, cluster, ns).Set(synced)

	healthy := float64(0)
	if status.HealthStatus == "Healthy" {
		healthy = 1
	}
	workloadArgoHealthy.WithLabelValues(name, cluster, ns).Set(healthy)
}

// updateAddonMetrics sets Prometheus gauges for an addon ArgoCD app.
func updateAddonMetrics(status ArgoAppStatus) {
	name := status.AddonName
	if name == "" {
		name = status.Name
	}
	cluster := status.ClusterName
	env := status.Environment
	ns := status.Namespace

	for _, p := range allArgoPhases {
		val := float64(0)
		if p == status.Phase {
			val = 1
		}
		addonPhase.WithLabelValues(name, cluster, env, ns, p).Set(val)
	}

	synced := float64(0)
	if status.SyncStatus == "Synced" {
		synced = 1
	}
	addonArgoSynced.WithLabelValues(name, cluster, env, ns).Set(synced)

	healthy := float64(0)
	if status.HealthStatus == "Healthy" {
		healthy = 1
	}
	addonArgoHealthy.WithLabelValues(name, cluster, env, ns).Set(healthy)
}
