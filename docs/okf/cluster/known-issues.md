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

*Update 2026-08-21:* the stored-event count fell from ~208k to 26,910 and then
**stopped falling** — 27,629 at the next measurement, i.e. flat-to-rising. That
is not slow GC, it is a second write loop still running: the
`kratix.io/manual-reconciliation: "true"` annotation on
`vclusterorchestratorv2s/vcluster-media` was set by hand on 2026-08-21T02:19:09Z
and never cleared, so Kratix re-triggers itself several times a second (8.69
`PUT/s` against a 0.03 baseline) and produces events as fast as GC removes them.
Clearing that annotation is the prerequisite; **`--event-ttl` is the wrong thing
to chase until it is cleared**, because the count is a symptom of the loop rather
than of retention. The annotation is not in git, so removing it is clearing
unmanaged manual state:

```bash
kubectl annotate vclusterorchestratorv2s.platform.integratn.tech \
  -n platform-requests vcluster-media kratix.io/manual-reconciliation-
```

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

A second independent resolver is therefore queued as
[`dns.yaml`](../../../matchbox/talos-machineconfigs/dns.yaml), unapplied — it
needs `talosctl`, and it carries two pre-checks, because the field *replaces* the
DHCP list and Talos distributes queries across upstreams rather than treating
later ones as failover-only.

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
