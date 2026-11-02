---
type: Operational Findings
title: Known issues & drift (snapshot 2026-08-20)
description: Everything found during a deep repo + live-cluster review that is broken, orphaned, cosmetic, or where docs/code disagree — with evidence.
tags: [known-issues, drift, findings, snapshot]
status: stable
generated: { by: claude-code/claude-opus-5-5, at: 2026-11-02T00:45:00Z }
stale_after: 2026-11-29
sources:
  - id: live-cluster
    resource: kubectl sweep of the-cluster (admin@the-cluster), 2026-08-20
    title: Live cluster observation
  - id: repo-review
    resource: full-repo review (source, docs, workflows, hooks), 2026-08-20
    title: Repository review
  - id: alert-sweep-0901
    resource: Prometheus /api/v1/alerts + kubectl sweep of the-cluster, 2026-09-01
    title: Alert triage sweep 2026-09-01
  - id: nfs-stale-1008
    resource: kubectl sweep of the-cluster + Prometheus ALERTS history, 2026-10-08
    title: Stale NFS mount triage 2026-10-08
  - id: kargo-wait-1029
    resource: kubectl get promotions -A, kargo-controller and bosun logs, GitHub pulls/statuses API, Kargo v1.12.5 source, 2026-10-29
    title: Orphaned Kargo waits triage 2026-10-29
  - id: disk-102-1029
    resource: talosctl dmesg / services / read /sys/block/sda/device/{ioerr,iotmo}_cnt on all three nodes, kubectl sweep, ladder.sh run log, 2026-10-29
    title: .102 SSD failure during the Talos ladder 2026-10-29
  - id: eso-frozen-op-1101
    resource: kubectl get application/crd --show-managed-fields on both clusters, server-side dry-runs of chart 2.12.0, Prometheus ALERTS history, Argo CD v3.5.1 controller source, 2026-11-01
    title: external-secrets stuck operation triage 2026-11-01
  - id: finite-retries-1101
    resource: throwaway Application probes on the-cluster, Prometheus range queries over 15d, Loki over 7d, promtool, Argo CD v3.5.1 controller source, 2026-11-01
    title: Finite sync retries decision 2026-11-01
---

# Sync retries are finite, and a wedged operation pages (decided 2026-11-01)

`retry.limit: -1` wedged kratix, mcp-system, both external-secrets apps (63 days)
and specmarshal. The chart default in
`addons/charts/application-sets/values.yaml` is now **`limit: 10`, with
`retry.refresh` off**, and every addon that restated `-1` now inherits it.

| Backoff | Addons | Time until the operation fails |
|---|---|---|
| 5s, x2, cap 10m | the default | about 41 minutes |
| 10s, x2, cap 5m | `argocd-vcluster`, `cert-manager-vcluster`, `nginx-gateway-fabric-vcluster` | about 30 minutes |
| 10s, x2, cap 5m, `limit: 30` | `metallb-config`, already finite and left alone | about 2h10m |

**Why a finite limit is enough.** Tested on a throwaway multi-source Application
in the factory's shape, against a namespace that did not exist, with a short limit:

- Retries ran out, the phase went `Failed`, and the app gained a `SyncError`
  condition.
- Changing only `spec.syncPolicy.syncOptions` and forcing a refresh started
  nothing. The revision had not moved, so `alreadyAttemptedSync` still matched.
- An unrelated empty commit started a new operation on the new revision, and it
  carried the changed `syncOptions`.

So the terminated operation is what lets every frozen field catch up. All 69
apps with a `HEAD` source tracked main's current commit when checked, so any
commit to main re-attempts every app in this state. A settings change in git is
itself such a commit.

**Why not `retry.refresh`.** The same probe with `refresh: true`, moving the spec's
source to a pinned revision and adding a syncOption mid-operation:

```
attempted (syncResult.revisions)    new commit, on both sources
spec.sources[1].targetRevision      87059530
syncResult.sources[1].targetRevision  still the frozen branch name
operation.sync.syncOptions          still the frozen list
operation.sync.revisions            still the old commit
```

`refresh` moves git sources to `HEAD` but not the chart version or inline values
in the frozen `Sources`. It would have helped the kratix, mcp-system and
specmarshal wedges, whose fixes were in git, and not external-secrets, whose fix
was a syncOption. Set alongside a limit, it saves at most 41 minutes, in exchange
for applying a chart and values no commit ever paired. The diagnostic in the
entry on retrying syncs also stops working, because `operation.sync.revisions`
no longer says what is being attempted. The deep merge is not the reason:
`refresh: false` and `limit: 0` both override correctly at addon level, and a
partial `retry` block inherits the rest.

**What the limit costs, measured.**

- In Loki's whole 7 day window, only the wedged apps logged `Retrying operation`.
  No other app retried once. The window has a 5.4 hour hole from 2026-10-29
  17:01Z, during the `.102` failure, so it says little about an outage that long.
  The only other success after retrying in the live `operationState` is
  `metallb-config`, 5 attempts over 11 minutes, in February.
- Only an app with an operation in flight spends its budget, so an outage does
  not exhaust all 73 at once.
- After an operation fails, auto-sync **and self-heal** skip that revision until
  the next commit. Since 2026-08-01 the gap between commits to main has a median
  of 8 minutes and a p90 of 8 hours, but the longest was 16 days, so a quiet
  stretch relies on the alert. `argocd app sync <app>` or any commit clears it.
- A rebuild from nothing that takes longer than the budget to settle will leave
  apps failed. Sync once more, or push a commit. This has not been exercised.

A limit does not end every wedge: **an operation that hangs without failing, such
as a hook that never completes, never reaches the retry path.** The controller's
`controller.sync.timeout.seconds` would end those, and is unset. It is not set
here because none has been seen; the alert below covers them.

**Detection: `ArgoCDAppSyncWedged`, critical, `for: 1h`.**

```
max by (name, project) (argocd_app_info{autosync_enabled="true", sync_status="OutOfSync"})
or
max by (name, project) (argocd_app_info{operation="sync"})
```

The controller sets `operation="sync"` for as long as `.operation` exists, retries
included, so that label is the in-flight signal. There is no operation-age metric.
It replaces `ArgoCDAppSyncFailing`, which had no rows in 15 days and, with one
operation now taking 41 minutes to complete, could not reach `>3` in an hour.

- **Backtest, 15 days and 73 apps at a 2 minute step:** every run of either branch
  longer than one sample was external-secrets or specmarshal, and the shortest of
  those was 2 hours, cut short by a metric gap or a termination. Healthy drift
  never lasted a second sample.
- **Real rule engine on a real wedged app:** a local Prometheus reading live data
  over `remote_read`, with no Alertmanager attached, took the rule from pending at
  00:02:38Z to firing by 01:03Z on `specmarshal-the-cluster`, retry 507 of its
  `-1` operation, `critical`. Delivery through `matrix-critical` needs it on main.
- **Unit tests with `promtool`:** fires on a retrying operation, on a hung
  operation on a `Synced` app, and on exhausted retries. Stays quiet on 2 minute
  drift, a 45 minute transient that recovers, and automation turned off.
  `keep_firing_for: 15m` carries it through an 8 minute series gap; removing it
  fails that test.
- `ArgoCDAppDegraded` does not silence it, and it now silences
  `ArgoCDAppOutOfSync`. Degraded already silenced that warning, which helped kratix
  read as only a workload problem for 43 hours.

Exhausted retries now reach `on-sync-failed`, whose condition is
`phase in [Error, Failed]`. A `-1` operation stays `Running`, so that notification
could never fire for the failure it is named after.

**New settings do not reach an operation already in flight.** An operation started
before this merged keeps `-1` until it is terminated. On 2026-11-01 that was only
`specmarshal-the-cluster`.

# .102's SSD is failing; it took etcd quorum with it for three minutes (2026-10-29)

The Samsung PM871b in 10.0.4.102 (`talos-4kg-ymz`) is returning medium
errors: `UNC`, *Unrecovered read error - auto reallocate failed*, and XFS
metadata I/O errors on `/var` (`sda4`). That is the drive, not the cable (no
`ICRC`, link stays at 6.0 Gbps) and not the 6.12.62 kernel (a drive does not
report a medium error because of a driver). Replacement (PNY CS900 250 GB)
arrives 2026-10-30; the procedure is
[replace-disk.md](../../../matchbox/talos-machineconfigs/replace-disk.md).

**Timeline.** .102 was upgraded to Talos 1.11.6 at ~16:33Z. At 17:06Z the
ladder drained .103 and its pods moved onto .101 and .102. At 17:08:51Z
.102's kubelet stopped heartbeating: the disk had started timing out (267 SCSI
command timeouts and 30 I/O errors before it was rebooted), XFS shut `/var`
down, and etcd and the kubelet could not restart (`failed rootfs umount: lstat
... input/output error`, forever). .103 was still rebooting, so **from 17:08
to ~17:11Z only .101's etcd member was up -- the host cluster had no quorum.**
It recovered when .103 returned. The ladder then hung for six hours on one
`talosctl etcd status`: a Talos API call against a node whose etcd is down
waits on machined's etcd client, which has no deadline. Fixed in ladder.sh
(`deadline` on every call, TERM then KILL -- Go ignores SIGALRM).

**Blast radius of a dead node that still answers ping.** 74 pods sat
`Terminating` on .102 for 6.5 hours: the node lifecycle controller marks them
for deletion, but only the kubelet can confirm it. Deployments replaced theirs
elsewhere; **StatefulSets cannot**, so Prometheus, Loki, Qdrant, Open WebUI,
the vcluster's ArgoCD controller and one vcluster etcd member were down the
whole time. Nothing alerted: Prometheus was one of the casualties. A
powercycle reboot of .102 (23:43Z) cleared it -- XFS log recovery succeeded --
but the disk logged 109 timeouts within 16 s of boot and XFS turned quotas off
on an unreadable inode cluster. It stays in service until the swap; its etcd
member is healthy and caught up.

**The early warning existed.** On 2026-10-16 .102's write latency was ~84 ms
against ~21 ms on .101's identical model and was attributed to load (below).
In hindsight that 4x gap on identical drives was wear. .101 carries the same
model; its counters are clean (0 timeouts over 1.67 billion I/Os). The cheap
check is `talosctl -n <ip> read /sys/block/sda/device/iotmo_cnt`: 0 on a
healthy disk here (`ioerr_cnt` is 8 on every boot and useless for this).
ladder.sh refuses to drain while any node's count is non-zero (`g_disks`).

**Until the swap:** do not reboot .101 or .103. With .102's disk in this
state, any second failure during a reboot is a quorum loss.


# A GitHub 503 ends a human-gated Kargo wait in one second, and the PR it watched is orphaned (2026-10-29)

Auto-merge was never broken. Of the 392 Promotions in the cluster, 173 ran
`git-merge-pr`, 173 merged their PR, GitHub rejected none, and each merge
landed 34–36 s after `git-open-pr` (the 30 s poll plus the ~30 s the gate
takes to render). The pile of green, open PRs merged by hand on 2026-10-29 was
the *other* branch of every Stage: `git-wait-for-pr`, the human-gated path,
whose Promotions had been dying on `GET /pulls/N: 503 []` and leaving the PR
behind with nothing watching it. Every hand-merged PR that day was on the wait
path by the policy in `kargo-projects/values.yaml`: loki 19.0.x and 20.0.x
(major against the 18.17.1 on main, under `minor`), podinfo 6.17.0 (minor
under `patch`, by design — it is the demo target), argo-cd 10.11.x and
unpoller v5.8.0 (minor under `patch`), trivy-operator 0.38.1 (a `0.x` minor),
kyverno 3.9.1 (gate red). Bosun's `/pipeline` report already carried the
podinfo and loki cases with the exact recovery commands.

## The mechanism, from source (Kargo v1.12.5)

`retry.errorThreshold` reads as "N consecutive failures". Two things it does
not do:

1. **A retry is immediate.** When a step returns a recoverable error the
   Promotion stays `Running` and the reconciler returns the error to
   controller-runtime (`pkg/controller/promotions/promotions.go`: *"Retryable
   error: use progressive backoff"*). Kargo sets no rate limiter of its own
   (`pkg/controller/options.go`), so the per-item backoff is the default: 5 ms,
   doubling, capped at 1000 s. Three attempts fit inside one second, and the
   podinfo wait proved it: `503` logged at 14:44:53 and 14:44:53, step
   `Errored` at 14:44:54.
2. **Success never resets the count.** `StepMetadata.Error()` is
   `ErrorCount++`; `Started()` zeroes it only while `StartedAt` is nil
   (`pkg/promotion/promotion.go`). A successful poll in between changes
   nothing, so a wait parked for a week accumulates every blip it meets. Waits
   that later timed out normally at 168h were carrying counts of 1 and 2 from
   blips days earlier.

With the chart's `errorThreshold: 3`, the first bodyless 503 that outlasts
~15 ms of backoff ends the wait. `Errored` is terminal: Kargo never
re-promotes that Freight, and the PR stays open, green, and orphaned. The
Stage's queue then advances to the next Pending Promotion, which opens a
*superseding* PR for the next version — still a major or a gated minor against
an unchanged main, so it parks too. That cascade is what a pile of
patch-titled, green, unmerged PRs looks like from outside.

The 503s were GitHub's, not the token's or the egress's: bosun's own log has
`GET /repos/.../pulls/546..549 returned 503: Service Unavailable` at
10:34:08–10:34:11, the same minute Kargo's four aligned polls (every wait had
been re-timestamped to 22:34 by the 1.12.5 upgrade on 10-27, so they polled in
step) got theirs at 10:33:55–10:34:00. No incident on githubstatus.com covered
any of the four windows; these are sub-incident blips of a few seconds.

## Per PR

| PR | Bump | Why it parked | Opened (UTC) | `addons-gate` | Wait ended | Cause |
|---|---|---|---|---|---|---|
| #471 | kyverno 3.9.1 | minor under `patch` | 10-16 16:03:28 | failure, 38 s | 10-23 04:03:44 | 503 |
| #509 | loki 19.0.0 | major under `minor` | 10-20 22:30:34 | success, 37 s | 10-23 04:03:45 | 503 |
| #510 | unpoller v5.7.0 | minor under `patch` | 10-21 01:29:54 | success, 24 s after the 10-27 re-push | 10-29 10:33:58 | 503 |
| #516 | kyverno 3.9.1 | minor under `patch` | 10-23 04:03:53 | failure | 10-29 10:33:57 | 503 |
| #517 | loki 19.0.1 | major vs main's 18.17.1 | 10-23 04:03:56 | success, 38 s | 10-29 10:34:00 | 503 |
| #520 | argo-cd 10.11.1 | minor under `patch` | 10-23 16:30:29 | success, 27 s | 10-29 10:33:55 | 503 |
| #552 | podinfo 6.17.0 | minor under `patch` | 10-29 12:44:35 | success, 30 s | 10-29 14:44:54 | 503 ×3 in one second |
| #561 | loki 20.0.0 | major under `minor` | 10-29 13:44:15 | success, 37 s | 10-29 15:19:32 | 503 ×3 in one second |

The gate answered every push within 40 s. The 45-minute `git-merge-pr`
timeout was never involved; neither was the token (replaced 2026-10-06), the
ruleset (squash allowed, no review required, `strict` off), or the egress.

What the hand-merges released: once #549 put loki 19.0.2 on main at 12:28Z,
19.0.3, 19.1.0, 19.1.1, 19.2.0, 19.2.1 and 19.3.0 merged themselves in six
PRs (#551–#559) between 12:40Z and 13:34Z, each 34–36 s after opening, until
20.0.0 (#561) parked again, correctly. unpoller v5.8.1–v5.8.3 did the same
behind the hand-merged v5.8.0.

## Fix, and what it does not fix

- `merge.errorThreshold: 100` in `kargo-projects/values.yaml`. With the 5 ms
  doubling capped at 1000 s, errors 1–18 cost ~22 minutes of continuous outage
  and each one after ~17 minutes, so a wait survives ~23 hours of GitHub being
  down, or a few dozen blips across its week. The per-step `timeout` still
  ends it. **Only Promotions created after ArgoCD syncs the Stages carry it**;
  a Promotion snapshots its steps at creation, so the waits in flight (loki
  20.0.1 on #562, kyverno 3.9.1 on #546, the re-created podinfo 6.17.0 on
  #564) keep `3`.
- The cost: a persistent provider error — a revoked token, a deleted PR —
  shows as a `Running` Promotion whose `status.message` carries the error for
  up to a day, instead of an `Errored` one within a second. A dead token still
  fails fast at `push`, whose threshold is Kargo's default of 1; that is how
  the 2026-09-20 expiry surfaced.
- **Not fixed: `git-merge-pr`.** Kargo wraps every error from the merge call
  as terminal, including a 503 on the PR read it makes first
  ([akuity/kargo#6811](https://github.com/akuity/kargo/issues/6811), closed
  2026-08-14 without a change; still so in v1.12.5 and on `main`). A blip in
  the ~35 s a policy-allowed PR lives would orphan it exactly as James
  described, with a green gate and a `Failed` Promotion. 173 merges, zero so
  far. The upstream report for both halves is in the PR that carries this fix.

## Recovery for an orphaned wait

Bosun's `/pipeline` prints both commands per finding, and they are the only
two that work. A Warehouse refresh re-discovers artifacts and does nothing for
Freight whose Promotion already ended.

```bash
# the Stage: re-create the Promotion for the same Freight (opens a fresh PR
# on a fresh branch; the old PR is then superseded)
kubectl -n addons create -f - <<'EOF'
apiVersion: kargo.akuity.io/v1alpha1
kind: Promotion
metadata:
  generateName: podinfo
  namespace: addons
spec:
  stage: podinfo
  freight: <freight name from `kubectl -n addons get freight`>
EOF
# the PR: close the one nothing watches any more
gh pr close <old> --comment 'superseded by #<new>'
```

# Stale NFS handles strand pods in CreateContainerError, and the alerts for it were ignored for 18 days (2026-10-08)

Five host pods were found stuck on stale `config-nfs-client` mounts in one
afternoon: Prometheus and specmarshal (since 2026-10-07 20:51Z), Grafana
(23:53Z), the Kratix controller (since **2026-09-20**), and the
`config-nfs-client` provisioner, which went the same way mid-triage after
losing its lease to an etcd timeout.[^nfs-stale-1008] The kubelet keeps a pod's
stale mount and never remounts it, so the next container start fails with
`CreateContainerError` (`failed to stat ...kubernetes.io~nfs/pvc-...: stale NFS
file handle`) or `CreateContainerConfigError` (`failed to prepare subPath`)
until the pod is deleted. Every replacement mounted cleanly. The parent-export
mount is **not** immune: the provisioner's own `nfs-client-root` went stale.

What it cost:

- **Prometheus recorded nothing from 2026-10-07 20:50Z to 2026-10-08 17:40Z.**
  Earlier history is intact. `Watchdog` was routed to `null`, so nothing
  outside the cluster noticed.
- **The 2026-10-08 `vcluster-media` etcd snapshot failed** (`no route to host`
  to the etcd ClusterIP, both attempts, deadline at 04:00Z). That is what
  surfaced all of this, via `platform-backups-the-cluster` going Degraded. A
  manual run at 17:48Z closed the gap.
- **Kratix did not reconcile for 18 days.**

It was not undetected. `KubePodNotReady`, `KubeContainerWaiting`,
`KubeDeploymentReplicasMismatch` and `TargetDown` fired for
`kratix-platform-system` continuously for the whole 18 days. They are warnings,
which repeat once a day in the digest. Nothing self-heals this state either:
the descheduler protects pods with PVCs and a container that never starts
accumulates no restarts, `cleanup-failed-pods` only matches phase `Failed`, and
`NFSMountErrors` watches retransmissions, which a stale handle does not cause.

Long-running pods are not evidence of health. Alertmanager was still `Running`
on 2026-10-08 with its data directory already stale (`chdir to cwd
("/alertmanager") ... stale NFS file handle` on exec); it will stick on its
next restart.

**Fix, merged 2026-10-15 (#399):** a Kyverno cleanup policy that deletes owned pods stuck in either
state; critical `PodStuckCreatingContainer` and `DeploymentHasNoReadyReplicas`/`StatefulSetHasNoReadyReplicas`
alerts; and `Watchdog` pushed to an external dead-man's switch. A pod that
keeps *running* on a stale volume (Alertmanager, above) is caught separately:
Promtail counts "stale file handle" log lines and `PodLoggingStaleFileHandle`
pages on them; nothing deletes that pod automatically. The server-side cause
(Unraid shfs invalidating handles) is untouched.

# Trivy scan coverage silently fell to zero, and one report 4% too big did it (2026-09-01)

`the-cluster` held **2 VulnerabilityReports**. The `trivy-operator` values file
records 149 as recently as 2026-08-21. Every ArgoCD Application was
`Synced`/`Healthy`, no alert fired, and nothing in the alert set watches scan
coverage at all.[^alert-sweep-0901]

The scanner had been wedged since `2026-08-28T09:07`. Thirteen scan Jobs were
created in that one second; nine of their Pods died on a Trivy fs-cache lock
timeout, and two — both for `ghcr.io/goauthentik/server:2025.12.4` — entered a
retry loop that never ends:

```
Reconciler error ... controllerKind=Job Job=scan-vulnerabilityreport-6c985449f5
error="rpc error: code = ResourceExhausted desc = trying to send message larger
than max (2190230 vs 2097152)"
```

The VulnerabilityReport for that image is **2190230 bytes against the 2097152
apiserver/etcd gRPC ceiling — 4% over**. It can never be written, so the Job
can never succeed, so it retries every ~17 minutes forever. With
`operator.scanJobsConcurrentLimit: 3`, those two Jobs hold two thirds of the
scanner's capacity permanently. The queue stops draining, existing reports age
out, and the count walks down to nothing.

Three things make this hard to see:

- **The margin is tiny.** A routine authentik image bump was enough to cross
  the line. Nothing about the change looks like it touches the scanner.
- **The Jobs report `active=0 succeeded=0 failed=0`.** They are not `Failed`,
  so `KubeJobFailed` never fires and a `kubectl get jobs` skim reads as idle.
- **`scanJobTTL: 30s` only reaps completed Jobs**, so the stuck ones accumulate
  rather than being cleaned up.

Fixed in git by `trivy.ignoreUnfixed: true` — unfixed findings are not
actionable, the same rationale the `severity` list already carries, and it buys
far more than the 4% needed. **The observability gap is still open**: there is
no alert on `count(vulnerabilityreports)` collapsing, and per the house pattern
(see `KargoMetricsMissing` in [observability](/platform/observability.md)) a
dead-man's-switch on scan coverage is the only thing that would have caught it.

Cleanup still owed once the fix reconciles: the 13 stale Jobs and 9 `Failed`
Pods in `trivy-system` must be deleted so the operator recreates them under the
new configuration. They are orphans — nothing in git manages them individually.

# Two of cert-manager's three scrape targets had never been reachable (2026-09-01)

`TargetDown{job="webhook"}` and `TargetDown{job="cainjector"}` had been firing
at 100% since 2026-08-22, and are the older half of a policy that has probably
been wrong since it was written.[^alert-sweep-0901]

Pods `Running`, EndpointSlices populated on 9402, and Prometheus reporting a
**timeout rather than a refusal** — the house signature for a NetworkPolicy
drop:

```
Get "http://10.244.0.245:9402/metrics": context deadline exceeded
```

`network-policies/cert-manager.yaml` carried three policies and only the first
admitted monitoring:

| Policy | podSelector | Ingress allowed |
|---|---|---|
| `allow-cert-manager` | `component: controller` | monitoring → 9402 ✅ |
| `allow-cert-manager-webhook` | `component: webhook` | node IPs + pod CIDR → **10250 only** |
| `allow-cert-manager-cainjector` | `component: cainjector` | `ingress: []` — **deny-all** |

cert-manager v1.21.1 serves metrics on 9402 from all three components, and the
chart ships a **single** ServiceMonitor whose selector matches all three. So
two of its three targets were denied from the start; only the controller ever
scraped. Fixed by adding the monitoring:9402 rule to both policies.

The generalisable form: a ServiceMonitor that selects several components needs
a matching ingress rule for **each** podSelector the namespace's policies carve
out. `ingress: []` under `policyTypes: [Ingress]` is deny-all, not "no
restriction" — it reads like an empty placeholder and behaves like a wall.

# The LM Studio endpoint does not enforce its API key (found 2026-08-22)

Bosun authenticates to the model endpoint on the workstation
(`192.168.0.57:1234`) with a bearer token from 1Password
(`bosun-llm`). The token is attached correctly and inference works —
but the endpoint accepts requests **without** it.

Tested from inside the agent's own pod, so through the real network path:

| Request | Result |
|---|---|
| real key | `200`, completion returned |
| deliberately wrong key | `200` |
| no `Authorization` header at all | `200` |

So the credential is currently decorative, and **any host on the LAN that can
route to `192.168.0.57:1234` can spend the workstation's GPU**. The cluster
side is constrained — the agent's egress is a `/32` and Kargo reaches only the
agent — but nothing constrains the endpoint itself.

The configuration is right and needs no change: the key is sent, so switching
enforcement on (or moving to a hosted provider) is a 1Password edit rather than
a redeploy. What is wrong is the assumption that it is *already* enforced.

Fix is at the LM Studio end, not in this repo.

# An NGF data plane replica can serve stale config forever (2026-08-22)

Found chasing an ArgoCD UI that answered **3 of 8 requests** and timed out on
the rest, while grafana, kargo, auth and chat were 8/8 through the same
gateway. Every ArgoCD Application was `Synced/Healthy`, both argocd-server pods
`Running 1/1` and `ready=true` in the EndpointSlice, and nothing was logged --
the requests never reached either pod.

The two NGINX data plane replicas disagreed about where the backend was:

| pod | upstream for `argocd_argo-cd-argocd-server_80` | |
|---|---|---|
| `…-b6s94` | 10.244.0.159, 10.244.1.150 | current |
| `…-hr47x` | 10.244.1.193, 10.244.2.225 | pods that no longer existed |

`hr47x` had been serving a config snapshot from its own start time
(2026-08-22T03:28:47Z) ever since. The control plane says why -- across both
`nginx-gateway-fabric` replicas, `nginxUpdater.commandService` logs
`Creating connection for nginx pod: …-b6s94` repeatedly and **never once names
`hr47x`**. The data plane dials the control plane for its config; that
connection was never established, and nothing retried it or reported it.

**Nothing surfaces this.** The pod is Ready (its own health check passes -- it
is a working NGINX, just pointing at dead addresses), the Gateway is
`Programmed=True`, the HTTPRoute is Accepted, and ArgoCD is Healthy because
desired state matches. The only symptom is that roughly half of requests to
whichever backends have moved since that snapshot hang, and which half depends
on which replica the LB picks.

Any backend whose pods reschedule after a data plane replica goes deaf is
exposed; ArgoCD was simply the one that had rolled (its pods were replaced by
the 10.4.0 bump and again by the extension fix).

**Remediation is a restart of the affected replica** -- runtime state, nothing
to change in git:

```bash
kubectl -n nginx-gateway delete pod <nginx-gateway-nginx-…>
```

Deleting the *stale* one is safe even single-handed: the surviving replica is
the one with correct config, so availability goes up immediately. After the
restart, all five hostnames returned 10/10.

**Worth building on:** the leader-election lease renewals in the same control
plane log time out against the kube-apiserver repeatedly
(`context deadline exceeded` at 10:10, 10:40, 12:00, 12:01, 13:20Z), which is
the etcd/DNS problem below. A control plane that keeps losing the apiserver is
a plausible trigger for a command connection that never gets set up. A
comparison of each data plane pod's upstreams against the live EndpointSlices
would detect the state directly and is the obvious alert to add.

# Promtail is EOL and has nowhere to move (2026-08-22)

Found while checking what else should follow Loki to the community charts.
Grafana has deprecated almost its entire community chart estate — `grafana`,
`promtail`, `tempo*`, `loki-*`, `grafana-agent-operator`, `grafana-mcp`,
`lgtm-distributed`, `fluent-bit` are all `deprecated: true` in
`grafana.github.io/helm-charts`. Only three of those touch this cluster, and
each lands differently:

| chart | status here |
|---|---|
| `loki` | **moved** to `grafana-community/loki` 18.11.0 |
| `grafana` | **already community** — no action needed |
| `promtail` | **stranded** — no fork exists |

**Grafana needed nothing.** kube-prometheus-stack 88.5.3 declares its `grafana`
dependency against `https://grafana-community.github.io/helm-charts` at 12.11.1,
so the migration already happened upstream of us. That is why the cluster runs
Grafana **13.2.0** while the deprecated `grafana/grafana` chart tops out at app
12.3.1 — nothing in this repo pins the grafana chart directly.

**Promtail is the real exposure.** The community org forked `grafana`, `loki`,
`tempo`, `tempo-distributed`, `tempo-vulture`, `synthetic-monitoring-agent` and
`grafana-mcp` — **not** `promtail`, because Promtail the *software* is EOL, not
just its chart. The last release is chart 6.17.1 / app **3.5.1**, published
2025-10-31, and the `promtail` addon is already pinned there. So there is no
version to bump and no repository to switch to: the DaemonSet feeding every log
line on the platform is frozen and will not receive security fixes.

The successor is **Grafana Alloy**, whose chart is alive and unaffected by any
of this (`grafana/alloy` 1.11.1 / app v1.18.1, 2026-08-06 — still in
`grafana.github.io/helm-charts`, not deprecated). Migrating is a real piece of
work rather than a pin change: Alloy replaces Promtail's YAML `scrape_configs`
with River/Alloy configuration blocks, and the host DaemonSet is what covers
vcluster-synced pods (`promtail-vcluster` is disabled by design), so the
cutover has to keep `/var/log/pods` tailing and the `cluster`/`environment`
external labels intact or the tenant loses its logs silently.

Kargo's `promtail` target will never propose anything again, which means this
will not resurface on its own.

# Four Kargo bumps were held, each for a different reason (2026-08-22)

Twelve of Kargo's sixteen day-one PRs merged. These four were open on purpose —
none is a version bump you can just take, and each needed a decision rather
than a config tweak. Loki's is resolved; three remain. Kargo will keep them parked (`git-wait-for-pr`), which also
holds back later versions of the same artifact until they are dealt with.

## The vcluster syncer does not add a new key to an already-synced Secret (found 2026-10-15)

When vcluster-media's agent moved to kube-prometheus-stack 92.x, the operator
added a second key, `1_monitoring_kube-root-ca.crt_ca.crt`, to the virtual
Secret `monitoring/prometheus-kube-prometheus-stack-prometheus-tls-assets-0`.
The host copy the pod actually mounts
(`vcluster-media/prometheus-kube-prometheus-stack-prometheus-tls-asse-f7b06baaf4`)
kept only the old key, and its resourceVersion never moved, even across a
restart of the `vcluster-media` StatefulSet. The agent failed every config
reload (`unable to read CA cert /etc/prometheus/certs/1_monitoring_kube-root-ca.crt_ca.crt`,
`prometheus_config_last_reload_successful 0`), and the last good scrape config
stayed loaded, so it looked healthy. Value changes to other synced Secrets do
propagate; the miss is specific to a key being added.

**Fix**: delete the host copy. The syncer recreated it from the virtual Secret
within 8 seconds with both keys, and the config-reloader succeeded about a
minute later once the kubelet refreshed the mount. Expect the same on any
future tls-assets change inside a vcluster.

## A CustomResourceState config change does not reach kube-state-metrics (found 2026-08-22)

`kargo-observability` ships the CustomResourceState ConfigMap that
kube-state-metrics reads, deliberately, so the two charts do not fight over
ownership of the same object. kube-state-metrics reads that file **once, at
startup**, and nothing watches it. When [PR #89](https://github.com/JamesAtIntegratnIO/gitops_homelab_2_0/pull/89)
corrected the metric names, ArgoCD updated the ConfigMap within a minute and
the running pod carried on emitting the old series; the fix only took effect
after a manual `kubectl -n monitoring rollout restart deploy/kube-prometheus-stack-kube-state-metrics`.

The usual answer is a Reloader annotation, which is not available here — see
below. Until one exists, **any change to the CRS config needs a deliberate
kube-state-metrics rollout**, and the symptom of forgetting is metrics that
silently stay stale rather than an error. (On 2026-10-15 the 0.1.2 change was
picked up without a restart, so recent kube-state-metrics may reload it; verify
with a fresh scrape rather than assuming either way.)

## `enable_stakater_reloader` is a dangling flag (found 2026-08-22)

`the-cluster` carries `enable_stakater_reloader: "true"`, and there is **no
addon definition for it anywhere in `addons/`**. No ApplicationSet generates
it, no ArgoCD Application exists, and no Deployment runs — `kubectl get deploy
-A | grep -i reloader` returns nothing. The label has presumably been set since
the cluster was built.

Consequence beyond the cosmetic: `reloader.stakater.com/auto` annotations are
inert on this cluster, so no workload picks up a ConfigMap or Secret change
without a rollout of its own. That is what makes the CRS issue above a manual
step rather than a one-line annotation.

## external-secrets → 2.10.0 — the API removal landed; the rewrite it needed did not (updated 2026-08-30)

This entry used to describe a three-step path, **none of which is done**. The
upgrade has since taken steps 1 and 3 and skipped step 2. `external-secrets` is
on 2.10.0 and the live CRD serves `v1` only:

```
$ kubectl get crd externalsecrets.external-secrets.io \
    -o jsonpath='{range .spec.versions[*]}{.name} served={.served}{"\n"}{end}'
v1      served=true
v1beta1 served=false
```

So the repo spent some number of days emitting an apiVersion the cluster no
longer serves. The bill arrived on 2026-08-30: `kratix-state-reconciler` had
been `OutOfSync` since 04:54Z with

```
one or more synchronization tasks are not valid:
ExternalSecret.external-secrets.io "" not found (retried 5 times)
```

**The empty name is the tell.** It is not a missing object — it is ArgoCD
failing to resolve the GVK, so it has nothing to name. All three ExternalSecrets
the app manages are `Ready=True`: they were created while `v1beta1` was served
and the apiserver has since rewritten them to `v1` storage. Only *re-applying*
them fails, which is why this presents as a stuck sync on a healthy app rather
than as broken credentials.

Step 2 is now done in code — **18 references across 11 files**, verified by
`grep -rn 'external-secrets.io/v1beta1' . --exclude-dir=.git` returning only
this file, which narrates the migration and keeps the old string on purpose.
The five that mattered are the manifest generators:
`promises/argocd-cluster-registration/workflows/resource/configure/builders.go`
(3) and `promises/external-secret/workflows/resource/configure/main.go` (2).
Two more in `cli/pkg/provisioners/provisioners.go` had hctl generating
manifests that could not apply; the rest were examples in docs, the pre-commit
hook's error message, and the `mcp-system` system prompt — all of them places
someone or something copies from.

**Not clean yet, and it cannot be fixed from this repo directly:** the generated
files in `kratix-platform-state` still say `external-secrets.io/v1beta1`,
because that repo holds pipeline *output*. It clears on its own — a merge to
`main` builds the pipeline images, Kargo bumps the promise pins (`autoMerge:
always` for both targets), and Kratix re-runs the workflows, which rewrites the
files. `kratix-state-reconciler` goes `Synced` at that point and not before, so
a red app immediately after the merge is expected, not a regression.

## The host CoreDNS Corefile is Talos-owned, and the platform needs it not to be (found 2026-08-30)

`auth.cluster.integratn.tech` resolves to the MetalLB VIP `10.0.4.205`, which is
unreachable *from inside a pod* — Cilium with kube-proxy replacement does not
hairpin traffic to a MetalLB L2 address, so the packet gets "no route to host".
The fix is a `rewrite` line in the host Corefile sending the name at the gateway
Service instead, which keeps the hostname (and so the TLS SNI and the OIDC
issuer claim) intact while taking a path that exists. Without it the Kargo UI's
login is a hang with zero bytes.

That line lived only in the cluster. Talos owns `kube-system/coredns`: it ships
as bootstrap manifest `11-core-dns`, rendered by `k8s.ManifestController` and
re-applied on every boot and every machine config change.

```
talosctl -n 10.0.4.101 get manifest 11-core-dns -o yaml \
  | yq '.spec[] | select(.kind=="ConfigMap") | .data.Corefile'
```

**`cluster.coreDNS` in the machine config carries only `disabled` and `image`.**
There is no Corefile field, so "put it in the Talos config instead" is not
available — the choice is to out-live Talos's writes or to take the component
over entirely.

Now covered by the `coredns-host-config` addon
(`addons/cluster-roles/control-plane/addons/coredns/`), which owns the ConfigMap
with `selfHeal: true` and `prune: false`. This is deliberately the second-best
option: Talos still wins the instant after a reboot, ArgoCD reverts it within
the sync interval, and CoreDNS's `reload` picks the file up ~30s later. A reboot
costs a few minutes without the rewrite instead of losing it until someone
notices. `prune: false` because pruning `kube-system/coredns` is a cluster-wide
DNS outage.

The clean version is **half done as of 2026-08-30**, and the order is the whole
risk: **ArgoCD takes ownership first, Talos is switched off second.** The
reverse leaves a window where Talos has stopped recreating CoreDNS and nothing
else has started.

**Step 1 is landed and verified** (PR #303, `a84e7e1`). The `coredns-workload`
addon is `enabled: true` and `coredns-workload-the-cluster` is `Synced`/
`Healthy` over five objects — ServiceAccount, ClusterRole, ClusterRoleBinding,
Deployment and the `kube-dns` Service — each now carrying an
`argocd.argoproj.io/tracking-id`. It behaved as an adoption exactly as the empty
`kubectl diff` predicted: **both CoreDNS pods kept their names and their AGE
(133m before the merge, 138m after) with 0 restarts**, and the staged image
`registry.k8s.io/coredns/coredns:v1.12.4` matched the live Deployment's. That
pod-AGE check is the one that matters — a restart there means the captured
manifests have drifted from what Talos renders, and the cutover stops rather
than continuing. Resolution verified afterwards from a policy-allowed pod:
`kubernetes.default.svc.cluster.local` → `10.96.0.1`, `github.com` → resolves,
and the rewrite `auth.cluster.integratn.tech` → `10.107.19.15`, which is the
`nginx-gateway-nginx` ClusterIP.

Worth knowing before the next person smoke-tests DNS the obvious way: **an
ad-hoc `kubectl run` pod in `kube-system` cannot resolve anything**. That
namespace carries the Kyverno-generated `default-deny-all`, and only the named
components have allow rules, so `nslookup` from a throwaway pod times out with
"no servers could be reached" whether CoreDNS is healthy or not. Exec into an
existing pod instead.

**Step 2 landed the same day.** `live-coredns-disabled.yaml` was applied to all
three nodes; `talosctl get manifests | grep core-dns` now returns nothing
anywhere, `coreDNS.disabled: true` reads back from all three machine configs,
and the `coredns` ConfigMap is owned solely by `coredns-host-config-the-cluster`
— the `selfHeal` race with Talos after a reboot is over, not merely won on
points. **The pods went through the entire cutover untouched: same two names,
145m AGE, 0 restarts**, and no reboot or static-pod restart on any node.

Two nodes printed `WARNING: extra kernel arguments are not supported when
booting using SDBoot` during the patch. Unrelated to CoreDNS — it is `all.yaml`'s
`extraKernelArgs` against a UKI-booted node, and `talosctl get securitystate`
says why only those two: `10.0.4.101` and `10.0.4.102` report
`bootedWithUKI: true`, `10.0.4.103` does not. Expect it on any `patch mc`
against those two.

**What the handover did not do, despite the runbook saying it would.**
`commands.md` claimed taking CoreDNS over "re-enables `cluster.local` caching,
which the Talos default Corefile disables". It does not: the Corefile was
captured from Talos verbatim, so `disable success cluster.local` and
`disable denial cluster.local` came across with it and are still live. Owning
the file makes deleting those two lines a one-line git change — it does not
delete them. That matters more than a doc slip, because disabled `cluster.local`
caching is the **amplifier** in the vcluster-media restart storms below: with it
off, every internal lookup is a live query, so a stalled upstream saturates
CoreDNS rather than being absorbed by cache.

Also still true: `forward . /etc/resolv.conf` continues to point at the Talos
host-DNS proxy. The single-upstream problem is untouched, and
`machine.features.hostDNS.forwardKubeDNSToHost: false` remains its own task.

What it now costs: **a Talos upgrade no longer bumps CoreDNS, and nothing else
does either.** The image is still Talos's unpinned tag
`registry.k8s.io/coredns/coredns:v1.12.4`, left that way precisely so the
adoption was a no-op. That was correct for the cutover and is wrong from here:
until it is digest-pinned and added to the Kargo target list in
[kargo-projects/values.yaml](../../../addons/cluster-roles/control-plane/addons/kargo-projects/values.yaml),
it stays on v1.12.4 forever. **This is the open follow-up.**

Found while writing the patch: **`talosctl patch mc --dry-run` prints the whole
machine config as a diff, embedded PEM private keys included.** Read it, do not
paste it.

## A retrying ArgoCD sync pins its revision and never re-plans (found 2026-08-30)

**This is why merging the fix did not fix it.**

`kratix-the-cluster` carries `retry.limit: -1` — unlimited retries. On
2026-08-28 at 20:35:34Z an automated sync started, applied the Deployment
(which is where the tini crashloop below came from), and then failed on a
resource the cluster could no longer serve:

```
one or more synchronization tasks are not valid: failed to discover server
resources for group version external-secrets.io/v1beta1: the server could not
find the requested resource. Retrying attempt #270 at 5:14PM.
```

It was still retrying **44 hours and 270 attempts later** — against revision
`d0f430a3`, a commit from PR #247, where every ExternalSecret in `addons/` was
still `v1beta1`.

**An in-flight operation is pinned to the revision it started on.** The
Application reconciled forward perfectly well — `status.sync.revisions` tracked
current `main` and reported `OutOfSync` — while underneath it
`status.operationState.operation.sync.revisions` stayed frozen on the old
commit. The two are different fields and they disagreed for two days. So no
number of new commits could rescue it: the fix landed on main, the app noticed,
and the operation carried on retrying a manifest set from months earlier.

Diagnose it by comparing exactly those two:

```bash
kubectl -n argocd get application <app> -o json | jq -r '
  "opPinnedTo=\(.status.operationState.operation.sync.revisions[0][0:8])
   appNowAt=\(.status.sync.revisions[0][0:8])
   started=\(.status.operationState.startedAt)
   retries=\(.status.operationState.retryCount)"'
```

If `opPinnedTo` differs from `appNowAt`, the operation is stale. The cure is to
terminate it so the controller re-plans from git — `argocd app terminate-op
<app>`, or the same thing without a CLI session:

```bash
kubectl -n argocd patch application <app> --type merge \
  -p '{"status":{"operationState":{"phase":"Terminating"}}}'
```

A fresh operation started immediately on current `main`, retry count reset to
0, and went `Synced`/`Healthy` inside a minute.

**What makes this dangerous is that nothing looks broken.** There is no
`Errored` phase and no failed Promotion — the phase is `Running`, which reads
as progress. `ArgoCDAppDegraded` fired, but on the *health* of the workload,
not on the age of the operation. Nothing anywhere alerts on "a sync has been
retrying for two days", and `retry.limit: -1` guarantees it never converts into
a failure that would.

## A patch bump removed every kubernetes-mcp flag, and the rollout sat stuck for nine days (found 2026-11-01)

**Fixed by #586, confirmed live 2026-11-02.** `mcp-system-the-cluster` is
`Synced`/`Healthy` with one Running pod on v0.0.69, 0 restarts, and the old ReplicaSets at
0. Through `mcp.cluster.integratn.tech/kubernetes` the server lists the 13 tools the
local boot predicted, real calls answer from the API server, `pods_delete` is an unknown
tool, and the alerts have cleared. What follows describes the state before that.
Calls through `mcpo` still fail, for a reason this did not cause: see the next entry.

Upstream's Config rework shipped in v0.0.68 and removed every runtime flag. Only
`--config`, `--config-dir` and `--version` remain, so the args this Deployment had
always passed now exit 1 with `Error: unknown flag: --port`. Kargo merged v0.0.67 →
v0.0.69 on its own (#522, 2026-10-23), because under `autoMerge: minor` a `0.0.x`
release is a patch whatever it contains. The release notes say only "Config rework";
the mapping is in upstream's `docs/configuration-changes.md`.

Everything worked as designed except the outcome:

- The `addons-gate` could not see it. A bump changes one image string, and whether a
  container accepts its args is not in any render.
- Kargo's verification did catch it: AnalysisRun `Failed` at 2026-10-23T18:11Z. That
  does not fail a promotion, and it did not stop the next one either.
- `ArgoCDAppDegraded` (critical) has been firing for about 8 of the 9 days, and
  `KubeDeploymentRolloutStuck` with it. The rolling update kept the old pod serving, so
  no user felt it and nobody acted on the alerts.

**Behaviour that changes with the fix**, measured by booting the pinned images
against the live v0.0.67 tool list:

- `configuration_view` is gone. v0.0.68 hides it in HTTP mode, and restoring it means
  an `enabled_tools` allowlist of every tool. Nothing in the `mcpo` system prompt uses
  it. The `config` toolset now contributes no tools here; it stays listed so the
  config matches the old args one for one.
- The other 13 tools keep their names and required parameters, and gain `limit` and
  `continue`.
- `max_backend_response_bytes` now defaults to 4 MiB, and a list over it errors rather
  than truncating. It is not close: the tools ask for table output, and a cluster-wide
  `pods_list` returned 84 KB live, where the same list as raw JSON is 2.7 MB. Set the key
  in `kubernetes-mcp-config` if a list ever does fail.

**Two things that must stay put.** The settings live in a separate file because Kargo
only rewrites the first YAML document of the Deployment's file. And `args:` stays
directly under `image:`: that one unchanged line is what lets a Kargo bump PR such as
#567 still merge cleanly beside a change to the args.

## `mcpo` answers no tool call on the kubernetes, grafana, github or argocd paths (found 2026-11-02, open)

Every call returns `500: {'message': 'MCP session is not available'}`, on all four
backends, while `/openapi.json` stays 200, so both probes and `mcp-system-the-cluster`
report healthy. Those four servers work directly: `kubernetes-mcp` was called through the
gateway at the same time. It is not from the #586 rollout, since `grafana`, `github` and
`argocd` fail identically and did not restart.

The current pod started 2026-11-01T19:34Z and its log holds no tool call before the three
made to test this, so it may never have worked, and nobody has been calling it. Its
startup log shows `prometheus` and `fetch` failing to fetch from `pypi.org`, then `umami`
ending in `CancelledError`. **Unproven:** that cancellation running mcpo's teardown, which
sets `app.state.session = None`, would produce exactly this error.

Its `/kubernetes` schema also still lists `configuration_view`, because mcpo reads the tool
list once at startup. Nothing in git points Open WebUI at mcpo, so what depends on it is
not established either.

## A change that only removes a key never reaches the cluster (found 2026-11-01)

`kratix-state-reconciler` ignores `kubectl.kubernetes.io/last-applied-configuration`
on the Kratix request kinds and on `Application`
(`addons/cluster-roles/control-plane/addons/kratix/values.yaml`, added 2026-02-08 to
silence metadata drift). With that annotation normalised away, ArgoCD no longer
appears to see a key that git removed: **it reports `Synced` and never applies the
removal.** That mechanism is inferred from the behaviour below, not read from
ArgoCD's source.

#582 removed two integration blocks from the pipeline's output. The new pipeline
ran, the state repo changed, and `ArgoCDApplication/vcluster-vcluster-media` still
held both blocks with the app `Synced` for 13 minutes. A targeted sync reported
`configured` and removed them at once, and the final `Application` needed the same.

A change that also adds or alters a key is seen, and its `kubectl apply` removes
the stale keys too. So a removal can sit unnoticed and then arrive with the next
unrelated change, which is how this would have rolled into rung 1 had nobody checked.

To release one, with the revision taken from the state repo's head:

```bash
kubectl -n argocd patch application kratix-state-reconciler --type merge -p '{"operation":{"initiatedBy":{"username":"<you>"},"sync":{"revision":"<sha>","prune":false,"resources":[{"group":"platform.integratn.tech","kind":"ArgoCDApplication","name":"vcluster-vcluster-media","namespace":"platform-requests"}]}}}'
```

Then the same for `group: argoproj.io`, `kind: Application`, `namespace: argocd`, once
the sub-request's pipeline has updated the state repo.

**Not fixed.** The real fix is to stop ignoring that annotation on these kinds, which
needs testing for the drift it was added to silence.

## external-secrets kept retrying without the fix that was already on main (found 2026-11-01, released the same day)

**Released 2026-11-01 ~21:20Z.** The operation on each app was terminated with the
patch below, tenant first. Auto-sync started a new one from the current spec,
server-side, at chart 2.12.0, as predicted. Both apps are `Synced`/`Healthy` on
v2.12.0 and all 45 ExternalSecrets on the host and all 4 in the tenant are Ready,
with the webhook's CA intact. Kargo's Stage stayed `Verified=False` until
re-verified by hand with `kargo.akuity.io/reverify`, and is now `Ready=True`. What
follows describes the state before that. `specmarshal-the-cluster` is a different
cause and was not touched.

`external-secrets-the-cluster` and `external-secrets-vcluster-media.integratn-tech`
have been `OutOfSync` since 2026-08-30T04:54Z, past 9,000 attempts, with:

```
CustomResourceDefinition "clustersecretstores.external-secrets.io" is invalid:
metadata.annotations: Too long: may not be more than 262144 bytes
```

That is client-side apply, and the error suggests adding `ServerSideApply=true`.
**That option has been on main since #287, and both live Applications carry it.**
The operation does not:

```
.spec.syncPolicy.syncOptions                         ServerSideApply=true, ...
.status.operationState.operation.sync.syncOptions    (no ServerSideApply)
.status.operationState.operation.sync.revisions      d1562ae9, 2.10.0
.status.sync.revisions                               4e4ee2fe, 2.12.0
```

#287 merged at 15:21Z, ten hours after the operation started, and the two CRDs
were fixed by hand with `kubectl apply --server-side` that morning. Nobody
terminated the operation, so it carried on with the options it started with.

**The entry above understates what an operation freezes.** It is not only the
revision. `state.Operation.Sync` is a snapshot, so a change to `syncOptions` or
`retry` does not reach an operation already running either. For the same reason,
`retry.refresh: true` would not have rescued this one: in Argo CD v3.5.1 it
clears `Sync.Revision` and `Sync.Revisions` on each retry and leaves
`Sync.SyncOptions` and `Sync.Sources` alone. It would have retried the same chart
2.10.0, still client-side, and failed the same way. (This said "the newest chart"
until 2026-11-01; the probe under
[finite retries](#sync-retries-are-finite-and-a-wedged-operation-pages-decided-2026-11-01)
showed otherwise.)

**What was left behind is small.** Rendering 2.10.0 and 2.12.0 with the app's own
values:

- Outside the CRDs, the only change is the image tag on the three Deployments,
  plus version labels. No RBAC, webhook or Service changes.
- The two CRDs gain 260 schema paths and lose 4, all of them `required` entries
  relaxed on `barbican` and `keepersecurity`. `v1` stays the storage version and
  `v1beta1` stays declared unserved, so `storedVersions: [v1beta1, v1]` is still
  legal.
- Every value this repo sets is still declared by the 2.12.0 chart.
- The frozen operation's `helm` block matches the current spec, so no values were
  lost, only the version.

So both clusters run ESO **2.10.0** while main says **2.12.0**. Chart 2.14.0 has
since been published. The pin went 2.10.0 -> 2.13.0 -> 2.12.0 on 2026-10-16: a
2.12.0 Promotion opened and merged half an hour after 2.13.0, with no reason
recorded. None of it ever applied.

**Checked read-only before terminating, on both clusters:** server-side dry-run
of all 45 rendered resources with `--field-manager=argocd-controller` passes. The
webhook `caBundle` and the four `external-secrets-webhook` Secret keys survive,
because the ESO controller co-owns them; conversion stays `None`; images become
`v2.12.0`. A client-side dry-run of the same two CRDs reproduces the error.

**The cure is runtime state, not a commit.** No merge can release this. Terminate
the operation on each app, as in the entry above, and auto-sync starts a new one
from the current spec, server-side, at 2.12.0.

**It was not silent, and one alert cannot see it:**

- `ArgoCDAppOutOfSync` was firing for both apps at every daily sample in
  Prometheus's 15d retention, routed to `matrix-digest`. A warning on a digest
  channel that stays on for two months is easy to stop reading.
- Kargo's `external-secrets` Stage has been `Verified=False` since the 2.12.0
  Promotion, on `argocd-apps-healthy`.
- `ArgoCDAppSyncFailing` cannot fire on this. `argocd_app_sync_total` has no series
  for either app, because the controller only counts an operation when it
  completes, and with `retry.limit: -1` a persistently failing one never does.
  The rule has since been replaced by `ArgoCDAppSyncWedged`.

**`specmarshal-the-cluster` is stuck too, for a different reason.** Same pinned
operation (since 2026-10-29, revision `6e29b5df`), but it does have server-side
apply. Its PreSync hook fails on its first line:

```
find: unrecognized: -uid
```

`home-ownership-job.yaml` calls `find -uid`/`-gid` and `alpine/k8s` ships BusyBox
find, which has only `-user`/`-group`. The sync stops before the Deployment, so
ArgoCD's last deployed chart is still rc.9, and the CVE fix in rc.12 has not
landed.

**Correcting the flags would only have moved the failure one line down.** Tested
on a scratch claim on the same export, with the Job's image and security context:

```
root's writes land as:           99:100
chown -h 1000:1000 <file>        Operation not permitted
find ... -exec chown -h ... {} + exit 1
```

The export [squashes every uid](/platform/storage.md), so the chown is refused for
everyone, and the hook would have failed the sync for good with `-user` as it did
with `-uid`. The hook never had a job to do: a second pod shaped like rc.12's
(1000:1000, no capabilities, read-only root) read a `0600` file root wrote, entered
a `0700` directory, appended, created, `chmod`ed, renamed over and unlinked, all
without a chown. Grafana has run as uid 1000 on this export the same way. The live
home is all `99:100` with not one `0:0` file, and neither rc.12's source nor core's
checks a file's owner or mode. So the hook is **deleted**, not repaired.

Two things the same test measured, for the next chown hook on BusyBox:

- `-exec ... {} \;` exits **0** when every chown is refused, so `sh -e` carries on
  to its success message. `{} +` exits 1.
- Root holding only `CAP_CHOWN` locks itself out mid-walk (on tmpfs). The batched
  chown gives a `0700` directory to 1000 before `find` descends, and root has no
  `CAP_DAC_READ_SEARCH` to enter it. The nested file stays root's, and a later walk
  reports `Permission denied` and matches nothing. `-depth` with `\;` gets it right.

**A merge does not release it.** Terminate the operation, as in the entry above.
That rolls rc.9 to rc.12 with `Recreate`, so the surface is down from the first
SIGTERM until the new pod is ready, and the old pod may take up to
`terminationGracePeriodSeconds` (11100s) if a run is in flight. Not yet verified
live.

## Pinning the kratix image froze half an artifact (found 2026-08-30)

The `kratix` chart has exactly **one** version, `v0.0.1`, and Syntasso
republishes it in place. The 2026-08-21 pin
(`addons/cluster-roles/control-plane/addons/kratix/values.yaml`) was added to
stop the controller image moving on ordinary syncs. It did — and by doing so it
desynchronised the container from the chart that renders it.

On 2026-08-28 at 11:30Z the index was regenerated: same `appVersion: 1.16.0`,
new chart digest, new entrypoint.

```
command: [/sbin/tini, --, /manager]      # was: [/manager]
```

`command` comes verbatim from the chart's `files/deployment.yaml`. The chart's
only container override is `containerPatch`, which merges `image` and
`resources` — **no value can move `command`**. So the new command was rendered
against the pinned old image, which has no `/sbin/tini`. From 20:35Z that day
every new pod died:

```
StartError, exit 128: unable to start container process:
exec: "/sbin/tini": stat /sbin/tini: no such file or directory
```

509 restarts over 43 hours. Nothing user-facing broke, because the 8-day-old pod
from the previous ReplicaSet kept serving — the Deployment sat at
`maxSurge`-plus-one with both ReplicaSets at `DESIRED=1`, old one Available, new
one never Ready.

**The platform did detect it.** `KubePodCrashLooping`, `KubePodNotReady`,
`KubeDeploymentRolloutStuck`, `ArgoCDAppDegraded` and `TargetDown` were all
firing continuously from 20:35–20:45Z. This was an attention failure, not a
monitoring gap; treat any claim that "nothing caught it" as unverified until the
firing list is read.

A second incoherence came with it and is easy to miss: the chart hard-codes
`PIPELINE_ADAPTER_IMG` in a ConfigMap with no override, so **pipeline Jobs were
already running the new image while the manager ran the old one.** Manager and
adapter are meant to be the same build.

Pin bumped to the digest the chart itself ships
(`sha256:88809ecd…`), which restores both halves at once. The durable fix is to
stop consuming a mutable chart version — vendor the chart into `addons/charts/`
so its content is a reviewable diff. Until then the pin is a tripwire that has
to be re-armed by hand after every republish, and the check is:

```
helm pull kratix --repo https://syntasso.github.io/helm-charts \
  --version 0.0.1 --untar --untardir /tmp/k
grep -n 'image:\|/sbin/tini' /tmp/k/kratix/files/deployment.yaml
```

**Resolved 2026-08-30** (PR #293). Live: `kratix-the-cluster` `Synced`/`Healthy`,
one pod, 0 restarts, `status.imageID` on the pinned digest, and manager ==
`PIPELINE_ADAPTER_IMG` again.

One correction worth carrying, because it was got wrong the first time: **this
app had two faults, not one.** The image pin explains the `Degraded` *health*.
It does not explain the sync, which was wedged by the external-secrets version
above and needed the operation terminated — see the entry on retrying syncs.
Fixing the pin alone would have left the app stuck.

(Note for whoever reads `status.containerStatuses[].image` to confirm the
digest: it reports containerd's local image-config ID, a different digest
entirely. `status.containerStatuses[].imageID` is the one that carries the
pulled reference.)

## kyverno 3.2.8 → 3.9.0 (PR #41) — seven minors under a `failurePolicy: Fail` webhook

Chart 3.9.0 is Kyverno **v1.19.0**, from v1.12.6. Three separate problems:

- `cleanupJobs.*` and `policyReportsCleanup.*` no longer exist in the chart.
  `addons/environments/production/addons/kyverno/values.yaml` sets both, and
  **six of the seven keys** in Kargo's `kubectl` target
  ([kargo-projects values](../../../addons/cluster-roles/control-plane/addons/kargo-projects/values.yaml))
  point at them — the target would keep rewriting config nothing reads.
- `webhooksCleanup.image` now defaults to `kyverno/readiness-checker` and the
  Job runs it with `delete-webhooks`. Our override points that container at the
  repo's kubectl image, which has no such subcommand.
- Seven minors of generate-rule change land on top of the
  `generate-default-deny-netpol` ClusterPolicy, whose `synchronize: true` owns
  every namespace's `default-deny-all`. The admission controller's webhooks are
  `failurePolicy: Fail` and exclude only `kube-system` and `kyverno`, so while
  it is unhealthy **no pod can be created anywhere else**.

## ~~loki chart 6.49.0 → 7.3.0 (PR #44) — wrong chart now~~ (resolved)

Not a version problem: the chart's own changelog says the repository "is now
maintained for Grafana Enterprise Logs (GEL) users only", and OSS users should
move to the community fork at `grafana-community/helm-charts` (forked at
6.55.0). This cluster runs OSS Loki (SingleBinary, filesystem, boltdb-shipper
on `config-nfs-client`). Merging 7.x follows the enterprise chart by accident.

**Resolved 2026-08-22** by following the OSS lineage instead: the addon and the
Kargo target both point at `https://grafana-community.github.io/helm-charts`,
pinned at chart **18.11.0** (Loki 3.7.6). PR #44 was closed unmerged. See
[observability](../platform/observability.md).

Two things worth carrying forward from the values rewrite, because they are
general to this chart family and not to Loki:

- The chart resolves security contexts with `coalesce`
  (`$component` → `.Values.defaults` → `.Values.loki`), so a **partial override
  replaces the chart's default rather than merging into it**. Our hand-written
  `singleBinary.podSecurityContext` — added for a Trivy finding, and carrying
  only `seccompProfile` — would have silently dropped `fsGroup: 10001` and run
  the pod as root against the existing NFS volume. Every hardening block in the
  values file was deleted; 18.x ships the same settings or stricter.
- `loki.storageConfig` in the values file had **never rendered**. The chart
  reads `loki.storage_config` (snake case), in 18.11.0 and in 6.49.0 alike, so
  the boltdb-shipper/filesystem block was decoration — the real config comes
  from `loki.storage.type: filesystem`.

Still ahead: Loki 3.7.6 keeps boltdb-shipper and schema v12 working, but both
are deprecated upstream and targeted for removal in Loki 4. Migrating the
schema to TSDB/v13 is a separate decision.

## trivy-operator-explorer 0.4.6 → 0.5.1 (PR #56) — v1.0.0 is a different application

Chart 0.5.1 ships app **v1.0.0**, which "split collector and frontend services,
with multi-tenant S3 backend for state". The chart dropped `clusterrole.yaml`
and `clusterrolebinding.yaml` outright, and the rendered Deployment gains
`TRIVY_OPERATOR_EXPLORER_DB_PATH`, `_S3_BUCKET/_PREFIX/_REGION` (all empty) and
an MCP port. The `trivy-explorer` addon syncs with `prune: true`, so the
existing ClusterRole would be **deleted** — leaving a UI with no permission to
read the 156 VulnerabilityReports it exists to display, and no S3 backend
configured to read them from instead.

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

## Leader changes come from disk stalls, not the steady latency (2026-10-16)

Measured over 14 days: fsync p99 sits at 50-64 ms on .101/.102 (about 15 ms on
.103) every day, and leader changes ran 0-7 a day. That steady latency does not
cause elections. Stalls do: on 2026-10-16 a Trivy full rescan (Standalone mode
unpacks the DB per scan Job onto the node disk) pushed .102 to 1-3 s and caused
196 leader changes. Mitigations: `scanJobsConcurrentLimit: 1`, etcd
heartbeat/election 250/2500 ms (Talos, step 6 of commands.md; takes effect only on a node reboot, since Talos 1.11 does not restart etcd for an extraArgs change), and alerts sized
to this baseline (`EtcdLeaderChurn`, `EtcdWalFsyncSlow`, `EtcdWalFsyncStalled`,
`EtcdBackendCommitSlow`; the mixin's `etcdHighFsyncDurations` and
`etcdHighCommitDurations` are disabled because the commit rule fired on
ordinary days). Trivy scan Jobs no longer run on .102 (`scanJobAffinity`,
hostname `talos-4kg-ymz`). The nodes (ThinkCentre M900 Tiny, one 256 GB Samsung
SATA SSD each) have no second disk. .102's PM871b writes ~84 ms average against
~21 ms on .101's identical model, but on 10-09/10-10 its fsync median was 8-9 ms,
faster than .101: load on that node matters more than drive wear.
**Resolved 2026-10-18 18:50Z** by Trivy ClientServer mode (#499): over the
following 22 hours every member's fsync and commit p99 stayed at baseline
(max 64 ms; .103 15-23 ms) with 0 leader changes. The 10-16..18 trouble was
entirely scan write load; .102's drive is fine. **Superseded 2026-10-29:
.102's drive was failing** -- medium errors, see the section at the top. The
4x latency gap on identical drives was the warning, not load.

**2026-10-18: the load was a Trivy retry loop, and it follows the scans.**
.102's disk shows no ATA, I/O or timeout errors and the same sysfs error
counters as the other two; when #487 restarted the operator (17:03Z) and scans
left .102, it dropped from ~17 MB/s and 140 ms write await to 0.5 MB/s and
4 ms, and .101 picked up the same ~27 MB/s. Cause: Standalone scan Jobs
re-downloading the DB, multi-container scans dying on the shared fs-cache lock
or OOM at 500M, and a 30s retry (`scanJobsRetryDelay`). Fixed by the built-in
Trivy server (ClientServer), a 1h retry and a 1G limit. cAdvisor's
`container_fs_writes_bytes_total` missed nearly all of it (~0.2 MB/s per pod);
per-cgroup `io.stat` via `talosctl read /sys/fs/cgroup/kubepods/.../io.stat`
and `kube_pod_info{namespace="trivy-system"}` counts are what show it.

## The database was 82% empty space (found 2026-08-21, **defragmented 2026-08-22**)

**Resolved 2026-08-22T03:4xZ:** `talosctl etcd defrag` on 102, 101, then the
leader 103. DB size went 584 / 628 / 635 MB → **135 / 135 / 134 MB** at
~100% in use, same leader and raft term throughout — no election triggered.
The fsync/commit latencies below are still the disk's problem and were
re-measured separately; the record of the finding follows.

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

**Closed 2026-11-01:** the inherited default is now `limit: 10`. See
[finite retries](#sync-retries-are-finite-and-a-wedged-operation-pages-decided-2026-11-01),
which also measures the cost this question was worried about.

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

**Update 2026-11-01 — the router is in the path now, on purpose.** All three
nodes resolve through `10.0.0.1, 1.1.1.1, 8.8.8.8`: .102 from its rebuild, .101
and .103 by James's decision when they got their explicit default route (see
[Matchbox PXE](../infrastructure/matchbox-pxe.md)). `dns.yaml` itself stays
withdrawn. The flip it warned about is not happening today: all 23 HTTPRoute
hostnames answer the same from all three resolvers, because the public records
now hold the same `10.0.4.x` addresses as the router's. The risk is that those
records drift apart. The router also answers names the public resolvers do not
(`tower`, node names). Whether the host-DNS proxy spreads queries across upstreams
or prefers the first was not tested; this page's claim that it distributes them
dates from August and is unverified. If a platform hostname ever resolves
differently from pod to pod, look here first.

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
  toleration defaults) — **applied to all three nodes 2026-08-22** as
  `watchdog.yaml` plus the two `live-*.yaml` strategic-merge patches; no
  reboots. Verified: `/dev/watchdog0` timers cycling, `toleration-seconds=60`
  on every kube-apiserver, `serviceaccounts.talos.dev` served. Caveat found:
  vcluster-synced pods still carry 300s — the syncer copies tolerations from
  the vcluster's own apiserver, so the host default never reaches them.

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
[^alert-sweep-0901]: Alert triage sweep 2026-09-01
