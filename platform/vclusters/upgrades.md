# Upgrading vcluster-media

Written 2026-11-01, when the tenant ran vCluster **0.31.1** and Kubernetes
**v1.34.3** on a host at Kubernetes 1.36.6. 0.31 reached end of life on
2026-07-29.

**Status: rung 1 is live and verified (0.32.3, 2026-11-01). Next is rung 2.**

Every rung is one pull request against
[vcluster-media.yaml](vcluster-media.yaml). Kratix renders it, ArgoCD applies it,
and [upgrade-check.sh](upgrade-check.sh) gates the next one.

## Target

**vCluster 0.37.6 with Kubernetes 1.36.6 and etcd 3.6.16.** On vCluster's
[lifecycle page](https://vcluster.com/docs/vcluster/manage/upgrade/supported_versions)
on 2026-11-01:

| Release | End of support | End of life | Default Kubernetes |
|---|---|---|---|
| 0.37, with Platform 4.12 (LTS) | 2027-10-15 | 2028-04-15 | v1.36 |
| **0.37, without Platform (this cluster)** | **2026-12-08** | **2027-03-08** | v1.36 |
| 0.36 | 2026-10-22 (past) | 2027-01-22 | v1.36 |
| 0.35 | 2026-09-16 (past) | 2026-12-16 | v1.36 |
| 0.34 and older | past | past | |

The LTS dates need vCluster Platform, which this cluster does not run. Plan on
**2026-12-08** as the next time this comes up. Nothing watches these dates:
[Kargo does not track this chart](../../docs/kargo.md#deliberately-not-tracked).

Host and tenant compatibility, from the same page:

| Host \ tenant | v1.36 | v1.35 | v1.34 |
|---|---|---|---|
| **v1.36** (this host) | Tested and verified | Likely compatible | Likely compatible |

So the target is the one tested pairing. The rungs below pass through "likely
compatible" pairings, as the cluster does today.

## The ladder

vCluster's [upgrade guide](https://vcluster.com/docs/vcluster/manage/upgrade/upgrade-version)
says to move one minor at a time and that larger jumps are neither tested nor
supported. Upstream Kubernetes does not support skipping a minor either. Each
rung changes one thing, the same rule as the
[Talos ladder](../../matchbox/talos-machineconfigs/upgrades.md).

| # | `targetRevision` | `controlPlane.statefulSet.image.tag` | `k8sVersion` | etcd `image.tag` |
|---|---|---|---|---|
| 0 | 0.31.1 | chart default | v1.34.3 | 3.6.8-0 |
| 1 | 0.32.3 | `0.32.3@sha256:80bb3664379b0a40d1b1a4539cdc20eb80e5dffb76e5423db32067a8d077ec97` | `K134` | `E368` |
| 2 | 0.33.4 | `0.33.4@sha256:c50ec106afaa97fbc2b081c4aed9f7d36b0a67e0dd7e690d397e8893d90cdf2a` | `K134` | `E368` |
| 3 | 0.34.9 | `0.34.9@sha256:99f98926245fa108a7a67765131df76a82b749f7c5078941b88ec90b4263fe60` | `K134` | `E368` |
| 4 | 0.35.5 | `0.35.5@sha256:c2d7656dc73f836957368c7cfda4ad54fedef4a990b25ebf74c373919a6d5a66` | `K134` | `E368` |
| 5 | 0.36.4 | `0.36.4@sha256:e4958f131e8f39ff21dda99c62a434bc16af3d26c57a270bc2b0261aa83258ec` | `K134` | `E368` |
| 6 | 0.37.6 | `0.37.6@sha256:99345bf4c01ec4ed777a72766c861d7d5435f195a6bd6643bb8f3db0f5eb89c8` | `K134` | `E368` |
| 7 | 0.37.6 | unchanged | `K135` | `E368` |
| 8 | 0.37.6 | unchanged | `K136` | `E368` |
| 9 | 0.37.6 | unchanged | `K136` | `E3616` |

```
K134  = v1.34.3@sha256:826be1580721885d0b1ab790e2957f1ab1325af580fb477848af69270d8ce2f1
K135  = v1.35.10@sha256:6113017f3f3508497063a82c3b3b553f5b9475c4383dbbe2a4d5479a47141286
K136  = v1.36.6@sha256:967b956bd407adcacc5e6323bb20346c58e190d8a292cc63dea7cef60b5e9cbc
E368  = 3.6.8-0@sha256:397189418d1a00e500c0605ad18d1baf3b541a1004d768448c367e48071622e5
E3616 = 3.6.16-0@sha256:b29d816b546a1966fe7034a33b34e231fa98c47ce4d2883f70b0555020b22132
```

Rung 9's digest is the one the snapshot CronJob already pins. After it, drop the
"vcluster-media's etcd" line from
[Kargo's untracked list](../../docs/kargo.md#deliberately-not-tracked) only if
you also add a target for it.

Keep the Talos rule: re-check for a newer patch on the day and take it, with its
digest. `tag@sha256` works in all three image fields; the rehearsal ran with it.

Digests come from the registry:

```bash
docker buildx imagetools inspect --format '{{json .Manifest.Digest}}' ghcr.io/loft-sh/vcluster-oss:0.37.6
docker buildx imagetools inspect --format '{{json .Manifest.Digest}}' ghcr.io/loft-sh/kubernetes:v1.36.6
docker buildx imagetools inspect --format '{{json .Manifest.Digest}}' registry.k8s.io/etcd:3.6.16-0
```

## Each rung

**Before the merge.** Once at the top of the ladder, and again before any
Kubernetes rung:

```bash
./platform/vclusters/upgrade-check.sh snapshot    # prints the file it wrote
```

The CronJob prunes `vcluster-media-etcd-*.db` after 14 days and a ladder can
outlast that, so keep the pre-ladder copy out of its reach. On Unraid:

```bash
cd /mnt/user/kube_storage/backups/vcluster-media-etcd
cp -p vcluster-media-etcd-<stamp>.db pre-ladder-0.31.1-k8s1.34.3.db
```

Keep the `-p`. The export squashes root, so the restore Job reads as `nobody`, and
a plain `cp` as root leaves a `root`-owned `0600` file that the Job cannot open.
The real pre-ladder copy hit this and was found by reading it back the same way:
run `etcdutl snapshot status` from a Job, not just `ls`.

Then, every rung:

```bash
./platform/vclusters/upgrade-check.sh baseline
```

**After the merge.** Kratix re-runs the pipeline, the state repo changes, and
ArgoCD rolls the three control-plane pods. Etcd rolls too on rung 1, which adds
its digest, and on rung 9. On the other rungs its pod template does not change.
Then:

```bash
./platform/vclusters/upgrade-check.sh verify 0.32.3 v1.34.3
```

Rung 1 took 5 min 43 s from merge to healthy: 2 min 41 s to reach the
`ArgoCDApplication`, 1 min 31 s more to the `Application`, and 1 min 31 s to roll.
Wait for that before running `verify`, or it will fail on a rung still arriving.

It fails unless the Application, the syncer image, the Kubernetes image and the
tenant apiserver all report the rung's versions, so a rung that did nothing
cannot pass. It also checks:

| Check | Fails when |
|---|---|
| Rollout | either StatefulSet is short of 3 ready or mid-rollout |
| etcd | fewer than 3 members answer, or they disagree on the leader |
| Workloads | any Running pod in `media` from the baseline has a new UID, or a tenant pod is newly not running |
| ArgoCD | a `vcluster-media` Application is newly not Synced/Healthy |
| Kyverno | a server-side dry-run write fails admission |
| cert-manager | a Certificate is not Ready, or `letsencrypt-prod` is missing |
| external-secrets | an ExternalSecret is not Ready, or `onepassword-store` is missing |
| metrics-server | `kubectl top` in the tenant returns nothing |

A rise in tenant container restarts is a WARN. Controllers can lose a lease to an
API blip; the rehearsal saw one Kyverno reports controller do that, and it
recovered without help. If the count is still rising after ten minutes, treat it
as a failure.

Exit 0 means open the next rung.

## Addons that run inside the tenant

| Addon | Tenant runs | Evidence on 1.36 |
|---|---|---|
| Kyverno | 1.12.6, chart 3.2.8 (the `production` layer has no vcluster exclusion) | The host has run it on 1.36.6 since 2026-11-01. In the rehearsal the tenant's Kyverno admitted writes and generated objects at every rung. It is far outside Kyverno's tested range, and its webhooks are `failurePolicy: Fail`, so it is the addon most likely to block writes. |
| external-secrets | 2.12.0 (same) | v1 CRDs. Rehearsed on 2.10.0, 2.12.0 is live on the host at 1.36.6. |
| cert-manager | v1.21.3 (`cert-manager-vcluster`) | Rehearsed at every rung; v1.21.1 is live on the host at 1.36.6. |

The tenant's only deprecated-API metric is `endpoints` `v1`, with no removal
release, so nothing in it uses an API that 1.35 or 1.36 removed.

## What changed between 0.31 and 0.37

Rendered with the live values, 0.31.1 against 0.37.6:

- The values pass the 0.37.6 schema unchanged.
- **Etcd's pod template is identical.** The selectors, `serviceName`,
  `podManagementPolicy` and `volumeClaimTemplates` of both StatefulSets are
  unchanged, so none of the immutable fields move.
- The syncer's Role gains `pods/resize`, `events.k8s.io` events, and `patch` and
  `update` on StatefulSets, ReplicaSets and Deployments in its namespace. That
  last one is new write access to its own control plane's objects.
- The ServiceMonitor's path moves from `/controller-manager/metrics` to
  `/metrics/controller-manager` and gains a `/metrics/syncer` endpoint. Nothing in
  this repo's alerts or dashboards names the old path.

From the release notes: `0.33` removes k3s, `0.35` deprecates and `0.36` removes
volume snapshots (`deploy.volumeSnapshotController`, which fails the chart if
set). None is in use. `controlPlane.distro.k8s.version` still works at 0.37.6.

## What the rehearsal proved

A throwaway kind cluster (host v1.36.4, three nodes) ran the real charts and
images with the live values: the same names and 3+3 replicas, plus a tenant with
Kyverno 3.2.8, external-secrets 2.10.0, cert-manager v1.21.3, a Deployment with a
PVC, a CronJob, a Certificate, an ExternalSecret and a generate policy. A sampler
on the host hit the API and the workload's Service every 2 seconds.

| Step | Roll | API samples down | Workload samples down | Workload pod |
|---|---|---|---|---|
| Integrations off, 0.31.1 | 29 s | 0 | 0 | same UID |
| 0.32.3 | 39 s | 0 | 0 | same UID |
| 0.33.4 | 35 s | 0 | 0 | same UID |
| 0.34.9 | 35 s | 1 | 0 | same UID |
| 0.35.5 | 34 s | 1 | 0 | same UID |
| 0.36.4 | 35 s | 0 | 0 | same UID |
| 0.37.6 | 39 s | 1 | 0 | same UID |
| Revert to 0.36.4, then 0.37.6 | 29 s each | 0, 1 | 0 | same UID |
| k8s v1.35.10 | 42 s | 0 | 0 | same UID |
| k8s v1.36.6 | 41 s | 1 | 0 | same UID |
| Revert to v1.35.10, then v1.36.6 (not supported) | 30 s, 29 s | 0 | 0 | same UID |
| Restore pre-ladder snapshot and revert | see Rollback | 4 | 0 | same UID |
| etcd 3.6.8 to 3.6.16, on the restored tenant | | 0 | 0 | same UID |

At every step, a new Deployment scheduled and synced, Kyverno generated into a
new namespace, the Certificate and ExternalSecret stayed Ready, `kubectl top`
answered, and the snapshot CronJob wrote a file with its pinned PKI.

A direct 0.31.1 to 0.37.6 jump with a populated tenant also passed. That does not
make it supported, and it does not save much: the rungs are small.

**What kind could not prove, and rung 1 then did on the real cluster:** NFS-backed
etcd, ArgoCD and Kratix applying the chart, Cilium and the host NetworkPolicies,
and the live workload set. Etcd rolled one member at a time, the tenant API
answered 42 of 42 probes about 8 seconds apart, and all five `media` pods kept
their UIDs with no restarts.

**Still not exercised on the real cluster:** any Kubernetes rung, any revert, and
the restore, including its ArgoCD pause step.

## Rollback

**Chart rung, Kubernetes unchanged:** revert the PR. The rehearsal went 0.37.6 to
0.36.4 and back with nothing else needed.

**Kubernetes rung:** Upstream does not support a downgrade. Reverting 1.36.6 to
1.35.10 worked in the rehearsal, with the workload undisturbed, so try it first if
the tenant still answers, then re-run `verify`. If tenant objects look wrong or
the apiserver will not start, restore.

**A revert that only removes keys will not apply by itself.** See
[known issues](../../docs/okf/cluster/known-issues.md). Reverting a rung also changes
values, so it applies normally, and `kubectl apply` removes the keys with it. Confirm
with `verify` either way.

**Restore** returns everything inside the tenant to the snapshot, so use the one
taken just before the bad rung, or the pre-ladder copy to leave the ladder
entirely. Follow
[Restore vcluster-media etcd](../../docs/operations.md#restore-vcluster-media-etcd),
with one change when the versions also go back:

1. Pause the automation and scale both StatefulSets to zero (steps 1 and 2).
2. Run the restore Jobs (step 3).
3. Scale **etcd only** back to 3.
4. Merge the revert of every rung newer than the snapshot, and wait for the
   Kratix pipeline to finish.
5. Resume the automation (step 5). ArgoCD applies the old chart and brings the
   control plane back on it.

**The control plane must stay at zero until the revert is in.** Otherwise a newer
apiserver writes to the restored data before you go back, which is the downgrade
you are avoiding.

The rehearsal did this from 0.37.6 / v1.36.6 back to 0.31.1 / v1.34.3 using the
pre-ladder snapshot:

- objects from before the snapshot came back, and objects made after it were gone;
- the Certificate, ExternalSecret and Kyverno worked straight away;
- the workload stayed up throughout, because host pods outlive their control
  plane;
- controllers inside the tenant restarted 17 times between them while the API was
  gone, then settled.

A restore does not cover PV contents, or the *arr databases on Postgres at
10.0.3.1. Sonarr's v5 schema is one-way but no rung touches it.
