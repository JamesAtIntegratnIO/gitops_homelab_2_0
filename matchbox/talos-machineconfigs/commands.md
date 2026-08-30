# Talos machine configuration

Only the patch fragments live here. The rendered configs embed cluster CA keys
and bootstrap tokens, so they are gitignored.

## Generate the configs (fresh cluster / reinstall)

```bash
talosctl gen config the-cluster https://10.0.4.100 \
  --config-patch @all.yaml \
  --config-patch @watchdog.yaml \
  --config-patch-control-plane @cp.yaml \
  --config-patch-worker @work.yaml
```

`dns.yaml` is deliberately absent from that list -- see step 3 below.

`watchdog.yaml` is a separate machine-config *document* (`kind:
WatchdogTimerConfig`) rather than a fragment of `machine:`, which is why it is
its own `--config-patch` instead of being merged into `all.yaml`.

## Apply to the running cluster

These patches are not applied by ArgoCD — Talos configuration is outside the
GitOps loop. Applying them to the live cluster is a manual step, and every one
of them needs `talosctl` plus a `talosconfig` (neither is on the workstation
that generated these notes).

Apply one node at a time and wait for the node to come back `Ready` before
moving on.

```bash
# 0. Confirm which nodes you are talking to.
talosctl -n 10.0.4.101,10.0.4.102,10.0.4.103 version

# 1. Hardware watchdog. Check the device exists first -- if /dev/watchdog0 is
#    absent this board has no hardware watchdog and the patch will not arm.
talosctl -n 10.0.4.101 list /dev | grep watchdog
talosctl -n 10.0.4.101 patch mc --patch-file watchdog.yaml
#    Verify: timeleft should count down and reset, not sit still.
talosctl -n 10.0.4.101 read /sys/class/watchdog/watchdog0/timeleft

# 2. Talos API access for etcd snapshots (all nodes) and the failover timings
#    (control plane). Do NOT `--patch-file all.yaml` / `cp.yaml` against a
#    running node: those are the generation-time fragments, and `patch mc`
#    APPENDS list items, so a dry-run on 2026-08-21 showed it duplicating
#    certSANs, extraKernelArgs and the entire network-interface/VIP block.
#    JSON 6902 patches are not an option either: Talos refuses them once the
#    config is multi-document, which it is as soon as watchdog.yaml is on.
#    The live-*.yaml files carry exactly the new keys as strategic-merge
#    patches; both dry-run clean with no reboot, the second restarts that
#    node's kube-apiserver static pod.
talosctl -n 10.0.4.101 patch mc --patch-file live-talos-api-access.yaml
talosctl -n 10.0.4.101 patch mc --patch-file live-apiserver-tolerations.yaml
#    Verify on that node before moving on:
kubectl -n kube-system get pod kube-apiserver-<node> -o jsonpath='{.spec.containers[0].command}' | tr ' ' '\n' | grep toleration
#    Once the first patch is on every node, `kubectl get crd
#    serviceaccounts.talos.dev` succeeds and the host etcd backup can be
#    switched on: set platform-backups-talos.enabled to true in
#    addons/cluster-roles/control-plane/addons/addons.yaml (one-line PR).

# 3. DNS -- dns.yaml is WITHDRAWN; do not apply it. Its pre-checks were run
#    on 2026-08-21 and both failed its premise:
#      talosctl -n 10.0.4.101 get resolvers   -> ["1.1.1.1","8.8.8.8"], both healthy
#      dig @10.0.0.1 media.integratn.tech     -> 10.0.4.200 (dnsmasq, split-horizon)
#    The nodes already have two public upstreams; the router is not in the
#    path, and adding it would make the cluster's own hostname flip between
#    the LAN and public answers. The single point of failure is the Talos
#    host-DNS proxy (169.254.116.108) that CoreDNS forwards everything to,
#    enabled by `machine.features.hostDNS.forwardKubeDNSToHost: true`. The
#    candidate fix is to set that false so CoreDNS forwards to the upstreams
#    directly and its own health checks get two targets -- untested, and it
#    changes how Talos renders the CoreDNS manifest, so it is its own task.

# 4. CoreDNS handover -- APPLIED 2026-08-30 on all three nodes. Kept as the
#    procedure for a rebuilt node or a rollback-and-retry. DO NOT RUN THIS
#    BEFORE THE ADDON IS ON: it stops Talos rendering 11-core-dns /
#    11-core-dns-svc, and Talos does not delete what it already applied, so
#    nothing breaks at the moment it lands but nothing recreates CoreDNS
#    either. Prerequisite, checks and rollback are in
#    addons/cluster-roles/control-plane/addons/coredns-workload/README.md.
#    No reboot, no static pod restart -- both CoreDNS pods kept their AGE
#    (145m) and 0 restarts straight through the patch on all three nodes.
talosctl -n 10.0.4.101 patch mc --patch-file live-coredns-disabled.yaml
talosctl -n 10.0.4.102 patch mc --patch-file live-coredns-disabled.yaml
talosctl -n 10.0.4.103 patch mc --patch-file live-coredns-disabled.yaml
#    Verify Talos has let go -- the manifests should stop being rendered:
talosctl -n 10.0.4.101,10.0.4.102,10.0.4.103 get manifests | grep core-dns   # expect nothing
#    Two nodes (.101, .102) also printed "WARNING: extra kernel arguments are
#    not supported when booting using SDBoot". Pre-existing and unrelated to
#    this patch -- it is all.yaml's extraKernelArgs against a UKI-booted node,
#    and `talosctl get securitystate` confirms why only those two: they report
#    `bootedWithUKI: true` and .103 does not. Expect it on any patch mc there.
#    All three said "Applied configuration without a reboot".

# 5. etcd defragmentation. Found 2026-08-21: ~600 MB on disk, ~110 MB in use
#    on every member. Blocks that member for a few seconds; non-leader first,
#    leader (`talosctl etcd status` shows it) last.
talosctl -n 10.0.4.101,10.0.4.102,10.0.4.103 etcd status
talosctl -n <non-leader-1> etcd defrag
talosctl -n <non-leader-2> etcd defrag
talosctl -n <leader> etcd defrag
talosctl -n 10.0.4.101,10.0.4.102,10.0.4.103 etcd status     # DB SIZE should be near IN USE
```

`cluster.apiServer.extraArgs` restarts the kube-apiserver static pod on that
node. With three control-plane nodes behind the `10.0.4.100` VIP that is
non-disruptive one node at a time, but it is worth watching:

```bash
kubectl get nodes -w
kubectl -n kube-system get pods -l component=kube-apiserver -o wide
```

## What each patch is for

| File | Contains | Why |
|---|---|---|
| `all.yaml` | install disk, certSANs, `kubernetesTalosAPIAccess` | Install by disk *label* so device reordering cannot wipe the wrong disk. The API access grants only `os:etcd:backup`, scoped to the `talos-backup` namespace — the prerequisite for scheduled etcd snapshots. |
| `cp.yaml` | VIP, CNI none, kube-proxy disabled, metrics bind addresses, apiserver toleration defaults | Cilium replaces CNI and kube-proxy. The toleration defaults cut node-failure failover from 5 minutes to 1 for *every* pod, including ones from charts this repo does not control. |
| `work.yaml` | DHCP networking | No dedicated workers exist today; kept for when one is added. |
| `dns.yaml` | `machine.network.nameservers` | Nothing set this, so nodes took a single DHCP resolver and CoreDNS forwarded everything to it. One upstream means "upstream unhealthy" and "no healthy upstream" are the same event, which is how a router stall becomes a cluster-internal DNS outage. |
| `live-coredns-disabled.yaml` | `cluster.coreDNS.disabled` | Hands CoreDNS to ArgoCD. `cluster.coreDNS` accepts only `disabled` and `image` — `talosctl validate` rejects `corefile`/`Corefile`/`extraConfig` as unknown keys — so a Corefile that needs a custom line has to be owned from git. Paired with the `coredns-workload` addon; apply it **second**. Applied 2026-08-30. |
| `watchdog.yaml` | `WatchdogTimerConfig` | Talos reboots itself on a kernel panic already. The watchdog covers the case where nothing panics — a hung kernel or a hardware fault — by resetting the machine after 2 minutes without a heartbeat. |

## Still open at the Talos layer

**Upstream DNS reliability.** Still open, and `dns.yaml` is not the answer —
it was withdrawn on 2026-08-21 when both its pre-checks failed its premise (see
step 3). The lever is `machine.features.hostDNS.forwardKubeDNSToHost: false`,
still untested. The measurement that motivated the work, from both CoreDNS
replicas: ~2,000 upstream health-check failures and ~4,200-4,700 occasions of
forwarding with no healthy upstream at all, over roughly five months of pod
uptime. Since CoreDNS sees only one upstream — the Talos host-DNS proxy — those
are the same event. Also worth a look while you are on the node:

```bash
talosctl -n 10.0.4.101 dmesg | grep -i dns
```

**CoreDNS tuning.** Talos owns the CoreDNS Deployment and ConfigMap as bootstrap
manifests and re-applies them, so the Corefile cannot be changed from git —
and `cluster.coreDNS` is not an escape hatch: it takes `disabled` and `image`
and nothing else, confirmed by feeding `corefile:` to `talosctl validate` and
getting "unknown keys found during decoding".

**Done 2026-08-30.** `coredns-workload` went on in PR #303 and the machine
config patch followed on all three nodes; `talosctl get manifests | grep
core-dns` now returns nothing anywhere, `coreDNS.disabled: true` is in all three
configs, and the `coredns` ConfigMap is owned solely by
`coredns-host-config-the-cluster` — no more `selfHeal` race with Talos after a
reboot. Both CoreDNS pods went through the whole cutover untouched.

Two things this unlocked but did **not** do on its own, and the first was
previously stated here as automatic:

- **`cluster.local` caching is still disabled.** The Corefile was captured from
  Talos verbatim, so `disable success cluster.local` / `disable denial
  cluster.local` came with it and are still in the ConfigMap. Owning the file
  makes removing them a one-line git change; taking the handover did not remove
  them. This is the amplifier in the vcluster restart storms — with it on, every
  internal lookup is a live query, so a stalled upstream saturates CoreDNS
  instead of being absorbed.
- **CoreDNS upgrades are now yours.** The image is still Talos's unpinned tag
  `registry.k8s.io/coredns/coredns:v1.12.4`, left that way so the adoption was a
  no-op. Nothing bumps it now, so it needs a digest pin and a line in the Kargo
  target list or it stays on v1.12.4 forever.

Still separate and still untouched: `machine.features.hostDNS.forwardKubeDNSToHost`,
the single-upstream problem itself. `forward . /etc/resolv.conf` in the Corefile
continues to point at the Talos host-DNS proxy.

## A `patch mc --dry-run` diff prints private keys

`talosctl patch mc --dry-run` renders a diff of the **whole** machine config,
which includes `cluster.etcd.ca.key` and the other embedded PEM blocks as
base64. That is why the rendered configs are gitignored, and it applies just as
much to terminal scrollback, a shared screen, or anything that captures command
output. Read the diff, do not paste it.
