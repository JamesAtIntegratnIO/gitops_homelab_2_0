---
type: Platform Design
title: GitOps flow, end to end
description: How a change travels — terraform bootstrap → ApplicationSets → addons; platform CR → Kratix pipeline → state repo → ArgoCD; workload → per-vcluster ArgoCD.
tags: [gitops, argocd, kratix, flow]
status: stable
generated: { by: claude-code/claude-fable-5, at: 2026-08-20T23:40:00Z }
stale_after: 2027-02-20
sources:
  - id: arch-doc
    resource: ../../architecture.md
    title: docs/architecture.md
  - id: live-cluster
    resource: kubectl get applications,appprojects,helmreleases on the-cluster, 2026-08-20
    title: Live ArgoCD state
---

# Layer 0 — bootstrap (imperative, once)

[`tofu apply`](/infrastructure/terraform-bootstrap.md) installs ArgoCD (helm
release `argo-cd`), creates the labeled cluster Secret, and applies two
bootstrap ApplicationSets (helm releases `addons-control-plane`,
`addons-vcluster`). These are the **only three Helm releases** on the cluster;
everything else is ArgoCD-rendered manifests.[^live-cluster]

# Layer 1 — addons

The bootstrap ApplicationSets render the
[application-sets chart](/addons/addon-system.md) with layered value files
(environment → cluster-role → cluster). Every enabled addon becomes its own
ApplicationSet named `bootstrap-<addon>` (or
`bootstrap-vcluster-<cluster>-<addon>` for vcluster addons), which generates
Applications per matching cluster — `<addon>-the-cluster`,
`<addon>-vcluster-media.integratn-tech`, etc. ~47 ApplicationSets and ~55
Applications exist today.

ArgoCD **self-manages**: the `argocd` addon deploys the argo-cd chart onto the
cluster ArgoCD itself runs in (chart pinned at 10.4.0 in addons.yaml; live
server image v3.5.1).

The pins themselves have a feedback loop: [Kargo](kargo.md) watches every
chart repository and image registry referenced from these layers and opens a
PR against `main` when something newer appears — merged by itself for
patch/minor changes under a per-target policy, otherwise waiting for a human.
Kargo only ever writes to git; the layers below see an ordinary commit.

# Layer 2 — platform requests (Kratix)

`platform/vclusters/*.yaml` and `platform/http-services/*.yaml` are synced
into `platform-requests` by the `platform-vclusters` / `platform-http-services`
addons. Kratix runs the promise pipeline, the rendered output lands in the
**public** `kratix-platform-state` repo, and the `kratix-state-reconciler`
ArgoCD Application applies it back to the cluster. Full mechanics:
[Kratix platform layer](/promises/kratix.md).

Composite promises chain through this same loop: a VClusterOrchestratorV2
pipeline emits ArgoCDProject/ArgoCDApplication/ArgoCDClusterRegistration
*ResourceRequests*, which round-trip through the state repo and trigger their
own pipelines.

# Layer 3 — workloads (inside vclusters)

Each vcluster runs its own ArgoCD (the `argocd-vcluster` addon), whose
`valuesObject` bootstraps a nested **workloads ApplicationSet** that re-invokes
the same application-sets chart against `workloads/<vcluster-name>/`
(`addons.yaml` + per-app values). `hctl deploy` writes Score-derived entries
there; the media stack lives in `workloads/vcluster-media/`.

# Reconciliation behavior

- Most apps: `automated: {selfHeal: true, prune: true}`; a few are
  deliberately manual (`kratix` at the environment layer sets selfHeal:false —
  overridden back to true at the cluster-role layer).
- Sync waves order addon internals (e.g. external-secrets at wave -3,
  cert-manager -1, routes +10).
- ArgoCD Projects in use: `default`, `platform`, `platform-services`,
  `appteam-global` (from the `argocd-projects` manifest addon), plus
  per-vcluster projects (e.g. `vcluster-media`) created by the
  [argocd-project promise](/promises/argocd-project.md).
- Global VPA note: ArgoCD is configured with `ignoreDifferences` for container
  resources so [Goldilocks/VPA](/platform/security-posture.md) can mutate
  requests without fighting self-heal.

[^arch-doc]: docs/architecture.md
[^live-cluster]: Live ArgoCD state
