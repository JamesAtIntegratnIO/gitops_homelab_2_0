---
type: Platform Design
title: Resilience — how the cluster heals itself
description: The seven layers that make the-cluster self-healing, what each one covers, and the failure modes that remain outside git's reach.
tags: [resilience, self-healing, availability, scheduling, disruption]
status: stable
generated: { by: claude-code/claude-fable-5, at: 2026-08-21T08:00:00Z }
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

Self-healing here is seven mechanisms stacked, not one feature. Each covers a
different failure and they fail independently.

| Layer | Mechanism | Covers |
|---|---|---|
| Node / OS | Talos auto-reboot on kernel panic; [hardware watchdog](/infrastructure/talos-nodes.md) (2m); 3-member etcd; API VIP `10.0.4.100` with Cilium on KubePrism | node hang, node loss, apiserver loss |
| Network | Cilium kube-proxy replacement; MetalLB speaker on every node; 2 NGINX data planes spread across hosts | VIP failover, ingress pod loss |
| Scheduling | `platform-critical` / `platform-batch` PriorityClasses; topology spread on gateway + Kyverno admission; 60s failover tolerations | capacity shortfall, slow rescheduling |
| Disruption | PodDisruptionBudgets on every component running ≥2 replicas, CoreDNS included | drains and rolling updates |
| GitOps | ArgoCD `selfHeal` + `prune` + unlimited retry on **all 47** rendered Applications; 60s reconciliation; `kratix-state-reconciler` | config drift, manual edits, failed syncs |
| Hygiene | Kyverno `ClusterCleanupPolicy` for suspended Jobs, long-failed Jobs, terminal Pods; descheduler every 30m | leftovers that look like failure, post-recovery imbalance, restart loops |
| Observability | see [observability](/platform/observability.md) | detection |

# The one rule

Automation may **restart and delete** ephemeral objects. Anything that changes
what *should* exist goes through a pull request. This is why the Kyverno cleanup
controller is granted delete on Pods and Jobs only — both recreatable by their
owner — and why there is no auto-remediation that edits desired state.

# Two properties worth not breaking

**Self-heal is the default, not per-addon.** The addon `syncPolicy` deep-merges
over the chart default rather than replacing it, so an addon that overrides only
part of the policy keeps the rest. Before that, 16 of 47 Applications had
silently lost `selfHeal` and ~30 had no retry — including cert-manager, Kyverno,
MetalLB and the gateway. See [addon system](/addons/addon-system.md).

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

# What is still outside git's reach

- **Upstream DNS.** CoreDNS forwards to the Talos host resolver at
  `169.254.116.108`. When that stalls, CoreDNS saturates and *cluster-internal*
  resolution fails with it — the documented cause of the vcluster-media restart
  storms. Talos owns the CoreDNS Deployment and Corefile as bootstrap manifests
  and re-applies them, so only a PodDisruptionBudget could be added from git.
  See [known issues](/cluster/known-issues.md) and the Talos runbook.[^talos-cmds]
- **etcd snapshots.** The Talos API access that enables them is committed as a
  machine-config patch but not applied; nothing backs etcd up today.
- **Rebuild from git.** Blocked by two fresh-clone bugs (`secrets.env`,
  `dockerconfig.json`). The DR path in [operations](../../operations.md) has
  never been exercised.
- **NFS server loss.** Single point of failure for every PV, by accepted design
  — see [storage](/platform/storage.md).

[^gameday]: docs/game-day.md
[^talos-cmds]: Talos machine-config runbook
[^live-sweep]: Live observation, 2026-08-21
