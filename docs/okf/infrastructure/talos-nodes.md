---
type: Infrastructure
title: Talos control-plane nodes
description: The three bare-metal Talos Linux nodes that form the-cluster, their machine-config patches, and how they are managed.
tags: [talos, bare-metal, nodes, control-plane]
status: stable
generated: { by: claude-code/claude-fable-5, at: 2026-08-20T23:40:00Z }
stale_after: 2027-02-20
sources:
  - id: node-observation
    resource: kubectl get nodes -o wide on the-cluster (admin@the-cluster), 2026-08-20
    title: Live node listing
  - id: machineconfigs
    resource: ../../../matchbox/talos-machineconfigs/all.yaml
    title: Talos machine-config patches (all.yaml, cp.yaml, work.yaml)
  - id: bootstrap-doc
    resource: ../../bootstrap.md
    title: Bootstrap & Bare-Metal Talos guide
---

# Node inventory

Three identical bare-metal machines run Talos Linux and form both the control
plane and the worker pool (`allowSchedulingOnControlPlanes: true` — there are
no dedicated workers today).[^node-observation]

| Node name       | IP         | CPU | Memory | Max pods | Role          |
|-----------------|------------|-----|--------|----------|---------------|
| `talos-xez-xys` | 10.0.4.101 | 4   | ~32 Gi | 110      | control-plane |
| `talos-4kg-ymz` | 10.0.4.102 | 4   | ~32 Gi | 110      | control-plane |
| `talos-1jv-u7d` | 10.0.4.103 | 4   | ~32 Gi | 110      | control-plane |

As of 2026-08-20: Talos **v1.11.5**, Kubernetes **v1.34.1**, kernel
6.12.57-talos, containerd 2.1.5. Nodes have been up ~221 days (provisioned
mid-January 2026). Utilization sits around 28–34% CPU and 33–47% memory per
node.[^node-observation]

The Kubernetes API is reached through a Talos-managed shared **VIP
`10.0.4.100`** (this is the `server:` in the local kubeconfig context
`admin@the-cluster`). Kubernetes version is pinned to the Talos release; only
vclusters can run different Kubernetes versions.

# Machine configuration

Machine configs are *generated* from patch fragments in
[matchbox/talos-machineconfigs/](../../../matchbox/talos-machineconfigs/)
via `talosctl gen config the-cluster https://10.0.4.100 --config-patch @all.yaml
--config-patch-control-plane @cp.yaml --config-patch-worker @work.yaml`.
Only the patches are committed — the rendered configs embed cluster CA keys and
bootstrap tokens and are gitignored.[^machineconfigs]

Key decisions encoded in the patches:

- **Install disk by label** — `/dev/disk/by-label/TALOS_INSTALL` with
  `wipe: true`, so device-name reordering can never wipe the wrong disk.
- **Static addressing via deviceSelector** — `deviceSelector.physical: true`
  with addresses in the `10.0.0.0/9` supernet (note: `cp.yaml` uses `/9`
  while docs/bootstrap.md shows `/24` — the patch is authoritative).
- **CNI none + kube-proxy disabled** — Cilium replaces both (see
  [networking](/platform/networking.md)).
- **Metrics exposure** — controller-manager/scheduler bind `0.0.0.0`, etcd
  serves metrics on `:2381`, for Prometheus scraping.
- **Kernel hardening boot args** — `init_on_alloc=1`, `init_on_free=1`,
  `slab_nomerge`, `pti=on` (set in the Matchbox boot profiles).

# Management

- No SSH; all node management goes through `talosctl` (available in the
  [Nix dev shell](/tooling/nix-dev-shell.md)). The `talosconfig` credential file
  is generated locally and never committed; it is not present at
  `~/.talos/config` on this workstation.
- Upgrades: `talosctl upgrade-k8s` for in-place Kubernetes upgrades, or new
  Matchbox boot profiles + reboot for Talos version upgrades. Runbooks live in
  [docs/bootstrap.md](../../bootstrap.md) and [docs/operations.md](../../operations.md).
- Nodes were PXE-booted via [Matchbox](/infrastructure/matchbox-pxe.md).

[^node-observation]: Live node listing
[^machineconfigs]: Talos machine-config patches (all.yaml, cp.yaml, work.yaml)
