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
