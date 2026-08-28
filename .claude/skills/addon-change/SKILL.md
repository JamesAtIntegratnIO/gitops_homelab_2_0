---
name: addon-change
description: Change an ArgoCD addon safely — anything under addons/ (enable or disable an addon, edit a values.yaml, add a chart, change a version pin, add a NetworkPolicy or HTTPRoute for a new service). Use before editing those files and before reviewing such a diff; it covers value-layer precedence, the two ways a host setting silently leaks into the vcluster, and the target-matrix check that catches it.
---

# Changing an addon

Read [docs/addons.md](../../../docs/addons.md) and
[addons/README.md](../../../addons/README.md) for the factory chart itself.
This skill is the failure modes and the check.

## The one check that matters

```bash
./.claude/skills/addon-change/scripts/addon-targets.sh > /tmp/before   # base revision
# ...edit...
./.claude/skills/addon-change/scripts/addon-targets.sh > /tmp/after
diff -u /tmp/before /tmp/after
```

It renders **both** bootstraps against the live ArgoCD cluster inventory and
prints which clusters every ApplicationSet actually targets. The sum equals the
live `-l addon=true` Application count, so it is not an approximation.

**Never reason about targeting from the addon definition alone.** Two
independent mechanisms move a setting somewhere you did not intend:

1. **Value folders resolve by `chartName`, not by addon key.**
   `cert-manager-vcluster` has `chartName: cert-manager`, so it reads
   `environments/{env}/addons/cert-manager/values.yaml` — the *host* file. Break
   the tie with `valuesFolderName`, or put role-specific settings under
   `cluster-roles/{role}/addons/<chart>/values.yaml`, which does resolve per
   target cluster.
2. **Several production-layer addons carry no `cluster_role` exclusion**, so
   the control-plane bootstrap generates an Application for every production
   cluster — the vcluster included. Today that is `external-secrets`,
   `kyverno`, `argocd-projects`, `gateway-api-crds` and `platform-scheduling`.
   The script shows them targeting `the-cluster,vcluster-media`.

This cost a live incident: `priorityClassName: platform-critical` was added to
three addons while the PriorityClasses shipped control-plane-only, and nine
Deployments inside vcluster-media froze mid-rollout. (Also: vcluster's syncer
does not propagate `priorityClassName` to the host pod, so inside a vcluster
that field only satisfies admission.)

`-> <none>` means the selector matches no cluster. That is benign and permanent
for `vcluster-coredns-config` and `gateway-api-crds-vcluster`; for anything else
it means the addon is not running.

## Value-layer precedence

`environments/{env}` → `cluster-roles/{role}` → `clusters/{name}`, later wins,
**resolved by chart name**. The vcluster bootstrap reads
`environments/{env}/addons/common.yaml` but *not* that layer's `addons.yaml` —
see the file list in `terraform/cluster/bootstrap/addons-vcluster.yaml`.

## Before you commit

- **A chart bump can drop the settings you rely on.** Helm ignores an
  unrecognised value rather than failing, so a chart that renames or removes a
  key takes the setting with it and renders green. Measured: kyverno
  3.2.8 → 3.9.0 stopped declaring **48 of the 77** values this repo sets, six of
  them Kargo-tracked. Diff the declared surface across versions:
  `$HELM show values <repo>/<chart> --version <old|new>`, plus
  `$HELM show readme` when the chart ships a helm-docs table and no
  `values.schema.json`. The in-cluster gate does this for you on a PR — see
  the `gate-triage` skill.
- **A partial `podSecurityContext`/`containerSecurityContext` override
  REPLACES the chart default** in the loki/grafana chart family (they resolve
  it with `coalesce`, not merge). A seccomp-only override would have dropped
  `fsGroup: 10001` and run the pod as root on the NFS volume.
- **A new service behind the gateway needs both NetworkPolicy halves.** The
  service's namespace must admit `nginx-gateway`, *and* the data plane's egress
  allow-list in
  [network-policies/nginx-gateway.yaml](../../../addons/cluster-roles/control-plane/addons/network-policies/nginx-gateway.yaml)
  must name the new namespace and port. The symptom of missing the second half
  is a hang with **zero bytes**, not an error.
- **`allow-controller-egress` in `network-policies/kargo.yaml` excepts every
  RFC1918 range** (`0.0.0.0/0:443` minus 10/8, 172.16/12, 192.168/16). Right for
  registries, wrong for anything in-cluster: a ClusterIP is RFC1918. A new
  in-cluster target the Kargo controller must reach needs its own explicit rule.
- **Cilium never matches NetworkPolicy `ipBlock` against node IPs**
  (`policy-cidr-match-mode` is unset) — use a CiliumNetworkPolicy with
  `toEntities: [host, remote-node]`. Likewise a ClusterIP cannot be an
  `ipBlock`: it is DNAT'd to the pod IP before policy evaluation, so select on
  namespace + pod and the **target port** (e.g. metrics-server on 10250).
- **Gateway API, not Ingress.** HTTPRoute everywhere.
- **Pin images and charts.** An unpinned `:latest` silently jumped grafana-mcp
  0.14.0 → 1.1.0 on a rollout. Never point a *liveness* probe at an external
  dependency.
- **If the key is Kargo-tracked**, obey the pin rules — see the `kargo-pins`
  skill. Adding a new pinned artifact means adding it to
  [kargo-projects/values.yaml](../../../addons/cluster-roles/control-plane/addons/kargo-projects/values.yaml).
- **Do not delete `clustersExport.knownAbsentLabels` from `.gitops-gate.yaml`.**
  The key name lies — it is *render* configuration, read in both gate modes.
  Without it the gate exits 2 and posts `error` on every open PR.
- **No `kind: Secret` near `promises/`** (the Kratix state repo is public) —
  use an `ExternalSecret` against the `onepassword-store` ClusterSecretStore.

## Render checks

```bash
source .claude/skills/homelab-toolchain/scripts/tools.sh
$HELM template addons/charts/application-sets -f <values...>          # an addon
$HELM template addons/charts/kargo-projects \
  -f addons/cluster-roles/control-plane/addons/kargo-projects/values.yaml
```

Rendering is not verification. A change is verified when it is on `main`,
ArgoCD reports the app `Synced`/`Healthy`, and the thing it was meant to fix
stopped happening — see the `ship-change` skill.
