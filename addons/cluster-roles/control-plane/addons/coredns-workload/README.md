# Taking CoreDNS off Talos

This directory is **staged and disabled**. It exists so that
`cluster.coreDNS.disabled: true` is a change you can make in one sitting
instead of a project.

## Why CoreDNS is split across two addons

| Addon | Owns | State |
|---|---|---|
| `coredns-host-config` (`../coredns/`) | the `coredns` ConfigMap | **on** |
| `coredns-workload` (here) | ServiceAccount, RBAC, Deployment, `kube-dns` Service | **off** |

Talos owns all of it today, as bootstrap manifests `11-core-dns` and
`11-core-dns-svc`, re-applied on every boot and machine config change. The
Corefile needs one `rewrite` line that Talos has nowhere to put —
`cluster.coreDNS` accepts only `disabled` and `image`, which `talosctl
validate` will confirm by rejecting anything else as an unknown key.

So the ConfigMap is owned now and wins on `selfHeal` — Talos overwrites it
after a reboot, ArgoCD puts it back within the sync interval, CoreDNS `reload`
picks it up ~30s later. The rest of CoreDNS is left with Talos, because
adopting it buys nothing until Talos is actually switched off.

That split is why the ConfigMap is **not** in this directory. Two ArgoCD
Applications owning the same object fight each other; the interim addon simply
keeps owning it, before and after the cutover.

## The manifests here are Talos's, verbatim

Rendered by Talos 1.11.3 and captured 2026-08-30 from the live cluster:

```bash
talosctl -n 10.0.4.101 get manifest 11-core-dns     -o yaml | yq '.spec'
talosctl -n 10.0.4.101 get manifest 11-core-dns-svc -o yaml | yq '.spec'
```

Byte-faithful on purpose. `kubectl diff -f .` against the live cluster was
**empty** when this was written, which is the property that makes step 1 below
a no-op rather than a rollout of the cluster's resolver. Re-check it before
cutting over — if that diff is not empty, Talos has moved and these files are
stale.

## Cutover

Order matters. ArgoCD takes ownership **first**, while Talos is still there as
a safety net; Talos is switched off **second**. The reverse leaves a window
where Talos has stopped recreating CoreDNS and nothing else has started.

```bash
# 0. Confirm the manifests still match what Talos would apply.
kubectl diff -f addons/cluster-roles/control-plane/addons/coredns-workload
#    Expect no output.

# 1. Set `coredns-workload.enabled: true` in
#    addons/cluster-roles/control-plane/addons/addons.yaml, merge, and wait.
argocd app get coredns-workload-the-cluster
#    Expect Synced/Healthy and, critically, NO pod churn:
kubectl -n kube-system get pods -l k8s-app=kube-dns
#    The two CoreDNS pods must keep their AGE. A restart here means the
#    manifests drifted from Talos and step 0 was skipped.

# 2. Only now, one node at a time, waiting for Ready in between.
talosctl -n 10.0.4.101 patch mc --patch-file \
  matchbox/talos-machineconfigs/live-coredns-disabled.yaml --dry-run
talosctl -n 10.0.4.101 patch mc --patch-file \
  matchbox/talos-machineconfigs/live-coredns-disabled.yaml
kubectl get nodes -w
```

Verify the fight is actually over — the manifest should stop being rendered:

```bash
talosctl -n 10.0.4.101 get manifests | grep core-dns   # expect nothing
```

## What you now own

From the moment step 2 lands, **a Talos upgrade no longer bumps CoreDNS**. The
image in `deployment.yaml` is Talos's tag, left unpinned so step 1 is a no-op;
once the cutover is verified it should be digest-pinned and added to the Kargo
target list in
[kargo-projects/values.yaml](../kargo-projects/values.yaml), or it will quietly
rot at whatever version Talos last shipped.

## Rolling back

Revert the machine config first, then disable the addon — the mirror of the
cutover, for the same reason.

```bash
talosctl -n 10.0.4.101 patch mc --patch-file - <<'PATCH'
cluster:
  coreDNS:
    disabled: false
PATCH
```

Talos re-applies `11-core-dns` on the next reconcile and takes the Corefile
back with it, so leave `coredns-host-config` on: it is what puts the `rewrite`
line back.
