---
type: Operational Findings
title: Known issues & drift (snapshot 2026-08-20)
description: Everything found during a deep repo + live-cluster review that is broken, orphaned, cosmetic, or where docs/code disagree — with evidence.
tags: [known-issues, drift, findings, snapshot]
status: stable
generated: { by: claude-code/claude-fable-5, at: 2026-08-21T05:30:00Z }
stale_after: 2026-09-20
sources:
  - id: live-cluster
    resource: kubectl sweep of the-cluster (admin@the-cluster), 2026-08-20
    title: Live cluster observation
  - id: repo-review
    resource: full-repo review (source, docs, workflows, hooks), 2026-08-20
    title: Repository review
---

# etcd is unhealthy, and a status write loop is feeding it (2026-08-21)

Found while validating the self-healing rollout. Two linked problems.

## etcd is well outside its operating envelope

| metric | 10.0.4.101 | 10.0.4.102 | 10.0.4.103 | target |
|---|---|---|---|---|
| p99 WAL fsync | **178 ms** | 62 ms | 15 ms | < 10 ms |
| p99 backend commit | **1.48 s** | 64 ms | 27 ms | < 25 ms |
| leader changes / 24h | 44 | 37 | 43 | ~0 |
| proposal failures / 24h | 118 | 48 | 53 | 0 |
| DB in-use (fragmentation) | 73% | 79% | 71% | — |
| apiserver etcd request p99 | **4.56 s** cluster-wide | | | |

All three nodes put `/var` — and therefore `/var/lib/etcd` — on the single
`sda` alongside the OS, containerd images and logs. `talos-xez-xys`
(10.0.4.101) is **72% busy with 85 ms average write latency**. Giving etcd its
own fast device is a Talos/hardware change; see the
[Talos runbook](../../../matchbox/talos-machineconfigs/commands.md).

This is not cosmetic. It is the direct cause of the NFS provisioner's restarts:

```
E0821 12:54:44 Failed to update lock: etcdserver: request timed out, possibly
                due to previous leader failure
F0821 16:15:57 leaderelection lost
```

Any leader-elected controller can lose its lease this way, and several have.

## The database is 82% empty space (found 2026-08-21T20:00Z)

After the 208k-event storm was drained, `etcd_mvcc_db_total_size_in_bytes` sits
at **584–635 MB per member** while `etcd_mvcc_db_total_size_in_use_in_bytes` is
**~114 MB** on all three. bbolt never returns freed pages to the OS without a
defragmentation, so every backend commit is still working through a file five
times larger than its contents. This is the cheap part of the etcd problem:

```bash
talosctl -n 10.0.4.101,10.0.4.102,10.0.4.103 etcd status   # find the leader
talosctl -n 10.0.4.102 etcd defrag                          # non-leaders first
talosctl -n 10.0.4.101 etcd defrag
talosctl -n 10.0.4.103 etcd defrag                          # the leader on 2026-08-22
```

Defrag blocks that member for the duration (seconds at this size); do the
leader last. `talosctl etcd status` on 2026-08-22: 584 / 628 / 635 MB on
disk, 110 MB in use on every member. It needs `talosctl`, like everything else on the node layer.

## A status write loop was making it worse

**202,435 of the 209,527 events in the cluster — 97% — were
`ReconcileStarted`** from Kratix's ResourceRequestController against the single
`vcluster-media` resource, with `vclusterorchestratorv2s PUT` running at
**9.45/second** against that one object.

Two defects in `platform-status-reconciler` rewrote `.status` on every 60s pass
even in a steady state, and each write woke Kratix's watch:

1. `lastTransitionTime` was re-stamped with `time.Now()` every pass. It means
   *when the condition last changed state*, not *when we last looked*. Two reads
   six seconds apart were byte-identical apart from five timestamps advancing
   exactly 60s.
2. The conditions array was replaced wholesale. A JSON merge patch replaces a
   list, and Kratix owns `WorksSucceeded` on the same resource — so every write
   deleted it and Kratix put it straight back. Neither side could converge,
   because each rewrote the array with its own timestamps.

Fixed by carrying the previous timestamp when nothing transitioned, preserving
conditions owned by other controllers, and skipping the patch entirely when the
result is equivalent to what is already stored.

**Still open**: the underlying disk problem is untouched — reducing write volume
helps, but it is not the fix.

*Update 2026-08-21, corrected after measurement:* the stored-event count fell
from ~208k to 26,910 and then **climbed again** — 47,670 within an hour — because
a second write loop is still running at ~10 status `PUT`/s on
`vclusterorchestratorv2s/vcluster-media`, 100% of them by Kratix itself
(`manager` in managedFields), alternating `status.workflowsSucceeded` 0→1→0 on
every single write.

The `kratix.io/manual-reconciliation` annotation was the **trigger, not the
cause**. Removing it (done 2026-08-21 ~18:00Z) changed nothing; two Kratix image
rollouts (02:37Z, 16:01Z) changed nothing. The stuck state lives in the object.

**Cause — a Kratix-owned condition was wiped, and two Kratix code paths then
disagree forever.** Read from Kratix `main` (`internal/controller/
dynamic_resource_request_controller.go`, `lib/workflow/reconciler.go`):

1. `generateWorkflowsCounterStatus` sets `workflowsSucceeded = 1` **only if the
   request carries `ConfigureWorkflowCompleted=True`**, otherwise `0`.
2. `determineWorkflowState` sets it to `1` if the most recent pipeline Job for
   the current hash is Complete.
3. Each writes whenever the stored value disagrees with its own answer.

The VCO has **no `ConfigureWorkflowCompleted` and no `Reconciled` condition** —
exactly the two Kratix sets only on pipeline transitions. Every other resource
request in the cluster has both. They were removed by the *pre-fix*
`platform-status-reconciler` (`:latest`, running 2026-02-28 → 2026-08-21T17:08Z),
which replaced the whole `conditions` list on every 60s pass — its own fix commit
`9878067` says so. The 15:41Z pipeline completed, the condition was wiped within
a minute, and the fixed build landed at 17:08Z with nothing left to preserve.

Timeline (Prometheus, 1m resolution): 0.04 PUT/s flat → **2.36 at 02:20Z → 8.4 at
02:21Z**, i.e. at the manual reconcile, 18 minutes *before* the first Kratix
rollout. Before it there was no VCO pipeline Job at all (the stale one had been
deleted out-of-band), so path 1 and path 2 both said 0 and agreed. The manual
reconcile created a Job; it completed; the disagreement began.

**Second silent consequence:** `shouldForcePipelineRun` returns false on a nil
condition, so the VCO **stopped re-running on Kratix's 10h reconciliation
interval** — every other request has Jobs at ~10h spacing; the VCO has none
between 02:18Z and the ArgoCD-driven 15:41Z run.

**Resolved 2026-08-21T18:46Z.** The manual-reconciliation label was applied; a
new pipeline Job ran (`…-f1582`, created 18:46:21Z, succeeded); Kratix rewrote
`ConfigureWorkflowCompleted` and `Reconciled`, removed the label itself, and VCO
`PUT`/s fell from ~10 to **0**. `workflowsSucceeded` is stable at 1. The
stored-event count (46k at that moment) drains from here. Kept below for the
record.

**Fix: make Kratix re-establish the condition by completing a pipeline run.**
Set the manual-reconciliation **label** (Kratix `main` reads labels —
`isManualReconciliation(obj.GetLabels())`; `hctl reconcile` does exactly this,
and Kratix removes the label itself afterwards). While the label is set
`jobToPipelineIndex` returns `(0,false)`, so both paths agree on 0 and the loop
stops at once; on completion Kratix writes `ConfigureWorkflowCompleted=True`,
both paths agree on 1, and the fixed reconciler preserves foreign conditions
(`MergeForeignConditions`). Live write — James:

```bash
kubectl label vclusterorchestratorv2s.platform.integratn.tech \
  -n platform-requests vcluster-media kratix.io/manual-reconciliation=true
```

Verify: `status.conditions` gains `ConfigureWorkflowCompleted=True`, VCO PUT/s
returns to ~0.03, and the event count starts falling.

**Three further defects found on the way — all fixed on the same branch
(PR #23), two of them pending rollout:**

- **The fixed reconciler's `statusUnchanged` guard did not work.** It patched
  every 60s with only `lastReconciled` changing. Cause: the patch carried
  `health.subApps.unhealthy: null` for an empty list; a merge patch treats null
  as *delete*, so the stored object never had the key while the patch always
  did, and a byte comparison of the two could never be equal. The guard now
  applies the patch (RFC 7386 semantics) to the current status and compares the
  result, after JSON-normalising both sides; a test reproduces the live object
  shape and fails against the old implementation. Merged in PR #23 (`946f6d4`); the pin bump to `main-55400cb` is PR #25 — once
  synced, the `reconciler` managedFields time on the VCO should stop advancing
  every minute.
- **The Kratix Helm chart was effectively unpinned.** `0.0.1` is republished
  under new app versions (index regenerated 2026-08-21T09:22Z, appVersion
  1.16.0); the controller image changed at 02:37Z and 16:01Z on ordinary syncs.
  `image:` is now pinned by digest in the kratix values to the running image, so
  the sync is a no-op. The pipeline-adapter image is hard-coded in the chart's
  ConfigMap with no value to override and still tracks the chart; bump the two
  together.
- **The nightly `kratix-pipeline-cleanup` CronJob reported success while
  deleting nothing.** `lastSuccessfulTime` 04:00:46Z, yet 12 succeeded Jobs
  older than 24h remained. It ran `bitnami/kubectl:latest` and piped into `jq`
  under `sh -c` with no `pipefail`, so a missing tool produced an empty loop and
  exit 0. Rewritten on the repo's pinned kubectl image, bash `-euo pipefail`,
  no `jq` (fixed-width RFC 3339 timestamps make the cutoff a string compare).
  This is why two completed Jobs per request accumulate — and Job presence is
  the input the write loop turns on.

# `retry.limit: -1` can wedge an app against its own fix (2026-08-21)

Found when `mcp-system` stayed `OutOfSync/Degraded` **after** both fixes for it
had merged to `main`. An hour later the live Job still carried the *old*
`hook=PostSync` annotation and no `sync-options`, so neither fix had been
applied.

The cause is not the fixes. ArgoCD's in-flight sync operation hard-pins the
revision it is applying:

```
.operation.sync.revisions = ["d397d53c…"]      # the commit from *before* both fixes
```

and the addon layer sets:

```yaml
retry:
  backoff: { duration: 5s, factor: 2, maxDuration: 10m }
  limit: -1        # unlimited
```

A retry **resumes the same operation against the same pinned revision** rather
than re-resolving `HEAD`. With `limit: -1` that operation never reaches a
terminal state, so auto-sync never starts a new one, so **no newer commit can
ever be applied**. An app whose sync fails for a persistent reason is therefore
pinned to the exact revision that is broken, and the commit that would repair it
is unreachable. Backoff caps at 10m, so it retries forever, roughly every ten
minutes, on stale manifests.

This is the inverse of self-healing: infinite retry is right for a *transient*
failure and actively harmful for a *persistent* one.

`limit: -1` is set at `addons/charts/application-sets/values.yaml` and inherited
widely — **53 of 59 Applications** carry it (1 has `limit: 30`, 5 have none).
Only `mcp-system` was wedged when this was found, but any of the 53 is exposed.

**Recovery is manual and out-of-band** — terminating a sync operation is runtime
state, not desired state, so there is nothing to change in git:

```bash
kubectl -n argocd patch application <app> --type=json -p '[{"op":"remove","path":"/operation"}]'
# or, with the CLI: argocd app terminate-op <app>
```

Auto-sync then starts a fresh operation at current `HEAD`.

**Open question:** whether the inherited default should become a finite limit.
With exponential backoff capped at 10m, a limit of ~10 spends about an hour on
genuine transients and then gives up, which lets the next commit be tried. The
cost is that after a terminal failure ArgoCD will not re-attempt the *same*
revision automatically, so a transient failure that outlasts the limit leaves
the app `OutOfSync` until a new commit or a manual sync. Not changed here — it
is a deliberate trade-off across every app, and belongs with the self-healing
work that set it.

# Root cause found (2026-08-21): the vcluster-media restart storms

Four pods in `vcluster-media` had 676-761 restarts since March, in bursts where
a dozen containers die within ~30 seconds of each other. Loki shows the chain:

1. CoreDNS logs a flood of `read udp …->169.254.116.108:53: i/o timeout` — the
   **Talos host DNS resolver**, which is CoreDNS's `forward . /etc/resolv.conf`
   upstream, stops answering.
2. CoreDNS saturates. Because the Talos Corefile disables `cluster.local`
   caching (`cache 30 { disable success cluster.local; disable denial
   cluster.local }`) every internal lookup is a live query, and the pods carry a
   200m CPU limit, so throttling delays even the queries CoreDNS could answer
   locally.
3. The vcluster syncer fails to resolve its own etcd:
   `dial tcp: lookup vcluster-media-etcd: operation was canceled`.
4. The vcluster apiserver loses etcd, `leaderelection lost`, and **every**
   controller running inside the vcluster shuts down together — which is why the
   restarts look synchronised across kyverno, cert-manager, KSM, argocd and the
   gateway inside the tenant.

Correlation across windows (Loki, count of the CoreDNS upstream-timeout line):

| Window | Crash burst? | CoreDNS upstream timeouts |
|---|---|---|
| 2026-08-05 09:00-09:20 | yes | 52 |
| 2026-08-09 06:50-07:10 | yes | 299 |
| 2026-08-16 06:35-06:55 | yes | 407 |
| 2026-08-18 04:20-04:40 | no | 0 |
| 2026-08-19 12:00-12:20 | no | 0 |
| 2026-08-20 18:00-18:20 | no | 0 |

Not etcd-on-NFS, which was the obvious first guess — the etcd PVCs are on
`config-nfs-client`, but the failure is name resolution, not storage latency.

**What can be fixed from git: almost nothing.** Talos owns the CoreDNS
Deployment and ConfigMap as bootstrap manifests and re-applies them, so replicas,
anti-affinity and the Corefile are not ours. A `PodDisruptionBudget` is a
separate object Talos does not manage and has been added, which at least stops a
drain from taking both CoreDNS pods.

**The real fixes are Talos-side** (see
[commands.md](../../../matchbox/talos-machineconfigs/commands.md)).

*Update 2026-08-21: the single-upstream hypothesis is now measured, not assumed.*
Scraping both CoreDNS replicas directly on `:9153` shows exactly one forward
target and no fallback behind it:

| counter | `…-f2tlz` | `…-gsh7d` |
|---|---|---|
| `coredns_proxy_healthcheck_failures_total{to="169.254.116.108:53"}` | 2,036 | 2,169 |
| `coredns_forward_healthcheck_broken_total` | 4,192 | 4,673 |

Only one `to=` label exists across either replica, so those two counters describe
the same event: *the* upstream being unhealthy **is** having no healthy upstream,
and CoreDNS falls back to forwarding blind. Both replicas agree to within ~10%
over ~5 months of uptime, which puts the fault upstream rather than in either
pod. The host resolver's own upstream list could not be read from here — that
needs `talosctl get resolvers` — but the nodes set no
`machine.network.nameservers` anywhere in `matchbox/talos-machineconfigs/`, so it
is whatever DHCP hands out, and the default gateway on `eno1` is `10.0.0.1`.

A second independent resolver was therefore queued as
[`dns.yaml`](../../../matchbox/talos-machineconfigs/dns.yaml), with two
pre-checks, because the field *replaces* the DHCP list and Talos distributes
queries across upstreams rather than treating later ones as failover-only.

**Correction 2026-08-22, with a talosconfig in hand — `dns.yaml` is
withdrawn.** `talosctl get resolvers` shows every node already resolving
through `["1.1.1.1","8.8.8.8"]`, both reported healthy by `get dnsupstreams`;
the router at `10.0.0.1` is not in the path at all. Worse, it is a dnsmasq
with a split-horizon zone (`media.integratn.tech` → `10.0.4.200`), so adding it
would have made the cluster's own hostnames flip between LAN and public
answers. The single upstream CoreDNS sees is the Talos host-DNS *proxy*
(`hostDNS.forwardKubeDNSToHost: true`), which multiplexes onto those two
resolvers — a stall there is the proxy stalling, not a router. The candidate
fix is `forwardKubeDNSToHost: false`, which makes CoreDNS forward to the
upstreams directly with its own per-upstream health checks; untested, and its
own task.

Owning CoreDNS outright (`cluster.coreDNS.disabled: true` plus a CoreDNS addon)
would additionally allow re-enabling `cluster.local` caching and raising the CPU
limit, which would make the whole chain far less brittle. Worth noting for that
work: 39% of everything CoreDNS forwards upstream (1.48M of 3.79M queries)
returns NXDOMAIN, which is search-domain expansion leaving the cluster.

# Resilience work (2026-08-21)

Branch `claude/self-healing-clusters-97da62`. Addresses the self-healing gaps
found alongside the items below; see
[resilience](/platform/resilience.md) for the resulting design and
[game day](../../game-day.md) for how to verify it.

- ArgoCD `syncPolicy` now deep-merges over the chart default: **50/50** rendered
  Applications self-heal with a retry policy, up from 32/48 self-healing and only
  12 with retry. `allowEmpty` is false everywhere now that prune covers 48 of 50.
- CPU requests right-sized from the VPA recommendations: 8759m -> 7199m, so
  two-node headroom goes from **-819m to +741m**.
- `platform-critical` / `platform-batch` PriorityClasses, assigned across the
  recovery path and the deferrable workloads.
- Kyverno admission controller 1 -> 3 replicas (its `failurePolicy: Fail`
  webhooks were a single-pod dependency for *all* pod creation), gateway data
  plane 1 -> 2, PDBs on everything running two or more.
- 60s node-failure tolerations in place of the 300s default.
- Kyverno `ClusterCleanupPolicy` for the leftovers behind items #1 and #4 below;
  descheduler for post-recovery imbalance and permanent restart loops.
- Talos patches committed (watchdog, `os:etcd:backup` API access, apiserver
  toleration defaults) — **not applied**, they need `talosctl`.

# Remediation status (2026-08-21) — quick wins COMPLETE

Applied 2026-08-20/21 via PR #2 (8 rebased commits, `f09773c..4d0e7fa`) and
PR #3 (`16ed316`), plus out-of-band cleanup of unmanaged resources. End
state verified: **54/54 ArgoCD apps Synced and Healthy** (the last holdout,
`nginx-gateway-fabric-the-cluster`, turned out to be an expired wildcard
certificate — see the follow-up finding below).

- #1 stale VCO status — **fixed**: suspended 170d job deleted, pipeline
  re-ran Complete in 27s; `Reconciled: Failing` cleared, all conditions True.
- #2 llmkube — **removed entirely**: git leftovers (netpols, addon stanza,
  llmkube-models) via PR #2; runtime (controller, CRs, CRDs, cluster RBAC,
  namespace, both 50Gi model-cache PVCs ≈100Gi NFS) deleted after the netpol
  prune synced.
- #3 git-indexer — **removed entirely**: CronJob/Jobs/ConfigMap/
  ExternalSecret/secret from `ai`; netpol and the two mcp-system secret
  references from git.
- #4 trivy — **fixed**: the three stuck scan jobs were deadlocking the
  operator; scanning resumed immediately after deletion (60+
  VulnerabilityReports within the hour, from zero for 88 days).
- #5 OutOfSync apps — **all four fixed**: etcd-secret-writer (invalid
  securityContext after Kyverno mutation + Job immutability → explicit
  allowPrivilegeEscalation, alpine/k8s image, Force+Replace sync-options,
  extended PolicyException; re-ran Complete in 6s), network-policies
  (immutable-generate-rule rejection → `generateExisting: true` + explicit
  mcp-system default-deny in git, then one-time `kubectl replace --force` of
  the ClusterPolicy; all 24 default-denies regenerated with zero coverage
  gap), mcp-system-app (git typo `allowPrivilegeEscalance` corrected),
  kratix-state-reconciler (ignoreDifferences for CRD-defaulted GatewayRoute
  spec fields + the kratix.io/promise-name label churn Kratix applies to
  requests — PR #3, matching the existing ArgoCD* request entries).
- #6 mcp-system — **adopted**: `mcp-system` is now a manifest addon
  (`mcp-system-the-cluster` app, Synced); the out-of-band `mcp-system-app`
  Application was deleted non-cascadingly. github-mcp/mcpo rolled out
  cleanly on the corrected `mcp-github-token` reference.
- #8 pre-commit hook — **fixed**: path filter now `promises/`, example store
  name corrected.

**Still open**: #7 (idle third NFS provisioner), #9–#20 as originally
described (portability, latent hctl bugs, doc drift), minus everything
above.

# Follow-up finding (2026-08-21): the wildcard certificate had EXPIRED

Investigating `nginx-gateway-fabric-the-cluster`'s "Progressing" health
(ArgoCD's Certificate check reports Progressing while `Issuing=True`) revealed
that **`*.cluster.integratn.tech` had been served expired since 2026-08-09** —
12 days — with cert-manager's `Ready=True "has not expired"` condition stale
and misleading. Every platform UI (ArgoCD, Grafana, Authentik, Open WebUI…)
and the vcluster Prometheus remote-write endpoint were affected.

- **Root cause**: renewal (scheduled 2026-07-10) was stuck on the Cloudflare
  DNS01 solver's cleanup step — `DELETE /zones//dns_records/…` → Cloudflare
  error 7003 — because cert-manager ≤1.16.3 read `zone_id` from the DNS-record
  listing, which the Cloudflare API stopped returning. The wildcard challenge
  validated but could never finish cleanup, so the order stayed pending forever.
  Upstream fix: cert-manager PR #7549 (1.16.4+ / 1.17.1+); the host ran v1.16.2
  while the vcluster layer already ran v1.19.3.
- **Fix**: PR #5 bumped the control-plane cert-manager addon to **v1.19.3**
  (same values shape as the vcluster layer); after rollout, the stuck
  `CertificateRequest …-2` was deleted to open a fresh ACME order, which issued
  in ~150s. One v1.16-chart leftover (`RoleBinding cert-manager-cert-manager-
  tokenrequest`) was pruned by hand because the addon intentionally runs
  without auto-prune.
- **Verified**: served cert now `notBefore 2026-08-21 / notAfter 2026-11-19`
  on all hostnames; both apps Synced/Healthy.
- **Residue**: one orphaned `_acme-challenge.cluster.integratn.tech` TXT record
  (id `caa61977…`) remains in the Cloudflare zone from the failed cleanups —
  harmless, remove manually.
- **Lesson**: cert-manager's Ready condition is not a monitoring signal. The
  `certmanager_certificate_expiration_timestamp_seconds` metric (scraped via the
  chart's ServiceMonitor) should back a `CertificateExpiringSoon` alert — an
  open follow-up.

# Live cluster findings (as found on 2026-08-20)

**Cosmetic / stale status**
1. `VClusterOrchestratorV2/vcluster-media` reports `Reconciled: Failing`
   ("A Configure Pipeline has failed: vco-v2-configure") while every health
   condition is True and all 35 pods run. Root cause: a **suspended, 170-day-old
   pipeline Job** from initial provisioning. This also produces ~61 warning
   events. Cleanup: delete the old job / force a reconcile
   (`hctl reconcile vcluster-media`). The docs even predict this class of
   false-Failing (kratix-troubleshooting "WorkPlacement Failing despite
   successful deployment").[^live-cluster]

**Orphaned resources (git intent removed, cluster remnants running)**
2. **llmkube** — addon disabled in commit `604c506`, but `llmkube-system`
   still runs the controller, a 0/1 `llama-3.1-8b` Deployment, InferenceService/
   Model CRs (some duplicated across `default` and `llmkube-system`), CRDs,
   and **two 50Gi model-cache PVCs**. No ArgoCD app manages any of it.
3. **git-indexer** — removed in commit `ce6ce24` (github-mcp replaced it), but
   the hourly CronJob remains in `ai` and **fails every run**
   (BackoffLimitExceeded, 3+ consecutive). Either delete the CronJob or
   re-adopt it. (`hctl ai reindex` still targets it.)

**Failing / degraded**
4. **Trivy scans**: 3 `scan-vulnerabilityreport-*` pods stuck `Init:Error` for
   88 days and **0 VulnerabilityReports exist** — scanning is effectively not
   producing results despite the operator running.
5. **OutOfSync ArgoCD apps** (4): `kratix-state-reconciler`,
   `kube-prometheus-stack-extras`, `network-policies`, `mcp-system-app`;
   `nginx-gateway-fabric` was Progressing at snapshot time. All Healthy —
   worth a diff-and-sync pass.

**GitOps gaps**
6. **mcp-system is applied out-of-band**: `mcp-system-app.yaml` is a plain
   Application manifest that no addon/kustomization applies — it exists in the
   cluster only because someone `kubectl apply`'d it. If the cluster is
   rebuilt from git, mcp-system won't come back.
7. The plain `nfs-subdir-external-provisioner` Deployment runs but backs **no
   StorageClass** (only config-/data- classes exist).

# Repo / tooling findings

**Security-relevant**
8. `.githooks/pre-commit` filters staged files on `promises-v2/` — a path that
   no longer exists — so the **local no-Secrets guard never fires**; only CI
   enforces it. One-line fix.[^repo-review]
9. `hack/setup-authentik-db.sh` takes a DB password as argv (process table /
   shell history exposure). `hack/get-kubeconfig.py` disables TLS verification
   for its ArgoCD calls.

**Broken-on-fresh-clone** — *both fixed 2026-08-21*
10. ~~`flake.nix` shellHook sources `./secrets.env` unguarded → `nix develop`
    errors until the gitignored file is created.~~ **Fixed**: the source is
    guarded and the shell prints a note instead of failing.
11. ~~`terraform/cluster/main.tf` reads gitignored `dockerconfig.json` at plan
    time → `tofu plan` fails on a clean checkout.~~ **Fixed**: the secret is
    created only when the file exists (`count = fileexists(...)`), and the
    `ghcr_login_secret` output reports which happened.

    Both mattered more than they looked: they are the first two steps of the
    documented disaster-recovery path, so the DR runbook could not be executed
    from a clean clone at all. The full path is still unexercised.

**Latent bugs**
12. hctl: `deploy run`'s git step captures its paths slice before population
    (adds nothing); a Score `route` renders two conflicting HTTPRoutes;
    Nix build omits the commit ldflag; `stage-only` gitMode falls through to
    a no-op.
13. Matchbox: arm64 worker group has a placeholder MAC (`…:xx`) and
    `default.json` references a nonexistent profile.
14. Addons chart: values-folder manifests are inert — the duplicate
    HTTPRoute files under `clusters/the-cluster/addons/{kube-prometheus-stack,loki}/`
    and `…/kratix/{promises,dashboards}/` are never applied (live copies come
    from other addons); `additionalResources` is silently ignored on
    `type: manifest` addons; addon-level `info:`/`annotations:` keys are
    dropped by the template.

**Doc drift (docs say X, code/cluster says Y)**
15. README/AGENTS.md don't mention **Cilium** (the actual CNI) and list stale
    versions (ArgoCD 9.0.3/9.4.3 vs running v3.3.1 server; MONITORING_SUMMARY
    lists Prometheus v2.51/Grafana 10.4 vs v3.9.1 running).
16. vcluster-orchestrator-v2 README/promise.yaml: dev preset memory
    (512Mi/1Gi vs code 768Mi/1536Mi), VIP offset (".100" vs code +200), and
    the delete-pipeline description are stale; `platform/vclusters/README.md`
    is a third disagreeing spec table.
17. `vcluster-media.yaml` sets `reconcile-at` under
    `spec.integrations.argocd.clusterAnnotations`, but the pipeline reads it
    from `metadata.annotations` — the sync-job re-run trigger never fires
    from that field.
18. `addons/README.md` documents a layer layout (`default/`,
    `clusters/in-cluster/`) that no longer exists; `addons/charts/cert-manager`
    is an unused stub chart; staging/development environment files are
    byte-identical latent config; AGENTS.md's bootstrap-app paths and
    copilot-instructions' `bootstrap/addons.yaml` reference are stale.
19. `.vscode/mcp.json` + settings hardcode another machine's absolute path
    (`/home/boboysdadda/...`).
20. Version pin mismatches: images/kubectl Dockerfile v1.34.4 vs workflow
    v1.34.1; trivy chart pins vs running images; unpinned `yq` download in CI.

# Suggested quick wins

Fix the pre-commit path filter (#8); delete or re-adopt the git-indexer
CronJob (#3); clean up llmkube remnants incl. 100Gi of PVCs (#2); bring
`mcp-system-app.yaml` into the addon framework (#6); delete the stale
suspended vco job (#1); sync the four OutOfSync apps (#5); investigate trivy
scan init failures (#4).

[^live-cluster]: Live cluster observation
[^repo-review]: Repository review
