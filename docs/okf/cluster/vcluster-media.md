---
type: Virtual Cluster
title: vcluster-media
description: The one live tenant vcluster — prod-preset HA control plane with external etcd, its own ArgoCD/gateway/DNS, running the media stack.
tags: [vcluster, media, tenant]
resource: https://media.integratn.tech:443
status: stable
generated: { by: claude-code/claude-fable-5, at: 2026-08-20T23:40:00Z }
stale_after: 2026-11-20
sources:
  - id: request
    resource: ../../../platform/vclusters/vcluster-media.yaml
    title: VClusterOrchestratorV2 request
  - id: live-vc
    resource: kubectl get vco,pods,svc -n vcluster-media / platform-requests, 2026-08-20
    title: Live vcluster state
  - id: workloads
    resource: ../../../workloads/vcluster-media/
    title: workloads/vcluster-media (addons.yaml + app values)
---

# Spec (from the ResourceRequest)

Requested via [vcluster-orchestrator-v2](/promises/vcluster-orchestrator-v2.md):
preset **prod** — 3 control-plane replicas (vcluster-oss 0.31.1, K8s v1.34.3
inside), dedicated **3-replica etcd** (registry.k8s.io/etcd:3.6.8-0,
cert-manager-issued certs, client-cert-auth disabled), 2Gi memory, exposed at
`media.integratn.tech:443`, NFS egress enabled plus a postgres egress rule to
`10.0.3.1:5432`.[^request]

# Live footprint on the host (2026-08-20)

- 41 pods in namespace `vcluster-media`: 3 control-plane + 3 etcd
  StatefulSet pods, plus **synced** pods (suffix `-x-<ns>-x-vcluster-media`)
  for everything running *inside* the vcluster.
- LoadBalancer IPs: API `10.0.4.200`, its own nginx-gateway `10.0.4.201`.
- Storage: 3×10Gi control-plane + 3×5Gi etcd PVCs, plus synced app PVCs.

# Inside the vcluster

Its own ArgoCD (with dex), cert-manager, external-dns, external-secrets,
kyverno, nginx-gateway-fabric, prometheus agent + kube-state-metrics
(remote-writing to the hub with `cluster=vcluster-media` labels), CoreDNS ×2 —
all delivered by the [vcluster-role addons](/addons/addon-inventory.md)
rendered on the host. Registered in host ArgoCD as cluster
`vcluster-media` (labels `cluster_role: vcluster`, `environment: production`).

**Workloads** (namespace `media`, defined in
[workloads/vcluster-media/](../../../workloads/vcluster-media/), deployed by
the vcluster's own ArgoCD): **sonarr, radarr, sabnzbd, otterwiki** — each a
Stakater application with config/media PVCs; the *arr apps use the external
Postgres at 10.0.3.1.[^workloads]

**configarr** is the fifth entry and the odd one out: the same Stakater chart,
but rendered as a **CronJob** (`configarr-sync`, 04:00 America/Denver) rather
than a Deployment, with `deployment.enabled` and `service.enabled` both forced
off. It holds no state — no PVC, just `emptyDir` for its git clone cache — and
its whole job is to reconcile Sonarr's and Radarr's **custom formats and
quality profiles** from the TRaSH-Guides via their REST APIs, so that config
lives in git instead of only in each app's database. It reads the two API keys
from the existing `homepage-secret` and needs an egress NetworkPolicy for
:443, because it clones the TRaSH-Guides and recyclarr template repos on every
run. See [configarr](/addons/configarr.md).

# Access

- kubeconfig: ExternalSecret `vcluster-media-kubeconfig` (1Password item of
  the same name), or `hctl vcluster kubeconfig vcluster-media`.
- Status: `kubectl get vco vcluster-media -n platform-requests` — the
  [status contract](/promises/kratix.md) reports Ready/ArgoSynced/PodsReady/
  KubeconfigAvailable. Note: the aggregate currently shows **Failing** purely
  because of a 170-day-old suspended pipeline job — the vcluster itself is
  fully operational. See [known issues](/cluster/known-issues.md).[^live-vc]

[^request]: VClusterOrchestratorV2 request
[^live-vc]: Live vcluster state
[^workloads]: workloads/vcluster-media (addons.yaml + app values)
