---
type: Platform Capability
title: Kargo — automated image and chart version updates
description: How every pinned image tag, digest and chart version in the repo is kept current — Kargo Warehouses watch the sources, one Stage per artifact opens a PR against main, merges it under a per-target policy, and verifies the affected ArgoCD Applications afterwards.
tags: [kargo, updates, images, charts, automation, gitops]
status: stable
generated: { by: claude-code/claude-opus-5, at: 2026-08-23T07:20:00Z }
stale_after: 2026-09-23
sources:
  - id: chart
    resource: ../../../delivery/charts/kargo-pipelines/
    title: kargo-pipelines factory chart (superseded addons/charts/kargo-projects in PR #86)
  - id: targets
    resource: ../../../addons/cluster-roles/control-plane/addons/kargo-projects/values.yaml
    title: target list (55 Warehouse/Stage pairs, measured live 2026-08-22)
  - id: install
    resource: ../../../addons/cluster-roles/control-plane/addons/kargo/values.yaml
    title: Kargo chart values
  - id: extras
    resource: ../../../addons/cluster-roles/control-plane/addons/kargo-extras/
    title: ExternalSecrets and HTTPRoute
  - id: netpol
    resource: ../../../addons/cluster-roles/control-plane/addons/network-policies/kargo.yaml
    title: kargo namespace network policy
  - id: guide
    resource: ../../kargo.md
    title: docs/kargo.md — the operator guide
  - id: agent
    resource: https://github.com/JamesAtIntegratnIO/bosun
    title: Bosun — the triage service the pipeline calls (its own repository since 2026-08-23)
  - id: reference
    resource: https://github.com/Tensure/gitops-apps-config (private)
    title: Tensure's Kargo chart-updater pipelines, the implementation reference
  - id: kargo-src
    resource: https://github.com/akuity/kargo/tree/v1.11.2 (pkg/yaml, pkg/promotion/runner/builtin)
    title: Kargo v1.11.2 — yaml-update semantics and step schemas
---

**Live since 2026-08-21 19:44Z** (PR #24 merged 19:42Z; ArgoCD generated the
three Applications two minutes later). Day-one observations are at the
bottom.

# What it is

Kargo (chart 1.11.2, `ghcr.io/akuity/kargo-charts/kargo`) runs in the `kargo`
namespace on the-cluster only. It is used as a **version-bump bot**, not as a
multi-environment promotion pipeline: this is a single environment, so every
tracked artifact has exactly one `Warehouse` and one `Stage`, and the Stage's
only job is to land the new version on `main` through a pull request. ArgoCD
(host and vcluster) then reconciles as it always does — Kargo never touches
the cluster directly.[^guide]

```
registry / chart repo ─poll─▶ Warehouse ─▶ Freight ─auto-promote─▶ Stage
     main ◀─ git-merge-pr (policy) ◀─ PR ◀─ git-push ◀─ yaml-update ◀─┘
                    └─ or git-wait-for-pr until a human merges
```

# How it is wired

| Piece | Location | Notes |
|---|---|---|
| `kargo` addon | control-plane `addons.yaml`, `selectorMatchLabels: cluster_role: control-plane` | OCI chart; `kargo-extras/` attached as an extra source; `api.secret.name` points at an ExternalSecret so the chart renders no Secret of its own[^install] |
| `kargo-projects` addon | local chart `addons/charts/kargo-projects` | one Namespace + Project + ProjectConfig per project, one Warehouse + Stage per target[^chart] |
| Projects | `addons` (35 targets), `promises` (7), `workloads` (6) | namespaces pre-created with `kargo.akuity.io/project: "true"` so Kargo adopts rather than races ArgoCD[^targets] |
| Credentials | `kargo-shared-resources/kargo-github-gitops-homelab` (git, PAT), `kargo/kargo-admin-credentials` | both ExternalSecrets from 1Password items `kargo-github-token` and `kargo-admin`[^extras] |
| Login | Authentik OIDC, PKCE public client `kargo` (blueprint `07-kargo-provider.yaml`); `authentik Admins` → Kargo admins; admin account as break-glass | no client secret exists anywhere |
| Route | `kargo.cluster.integratn.tech` → `kargo-api:80` (TLS at the gateway, `api.tls.terminatedUpstream: true`) | external-dns publishes it |
| Network | default-deny from Kyverno; explicit: DNS, kube-apiserver, webhook ingress on 9443, gateway → API 8080, monitoring → 9090, controller → internet 443, API → nginx-gateway pod 443 for OIDC discovery[^netpol] | Cilium DNATs before policy, so the OIDC rule names the gateway pod, not the VIP |
| Metrics | ServiceMonitors for controller, management-controller, webhooks-server | scraped by kube-prometheus-stack |
| Verification | `argo-rollouts` addon (chart 2.41.1: CRDs + the controller, one replica, no dashboard, itself a Kargo target); AnalysisTemplate `argocd-apps-healthy` per Project; `verify.apps` on every `addons`/`promises` target | Kargo creates the AnalysisRuns, the Argo Rollouts controller executes them. Prometheus query over `argocd_app_info` 3m after the merge, 5×1m, 2 failures allowed. `autoRollback` available but off |

Post-merge verification exists for the `addons` and `promises` projects only:
the vcluster's ArgoCD is not scraped by the host Prometheus, so `workloads`
Stages have no `argocd_app_info` to check.

Resource requests total ~125m CPU across five pods, all `platform-batch`
priority. The external-webhooks server is disabled; polling is the only
trigger.

# The pipeline each Stage runs

`git-clone main` → `yaml-parse` (current pin + the version inside it) →
`yaml-update` every listed key, *only if* `semverDiff(new, current) != "None"`
(string inequality for non-semver strategies) → `git-commit`
`chore(<scope>): bump <name> to <v>` → `git-push` to a unique
`kargo/promotion/<promotion>` branch → `git-open-pr` → then **exactly one of**
`git-merge-pr` (squash, when the policy allows) or `git-wait-for-pr` (parks
the promotion until a human merges or closes; later Freight queues behind it).

After `git-open-pr`, a **triage** `http` step POSTs the freight context to the
[Bosun](https://github.com/JamesAtIntegratnIO/bosun) — live since
2026-08-22 (PR #92), and in its own repository since 2026-08-23, consumed as
the pinned OCI chart `oci://ghcr.io/jamesatintegratnio/charts/bosun` (Kargo
target `bosun` tracks the chart version; the image follows the chart's
appVersion). `when: gated` means it fires only for pull requests the
merge policy will *not* auto-merge, i.e. the ones already parked on
`git-wait-for-pr` waiting for a human; all 55 Stages carry it. The step is
`continueOnError: true` and the service answers `202` and works
asynchronously, so triage can be down, slow or asleep without ever failing a
promotion — the model runs on the workstation, which is not always awake.

Merge policy (`autoMerge`): `patch` for CNI / ArgoCD / Kyverno / cert-manager
/ MetalLB / NGF / external-secrets / Kargo; `minor` (default — patch, plus
minors when major > 0) for apps and tooling; `always` for our own `main-<sha>`
images and the linuxserver media images (previously `latest`); `never` for
`mcpo:main` (digest only). A first-time pin (`latest` → semver) is
`Incomparable` and always waits.[^chart]

Strategies: `SemVer` with `allowTags` regexes for most things; `NewestBuild`
(+`platform: linux/amd64`, `discoveryLimit: 5`) for `main-<sha>` and
linuxserver's four-part tags; `Digest` for `mcpo:main`. Intervals 6h images /
15m own builds / 12h linuxserver / 24h charts; `cacheByTag` on for immutable
tags.

# Constraints that shape the target list

From Kargo's `yaml-update` implementation[^kargo-src]: it parses **only the
first YAML document** of a file and rewrites the matched scalar's line
in place from the value onward. Therefore a tracked key must exist, be a
scalar, sit in the first `---` document, and carry no trailing comment. Two
files were reordered to put their Job first (`etcd-cert-extractor.yaml`,
`grafana-sa-token-job.yaml`), and the open-webui `image.tag` comment moved to
the line above.

Keys containing dashes (every chart in an `addons.yaml`) are read back through
`$env["kube-prometheus-stack"].defaultVersion` in `yaml-parse`, because
expr-lang cannot dot-access them.

# Deliberately untracked

`gateway-api-crds` (moves with NGF), the `kratix` chart (placeholder `0.0.1`),
vcluster-media's etcd tag, the unused development/staging layers, and every
disabled addon. The Kratix pipeline-cleanup CronJob's kubectl image *is*
tracked (it joined the `kubectl` target once main moved it onto the repo's
own image), but by list position — `additionalResources.10` — so inserting
an entry above it in the Kratix values breaks that target loudly.

# Day one (2026-08-21, 19:44Z–20:00Z)

Within ten minutes of the merge: all four pods Running, both ExternalSecrets
`SecretSynced`, three Projects ready, 48 Warehouses + 48 Stages, 39 Freight
discovered on the first poll, **34 pull requests opened, 15 merged by Kargo
itself**, 19 waiting for a human — exactly the burst the guide predicted.
Assumptions 1–3 below held: dashed-key `$env[...]` reads worked (qdrant,
metrics-server merged), `7.4.11-alpine` was selected for authentik-redis, and
the API came up against Authentik first time.

Two defects surfaced and were fixed the same hour:

- **Warehouse drift.** Kargo's webhook stores `interval: 6h` as `6h0m0s` and
  fills in `freightCreationPolicy: Automatic`; ArgoCD saw all 48 Warehouses
  as OutOfSync and re-synced seven times in ten minutes. The chart now emits
  canonical Go durations and the policy explicitly.
- **Prerelease chart versions.** With no `semverConstraint`, Kargo picked the
  greatest version including prereleases: `open-webui 16.0.1-dev.203.1` and
  `cilium 1.21.0-pre.0` (both manual-merge, so nothing landed). Chart and git
  subscriptions now default to `>=0.0.0`, which excludes prereleases.

- **Verification never ran.** The first design installed only the three
  Rollouts *analysis* CRDs on the belief that Kargo executes AnalysisRuns
  itself. It does not — Kargo v1.11.2 has an AnalysisRun *builder*
  (`pkg/rollouts/analysis_run_builder.go`) and no reconciler; its controller
  runs `promotion`, `stage`, `warehouse` only. Twenty-three runs sat with no
  phase for fifteen minutes. Fixed by replacing the CRD-only addon with the
  `argo-rollouts` chart (controller, no dashboard).

- **UI unreachable.** `kargo.cluster.integratn.tech` hung with zero bytes
  while ArgoCD answered through the same gateway IP in 14 ms. The `kargo`
  namespace allowed the gateway in, but the gateway data plane's own egress
  allow-list (`allow-nginx-dataplane`) names backend namespaces one by one
  and had no `kargo` entry, so nginx's SYN to `kargo-api:8080` was dropped.
  Same class of trap [docs/mcp.md](../../mcp.md) warns about; both halves
  are now in the guide's troubleshooting table.
- **Residual Warehouse drift.** After the duration fix, 13 Warehouses still
  drifted: the webhook defaults `strictSemvers: true` on *every* image
  subscription, the chart only emitted it for `SemVer`. Now always emitted.

With the Argo Rollouts controller in place (20:13Z) the backlog drained at
once: **25 of 26 AnalysisRuns Successful within eight minutes**, five
measurements each, every Prometheus result equal to the expected app count
(`kubectl` → 4, `alpine-k8s` → 2). Verification is working as designed. One
harmless artefact of a fresh install: the Rollouts pod restarted once because
its namespace's NetworkPolicies (the separate `network-policies` app) landed a
few seconds after the pod, so its first kube-API call hit Kyverno's
default-deny — the same race any new namespace can hit on day one.

Also observed: GitHub reports `mergeable: UNKNOWN` for a minute or two after
each merge to `main` moves every other PR's base; `git-merge-pr` with
`wait: true` simply retries until it clears.

# Assumptions, and what day one settled

1. ~~`yaml-parse` evaluates `fromExpression` with the parsed document as the
   expr-lang environment, so `$env["…"]` works for dashed keys.~~ Verified
   day one.
2. ~~The `SemVer` strategy with no constraint still selects `7.x.y-alpine`
   tags for `authentik-redis`.~~ Verified day one (PR #49).
3. ~~The Kargo API starts against Authentik.~~ Verified day one; what
   happens if Authentik is down at startup is still unobserved.
3b. ~~Kargo runs AnalysisRuns itself; only the Rollouts CRDs are needed.~~
   **Wrong** — see day one. The Rollouts controller is required.
4. `autoRollback` is deliberately off: whether Kargo would re-promote the
   newer (failed) Freight over a rollback has not been observed.
5. GitHub's "Automatically delete head branches" is the intended cleanup for
   `kargo/promotion/*` branches; it is a repo setting, not in git.

# Related

- [Addon system](../addons/addon-system.md) — how the two addons become
  Applications; [Addon inventory](../addons/addon-inventory.md)
- [Security posture](security-posture.md) — Kyverno default-deny, the
  restrict-registries allowlist Kargo's images fall under
- [AI stack](ai-stack.md) — the `mcp-system` images that motivated pinning
  in the first place

[^guide]: docs/kargo.md.
[^install]: Kargo chart values.
[^chart]: kargo-projects factory chart.
[^targets]: target list (48 Warehouse/Stage pairs).
[^extras]: ExternalSecrets and HTTPRoute.
[^netpol]: kargo namespace network policy.
[^kargo-src]: Kargo v1.11.2 — yaml-update semantics and step schemas.
