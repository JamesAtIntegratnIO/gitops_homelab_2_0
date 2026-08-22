# Kargo — automated version updates

Every image tag, digest and chart version pinned in this repository is kept
current by [Kargo](https://kargo.io). It watches the registries and chart
repositories those pins come from, and when something newer appears it opens a
pull request that rewrites the pin — and merges it itself when the per-target
policy says the change is small enough. ArgoCD then does what it always does
with `main`.

Kargo is a promotion engine built for dev → staging → prod pipelines. None of
that is used here: this is a single-environment homelab, so every artifact
gets exactly one Warehouse and one Stage, and the Stage's entire job is "put
the new version on `main` through a PR".

```
registry / chart repo ──poll──▶ Warehouse ──▶ Freight ──auto-promote──▶ Stage
                                                                          │
      main ◀── git-merge-pr (policy) ◀── PR ◀── git-push ◀── yaml-update ◀┘
       │                 └── or git-wait-for-pr until a human merges
       ▼
    ArgoCD (host and vcluster) reconciles as usual
```

## Pieces

| What | Where | Role |
|---|---|---|
| `kargo` addon | [addons.yaml](../addons/cluster-roles/control-plane/addons/addons.yaml), values in [kargo/values.yaml](../addons/cluster-roles/control-plane/addons/kargo/values.yaml) | The Kargo chart (OCI, `ghcr.io/akuity/kargo-charts/kargo` 1.11.2) in namespace `kargo`, control-plane cluster only |
| `kargo-extras/` | [kargo-extras/](../addons/cluster-roles/control-plane/addons/kargo-extras/) | Attached to the `kargo` app as an extra source: the two ExternalSecrets and the HTTPRoute for `kargo.cluster.integratn.tech` |
| `kargo-projects` addon | local chart [addons/charts/kargo-projects](../addons/charts/kargo-projects/), targets in [kargo-projects/values.yaml](../addons/cluster-roles/control-plane/addons/kargo-projects/values.yaml) | Renders the Projects, Warehouses and Stages from the target list |
| Network policy | [network-policies/kargo.yaml](../addons/cluster-roles/control-plane/addons/network-policies/kargo.yaml) **and** the `kargo` entry in [network-policies/nginx-gateway.yaml](../addons/cluster-roles/control-plane/addons/network-policies/nginx-gateway.yaml) | DNS, kube-apiserver, webhook ingress, gateway → API (both halves: the gateway's egress allow-list names backend namespaces one by one), controller → internet:443, API → Authentik |
| `argo-rollouts` addon | [addons.yaml](../addons/cluster-roles/control-plane/addons/addons.yaml), values in [argo-rollouts/values.yaml](../addons/cluster-roles/control-plane/addons/argo-rollouts/values.yaml) | Argo Rollouts controller (chart 2.41.1, one replica, no dashboard) — Kargo *creates* the verification `AnalysisRun`s, this is what *executes* them; its netpol is [network-policies/argo-rollouts.yaml](../addons/cluster-roles/control-plane/addons/network-policies/argo-rollouts.yaml) |
| Authentik blueprint | `07-kargo-provider.yaml` in [authentik-blueprints-configmap.yaml](../addons/clusters/the-cluster/addons/authentik/authentik-blueprints-configmap.yaml) | OIDC login for the UI/CLI — a PKCE *public* client, so no client secret exists anywhere |

Three Kargo Projects, each a namespace of the same name, mirror the repo's
top-level areas:

| Project | Covers | Targets |
|---|---|---|
| `addons` | chart versions in every `addons.yaml` (host *and* the vcluster copies, moved together, Argo Rollouts included) and the images in raw-manifest addons (`mcp-system`, reconciler, Jobs, nfs, authentik-redis, open-webui image, our own kubectl image) | 35 |
| `promises` | the `*-configure` pipeline images in `promises/*/promise.yaml`, pinned to `main-<sha>` | 7 |
| `workloads` | the media apps in `workloads/vcluster-media/` | 6 |

The UI is at <https://kargo.cluster.integratn.tech>; ArgoCD deep-links are
wired to <https://argocd.cluster.integratn.tech>.

## Promotion chains

Charts that run in **both** the host and the vcluster are promoted as a chain:
the vcluster first, then the host, and the host only once the vcluster's
Application has gone Synced and Healthy. Before this they moved in a single
pull request, so a bad chart reached the tenant and the platform at the same
moment.

| Target | Canary stage | Terminal stage |
|---|---|---|
| `argo-cd` | `argo-cd-vcluster` | `argo-cd` |
| `cert-manager` | `cert-manager-vcluster` | `cert-manager` |
| `external-dns` | `external-dns-vcluster` | `external-dns` |
| `nginx-gateway-fabric` | `nginx-gateway-fabric-vcluster` | `nginx-gateway-fabric` |
| `kube-prometheus-stack` | `kube-prometheus-stack-vcluster` | `kube-prometheus-stack` |

The gate is Kargo's own: it only makes Freight available downstream once it has
been **verified** upstream, so a canary that fails verification simply never
offers the version on. Nothing polls and there is no state of our own.

**The host stages ship with `autoPromotion: false`.** Watch one artifact
traverse a chain by hand in the Kargo UI, confirm the canary verifies and the
host stage then offers the Freight, and only then remove that line.

Two things this deliberately does *not* cover, worth stating so the chain is
not over-trusted:

- **`promtail` is not chained.** `promtail-vcluster` is disabled, so a canary
  stage would bump a pin nothing deploys and verify nothing.
- **Everything with no vcluster copy gets no staging at all** — `cilium`,
  `metallb`, `kyverno`, `loki`, `authentik` and every image target go straight
  to the host exactly as before. The vcluster is also materially smaller than
  the host, so a green canary proves less than it looks like it does.

`external-secrets` is a special case: it has one pin serving both clusters (the
production-layer selector has no vcluster exclusion), so it cannot be chained
until that addon is split — which renames the generated Application and makes
ArgoCD prune and recreate the operator every credential in the vcluster depends
on. That is its own change.

## What a Stage does

Each Stage runs the same generated pipeline
([stage.yaml](../addons/charts/kargo-projects/templates/stage.yaml)):

1. `git-clone` `main`.
2. `yaml-parse` the first file the target touches, to read the current pin and
   the version inside it.
3. `yaml-update` every listed key — **only if** the Warehouse's version differs
   (`semverDiff(new, current) != "None"`; for non-semver targets, plain string
   inequality). A pin that is already current produces no commit, no PR, and a
   successful, empty promotion.
4. `git-commit` as `chore(<scope>): bump <name> to <version>`, `git-push` to a
   unique `kargo/promotion/<promotion>` branch, `git-open-pr` against `main`.
5. Exactly one of:
   - `git-merge-pr` (squash) when the policy allows, or
   - `git-wait-for-pr`, which parks the promotion until someone merges or
     closes the PR. While parked, newer Freight for the same target queues
     behind it — so an ignored major-version PR holds back later patches of
     that artifact until it is dealt with.

### How the PR steps retry

All three PR steps carry a `retry` policy from `merge.*` in the chart values.
Kargo's system-wide default `errorThreshold` is **1** — no retries at all —
which is how two transient `error getting pull request N` blips from
api.github.com became Errored promotions on day one. It is `3` here.

`git-merge-pr` and `git-wait-for-pr` also carry a `timeout`, which caps how
long a step returning *Running* may keep being retried:

| Step | Timeout | Why |
|---|---|---|
| `git-merge-pr` | `45m0s` | It returns Running while the PR is not mergeable, which includes *a required status check is pending or red*. The cap is what stops a red gate spinning forever — the promotion parks as `Failed` and alerts, instead of looking identical to one waiting on a human. |
| `git-wait-for-pr` | `168h0m0s` | This one parks deliberately, so the cap is generous. It exists to surface a PR everyone forgot, not to police review. |

Write both in Go's canonical duration form (`45m0s`, not `45m`) — the webhook
rewrites them otherwise and ArgoCD reads the difference as permanent drift.

### Merge policy

`autoMerge` per target, evaluated against `semverDiff(new, current)`:

| Policy | Merges by itself | Used for |
|---|---|---|
| `patch` | patch / metadata only | CNI, ArgoCD, Kyverno, cert-manager, MetalLB, NGINX Gateway Fabric, external-secrets, Kargo — a bad minor here can take the cluster down |
| `minor` (default) | patch, plus minor when major > 0 (`0.x` minors count as breaking) | everything else: apps, observability, tooling |
| `always` | whatever the Warehouse found | our own CI images (`main-<sha>` of the reconciler and the promise pipelines) and the linuxserver media images, which were `latest` before and already updated themselves on every restart |
| `never` | nothing | `mcpo`, which only publishes a moving `main` tag — there is no version to judge |

Everything *not* merged automatically is still opened as a PR; the policy only
decides who clicks merge. A first-time pin (e.g. `latest` → `v1.10.1`) is
`Incomparable` and therefore always waits for a human.

### Verification — did the merge actually work?

Every `addons` and `promises` target names the ArgoCD Applications its pin
feeds (`verify.apps`). After the PR merges, the Stage runs an
[AnalysisTemplate](https://docs.kargo.io/user-guide/reference-docs/analysis-templates)
(`argocd-apps-healthy`, rendered into each Project by the chart): starting
3 minutes after the promotion — ArgoCD polls `main` every 60s, then syncs —
it asks Prometheus once a minute, five times, whether

```promql
sum(max by (name) (argocd_app_info{name=~"<apps>", health_status="Healthy", sync_status="Synced"}))
```

equals the number of apps listed; up to two of the five may fail. The result
shows on the Stage and on the Freight in the UI, and a failure marks that
Freight as *failed verification* for the Stage. Nothing is rolled back by
itself: `autoRollback` exists (per project or per target in the values) and
would promote the previously verified Freight back — i.e. open and merge a
revert PR — but it stays **off** until the interaction with auto-promotion
has been watched on a real failure (a newer Freight could be re-promoted on
top of the rollback).

The mechanics come from Argo Rollouts, and the division of labour matters:
Kargo only *builds* the `AnalysisRun` from the template (there is no run
reconciler in Kargo — day one proved it, with 23 runs sitting at no phase);
the Argo Rollouts controller (`argo-rollouts` addon, no Rollout objects, no
dashboard) executes the measurements. Templates use Rollouts' `{{args.x}}`
syntax, not Kargo's `${{ }}`.

The `workloads` project has no verification: the vcluster's ArgoCD is not
scraped by the host Prometheus, so there is no `argocd_app_info` to ask.

### Selection strategies in use

| Strategy | Targets | Notes |
|---|---|---|
| `SemVer` | most images, all charts | `allowTags` regexes keep out `-alpine`, per-arch and `git-*` tags; `alpine/k8s` is constrained to the cluster's minor ±1 |
| `NewestBuild` | `main-<sha>` images, linuxserver `A.B.C.D-lsNNN` tags | Kargo reads each candidate's build time; `discoveryLimit` is kept at 5 and `platform: linux/amd64` set |
| `Digest` | `mcpo:main` | follows the tag's digest |

(The chart also supports `git` targets that follow a repository's release tags; nothing uses one today.)

Polling: images 6h, our own build images 15m, linuxserver 12h, charts 24h.
`cacheByTag` is on for every immutable-tag strategy, so a registry is only
asked about tags it has not seen before.

## Before the first sync — what needs a human

Two 1Password items, read through the `onepassword-store` ClusterSecretStore:

| Item | Property | Used by |
|---|---|---|
| `kargo-github-token` | `token` — fine-grained PAT on `jamesatintegratnio/gitops_homelab_2_0` with **Contents: read+write**, **Pull requests: read+write**, Metadata: read | every Stage (clone, push, open/merge/poll PRs) via [externalsecret-github.yaml](../addons/cluster-roles/control-plane/addons/kargo-extras/externalsecret-github.yaml) |
| `kargo-admin` | `password-hash` (`htpasswd -bnBC 10 "" '<pw>' \| tr -d ':\n'`), `token-signing-key` (`openssl rand -base64 29 \| tr -d "=+/"`) | the break-glass admin login via [externalsecret-admin.yaml](../addons/cluster-roles/control-plane/addons/kargo-extras/externalsecret-admin.yaml) |

Until `kargo-github-token` resolves, Warehouses still discover artifacts but
every promotion fails at `git-clone`. Until `kargo-admin` resolves, the API pod
has no Secret to mount and stays Pending; the controller keeps working.

Recommended, not required: turn on **Settings → General → Automatically delete
head branches** on the GitHub repo, so the `kargo/promotion/*` branches vanish
on merge. (Tensure keeps a scheduled workflow for the same purpose; the repo
setting is simpler.)

### What day one looks like

On the first sync every Warehouse discovers its newest artifact and, since
Kargo has no memory of what is deployed, promotes it. Each Stage then compares
against the pin in git:

- Already current → nothing happens (`exportarr v2.3.0`, `nfs-subdir …
  v4.0.2`, the `kubectl` image, everything built this week).
- Behind by a patch/minor under `minor`/`patch` policy → a PR is opened **and
  merged** within minutes (`alpine/k8s` 1.33.0 → 1.34.x, `mcp-for-argocd`
  stays — 0.x minor is manual — `matrix-alertmanager-receiver` 2026.2 →
  2026.8, chart patch releases).
- Behind by more, or not comparable → a PR is opened and waits
  (`github-mcp-server`/`kubernetes_mcp_server` leaving `latest`,
  `external-secrets` 0.10 → whatever is current, `redis:7-alpine` → a pinned
  `7.x.y-alpine`, major chart bumps).
- `always` targets → the promise images move from `:latest` to the
  `main-<sha>@sha256:…` that `latest` already points at, and merge.

Expect a burst of PRs. Closing one is a valid answer; the Stage will not
re-open it for the same version, and the next version starts over.

## Adding, changing, removing a target

Edit [kargo-projects/values.yaml](../addons/cluster-roles/control-plane/addons/kargo-projects/values.yaml);
the schema is documented in the chart's
[values.yaml](../addons/charts/kargo-projects/values.yaml). A minimal image
target:

```yaml
projects:
  addons:
    targets:
      my-thing:
        scope: my-namespace            # PR title: chore(my-namespace): bump my-thing to …
        image:
          repoURL: ghcr.io/org/my-thing
          allowTags: ['^v\d+\.\d+\.\d+$']
        updates:
          - file: addons/cluster-roles/control-plane/addons/my-thing/deployment.yaml
            keys: [spec.template.spec.containers.0.image]
```

Rules that come from how Kargo's `yaml-update` works — it rewrites the
matched line in place and parses only the first YAML document:

- the key must already exist, address a scalar, and live in the **first**
  `---` document of the file (the Jobs in `etcd-cert-extractor.yaml` and
  `grafana-sa-token-job.yaml`, and the CronJob in `talos-etcd-snapshot.yaml`,
  were moved to the top for exactly this reason). This bites at `yaml-parse`,
  before any update is attempted, and the failure is easy to miss: the
  `talosctl` target Errored on every promotion from the day it was created
  with `cannot fetch spec from <nil>`, and nothing reported it;
- no trailing `# comment` on that line — it would be deleted on the first
  update; put it on the line above;
- the `repoURL` is written verbatim for `image`/`image-tag` formats, so use
  the string the file already uses (`redis`, `metio/…`, `docker.io/…`);
- **a `-suffix` in a tag is a semver PRERELEASE.** `3.6.8-0` (etcd) and
  `7.4.11-alpine` (redis) both parse as prereleases, and a plain range
  constraint excludes prereleases by rule. Pair such tags either with no
  constraint at all, or with bounds that carry their own prerelease —
  `>=3.6.0-0 <3.7.0-0`. Getting this wrong matches *nothing*: the Warehouse
  sits at `NoImageReferencesDiscovered` with zero Freight and reports no error,
  which is how `vcluster-etcd-snapshot-client` stayed dead from the day it was
  written. `hctl kargo warehouses --problems` is what surfaces it;
- write `interval` in Go's canonical form (`12h0m0s`, not `12h`) — Kargo's
  webhook stores it normalised and ArgoCD would otherwise see permanent
  drift and keep re-syncing;
- chart and git subscriptions default to `constraint: ">=0.0.0"`, which is
  what keeps prereleases (`16.0.1-dev.203.1`, `1.21.0-pre.0`) out; images
  rely on `allowTags` for the same job.

Removing a target removes its Warehouse and Stage on the next sync (ArgoCD
prunes). Removing a whole project also removes its namespace.

Then verify the render before pushing:

```bash
helm template kargo-projects addons/charts/kargo-projects \
  -f addons/cluster-roles/control-plane/addons/kargo-projects/values.yaml
```

## Deliberately not tracked

| Pin | Why |
|---|---|
| `gateway-api-crds` (`v1.5.1`) | Gateway API must move in step with NGINX Gateway Fabric; bump both by hand |
| `kratix` chart `0.0.1` | placeholder version; the images inside are sha-pinned by Syntasso's chart |
| vcluster-media's etcd `3.6.8-0` | coupled to the vcluster/Kubernetes version |
| `environments/development` and `staging` | no clusters use them |
| the disabled `mcp-*`, `ai-platform`, ACK addons | disabled |

## Operating it

```bash
# UI
open https://kargo.cluster.integratn.tech            # Authentik SSO, or the admin account

# CLI
kargo login https://kargo.cluster.integratn.tech --sso
kargo get warehouses --project addons
kargo refresh warehouse grafana-mcp --project addons  # poll now instead of waiting
kargo get promotions --project addons --stage grafana-mcp
kargo promote --project addons --stage grafana-mcp --freight <name>   # manual promotion

# what ArgoCD thinks
kubectl -n argocd get application kargo-the-cluster kargo-projects-the-cluster
```

Pausing: set `autoPromotion: false` on a target (Freight is still discovered,
nothing is promoted until someone clicks), or `enabled: false` on the
`kargo-projects` addon to stop everything while keeping Kargo installed.

Registry budget: Docker Hub is polled anonymously. Seven targets at 6h with
`cacheByTag` stay well under the anonymous limit; if Kargo ever logs 429s,
add a Docker Hub credential Secret labelled `kargo.akuity.io/cred-type: image`
with `repoURL: docker.io` in `kargo-shared-resources`, or lengthen the
intervals.

Inbound webhooks (GitHub push, registry events) are not exposed — the
`externalWebhooksServer` is off and nothing routes to it. Polling is the only
trigger.

## Troubleshooting

| Symptom | Look at |
|---|---|
| Warehouse shows no Freight | `kubectl -n <project> describe warehouse <name>` — `allowTags` too strict, or the registry rejected the request (Docker Hub 429) |
| Promotion fails at `git-clone` | the `kargo-github-gitops-homelab` Secret in `kargo-shared-resources`: `kubectl -n kargo-shared-resources get externalsecret` |
| Promotion fails at `yaml-parse` / `yaml-update` | the key moved, is not in the first document, or the file was renamed; fix the target list |
| PR opened, promotion "Running" forever | `git-wait-for-pr` — intended; merge or close the PR |
| `git-merge-pr` fails "not mergeable" | branch protection or a conflict with a PR merged moments earlier; re-run the promotion from the UI |
| UI times out with zero bytes while `argocd.cluster…` answers instantly | the gateway data plane's egress allow-list (`allow-nginx-dataplane` in `network-policies/nginx-gateway.yaml`) — it must name `kargo`/`app.kubernetes.io/component: api` on 8080; the `kargo`-side ingress rule alone is not enough |
| API pod Pending | `kargo-admin-credentials` Secret missing — the 1Password item |
| API pod CrashLoopBackOff | OIDC discovery against Authentik failing; `kubectl -n kargo logs deploy/kargo-api`, then the `allow-api-oidc` CiliumNetworkPolicy |
| UI login loops back | the Authentik `kargo` application/provider (blueprint `07-kargo-provider.yaml`) — check `kubectl -n authentik logs deploy/authentik-server \| grep -i blueprint` |
| Webhook timeouts creating Projects/Stages | `allow-webhooks-server` / `allow-kube-api` in the network policy; the webhooks server listens on 9443 |
| AnalysisRuns exist but never get a phase or a measurement | the Argo Rollouts controller is not running — `kubectl -n argo-rollouts get pods`, `argo-rollouts-the-cluster` in ArgoCD. Kargo creates runs, Rollouts executes them |
| Stage verification `Error` | `kubectl -n <project> get analysisrun` then `describe` — Prometheus unreachable from the Rollouts controller (`allow-prometheus-egress` in `network-policies/argo-rollouts.yaml`, the argo-rollouts rule in `monitoring.yaml`) |
| Stage verification `Failed` | one of the `verify.apps` is not Synced/Healthy three minutes after the merge — look at that Application in ArgoCD; re-verify from the Kargo UI once it recovers |

## See also

- [docs/okf/platform/kargo.md](okf/platform/kargo.md) — the concept entry in
  the knowledge bundle
- [docs/addons.md](addons.md) — how the two addons become Applications
- [CLAUDE.md](../CLAUDE.md) — the repo's rules, including the one Kargo is
  allowed to bend (merging to `main`)
