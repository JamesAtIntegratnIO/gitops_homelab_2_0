---
type: Storage Architecture
title: Storage — NFS-backed dynamic provisioning
description: Two NFS storage classes off the Unraid server, who uses them, and the limits of this design.
tags: [storage, nfs, pvc, unraid]
status: stable
generated: { by: claude-code/claude-fable-5, at: 2026-08-20T23:40:00Z }
stale_after: 2027-02-20
sources:
  - id: live-storage
    resource: kubectl get sc,pvc,pv -A on the-cluster, 2026-08-20
    title: Live storage state
  - id: nfs-addon
    resource: ../../../addons/environments/production/addons/addons.yaml
    title: nfs-subdir-external-provisioner addon (+ StorageClasses in additionalResources)
---

# Storage classes

Backed by the Unraid NFS server `10.0.0.12:/mnt/user/kube_storage`, via
nfs-subdir-external-provisioner v4.0.2 (three provisioner Deployments run in
`nfs-provisioner`):[^live-storage]

| StorageClass | Provisioner | Purpose |
|---|---|---|
| `config-nfs-client` **(default)** | `k8s-sigs.io/config-nfs-subdir-external-provisioner` | App config / state (SSD-backed share) |
| `data-nfs-client` | `k8s-sigs.io/data-nfs-subdir-external-provisioner` | Bulk data (HDD-backed share) |

Reclaim `Delete`, binding `Immediate`, **no volume expansion**. (Older docs
mention a third generic `nfs-client` class; it no longer exists — the plain
`nfs-subdir-external-provisioner` Deployment still runs but backs no class.)

# Who uses it (23 PVs, 2026-08-20)

- **Monitoring**: Prometheus 30Gi, Loki 50Gi, Grafana 5Gi, Alertmanager 5Gi.
- **AI**: open-webui 2Gi, qdrant 5Gi, and two 50Gi `llmkube-model-cache`
  claims (one in `default`, one in `llmkube-system` — leftovers of the
  disabled llmkube addon; see [known issues](/cluster/known-issues.md)).
- **Kratix**: 1Gi RWX git storage.
- **Authentik**: redis 2Gi.
- **vcluster-media**: 3×10Gi control plane + 3×5Gi etcd, plus synced PVCs for
  radarr/sonarr/sabnzbd/otterwiki config & media (vcluster storage classes are
  the host classes synced through, named
  `vcluster-…-x-vcluster-media-x-vcluster-media`).

# Design limits (accepted trade-offs)

- NFS server is a single point of failure — but PV data survives cluster
  rebuilds, which is why the production-readiness plan **skipped Velero**:
  git + NFS + Talos configs are the recovery story.
- Not suitable for heavy-write databases; media apps use an external Postgres
  (`10.0.3.1`) instead.
- After an NFS server reboot, stale handles are recovered with
  [hack/restart-nfs-pods.sh](../../../hack/restart-nfs-pods.sh), which
  discovers NFS-backed PVs and rollout-restarts their owning controllers.

[^live-storage]: Live storage state
[^nfs-addon]: nfs-subdir-external-provisioner addon (+ StorageClasses in additionalResources)
