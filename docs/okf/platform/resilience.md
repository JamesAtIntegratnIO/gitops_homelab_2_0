---
type: Platform Design
title: Resilience — how the cluster heals itself
description: The seven layers that make the-cluster self-healing, what each one covers, and the failure modes that remain outside git's reach.
tags: [resilience, self-healing, availability, scheduling, disruption]
status: stable
generated: { by: claude-code/claude-opus-5, at: 2026-08-30T17:52:00Z }
stale_after: 2027-02-21
sources:
  - id: gameday
    resource: ../../game-day.md
    title: docs/game-day.md
  - id: talos-cmds
    resource: ../../../matchbox/talos-machineconfigs/commands.md
    title: Talos machine-config runbook
  - id: live-sweep
    resource: kubectl + Loki sweep of the-cluster, 2026-08-21
    title: Live observation, 2026-08-21
---

# The layers

Self-healing here is seven mechanisms stacked, not one feature, plus an eighth
that is about recovery rather than healing. Each covers a different failure
and they fail independently.

| Layer | Mechanism | Covers |
|---|---|---|
| Node / OS | Talos auto-reboot on kernel panic; [hardware watchdog](/infrastructure/talos-nodes.md) (2m); 3-member etcd; API VIP `10.0.4.100` with Cilium on KubePrism | node hang, node loss, apiserver loss |
| Network | Cilium kube-proxy replacement; MetalLB speaker on every node; 2 NGINX data planes spread across hosts | VIP failover, ingress pod loss |
| Scheduling | `platform-critical` / `platform-batch` PriorityClasses; topology spread on gateway + Kyverno admission; 60s failover tolerations — per-workload on the recovery-path addons and, since 2026-08-22, the apiserver default for every new host pod (vcluster-synced pods still get 300s from the vcluster's own apiserver) | capacity shortfall, slow rescheduling |
| Disruption | PodDisruptionBudgets on every component running ≥2 replicas, CoreDNS and the vcluster-media etcd included | drains and rolling updates |
| GitOps | ArgoCD `selfHeal` + `prune` + unlimited retry on **all 50** rendered Applications; 60s reconciliation; `kratix-state-reconciler` | config drift, manual edits, failed syncs |
| Hygiene | Kyverno `ClusterCleanupPolicy` for suspended Jobs, long-failed Jobs, terminal Pods; descheduler every 30m | leftovers that look like failure, post-recovery imbalance, restart loops |
| Observability | see [observability](/platform/observability.md) | detection |
| Recovery | nightly etcd snapshots of both clusters to the Unraid share — see [backups](/platform/backups.md) | etcd loss, tenant corruption, the rebuild losing PV bindings |

# The one rule

Automation may **restart and delete** ephemeral objects. Anything that changes
what *should* exist goes through a pull request. This is why the Kyverno cleanup
controller is granted delete on Pods and Jobs only — both recreatable by their
owner — and why there is no auto-remediation that edits desired state.

# Two properties worth not breaking

**Self-heal is the default, not per-addon.** The addon `syncPolicy` deep-merges
over the chart default rather than replacing it, so an addon that overrides only
part of the policy keeps the rest. Before that, 16 of 48 Applications had
silently lost `selfHeal` and 36 had no retry — including cert-manager, Kyverno,
MetalLB and the gateway. See [addon system](/addons/addon-system.md).

**PriorityClasses must exist in every cluster that references them.** They are
cluster-scoped, and a vcluster has its own set. Three addons that set
`priorityClassName` also render into vclusters, so `platform-scheduling`
deliberately targets vclusters too. Note that vcluster's syncer does **not**
propagate `priorityClassName` to the host pod it creates, so inside a vcluster
the field satisfies admission but does not affect host scheduling — the classes
are there so the Deployments can roll, not because they change placement.

**PDBs only where replicas ≥ 2.** `minAvailable: 1` against a single replica
blocks every node drain forever, and would deadlock any future automated Talos
upgrade. cert-manager, external-dns, Kratix and Redis deliberately have none.

# N+1 is a number, and it drifts

Total CPU requests must fit on any two of the three nodes. As of 2026-08-21,
after right-sizing from the VPA recommendations: **7199m of requests against
~7940m of two-node capacity** — +741m of headroom, where it had been −819m (a
node loss left ~800m of pods unschedulable).

This is the check most likely to rot as workloads are added, which is why it is
the first step of the [game day](../../game-day.md).[^gameday]

**There is an alert for this, and as of 2026-08-21 it is no longer muted.**
`KubeCPUOvercommit` compares
`namespace_cpu:kube_pod_container_resource_requests:sum` against
`sum(allocatable) - max(allocatable)` — precisely the N+1 arithmetic above. On
2026-08-21 it was **firing** (8.399 against a 7.9 threshold), and the alerting
consolidation of the same day routed it to `null` on the reasoning that
"cannot tolerate node failure" was the intended steady state. That was true when
it was written. After the right-sizing it is not: requests land near 6.84, the
alert stops firing, and it becomes an accurate early warning that N+1 has
regressed. It was un-muted on 2026-08-21 and now lands in the warning digest;
do not re-mute it — fix the requests.

(The `10.4` figure in that mute's comment is `sum(kube_pod_container_resource_requests)`,
which counts init containers separately; the alert itself uses the recording
rule, which was 8.399.)

# What is still outside git's reach

- **Upstream DNS.** CoreDNS forwards to the Talos host resolver at
  `169.254.116.108`. When that stalls, CoreDNS saturates and *cluster-internal*
  resolution fails with it — the documented cause of the vcluster-media restart
  storms. The ownership half is nearly closed: the Corefile has been git-owned
  since 2026-08-30 (`coredns-host-config`), and on the same day ArgoCD adopted
  the Deployment, Service, SA and RBAC too (`coredns-workload`). Talos still
  renders `11-core-dns` / `11-core-dns-svc` and re-applies them on every boot
  and machine config change until `cluster.coreDNS.disabled: true` goes on all
  three nodes, which is a manual `talosctl patch mc`. The upstream itself —
  `forwardKubeDNSToHost` — is untouched and remains the single point of failure.
  See [known issues](/cluster/known-issues.md) and the Talos runbook.[^talos-cmds]
- **Host etcd snapshots** — no longer outside git's reach: the Talos API access
  patch was applied 2026-08-22 and `platform-backups-talos` is enabled. See
  [backups](/platform/backups.md).
- **Rebuild from git.** The two fresh-clone bugs (`secrets.env`,
  `dockerconfig.json`) are fixed, but the DR path in
  [operations](../../operations.md) has still never been exercised end to end.
- **NFS server loss.** Single point of failure for every PV, by accepted design
  — see [storage](/platform/storage.md).

[^gameday]: docs/game-day.md
[^talos-cmds]: Talos machine-config runbook
[^live-sweep]: Live observation, 2026-08-21
