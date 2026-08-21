---
type: Kratix Promise
title: argocd-application
description: Leaf promise that turns an ArgoCDApplication ResourceRequest into a real argoproj.io Application manifest.
tags: [kratix, promise, argocd]
status: stable
generated: { by: claude-code/claude-fable-5, at: 2026-08-20T23:40:00Z }
stale_after: 2027-02-20
sources:
  - id: promise-yaml
    resource: ../../../promises/argocd-application/promise.yaml
    title: promise.yaml
  - id: pipeline-src
    resource: ../../../promises/argocd-application/workflows/resource/configure/main.go
    title: Pipeline source
---

# API

`platform.integratn.tech/v1alpha1` **ArgoCDApplication**. Required: `name`,
`namespace` (default `argocd`), `project`, `source` (`repoURL` +
`targetRevision`), `destination` (`server` + `namespace`).
`source.helm.valuesObject` and `syncPolicy` are free-form
(`x-kubernetes-preserve-unknown-fields`).[^promise-yaml]

# Behavior

A thin translator: validates required fields and writes a single
`argoproj.io/v1alpha1 Application` manifest (labels/annotations/finalizers
passed through; a `helm:` block only when `releaseName` or `valuesObject` is
present). Delete writes a minimal delete stub.[^pipeline-src]

# Notes

- **Consumers**: [vcluster-orchestrator-v2](/promises/vcluster-orchestrator-v2.md)
  (the vcluster Helm app) and [http-service](/promises/http-service.md)
  (the Stakater app). Both compose with it via sub-ResourceRequests.
- One of the three older promises that does **not** use
  `promises/_shared/kratixutil` — it carries local copies of the helper types
  (candidate for consolidation).

[^promise-yaml]: promise.yaml
[^pipeline-src]: Pipeline source
