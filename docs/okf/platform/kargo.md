---
type: Platform Capability
title: Kargo — automated image and chart version updates
description: How every pinned image tag, digest and chart version in the repo is kept current — Kargo Warehouses watch the sources, one Stage per artifact opens a PR against main, merges it under a per-target policy, and verifies the affected ArgoCD Applications afterwards.
tags: [kargo, updates, images, charts, automation, gitops]
status: draft
generated: { by: claude-code/claude-fable-5, at: 2026-08-21T22:30:00Z }
stale_after: 2026-09-21
sources:
  - id: chart
    resource: ../../../addons/charts/kargo-projects/
    title: kargo-projects factory chart
  - id: targets
    resource: ../../../addons/cluster-roles/control-plane/addons/kargo-projects/values.yaml
    title: target list (48 Warehouse/Stage pairs)
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
  - id: reference
    resource: https://github.com/Tensure/gitops-apps-config (private)
    title: Tensure's Kargo chart-updater pipelines, the implementation reference
  - id: kargo-src
    resource: https://github.com/akuity/kargo/tree/v1.11.2 (pkg/yaml, pkg/promotion/runner/builtin)
    title: Kargo v1.11.2 — yaml-update semantics and step schemas
---

**Status: authored 2026-08-21 on a branch; nothing is live until it merges.**
The first person to see it running should set `status: stable`, record the
day-one PR burst in the [log](../log.md), and re-check the assumptions listed
at the bottom.

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
| Verification | `argo-rollouts-crds` addon (three analysis CRDs from `argoproj/argo-rollouts` v1.9.1, itself a Kargo git-tag target); AnalysisTemplate `argocd-apps-healthy` per Project; `verify.apps` on every `addons`/`promises` target | Kargo's controller runs the AnalysisRuns; no Rollouts controller. Prometheus query over `argocd_app_info` 3m after the merge, 5×1m, 2 failures allowed. `autoRollback` available but off |

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

# Assumptions to verify on first run

1. `yaml-parse` evaluates `fromExpression` with the parsed document as the
   expr-lang environment, so `$env["…"]` works for dashed keys (expr v1.17.8
   is vendored[^kargo-src]). If chart Stages fail at `yaml-parse`, this is
   why.
2. The `SemVer` strategy with no constraint still selects `7.x.y-alpine`
   tags for `authentik-redis` (semver treats `-alpine` as a prerelease).
3. The Kargo API starts even while Authentik is briefly unreachable — if it
   crash-loops on OIDC discovery, the `allow-api-oidc` policy or Authentik
   itself is the first suspect.
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
