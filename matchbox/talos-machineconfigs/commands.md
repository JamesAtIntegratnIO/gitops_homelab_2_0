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
#    (control plane). Both are in the fragments used at generation time, so
#    applying them here keeps the live cluster and git in agreement.
talosctl -n 10.0.4.101 patch mc --patch-file all.yaml
talosctl -n 10.0.4.101 patch mc --patch-file cp.yaml
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
| `watchdog.yaml` | `WatchdogTimerConfig` | Talos reboots itself on a kernel panic already. The watchdog covers the case where nothing panics — a hung kernel or a hardware fault — by resetting the machine after 2 minutes without a heartbeat. |

## Still open at the Talos layer

**Upstream DNS reliability.** The CoreDNS pods forward everything they cannot
answer to the Talos host DNS resolver at `169.254.116.108:53`. When that stalls,
CoreDNS saturates and *cluster-internal* resolution fails too, which is the
documented cause of the vcluster-media restart storms
(see [known issues](../../docs/okf/cluster/known-issues.md)). Worth checking:

```bash
talosctl -n 10.0.4.101 get resolvers          # which upstreams host DNS uses
talosctl -n 10.0.4.101 dmesg | grep -i dns
```

If the upstreams are a single home router, adding a second independent resolver
under `machine.network.nameservers` is the cheapest fix.

**CoreDNS tuning.** Talos owns the CoreDNS Deployment and ConfigMap as bootstrap
manifests and re-applies them, so the Corefile cannot be changed from git. To
own it — for example to re-enable `cluster.local` caching, which is disabled in
the Talos default Corefile and makes every internal lookup a live query — set
`cluster.coreDNS.disabled: true` and deploy CoreDNS as a normal addon.
