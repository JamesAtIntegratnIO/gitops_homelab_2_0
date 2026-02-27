# Platform Status Contract — Implementation Plan

## Problem Statement

When a user creates a `VClusterOrchestratorV2` resource, they currently get:

- A CRD with `phase: Scheduled` set **once** at pipeline execution time
- 3 sub-ResourceRequests generated (ArgoCD project, application, cluster-registration)
- Some ArgoCD apps, some pods — somewhere
- **No unified "is my vCluster ready?" signal**

The pipeline status is **static** — it never updates after initial execution. Answering
"is my vCluster working?" requires manual `kubectl` archaeology across 5+ resource types.

## Goal

Every golden-path resource gets a **self-updating status contract**:

```yaml
status:
  phase: Ready
  endpoints:
    api: https://media.integratn.tech:443
    argocd: https://argocd.cluster.integratn.tech/applications/vcluster-media
  credentials:
    kubeconfigSecret: vcluster-media-kubeconfig
    onePasswordItem: vcluster-media-kubeconfig
  health:
    argocd: Healthy
    workloads: 5/5 Ready
    lastSync: 2m ago
```

No `kubectl` archaeology. Ask the CR, get the answer.

---

## Architecture

```
┌──────────────────────────────────────────────────┐
│         VClusterOrchestratorV2 CR                │
│  .spec   (user intent)                           │
│  .status (contract — updated continuously)       │
└────────────┬─────────────────────────────────────┘
             │ patches .status every 60s
┌────────────┴─────────────────────────────────────┐
│     platform-status-reconciler (Deployment)      │
│                                                  │
│  For each VClusterOrchestratorV2:                │
│    1. Check ArgoCD Application sync + health     │
│    2. Check pod readiness in target namespace    │
│    3. Check Kratix Work / WorkPlacement status   │
│    4. Check kubeconfig secret existence          │
│    5. Compute aggregate phase                    │
│    6. Patch .status on the CR                    │
│    7. Update Prometheus metrics                  │
│                                                  │
│  GET /metrics → Prometheus scrape                │
└──────────────────────────────────────────────────┘
             │
      ┌──────┴──────┐
      │ Prometheus   │──→ RAG pipeline (existing QUERY_CATALOG)
      │ metrics      │──→ Grafana dashboard
      │              │──→ PrometheusRule alerts
      └──────────────┘
             │
      ┌──────┴──────┐
      │ hctl CLI     │──→ reads .status directly via K8s API
      └─────────────┘
```

### Phase Lifecycle State Machine

```
Scheduled          (pipeline wrote status, work not yet created)
    ↓
Progressing        (ArgoCD syncing, pods starting, kubeconfig pending)
    ↓
Ready              (ArgoCD healthy, pods ready, kubeconfig available)
    ↔
Degraded           (some pods restarting, ArgoCD out-of-sync)
    ↓
Failed             (ArgoCD failed, pods crash permanently)

Special states:
  Deleting         (delete pipeline ran)
  Unknown          (reconciler can't determine state)
```

**Transition rules:**
| Current   | Condition                                          | Next        |
|-----------|----------------------------------------------------|-------------|
| Scheduled | Work resource exists                               | Progressing |
| Progressing | ArgoCD Healthy + all pods Ready + kubeconfig exists | Ready       |
| Progressing | > 15 min since creation, still not ready           | Degraded    |
| Ready     | Pods restarting OR ArgoCD OutOfSync                 | Degraded    |
| Degraded  | All healthy again                                  | Ready       |
| Degraded  | ArgoCD Failed OR > 50% pods down for > 10 min      | Failed      |
| Failed    | All healthy again                                  | Ready       |
| *         | Delete pipeline ran                                 | Deleting    |

---

## Status Contract Schema

```yaml
status:
  # Top-level summary
  phase: Ready | Scheduled | Progressing | Degraded | Failed | Deleting | Unknown
  message: "VCluster media is fully operational"
  lastReconciled: "2026-02-26T10:30:00Z"

  # Where to find things (set by pipeline, static)
  endpoints:
    api: "https://media.integratn.tech:443"
    argocd: "https://argocd.cluster.integratn.tech/applications/vcluster-media"

  # How to authenticate (set by pipeline, static)
  credentials:
    kubeconfigSecret: "vcluster-media-kubeconfig"
    onePasswordItem: "vcluster-media-kubeconfig"

  # Live health (updated by reconciler every 60s)
  health:
    argocd:
      syncStatus: Synced       # Synced | OutOfSync | Unknown
      healthStatus: Healthy    # Healthy | Degraded | Progressing | Missing | Unknown
    workloads:
      ready: 5
      total: 5
    subApps:
      healthy: 3
      total: 3
      unhealthy: []            # list of app names that aren't healthy

  # Standard Kubernetes conditions
  conditions:
    - type: Ready
      status: "True"
      lastTransitionTime: "2026-02-26T10:25:00Z"
      reason: AllHealthy
      message: "All components healthy"
    - type: ArgoSynced
      status: "True"
      lastTransitionTime: "2026-02-26T10:20:00Z"
      reason: Synced
    - type: PodsReady
      status: "True"
      lastTransitionTime: "2026-02-26T10:22:00Z"
      reason: AllPodsRunning
    - type: KubeconfigAvailable
      status: "True"
      lastTransitionTime: "2026-02-26T10:24:00Z"
      reason: SecretExists
```

---

## Implementation Phases

### Phase 1: Enhanced Pipeline Status (Quick Win)

**What:** Enrich the static status written by the vcluster-orchestrator-v2 pipeline.

**Files changed:**
- `promises/vcluster-orchestrator-v2/workflows/resource/configure/main.go`

**Changes to `handleConfigure()`:**
```go
// Existing status fields (keep as-is)
status.Set("phase", "Scheduled")
status.Set("message", "VCluster resources scheduled for creation")
// ... existing fields ...

// NEW: endpoint references
status.Set("endpoints", map[string]string{
    "api":    config.ExternalServerURL,
    "argocd": fmt.Sprintf("https://argocd.cluster.integratn.tech/applications/vcluster-%s", config.Name),
})

// NEW: credential references
status.Set("credentials", map[string]string{
    "kubeconfigSecret": fmt.Sprintf("vcluster-%s-kubeconfig", config.Name),
    "onePasswordItem":  config.OnePasswordItem,
})
```

**Build & deploy:**
```bash
cd promises/vcluster-orchestrator-v2/workflows/resource/configure
docker build -t ghcr.io/jamesatintegratnio/vcluster-orchestrator-v2-configure:latest .
docker push ghcr.io/jamesatintegratnio/vcluster-orchestrator-v2-configure:latest
```

**Impact:** Every new or reconciled VClusterOrchestratorV2 immediately shows endpoint URLs and credential references in `.status`. Eliminates the most common question: "where is my vCluster?"

---

### Phase 2: Platform Status Reconciler (Core Feature)

**What:** A lightweight Go Deployment that continuously reconciles `.status` on all VClusterOrchestratorV2 resources.

**New files:**
```
images/platform-status-reconciler/
├── Dockerfile
├── go.mod
├── go.sum
├── main.go           # Entry point: reconcile loop + HTTP metrics server
├── reconciler.go     # Core: walk lifecycle chain, compute phase, patch status
├── metrics.go        # Prometheus metric definitions + update logic
└── README.md

addons/cluster-roles/control-plane/addons/kratix/platform-status-reconciler/
├── deployment.yaml
├── serviceaccount.yaml
├── clusterrole.yaml
├── clusterrolebinding.yaml
├── service.yaml
└── servicemonitor.yaml
```

**Reconciler logic (`reconciler.go`):**

```go
func (r *Reconciler) reconcile(ctx context.Context, vcr unstructured.Unstructured) StatusResult {
    name := vcr.GetName()
    ns := vcr.GetNamespace()
    targetNS := getNestedString(vcr, "spec", "targetNamespace")
    if targetNS == "" { targetNS = ns }

    result := StatusResult{Phase: "Unknown"}

    // 1. Check ArgoCD Application
    argoApp, err := r.getArgoCDApp(ctx, "vcluster-"+name)
    if err != nil {
        result.Health.ArgoCD = ArgoCDHealth{SyncStatus: "Unknown", HealthStatus: "Missing"}
    } else {
        result.Health.ArgoCD = extractArgoCDHealth(argoApp)
    }

    // 2. Check pod readiness in target namespace
    pods, _ := r.clientset.CoreV1().Pods(targetNS).List(ctx, metav1.ListOptions{})
    ready, total := countReadyPods(pods)
    result.Health.Workloads = WorkloadHealth{Ready: ready, Total: total}

    // 3. Check sub-app health (ArgoCD apps targeting the vcluster server)
    subApps := r.getSubApps(ctx, name)
    result.Health.SubApps = aggregateSubAppHealth(subApps)

    // 4. Check kubeconfig secret
    kubeconfigExists := r.secretExists(ctx, targetNS, "vc-"+name)

    // 5. Compute phase
    result.Phase = computePhase(result, vcr, kubeconfigExists)
    result.Message = phaseMessage(result.Phase, name)

    return result
}

func computePhase(result StatusResult, vcr unstructured.Unstructured, kubeconfigExists bool) string {
    currentPhase := getNestedString(vcr, "status", "phase")
    if currentPhase == "Deleting" { return "Deleting" }

    argoHealthy := result.Health.ArgoCD.HealthStatus == "Healthy"
    argoSynced := result.Health.ArgoCD.SyncStatus == "Synced"
    allPodsReady := result.Health.Workloads.Ready == result.Health.Workloads.Total && result.Health.Workloads.Total > 0
    allSubAppsHealthy := result.Health.SubApps.Healthy == result.Health.SubApps.Total && result.Health.SubApps.Total > 0

    if argoHealthy && argoSynced && allPodsReady && allSubAppsHealthy && kubeconfigExists {
        return "Ready"
    }

    if result.Health.ArgoCD.HealthStatus == "Missing" {
        return "Scheduled" // ArgoCD hasn't picked it up yet
    }

    age := time.Since(vcr.GetCreationTimestamp().Time)
    podsDown := result.Health.Workloads.Total > 0 && float64(result.Health.Workloads.Ready)/float64(result.Health.Workloads.Total) < 0.5

    if result.Health.ArgoCD.HealthStatus == "Degraded" || podsDown {
        if age > 15*time.Minute { return "Failed" }
        return "Degraded"
    }

    return "Progressing"
}
```

**Prometheus metrics (`metrics.go`):**

```go
var (
    vclusterPhase = prometheus.NewGaugeVec(prometheus.GaugeOpts{
        Name: "platform_vcluster_phase",
        Help: "Current phase of vcluster (1=active for the labeled phase)",
    }, []string{"name", "namespace", "phase"})

    vclusterReady = prometheus.NewGaugeVec(prometheus.GaugeOpts{
        Name: "platform_vcluster_ready",
        Help: "Whether vcluster is in Ready phase (1=ready, 0=not)",
    }, []string{"name", "namespace"})

    vclusterPodsReady = prometheus.NewGaugeVec(prometheus.GaugeOpts{
        Name: "platform_vcluster_pods_ready",
        Help: "Number of ready pods in vcluster namespace",
    }, []string{"name", "namespace"})

    vclusterPodsTotal = prometheus.NewGaugeVec(prometheus.GaugeOpts{
        Name: "platform_vcluster_pods_total",
        Help: "Total pods in vcluster namespace",
    }, []string{"name", "namespace"})

    vclusterReconcileDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
        Name:    "platform_vcluster_reconcile_duration_seconds",
        Help:    "Time taken to reconcile a single vcluster status",
        Buckets: prometheus.DefBuckets,
    }, []string{"name"})

    reconcileErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
        Name: "platform_status_reconcile_errors_total",
        Help: "Total reconciliation errors by vcluster",
    }, []string{"name"})
)
```

**RBAC (`clusterrole.yaml`):**

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: platform-status-reconciler
rules:
  # Read VClusterOrchestratorV2 resources
  - apiGroups: ["platform.integratn.tech"]
    resources: ["vclusterorchestratorv2s"]
    verbs: ["get", "list", "watch"]
  # Patch status subresource
  - apiGroups: ["platform.integratn.tech"]
    resources: ["vclusterorchestratorv2s/status"]
    verbs: ["patch", "update"]
  # Read ArgoCD Applications
  - apiGroups: ["argoproj.io"]
    resources: ["applications"]
    verbs: ["get", "list"]
  # Read pods + secrets (for readiness + kubeconfig check)
  - apiGroups: [""]
    resources: ["pods", "secrets"]
    verbs: ["get", "list"]
  # Read Kratix Work/WorkPlacement
  - apiGroups: ["platform.kratix.io"]
    resources: ["works", "workplacements"]
    verbs: ["get", "list"]
```

**Deployment resources:**
- CPU request: 10m, limit: 100m
- Memory request: 32Mi, limit: 64Mi
- Single replica
- Liveness: HTTP GET /healthz
- Readiness: HTTP GET /readyz

---

### Phase 3: RAG Pipeline Integration

**What:** Add platform status queries to the RAG pipeline so the LLM can answer "is my vCluster working?" with live data.

**File changed:** `images/git-indexer/rag_pipeline.py`

**New QUERY_CATALOG entries:**

```python
"platform_status": [
    (
        "VCluster readiness",
        "platform_vcluster_ready",
        "platform_ready",          # new format type
    ),
    (
        "VCluster phase",
        'platform_vcluster_phase == 1',
        "platform_phase",          # new format type
    ),
    (
        "VCluster pod health",
        "platform_vcluster_pods_ready / platform_vcluster_pods_total",
        "platform_pod_ratio",      # new format type
    ),
],
```

**New TOPIC_KEYWORDS:**

```python
"vcluster":     {"platform_status", "pods"},
"platform":     {"platform_status", "nodes", "pods"},
"golden path":  {"platform_status"},
"ready":        {"platform_status", "pods"},
```

**New format handlers in `_format_query_result()`:**

```python
if fmt == "platform_ready":
    lines = [f"**{label}**:"]
    for r in result:
        name = r["metric"].get("name", "?")
        ns = r["metric"].get("namespace", "?")
        val = "Ready" if r.get("value", [0, "0"])[1] == "1" else "Not Ready"
        lines.append(f"  - {ns}/{name}: {val}")
    return "\n".join(lines) + "\n"
```

---

### Phase 4: hctl CLI Enhancement

**What:** Make `hctl status` and `hctl diagnose` read and display the `.status` contract.

**Files changed:**
- `cli/internal/platform/diagnose.go`
- `cli/cmd/commands.go`

**Display format (hctl vcluster status media):**

```
┌─ VCluster: media ─────────────────────────
│ Phase:     Ready ✓
│ Message:   VCluster media is fully operational
│ Last Check: 45s ago
│
│ Endpoints:
│   API:     https://media.integratn.tech:443
│   ArgoCD:  https://argocd.cluster.integratn.tech/applications/vcluster-media
│
│ Credentials:
│   Secret:  vcluster-media-kubeconfig
│   1Password: vcluster-media-kubeconfig
│
│ Health:
│   ArgoCD:   Synced / Healthy
│   Pods:     5/5 Ready
│   Sub-Apps: 3/3 Healthy
│
│ Conditions:
│   ✓ Ready              (AllHealthy, 2h ago)
│   ✓ ArgoSynced         (Synced, 2h ago)
│   ✓ PodsReady          (AllPodsRunning, 2h ago)
│   ✓ KubeconfigAvailable (SecretExists, 2h ago)
└────────────────────────────────────────────
```

---

## Implementation Order

| # | Phase | Effort | Impact | Dependencies |
|---|-------|--------|--------|--------------|
| 1 | Enhanced Pipeline Status | ~30 min | Medium | None — immediate static metadata |
| 2 | Status Reconciler | ~3-4 hrs | **High** | Phase 1 writes initial status |
| 3 | RAG Pipeline Integration | ~30 min | Medium | Phase 2 exports Prometheus metrics |
| 4 | hctl CLI Enhancement | ~1 hr | Medium | Phase 2 populates .status |

Phases 1 and 2 are the priority. Phase 1 is a quick win that enriches pipeline output.
Phase 2 is the core feature that makes status **live and self-updating**.

---

## Key Design Decisions

1. **Deployment over CronJob** — needs to serve `/metrics` for Prometheus scrape.
   60s reconcile interval, negligible resource footprint (10m CPU, 32Mi mem).

2. **Dynamic client over typed client** — avoids importing CRD-generated types.
   Uses `unstructured.Unstructured` to read VClusterOrchestratorV2 resources. Standard
   `client-go` for core resources (pods, secrets).

3. **Phase over boolean** — a single `phase` field with 7 possible values gives
   richer information than `ready: true/false`. Follows the Kubernetes convention
   of phase + conditions.

4. **Conditions array** — standard Kubernetes condition pattern. Allows components
   to report independently (ArgoSynced, PodsReady, KubeconfigAvailable).

5. **Prometheus metrics over direct API** — the RAG pipeline already queries
   Prometheus. Exporting status as metrics avoids giving the pipeline pod direct
   Kubernetes API access and reuses existing infrastructure.

6. **Status subresource patching** — the CRD already has `x-kubernetes-preserve-unknown-fields: true`
   on `.status`. The reconciler uses strategic-merge-patch on `/status` subresource
   to avoid conflicting with Kratix's pipeline status writes.

7. **No CRD schema change needed** — `x-kubernetes-preserve-unknown-fields: true`
   means we can add any fields to `.status` without modifying the CRD. The schema
   is the contract documented above, enforced by convention.

---

## Alerts (Phase 2 deliverable)

```yaml
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: platform-status-alerts
  namespace: monitoring
spec:
  groups:
    - name: platform.vcluster
      rules:
        - alert: VClusterNotReady
          expr: platform_vcluster_ready == 0
          for: 15m
          labels:
            severity: warning
          annotations:
            summary: "VCluster {{ $labels.name }} not Ready for 15 minutes"

        - alert: VClusterPodsUnhealthy
          expr: |
            platform_vcluster_pods_ready / platform_vcluster_pods_total < 0.5
            and platform_vcluster_pods_total > 0
          for: 10m
          labels:
            severity: warning
          annotations:
            summary: "VCluster {{ $labels.name }} has <50% pods ready"

        - alert: PlatformReconcilerDown
          expr: absent(up{job="platform-status-reconciler"} == 1)
          for: 5m
          labels:
            severity: warning
          annotations:
            summary: "Platform status reconciler is not running"
```

---

## Success Criteria

After implementation, a user should be able to:

```bash
# Single command to see vCluster status
kubectl get vclusterorchestratorv2 media -o jsonpath='{.status.phase}'
# → Ready

# Full status contract
kubectl get vclusterorchestratorv2 media -o yaml | yq '.status'
# → complete status contract as documented above

# hctl
hctl vcluster status media
# → formatted status dashboard

# Ask the AI
"Is my media vCluster working?"
# → "Yes, vcluster media is Ready. API endpoint: https://media.integratn.tech:443,
#    all 5/5 pods running, ArgoCD synced and healthy."

# Prometheus alert if broken
# → VClusterNotReady fires after 15 minutes of non-Ready state
```
