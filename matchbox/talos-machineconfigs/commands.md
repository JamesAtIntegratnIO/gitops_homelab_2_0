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

# 4. CoreDNS handover -- DO NOT RUN THIS BEFORE THE ADDON IS ON. Stops Talos
#    rendering 11-core-dns / 11-core-dns-svc so the Corefile can be owned from
#    git. Talos does not delete what it already applied, so the moment this
#    lands nothing breaks -- but nothing recreates CoreDNS either. The
#    ArgoCD-side prerequisite, the checks and the rollback are in
#    addons/cluster-roles/control-plane/addons/coredns-workload/README.md.
#    No reboot; dry-run on 2026-08-30 was a clean two-line addition.
talosctl -n 10.0.4.101 patch mc --patch-file live-coredns-disabled.yaml --dry-run
talosctl -n 10.0.4.101 patch mc --patch-file live-coredns-disabled.yaml
#    Verify Talos has let go -- the manifests should stop being rendered:
talosctl -n 10.0.4.101 get manifests | grep core-dns          # expect nothing

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
| `live-coredns-disabled.yaml` | `cluster.coreDNS.disabled` | Hands CoreDNS to ArgoCD. `cluster.coreDNS` accepts only `disabled` and `image` — `talosctl validate` rejects `corefile`/`Corefile`/`extraConfig` as unknown keys — so a Corefile that needs a custom line has to be owned from git. Paired with the `coredns-workload` addon; apply it **second**. |
| `watchdog.yaml` | `WatchdogTimerConfig` | Talos reboots itself on a kernel panic already. The watchdog covers the case where nothing panics — a hung kernel or a hardware fault — by resetting the machine after 2 minutes without a heartbeat. |

## Still open at the Talos layer

**Upstream DNS reliability.** Now covered by `dns.yaml` above, pending its two
pre-checks. The measurement that motivated it, from both CoreDNS replicas:
~2,000 upstream health-check failures and ~4,200-4,700 occasions of forwarding
with no healthy upstream at all, over roughly five months of pod uptime. Since
there is only one upstream those are the same event. Also worth a look while
you are on the node:

```bash
talosctl -n 10.0.4.101 dmesg | grep -i dns
```

**CoreDNS tuning.** Talos owns the CoreDNS Deployment and ConfigMap as bootstrap
manifests and re-applies them, so the Corefile cannot be changed from git —
and `cluster.coreDNS` is not an escape hatch: it takes `disabled` and `image`
and nothing else, confirmed by feeding `corefile:` to `talosctl validate` and
getting "unknown keys found during decoding".

Half-done as of 2026-08-30. The ConfigMap is owned from git by the
`coredns-host-config` addon, which carries the `rewrite` line the Kargo UI's
OIDC login needs and wins by `selfHeal` rather than by ownership — Talos still
overwrites it after a reboot and ArgoCD puts it back within the sync interval.
The rest (`live-coredns-disabled.yaml` here, plus the staged `coredns-workload`
addon) ends the fight for good and is ready to apply; step 4 above is the
procedure. Doing it also re-enables `cluster.local` caching, which the Talos
default Corefile disables, and makes CoreDNS upgrades yours rather than Talos's.

## A `patch mc --dry-run` diff prints private keys

`talosctl patch mc --dry-run` renders a diff of the **whole** machine config,
which includes `cluster.etcd.ca.key` and the other embedded PEM blocks as
base64. That is why the rendered configs are gitignored, and it applies just as
much to terminal scrollback, a shared screen, or anything that captures command
output. Read the diff, do not paste it.
