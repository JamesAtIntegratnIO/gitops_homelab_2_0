---
type: Kratix Promise
title: argocd-cluster-registration
description: The kubeconfig→1Password→ArgoCD loop — registers any cluster with ArgoCD without a Secret ever touching git.
tags: [kratix, promise, argocd, 1password, kubeconfig]
status: stable
generated: { by: claude-code/claude-fable-5, at: 2026-08-20T23:40:00Z }
stale_after: 2027-02-20
sources:
  - id: promise-yaml
    resource: ../../../promises/argocd-cluster-registration/promise.yaml
    title: promise.yaml
  - id: builders
    resource: ../../../promises/argocd-cluster-registration/workflows/resource/configure/builders.go
    title: Pipeline builders (sync job, ExternalSecrets)
---

# API

`platform.integratn.tech/v1alpha1` **ArgoCDClusterRegistration**. Required:
`name`, `targetNamespace`, `kubeconfigSecret`, `externalServerURL`. Defaults:
`kubeconfigKey: config`, `onePasswordItem: {name}-kubeconfig`,
`onePasswordConnectHost: https://connect.integratn.tech`, plus passthrough
cluster labels/annotations.[^promise-yaml]

# The credential loop (no Secret in git, ever)

Four outputs implement a full circle:[^builders]

1. **RBAC + token ExternalSecret** — a ServiceAccount/Role scoped to exactly
   the kubeconfig secret, plus an ExternalSecret pulling the 1Password Connect
   token (`onepassword-access-token` item) into the target namespace.
2. **Kubeconfig sync Job** — waits for the vcluster's `vc-{name}` kubeconfig
   secret, extracts CA/client cert/key, builds an ArgoCD `tlsClientConfig`
   JSON, and creates/updates a SERVER-category **1Password item** with fields
   `kubeconfig`, `argocd-name`, `argocd-server`, `argocd-config`. All
   credentials arrive via `secretKeyRef` env — nothing literal in git.
3. **Kubeconfig ExternalSecret** — materializes `{name}-kubeconfig-external`
   back from 1Password (refresh 15m) for developer consumption.
4. **ArgoCD cluster ExternalSecret** — in namespace `argocd`, creates Secret
   `cluster-{name}` labeled `argocd.argoproj.io/secret-type: cluster` from the
   same 1Password item, refresh **1m** (so ArgoCD picks up a new CA quickly
   after a vcluster is recreated).

The resulting cluster Secret's labels (`cluster_role: vcluster`,
`environment`, `enable_*` flags) are what make the
[vcluster addon ApplicationSets](/addons/addon-system.md) target the new
cluster. Live example: `cluster-vcluster-media` in the `argocd` namespace.

Re-runs are forced by a digits-suffix on the Job name derived from the
`platform.integratn.tech/reconcile-at` **metadata annotation** (note: setting
it under `spec.integrations.argocd.clusterAnnotations`, as
`platform/vclusters/vcluster-media.yaml` currently does, does *not* reach the
job name — see [known issues](/cluster/known-issues.md)).

Delete writes stubs for all seven resources. Not vcluster-specific — any
cluster with a kubeconfig secret can be registered. Consumed by
[vcluster-orchestrator-v2](/promises/vcluster-orchestrator-v2.md).

[^promise-yaml]: promise.yaml
[^builders]: Pipeline builders (sync job, ExternalSecrets)
