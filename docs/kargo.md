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
| Network policy | [network-policies/kargo.yaml](../addons/cluster-roles/control-plane/addons/network-policies/kargo.yaml) | DNS, kube-apiserver, webhook ingress, gateway → API, controller → internet:443 and → Prometheus:9090, API → Authentik |
| `argo-rollouts-crds` addon | [addons.yaml](../addons/cluster-roles/control-plane/addons/addons.yaml) | Only the three *analysis* CRDs from `argoproj/argo-rollouts` v1.9.1 — Kargo reuses `AnalysisTemplate`/`AnalysisRun` for post-merge verification and runs the analysis itself; no Rollouts controller |
| Authentik blueprint | `07-kargo-provider.yaml` in [authentik-blueprints-configmap.yaml](../addons/clusters/the-cluster/addons/authentik/authentik-blueprints-configmap.yaml) | OIDC login for the UI/CLI — a PKCE *public* client, so no client secret exists anywhere |

Three Kargo Projects, each a namespace of the same name, mirror the repo's
top-level areas:

| Project | Covers | Targets |
|---|---|---|
| `addons` | chart versions in every `addons.yaml` (host *and* the vcluster copies, moved together), the Argo Rollouts CRD tag, and the images in raw-manifest addons (`mcp-system`, reconciler, Jobs, nfs, authentik-redis, open-webui image, our own kubectl image) | 35 |
| `promises` | the `*-configure` pipeline images in `promises/*/promise.yaml`, pinned to `main-<sha>` | 7 |
| `workloads` | the media apps in `workloads/vcluster-media/` | 6 |

The UI is at <https://kargo.cluster.integratn.tech>; ArgoCD deep-links are
wired to <https://argocd.cluster.integratn.tech>.

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

The mechanics come from Argo Rollouts: only its three analysis CRDs are
installed (the `argo-rollouts-crds` addon), Kargo's controller reconciles
the `AnalysisRun`s, and the Rollouts controller is not installed. Templates
use Rollouts' `{{args.x}}` syntax, not Kargo's `${{ }}`.

The `workloads` project has no verification: the vcluster's ArgoCD is not
scraped by the host Prometheus, so there is no `argocd_app_info` to ask.

### Selection strategies in use

| Strategy | Targets | Notes |
|---|---|---|
| `SemVer` | most images, all charts | `allowTags` regexes keep out `-alpine`, per-arch and `git-*` tags; `alpine/k8s` is constrained to the cluster's minor ±1 |
| `SemVer` on git tags | `argo-rollouts-crds` | a `git` subscription following the repo's release tags (blobless clone), rewriting both the addon's `defaultVersion` and its `generatorValues` revision |
| `NewestBuild` | `main-<sha>` images, linuxserver `A.B.C.D-lsNNN` tags | Kargo reads each candidate's build time; `discoveryLimit` is kept at 5 and `platform: linux/amd64` set |
| `Digest` | `mcpo:main` | follows the tag's digest |

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
  `grafana-sa-token-job.yaml` were moved to the top for exactly this reason);
- no trailing `# comment` on that line — it would be deleted on the first
  update; put it on the line above;
- the `repoURL` is written verbatim for `image`/`image-tag` formats, so use
  the string the file already uses (`redis`, `metio/…`, `docker.io/…`).

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
| `gateway-api-crds` (`v1.4.0`) | Gateway API must move in step with NGINX Gateway Fabric; bump both by hand |
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
| API pod Pending | `kargo-admin-credentials` Secret missing — the 1Password item |
| API pod CrashLoopBackOff | OIDC discovery against Authentik failing; `kubectl -n kargo logs deploy/kargo-api`, then the `allow-api-oidc` CiliumNetworkPolicy |
| UI login loops back | the Authentik `kargo` application/provider (blueprint `07-kargo-provider.yaml`) — check `kubectl -n authentik logs deploy/authentik-server \| grep -i blueprint` |
| Webhook timeouts creating Projects/Stages | `allow-webhooks-server` / `allow-kube-api` in the network policy; the webhooks server listens on 9443 |
| Stage verification `Error` | `kubectl -n <project> get analysisrun` then `describe` — Prometheus unreachable (`allow-controller-prometheus-egress` here, the kargo rule in `monitoring.yaml` there) or the AnalysisTemplate CRD missing (`argo-rollouts-crds-the-cluster`) |
| Stage verification `Failed` | one of the `verify.apps` is not Synced/Healthy three minutes after the merge — look at that Application in ArgoCD; re-verify from the Kargo UI once it recovers |

## See also

- [docs/okf/platform/kargo.md](okf/platform/kargo.md) — the concept entry in
  the knowledge bundle
- [docs/addons.md](addons.md) — how the two addons become Applications
- [CLAUDE.md](../CLAUDE.md) — the repo's rules, including the one Kargo is
  allowed to bend (merging to `main`)
