---
type: Platform Design
title: Backups — the recovery layer
description: Nightly etcd snapshots of the-cluster and vcluster-media onto the Unraid share, what they do and do not cover, how they are watched, and how to restore from them.
tags: [backup, recovery, etcd, disaster-recovery, resilience]
status: stable
generated: { by: claude-code/claude-fable-5, at: 2026-08-21T21:30:00Z }
stale_after: 2027-02-21
sources:
  - id: backups-addon
    resource: ../../../addons/cluster-roles/control-plane/addons/platform-backups/vcluster-media-etcd-snapshot.yaml
    title: platform-backups addon
  - id: talos-addon
    resource: ../../../addons/cluster-roles/control-plane/addons/talos-etcd-backup/talos-etcd-snapshot.yaml
    title: talos-etcd-backup addon
  - id: restore-runbook
    resource: ../../operations.md
    title: docs/operations.md — Backup and Recovery
  - id: live-sweep
    resource: kubectl + Prometheus sweep of the-cluster, 2026-08-21
    title: Live observation, 2026-08-21
---

# Why this layer exists

The seven layers in [resilience](/platform/resilience.md) are all about
*healing* — restarting, rescheduling, re-syncing. None of them is about
*recovery*. Before 2026-08-21 there was no copy of either cluster's etcd
anywhere, and the [game day](../../game-day.md) said so plainly: etcd loss
meant the 2–3 hour rebuild, and the rebuild loses everything git does not
hold — every PV/PVC binding, Kratix request status, ArgoCD's cluster
registrations, cert-manager's issued certificates, and for the tenant, the
whole of vcluster-media.

Velero was deliberately skipped by the production-readiness plan: PV data
lives on NFS and survives a rebuild on its own ([storage](/platform/storage.md)).
What did not survive was the *state that points at it*. That is what etcd
snapshots cover.

# What runs

| CronJob | Namespace | Schedule (UTC) | Takes | Writes to | Status |
|---|---|---|---|---|---|
| `vcluster-media-etcd-snapshot` | `vcluster-media` | 03:45 daily | `etcdctl snapshot save` against `vcluster-media-etcd:2379` | `10.0.0.12:/mnt/user/kube_storage/backups/vcluster-media-etcd/` | **live** (addon `platform-backups`)[^backups-addon] |
| `talos-etcd-snapshot` | `talos-backup` | 03:15 daily | `talosctl etcd snapshot` on the pod's own node | `…/backups/the-cluster-etcd/` | **disabled** — needs the Talos patch (addon `platform-backups-talos`)[^talos-addon] |

**First run verified 2026-08-22T01:34Z** (manual Job from the CronJob, right
after #68 merged): 33 MB snapshot fetched in 0.8 s, written as
`vcluster-media-etcd-2026-08-22T0134Z.db`, owned by uid 99 — the export
root-squashes, which the Job tolerates because it only ever writes into its
own directory.

Both keep **14 days** of nightlies and prune older files in the same run.
Both mount the NFS export directly (an inline `nfs:` volume, not a PVC) so the
target path is fixed and no object in etcd is needed to reach it — the backup
must not depend on the thing it backs up.

## Tenant etcd: why `etcdctl` and not `vcluster snapshot`

vcluster 0.31 has a native `vcluster snapshot create`, but in that release the
request is processed by a controller *inside the vcluster pod* and its
`container://` target is the vcluster's own data PVC — the NFS volume whose
loss is the scenario being covered. The etcd-native snapshot is the one that
lands somewhere else. It uses the same PKI the etcd pods mount
(`vcluster-media-certs`, the `etcd-healthcheck-client` pair), so if etcd trusts
its own health check it trusts this.

## Host etcd: the one gate

`talosctl etcd snapshot` from inside a pod needs
`machine.features.kubernetesTalosAPIAccess`. The gen-time fragment is
[`all.yaml`](../../../matchbox/talos-machineconfigs/all.yaml), but on a running
node it must be applied as a JSON patch of just that key — the fragment itself
duplicates list items under `patch mc` (see the runbook). A talosconfig now
exists on James's Mac; the apply is a human step because the agent permission
layer blocks live machine-config writes. Until
it is applied the `talos.dev/v1alpha1 ServiceAccount` kind does not exist, so
the addon ships `enabled: false`; an Application that cannot create one of its
resources never reaches Synced. After the patch:

```bash
kubectl get crd serviceaccounts.talos.dev   # exists → flip platform-backups-talos to enabled: true
```

# What is watched

`platform.backups` in the [platform-alerts](/addons/addon-system.md) rules, all
`warning` (digest, not page):

| Alert | Fires when |
|---|---|
| `EtcdSnapshotStale` | no successful run of a `*-etcd-snapshot` CronJob in 36h |
| `EtcdSnapshotNeverSucceeded` | the CronJob exists but has never completed (36h) |
| `HostEtcdBackupMissing` | the `talos-etcd-snapshot` CronJob is not deployed at all — the standing nag until the Talos patch lands |

A run that fails outright is already upstream `KubeJobFailed`.

# What this does not cover

- **PV contents.** Still NFS-only, by design. A snapshot restores the PVC
  *objects*; the directories they point at have to still be on the share.
- **The NFS server itself.** Single point of failure for the backups too —
  they live on the same Unraid box as the data. Off-site (an OCI registry or
  S3 target) is the obvious next step and needs a credential in 1Password.
- **Two nodes at once.** A host etcd snapshot turns that from "rebuild and
  re-attach everything by hand" into "rebuild and `bootstrap --recover-from`";
  it does not make it fast.
- **A restore that has been rehearsed.** Neither procedure in
  [operations](../../operations.md) has been run end to end yet. The first
  restore drill — against a throwaway vcluster — is the real acceptance test.

# Restore

The procedures live in [docs/operations.md → Backup and Recovery](../../operations.md),
one per cluster. In one line each:

- **vcluster-media**: scale the vcluster and its etcd to zero, run one
  `etcdutl snapshot restore` Job per member into its existing PVC, scale back.
- **the-cluster**: wipe etcd on all three nodes, `talosctl bootstrap
  --recover-from=<snapshot>` on one, let the others rejoin.

[^backups-addon]: platform-backups addon
[^talos-addon]: talos-etcd-backup addon
[^restore-runbook]: docs/operations.md — Backup and Recovery
[^live-sweep]: Live observation, 2026-08-21
