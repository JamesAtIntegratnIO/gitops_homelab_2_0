---
type: Infrastructure
title: Talos nodes
description: The three bare-metal Talos control-plane nodes and the Raspberry Pi 5 and Pi 4 arm64 workers that form the-cluster, their machine-config patches, and how they are managed.
tags: [talos, bare-metal, nodes, control-plane, worker, arm64, raspberry-pi]
status: stable
generated: { by: claude-code, at: 2026-11-08T17:30:00Z }
stale_after: 2027-02-20
sources:
  - id: pi4-nodes
    resource: kubectl get nodes -o wide, Prometheus up{} for 10.0.4.105/106, talosctl get disks on both Pi 4s, 2026-11-08
    title: Raspberry Pi 4 workers join
  - id: pi4-runbook
    resource: ../../../matchbox/talos-machineconfigs/rpi4-worker.md
    title: Raspberry Pi 4 worker runbook and patch
  - id: pi5-node
    resource: kubectl get node talos-pi5-1 -o wide and talosctl get volumestatus,disks on 10.0.4.104, 2026-11-07
    title: Raspberry Pi 5 worker join
  - id: pi5-runbook
    resource: ../../../matchbox/talos-machineconfigs/rpi5-worker.md
    title: Raspberry Pi 5 worker runbook and patch
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
plane and the main worker pool (`allowSchedulingOnControlPlanes: true`).[^node-observation]
Since 2026-11-07 a Raspberry Pi 5 is the first dedicated worker and the first
arm64 node. It is tainted `node.integratn.tech/arch=arm64:NoSchedule`, so only
workloads that tolerate it run there (cilium, cilium-envoy, node-exporter,
promtail, loki-canary).[^pi5-node] Since 2026-11-08 two Raspberry Pi 4s
(8 GB) are arm64 workers too, with the same taint.[^pi4-nodes]

| Node name       | IP         | CPU | Memory | Max pods | Role          |
|-----------------|------------|-----|--------|----------|---------------|
| `talos-xez-xys` | 10.0.4.101 | 4   | ~32 Gi | 110      | control-plane |
| `talos-4kg-ymz` | 10.0.4.102 | 4   | ~32 Gi | 110      | control-plane |
| `talos-1jv-u7d` | 10.0.4.103 | 4   | ~32 Gi | 110      | control-plane |
| `talos-pi5-1`   | 10.0.4.104 | 4   | 16 Gi  | 110      | worker (arm64) |
| `talos-pi4-1`   | 10.0.4.105 | 4   | 8 Gi   | 110      | worker (arm64) |
| `talos-pi4-2`   | 10.0.4.106 | 4   | 8 Gi   | 110      | worker (arm64) |

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

# Raspberry Pi 5 worker

`talos-pi5-1` boots a community Talos v1.13.12 build (yama6a/talos-raspberry-pi5,
built on holocron) from its own Samsung 990 PRO 1 TB NVMe, not a Matchbox PXE
boot: the official Image Factory `rpi_5` overlay ships a U-Boot that cannot read
NVMe or USB on a Pi 5. EPHEMERAL is capped at 100 GiB and the rest of the NVMe is
the `local-nvme` user volume at `/var/mnt/local-nvme`. Its NIC is `end0`, so
Cilium's `devices` lists it beside `eno1`; MetalLB's L2Advertisement stays
`eno1`-only so the Pi never announces a VIP. Upgrades need its installer in a
registry the node can reach, which does not exist yet. Full runbook:
[rpi5-worker.md](../../../matchbox/talos-machineconfigs/rpi5-worker.md).[^pi5-runbook]

Its kubelet, node-exporter and hubble targets were down from the day it
joined (TargetDown). Nothing was wrong on the node: Prometheus's 10.0.4.0/24
`ipBlock` never matches a node IP in Cilium, and the only rule that did reach
nodes was `allow-kube-api`'s `kube-apiserver` entity, which covers the three
control-plane nodes and not a worker. `cilium-dbg monitor --type drop` on
Prometheus's node showed `Policy denied ... -> remote-node: 10.0.4.104:10250`.
Fixed by CiliumNetworkPolicy `allow-prometheus-node-scrape` in `monitoring`
(`toEntities: [host, remote-node]`, ports 10250, 9100, 9965); any future
worker needs nothing more.

[^pi5-node]: Raspberry Pi 5 worker join
[^pi5-runbook]: Raspberry Pi 5 worker runbook and patch

# Raspberry Pi 4 workers

`talos-pi4-1` (10.0.4.105) and `talos-pi4-2` (10.0.4.106) run the official Image
Factory Talos v1.13.12 image (overlay `rpi_generic`, schematic `ee21ef4a...`),
booted from USB sticks that hold boot, STATE and EPHEMERAL. Shared patch
`rpi4-worker.yaml` plus a per-node HostnameConfig. Label
`node.integratn.tech/board=rpi4`, the same arm64 taint via `registerWithTaints`,
NIC `end0`. Upgrades use `factory.talos.dev/metal-installer/<schematic>:<version>`,
so no private registry is needed. Prometheus scraped kubelet, node-exporter and
hubble on both from the start (the node-scrape policy covers any worker). Runbook:
[rpi4-worker.md](../../../matchbox/talos-machineconfigs/rpi4-worker.md).[^pi4-runbook]

[^pi4-nodes]: Raspberry Pi 4 workers join
[^pi4-runbook]: Raspberry Pi 4 worker runbook and patch
