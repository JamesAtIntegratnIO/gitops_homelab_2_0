# Upgrading Talos and Kubernetes on the-cluster

Written 2026-10-29, when the cluster was Talos **1.11.5** / Kubernetes **v1.34.1**
and both were out of support.

Talos configuration is outside the GitOps loop, so none of this is applied by
ArgoCD. Every `talosctl upgrade` here is a manual step that reboots a node and
replaces its OS image, and `talosctl` writes are not available from the agent
workstation — the commands below are James's to run. The agent can run
`preflight.sh`, read state and verify each step.

## Why this is necessary

| | Running | |
|---|---|---|
| Kubernetes | v1.34.1 | **EOL 2026-10-27** — no further patches, including security |
| Talos | v1.11.5 | the 1.11 line no longer appears in the Sidero support matrix; latest 1.11 patch is 1.11.10 |

And the coupling that decides the order:

**A Talos minor caps the Kubernetes maximum.** Talos 1.11 supports Kubernetes
1.29–1.34 and nothing above it, so there is no route off EOL Kubernetes without
a Talos minor upgrade first. Neither project's changelog states this; it lives
only in the support matrix.

| Talos | Supported Kubernetes |
|---|---|
| 1.11 | 1.29 – 1.34 |
| 1.12 | 1.30 – 1.35 |
| 1.13 | 1.31 – 1.36 |
| 1.14 | 1.33 – 1.37 |

The useful consequence: **1.12, 1.13 and 1.14 all still support 1.34.** So Talos
can move the whole ladder with Kubernetes held constant, and Kubernetes moves
afterwards as a separate change. Two steps with one variable each beats one step
with two — when something breaks you know which half did it.

`preflight.sh` encodes that table and refuses a target whose range excludes the
running Kubernetes, or a jump that skips a minor.

## Step 0 — renew the os:admin certificate (do this first, today)

On 2026-10-29 the cert had **one day** left:

```
subject= /O=os:admin
notAfter= Oct 30 17:10:25 2026 GMT
```

This gates everything. `talosctl upgrade` needs a valid `os:admin` cert, and if
it lapses mid-ladder you cannot finish *or* roll back without regenerating from
the CA in `matchbox/assets/talos/<ver>/controlplane1.yaml` (gitignored, present
on the workstation that generated it). Renew while the current one still works:

```bash
talosctl --context the-cluster config new --roles os:admin --crt-ttl 8760h talosconfig-new
# inspect, then merge:
talosctl config merge ./talosconfig-new
talosctl --context the-cluster version --short     # still answers?
```

`preflight.sh` FAILs under 7 days remaining and WARNs under 30, so this cannot
be forgotten again.

## Step 0b — bump talosctl in the flake

The flake currently carries **talosctl 1.11.3 only**, which is already behind
the running nodes (1.11.5) and behind every target on the ladder. Drive an
upgrade with a client at least as new as the target: bump `talosctl` in
`flake.nix`, `nix flake update` the relevant input, and confirm:

```bash
source .claude/skills/homelab-toolchain/scripts/tools.sh
"$TALOSCTL" version --client --short
```

This matters more than it looks, because `talosctl upgrade`'s default image is
derived from the client version — see the trap in Step 3.

## Step 1 — patch within the line first: 1.11.5 → 1.11.10

Same minor, no Kubernetes implications. Its purpose is to prove the upgrade
mechanics on the least dangerous change available: that the image URL is right,
that a node comes back, that etcd re-joins, that `preflight.sh` goes green
again. If anything about the procedure is wrong, learn it here.

## Step 2 — the Talos ladder, one minor at a time

```
1.11.10  →  1.12.x  →  1.13.x  →  (1.14.x)
```

Kubernetes stays on 1.34 throughout. Do not skip a minor — Talos supports
`n → n+1`, and `preflight.sh` FAILs on a skip.

Stop at 1.13 if you want a shorter hop; it supports Kubernetes up to 1.36,
which is enough to get back into support.

## Step 3 — per node, in this order

Workers first if there were any; this cluster is three control-plane nodes, so
go **non-VIP-holder first and the VIP holder last**. Find the VIP holder:

```bash
talosctl --context the-cluster -n 10.0.4.101,10.0.4.102,10.0.4.103 get addresses \
  | grep 10.0.4.100
```

For each node, in turn:

```bash
# 1. Gate.
./matchbox/talos-machineconfigs/preflight.sh 1.12.0     # must exit 0

# 2. Upgrade one node. --image is NOT optional -- see the trap below.
talosctl --context the-cluster -n <NODE_IP> upgrade \
  --image ghcr.io/siderolabs/installer:v1.12.0

#    --wait defaults to true, so this blocks and streams progress.
#    If it hangs on kexec, retry that node with: -m powercycle

# 3. Confirm it rejoined. Not just Ready -- etcd too.
kubectl get nodes -w                                     # that node Ready
talosctl --context the-cluster -n 10.0.4.101 etcd members # 3 members, no learner
talosctl --context the-cluster -n 10.0.4.101 etcd alarm list   # empty

# 4. Gate again before touching the next one.
./matchbox/talos-machineconfigs/preflight.sh 1.12.0
```

Three things about this command, all verified against `talosctl upgrade --help`
rather than remembered:

- **`--image` is mandatory in practice.** Its default is the *client's* own
  version — `installer:v1.11.3` for the talosctl in this flake. Running bare
  `talosctl upgrade` against these 1.11.5 nodes would **downgrade them**. Always
  pass the target explicitly.
- **Never use `-f/--force`.** Its own help says it skips the etcd health and
  member checks and "might lead to data loss". Those checks are the thing
  standing between a one-node reboot and a lost quorum.
- **There is no `--preserve` flag.** Older Talos had one; modern `upgrade`
  preserves `EPHEMERAL` (`/var`) by default, which is what keeps etcd data and
  this cluster's PVs. If you find `--preserve` in an old note, it is from a
  different era and will fail as an unknown flag.

Between nodes, `preflight.sh` will report mixed Talos versions as a FAIL — that
is expected *during* the ladder and is the one FAIL you knowingly walk past.
It exists to stop you starting a **new** upgrade on top of an unfinished one.

Do not proceed to the next node until etcd has three voting members. With three
members quorum is two, so exactly one may be down; starting a second reboot
early is how a maintenance window becomes an outage.

## Step 4 — Kubernetes, only once Talos is settled

```bash
talosctl --context the-cluster --nodes 10.0.4.101 upgrade-k8s --to 1.35.0
```

`upgrade-k8s` drives the whole control plane from one node; it is not per-node.
Then repeat for 1.36 if you went to Talos 1.13+.

Afterwards, update the flake's kubectl so the client matches:

```bash
# .claude/skills/homelab-toolchain/scripts/tools.sh
export KUBECTL_WANT_MINOR="${KUBECTL_WANT_MINOR:-1.36}"
```

The tooling follows the cluster; it never drives it.

## Step 5 — separately, the vcluster

`vcluster-media` runs its own Kubernetes (v1.34.3 on 2026-10-29). That is a
vcluster chart version bump through the normal addon path, not a Talos
operation, and it belongs in its own PR.

## Rollback

Talos keeps the previous image and will roll back on its own if the new one
fails to boot. To force it:

```bash
talosctl --context the-cluster -n <NODE_IP> rollback
```

If a node is lost rather than merely unhappy, the recovery is a Matchbox
reinstall from `matchbox/assets/talos/<ver>/` plus an etcd member replace — see
`matchbox/README.md`. That is why Step 0 of every window is a fresh etcd
snapshot, and why `preflight.sh` FAILs without one newer than 24h.

## Traps specific to this cluster

- **etcd shares a disk with the OS and containerd on all three nodes.**
  Controllers drop leases (`leaderelection lost`) for no local reason. During an
  upgrade this looks exactly like the upgrade's fault. `preflight.sh` WARNs when
  leader-election events are already elevated, so check it *before* believing
  the upgrade caused one.
- **The machine config is multi-document** once `watchdog.yaml` is on, so JSON
  6902 patches are refused. Use the `live-*.yaml` strategic-merge patches.
- **Never `patch mc` with `all.yaml` / `cp.yaml` / `work.yaml`.** Those are
  generation-time fragments and `patch mc` *appends* list items — a dry-run on
  2026-08-21 showed it duplicating `certSANs`, `extraKernelArgs` and the entire
  network-interface/VIP block.
- **`dns.yaml` is withdrawn.** Do not apply it; both of its premises were
  disproved on 2026-08-21.
- **A Talos upgrade no longer bumps CoreDNS** — see
  `docs/okf/cluster/known-issues.md`. Check it separately afterwards.
- **`/var` layout is not uniform**: `sda4` on .101/.102, `sda6` on .103. Harmless,
  but do not pattern-match a disk path across nodes in a script.

## Sources

- [Kubernetes 1.34 release page](https://kubernetes.io/releases/1.34/) — EOL 2026-10-27
- [Talos v1.14 support matrix](https://docs.siderolabs.com/talos/v1.14/getting-started/support-matrix)
- [Talos v1.12 support matrix](https://docs.siderolabs.com/talos/v1.12/getting-started/support-matrix)
- [Talos v1.11 support matrix](https://www.talos.dev/v1.11/introduction/support-matrix)
