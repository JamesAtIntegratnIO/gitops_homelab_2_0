---
type: Reference
title: Getting started with this bundle
description: Orientation for the GitOps Homelab 2.0 knowledge bundle — what the platform is, how to navigate, and how this bundle was produced.
tags: [getting-started]
status: stable
generated: { by: claude-code/claude-fable-5, at: 2026-08-20T23:40:00Z }
sources:
  - id: repo
    resource: ../../README.md
    title: Repository README
---

# What you're looking at

This bundle documents **GitOps Homelab 2.0**: a self-service internal
developer platform on three bare-metal Talos nodes, reconciled by ArgoCD,
with Kratix promises as the platform API and vcluster for tenancy. It covers
both the **repository** (how things are defined) and the **live cluster
`the-cluster`** (what is actually running), including where the two disagree.

# How to navigate

- **Understand the design** → [platform/architecture.md](platform/architecture.md),
  then [platform/gitops-layers.md](platform/gitops-layers.md).
- **See what's running right now** → [cluster/](cluster/index.md) — the
  inventory/version concepts are dated snapshots with `stale_after`.
- **Change or add a platform service** → [addons/addon-system.md](addons/addon-system.md).
- **Request infrastructure (vcluster, HTTP app)** → [promises/](promises/index.md).
- **Operate / debug** → [cluster/known-issues.md](cluster/known-issues.md)
  plus the prose runbooks in [../operations.md](../operations.md) and
  [../kratix-troubleshooting.md](../kratix-troubleshooting.md).

This bundle complements — does not replace — the narrative docs in
[docs/](../README.md). Concepts cite those docs and the underlying source
files in their `sources` frontmatter; live-cluster claims cite the kubectl
observations they came from.

# Conventions

- Frontmatter `generated` records who/when wrote a concept; snapshot-type
  concepts carry `stale_after` — treat anything past that date as needing
  re-verification against the cluster.
- No secret **values** appear anywhere in this bundle (only names of secrets
  and 1Password items, which are already public in the repo). Keep it that
  way when updating.
- Bundle-internal links are bundle-absolute (`/platform/architecture.md`);
  links out to the repo are relative (`../../…`).

# Maintaining

When the platform changes, update the affected concept(s), refresh
`generated.at`, fix cross-links, append a dated entry to [log.md](log.md),
and validate with the OKF checker (`/okf:validate docs/okf --strict`).
