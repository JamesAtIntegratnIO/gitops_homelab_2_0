---
type: Kratix Promise
title: argocd-project
description: Leaf promise that renders an argoproj.io AppProject from an ArgoCDProject ResourceRequest.
tags: [kratix, promise, argocd]
status: stable
generated: { by: claude-code/claude-fable-5, at: 2026-08-20T23:40:00Z }
stale_after: 2027-02-20
sources:
  - id: promise-yaml
    resource: ../../../promises/argocd-project/promise.yaml
    title: promise.yaml
  - id: pipeline-src
    resource: ../../../promises/argocd-project/workflows/resource/configure/main.go
    title: Pipeline source
---

# API

`platform.integratn.tech/v1alpha1` **ArgoCDProject**. Required: `name`,
`namespace` (default `argocd`), `sourceRepos`, `destinations[]`
(`server` + `namespace` each); optional cluster/namespace resource
whitelists as `[{group, kind}]`.[^promise-yaml]

# Behavior

Writes a single `argoproj.io/v1alpha1 AppProject` manifest; delete writes a
stub. Used by [vcluster-orchestrator-v2](/promises/vcluster-orchestrator-v2.md)
to give each vcluster its own project scoped to `charts.loft.sh` and the
target namespace ([http-service](/promises/http-service.md) reuses the
`default` project instead). Live example: AppProject `vcluster-media`.[^pipeline-src]

Like the other `argocd-*` promises, it predates `_shared/kratixutil` and
carries local helper copies.

[^promise-yaml]: promise.yaml
[^pipeline-src]: Pipeline source
