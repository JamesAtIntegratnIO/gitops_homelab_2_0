---
type: Platform Framework
title: Kratix platform layer
description: How Kratix turns ResourceRequests into rendered manifests via Go pipelines, a public git state store, and the ArgoCD state reconciler — plus the custom status reconciler.
tags: [kratix, platform-engineering, state-store, pipelines]
status: stable
generated: { by: claude-code/claude-fable-5, at: 2026-08-20T23:40:00Z }
stale_after: 2027-02-20
sources:
  - id: kratix-addon
    resource: ../../../addons/cluster-roles/control-plane/addons/kratix/values.yaml
    title: Kratix addon values (destination, state reconciler, RBAC)
  - id: live-kratix
    resource: kubectl get promises,destinations,gitstatestores,works on the-cluster, 2026-08-20
    title: Live Kratix state
  - id: kratix-troubleshooting
    resource: ../../kratix-troubleshooting.md
    title: docs/kratix-troubleshooting.md
  - id: status-contract
    resource: ../../platform-status-contract.md
    title: docs/platform-status-contract.md
---

# Flow

```
ResourceRequest (CR in platform-requests)
  → pipeline Job (Go binary, ghcr.io/jamesatintegratnio/<promise>-configure)
  → Work → WorkPlacement (destination: the-cluster)
  → GitStateStore commit → github.com/jamesatintegratnio/kratix-platform-state (PUBLIC, branch main, path clusters/)
  → ArgoCD app `kratix-state-reconciler` applies the manifests
```

Key live facts (2026-08-20):[^live-kratix]

- **7 promises installed and Available**: [vcluster-orchestrator-v2](/promises/vcluster-orchestrator-v2.md),
  [http-service](/promises/http-service.md), [argocd-application](/promises/argocd-application.md),
  [argocd-project](/promises/argocd-project.md), [argocd-cluster-registration](/promises/argocd-cluster-registration.md),
  [gateway-route](/promises/gateway-route.md), [external-secret](/promises/external-secret.md).
  Promises are themselves GitOps-installed by the `kratix-promises` addon
  (manifestSource → `promises/` in this repo @ main, excluding workflows/examples).
- **One Destination** — `the-cluster` (labels include `capability.vcluster: "true"`,
  which every promise's `destinationSelectors` matches), backed by GitStateStore
  `default` using **githubApp auth**.[^kratix-addon]
- Kratix runs in `kratix-platform-system` (syntasso community image, SHA-pinned);
  pipeline Jobs run in `platform-requests` and are re-run periodically (~every
  10h) by Kratix's reconciliation; a `kratix-pipeline-cleanup` CronJob prunes
  old pipeline pods daily at 04:00.

# The public-state-repo constraint

The state repo is **public**, which drives the platform's most important rule:
**pipelines must never emit `kind: Secret`** — only `ExternalSecret` resources
referencing the 1Password ClusterSecretStore. Enforced by CI
(see [CI workflows](/tooling/ci-workflows.md)) and by convention in every
pipeline. See [secret management](/platform/secret-management.md).

# Pipeline conventions

All 7 promises share: CRD group `platform.integratn.tech/v1alpha1`, namespaced,
single Go image per promise handling both `configure` and `delete` actions
(branched on `sdk.WorkflowAction()`), hardened pod security (non-root 65532,
read-only rootfs, drop ALL), `imagePullPolicy: Always` with `:latest` tags.
Four newer promises share the typed helper library
`promises/_shared/kratixutil` (Resource envelope, sub-request spec types,
DeepMerge for helmOverrides, delete-stub writers); the three `argocd-*`
promises predate it and carry local copies. A change to `_shared/` rebuilds
every promise image in CI.

Composite promises emit **sub-ResourceRequests** into `platform-requests`
(git-mediated composition — the state repo round-trip applies the child CR,
which triggers the child pipeline). Ordering is governed by ArgoCD sync-waves
in the emitted manifests (-2 namespace → -1 project → 0 app → 5 netpols → 10
routes), not by Kratix scheduling.

# Status: two writers

1. **Pipeline** writes a static status once (phase, endpoints, credentials
   *references* — secret names, never values).
2. **platform-status-reconciler** (custom controller from
   `images/platform-status-reconciler/`, running in its own namespace)
   re-patches `.status` on every `VClusterOrchestratorV2` every 60s: ArgoCD
   sync/health, pod readiness, sub-app rollup, kubeconfig-secret existence,
   plus Prometheus metrics (`platform_vcluster_*`) and alerts. Design doc:
   [docs/platform-status-contract.md](../../platform-status-contract.md).[^status-contract]

# Known operational quirks

- A WorkPlacement (or the request's `Reconciled` condition) can show
  **Failing while everything actually works** — e.g. stale "no files changed"
  errors or a leftover suspended Job from initial provisioning. This exact
  cosmetic state is live on `vcluster-media` today
  (see [known issues](/cluster/known-issues.md)).
- GitStateStore `/tmp/kratix-repo` corruption is a known Kratix bug; the fix is
  restarting `kratix-platform-controller-manager`.[^kratix-troubleshooting]
- Force a pipeline re-run with the `kratix.io/manual-reconciliation` /
  `platform.integratn.tech/reconcile-at` annotations, or `hctl reconcile`.

[^live-kratix]: Live Kratix state
[^kratix-addon]: Kratix addon values (destination, state reconciler, RBAC)
[^kratix-troubleshooting]: docs/kratix-troubleshooting.md
[^status-contract]: docs/platform-status-contract.md
