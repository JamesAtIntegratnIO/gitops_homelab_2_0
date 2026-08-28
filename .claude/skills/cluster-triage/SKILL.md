---
name: cluster-triage
description: Diagnose something broken on the-cluster or vcluster-media — a controller restarting for no local reason, a connection that hangs with zero bytes, a pod serving stale config while everything reports Healthy, a status/event write loop, a Prometheus query that returns no rows, a leaderelection loss. Use before changing anything in response to a live symptom.
---

# Diagnosing the live platform

Start with [docs/okf/cluster/known-issues.md](../../../docs/okf/cluster/known-issues.md) —
the OKF bundle is the platform's long-term memory and is usually newer than
`docs/`. Then:

```bash
source .claude/skills/homelab-toolchain/scripts/tools.sh
k -n argocd get applications -o json | $JQ -r '
  [.items[] | select(.status.sync.status!="Synced" or .status.health.status!="Healthy")
   | {n:.metadata.name, s:.status.sync.status, h:.status.health.status,
      m:((.status.health.message // "")[0:100])}]
  | if length==0 then "all Synced/Healthy" else (.[] | "\(.n)\t\(.s)/\(.h)\t\(.m)") end'
```

## The house style of failure here

Nearly every incident on this platform has been an **absence that nothing
observes**, not an error. Green dashboards are not evidence. Four shapes, and
what actually distinguishes them:

### A hang with zero bytes → NetworkPolicy

Not an error, not a refusal — a hang. Check **both halves**; one-sided policies
are the recurring cause.

- The gateway path needs the service's namespace to admit `nginx-gateway`
  *and* the data plane's egress allow-list in `network-policies/nginx-gateway.yaml`
  to name the namespace and port.
- `allow-controller-egress` in `network-policies/kargo.yaml` is
  `0.0.0.0/0:443` **minus every RFC1918 range** — so it permits registries and
  forbids every ClusterIP. Kargo's verification queries against Prometheus were
  dropped from the day the rule was written.
- **A ClusterIP cannot be an `ipBlock`**: it is DNAT'd to the pod IP before
  policy evaluation. Select on namespace + pod and the **target port**
  (metrics-server is 10250, not the Service port).
- **Cilium never matches `ipBlock` against node IPs** (`policy-cidr-match-mode`
  unset). Use a CiliumNetworkPolicy with `toEntities: [host, remote-node]`, or
  `[kube-apiserver]` for the API.

### A controller drops its lease for no local reason → etcd, not the controller

All three nodes put `/var/lib/etcd` on the same disk as the OS and containerd.
Measured p99 WAL fsync 178ms against a 10ms target, 1.48s backend commit, 44
leader changes/24h. `Failed to update lock: etcdserver: request timed out` →
`leaderelection lost` is the signature. Suspect this **before** the controller.
The real fix is a dedicated device — a Talos/hardware change, James-only.

### vcluster-media restart storms → host DNS, not etcd-on-NFS

Talos host DNS at `169.254.116.108` stalls → CoreDNS saturates (the Talos
Corefile disables `cluster.local` caching, so every internal lookup is a live
query) → the syncer cannot resolve `vcluster-media-etcd` → the apiserver loses
etcd and every in-vcluster controller dies together. Verified via Loki
correlation: 52/299/407 CoreDNS upstream timeouts during crash bursts vs 0 in
quiet windows. Talos owns CoreDNS as a bootstrap manifest and re-applies it; the
untested lever is `hostDNS.forwardKubeDNSToHost: false`.

### Serving stale config while every signal is green → NGF data plane

A data plane replica that never establishes its command connection to the
control plane keeps its start-time config snapshot **indefinitely**, pointing at
pod IPs that no longer exist. Pods Ready, endpoints ready, Gateway
`Programmed=True`, Application `Healthy` — and 3 of 8 requests answered.
**Nothing detects it.**

```bash
k -n nginx-gateway exec <pod> -c nginx -- \
  grep -A3 "upstream <ns>_<svc>_<port>" /etc/nginx/conf.d/http.conf   # compare replicas
k -n nginx-gateway logs <control-plane-pod> | grep 'Creating connection for nginx pod'
```

Fix by deleting the stale replica — safe single-handed, the survivor holds
correct config.

## Recipes

**Attribute a status write loop in ~8 seconds.** Two reads five seconds apart
miss a two-state flap half the time; `managedFields` names the writer:

```bash
k get <obj> -w -o json --show-managed-fields | $JQ '{
  rv: .metadata.resourceVersion,
  writer: ([.metadata.managedFields[]|select(.subresource=="status")]|max_by(.time).manager)}'
```

The Kratix case worth knowing: two code paths compute `workflowsSucceeded` and
each writes when the stored value disagrees with its own answer. A request
missing `ConfigureWorkflowCompleted` alternates 0→1→0 forever (~10 PUT/s, 97% of
all cluster events). The fix is to complete a pipeline run so Kratix rewrites
the condition — set the **label**, not the annotation; Kratix reads
`isManualReconciliation(obj.GetLabels())` and removes it itself:

```bash
k -n platform-requests label vclusterorchestratorv2s.platform.integratn.tech \
  vcluster-media kratix.io/manual-reconciliation=true
```

**A metric that parses is not a metric that exists.** kube-state-metrics
prefixes every CustomResourceState metric with `kube_customresource_` unless the
resource sets `metricNamePrefix`; alerts querying `kargo_*` matched nothing
while data flowed the whole time. Assert a query **returns rows**, not that it
parses. The pattern worth copying is `KargoMetricsMissing` — a dead-man's-switch
alert that can fire *without* the metrics it watches was the only thing in an
eight-alert set able to catch its own set being broken.

**A projected service-account token is bound and rotates roughly hourly**, and
the kubelet rewrites the file in place. A client that reads it once at start-up
works for fifty minutes then 401s forever — which, on a service called a few
times a day, looks fine in every test. Same shape as a GitHub App installation
token.

**`metadata.remainingItemCount` is best-effort** — set only for etcd-served
lists, not the watch-cache path. Treating its absence as "no more items"
under-counts and then presents the wrong number as fact. Walk pages.

## Before you fix it

- Fix things with commits ArgoCD reconciles. Direct `kubectl` mutation is for
  **orphans** — resources nothing in git manages any more — and you say so
  explicitly. Verify orphan status from ArgoCD tracking-id annotations and check
  owners/finalizers before deleting.
- The Kyverno ClusterPolicy `generate-default-deny-netpol` owns every
  namespace's `default-deny-all` NetworkPolicy with `synchronize: true`.
  Deleting or replacing it without `generateExisting: true` in the desired state
  cascade-deletes every default-deny with nothing to regenerate them.
- Restoring a vcluster means pausing a 4-level selfHeal chain top-down
  (`bootstrap-kratix` ApplicationSet → `kratix-the-cluster` →
  `kratix-state-reconciler` → `vcluster-vcluster-media`); scaling to zero is
  reverted within a minute. Runbooks:
  [docs/operations.md](../../../docs/operations.md),
  [docs/okf/platform/backups.md](../../../docs/okf/platform/backups.md).
  Neither restore path has been rehearsed.
