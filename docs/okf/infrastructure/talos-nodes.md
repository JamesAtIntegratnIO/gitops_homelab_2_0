---
type: Infrastructure
title: Talos nodes
description: The three bare-metal Talos control-plane nodes and the Raspberry Pi 5 and Pi 4 arm64 workers that form the-cluster, their machine-config patches, how they are managed, and what is scheduled onto the Pis.
tags: [talos, bare-metal, nodes, control-plane, worker, arm64, raspberry-pi]
status: stable
generated: { by: claude, at: 2026-11-13T02:10:00Z }
stale_after: 2027-02-08
sources:
  - id: arm64-scheduling
    resource: ../../../addons/cluster-roles/control-plane/addons/network-policies/kyverno-schedule-arm64-images.yaml
    title: The schedule-arm64-images Kyverno policy and its image allowlist
  - id: arm64-image-check
    resource: registry manifest and paginated tag-list lookups for the 55 images on the host cluster's Deployments, and kubectl get pods in hermes, cq, mcp-system and vcluster-media, 2026-11-08
    title: arm64 image check and Pi occupants
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
(8 GB) are arm64 workers too, with the same taint.[^pi4-nodes] What else gets
onto them is covered under
[Scheduling onto the Pi workers](#scheduling-onto-the-pi-workers).

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
Cilium's `devices` lists it beside `eno1`. The L2Advertisement lists `end0` for
the Pi 4s, but the MetalLB speaker DaemonSet excludes `board=rpi5`, so the Pi 5
never announces a VIP. Upgrades need its installer in a
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

# Scheduling onto the Pi workers

All three Pis carry `node.integratn.tech/arch=arm64:NoSchedule`, so a pod needs
the toleration to land on one. Two things supply it.

**By hand, in the manifest.** hermes, cq-server and arr-mcp each set
`nodeSelector: node.integratn.tech/board: rpi5` and tolerate the taint; promtail
and loki-canary tolerate it too. On 2026-11-08 `talos-pi5-1` was running
hermes (namespace `hermes`), cq-server (`cq`) and arr-mcp (`mcp-system`).[^arm64-image-check]
James's intent is that the Pi 5 is for hermes and cq.

**By a Kyverno ClusterPolicy, `schedule-arm64-images`.** For a pod in any
namespace except a protected set (kube-system, kube-public, kube-node-lease,
kyverno, cilium-secrets, metallb-system, nfs-provisioner, local-path-storage,
trivy-system, hermes, cq), if every init and regular container image (digest
stripped) is on the `arm64-image-allowlist` ConfigMap in the `kyverno`
namespace, the policy adds, at pod CREATE:

- the `node.integratn.tech/arch=arm64:NoSchedule` toleration;
- a required nodeAffinity excluding `node.integratn.tech/board=rpi5`, so the Pi 5
  stays free for hermes and cq.

Nothing prefers or avoids the Pi 4s (James asked for balance, not a Pi 4
preference): the scheduler treats them as candidates alongside the three
control-plane nodes and places the pod by its normal scoring. If the Pi 4s are
full or down, the pod runs on the control-plane nodes. One unlisted image leaves
the whole pod untouched. A pod is also skipped if it already sets `nodeName`, a
control-plane or board nodeSelector, any nodeAffinity (every DaemonSet pod has
one), or an arch selector other than arm64. A second, Audit-only rule reports a
pod that selects arm64 by hand while running an unlisted image.[^arm64-scheduling]

The allowlist has one entry per image repository. A `repo:*` entry matches every
tag, so a version bump (Kargo or by hand) keeps landing on the Pis without
editing the list. An entry was written as `repo:*` only after the newest tags of
its repository (up to six of the same tag format, from the registry's paginated
tag list, 2026-11-08) all showed a `linux/arm64` manifest. Of the 55 distinct
images on the host cluster's Deployments, 52 had an arm64 manifest at their
current tag; `bosun` and `delivery-agent` are amd64-only and `specmarshal` could
not be checked (registry returned 403), so none of the three is listed.[^arm64-image-check]
Four entries are not wildcards: `prometheus-operator` (its newest tag, v0.95.1,
did not show an arm64 manifest; absent or a failed lookup, not investigated),
`platform-status-reconciler` (our own image, tags not sampled), the `ecr-public`
redis mirror (no tag listing to sample), and `kratix-platform`, whose reference
has no tag and so already matches any digest. A reference with no tag at all
(implicit `latest`) matches none of the `repo:*` entries.

Limits, stated plainly:

- The Pi 5 reservation covers pods this policy mutates. It is not a taint: a pod
  that tolerates the arch taint and sets no node constraint could still schedule
  there. A hard reservation needs a second taint on the Pi 5, which is a Talos
  machine-config change.
- A wildcard trusts future tags. If a listed repository drops its arm64 build, a
  pod using the new tag can still be scheduled onto a Pi 4 and fail to start
  there, because the scheduler does not know about image architecture. Remove the
  entry or go back to exact tags.
- MetalLB speakers run on the Pi 4s (arch toleration, `board!=rpi5`) and the
  L2Advertisement announces on `eno1` and `end0`. Before that, speakers ran only
  on the amd64 nodes, and a pod behind an `externalTrafficPolicy: Local`
  LoadBalancer that landed on a Pi had nobody announcing its VIP: from about
  2026-11-09 to 11-13 the vcluster-media gateway data plane sat on `talos-pi4-2`
  and every `*.vcluster-media.integratn.tech` app timed out while all pods
  showed Running. Verified 2026-11-13 after #795: with the gateway pod on
  `talos-pi4-1`, MetalLB announced 10.0.4.201 from that node and all seven
  apps returned 200. Moving the pod costs roughly 30-60 s of timeouts while
  the endpoint turns ready and the announcement moves.
- Mutation is CREATE-only, so existing pods move only when recreated.
- Mutate rules have no Audit mode; the allowlist is the control.
- Not verified: how the scheduler actually spreads these pods across the Pi 4s
  (8 GB each) and the 32 GB control-plane nodes once pods are recreated; the
  total resource requests of the allowlisted workloads against the Pi 4s'
  allocatable (about 3.95 CPU and 7.1 GiB each); and whether every PVC the
  allowlisted workloads use is reachable from a Pi. `bazarr` mounted an NFS PVC
  on `talos-pi4-1` on 2026-11-08.

[^arm64-scheduling]: The schedule-arm64-images Kyverno policy and its image allowlist
[^arm64-image-check]: arm64 image check and Pi occupants
