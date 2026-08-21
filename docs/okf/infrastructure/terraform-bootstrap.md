---
type: Infrastructure Bootstrap
title: Terraform/OpenTofu cluster bootstrap
description: What `tofu apply` in terraform/cluster creates — ArgoCD + bootstrap ApplicationSets, the 1Password token seed, Cloudflare DNS, and the GHCR pull secret.
tags: [terraform, opentofu, bootstrap, argocd, cloudflare]
status: stable
generated: { by: claude-code/claude-fable-5, at: 2026-08-20T23:40:00Z }
stale_after: 2027-02-20
sources:
  - id: tf-main
    resource: ../../../terraform/cluster/main.tf
    title: terraform/cluster/main.tf
  - id: tf-eso
    resource: ../../../terraform/cluster/external_secrets_operator.tf
    title: external_secrets_operator.tf
  - id: tf-bootstrap
    resource: ../../../terraform/cluster/bootstrap/addons-control-plane.yaml
    title: Bootstrap ApplicationSet manifests
  - id: terraform-doc
    resource: ../../terraform.md
    title: docs/terraform.md
---

# What it is

`terraform/cluster/` is the **one imperative step** in the platform: it runs
once against a freshly bootstrapped Talos cluster and hands control to ArgoCD.
It is an OpenTofu workspace (lockfile resolves against `registry.opentofu.org`)
with a **PostgreSQL state backend** configured at init time via a gitignored
`backend.hcl`.[^tf-main]

Providers: `kubernetes` 2.31.0 and `helm` 2.10.1 (both pointed at the
gitignored kubeconfig `matchbox/assets/talos/1.11.5/kubeconfig`, context
`admin@<cluster_name>`), `cloudflare` 4.39.0, `onepassword` ~>3.0.2 (declared
but currently unused by any resource).

# What `tofu apply` creates

Four largely parallel branches:

1. **ExternalSecrets seed** — namespace `external-secrets` plus Secret
   `eso-onepassword-token` (key `token`) from the sensitive
   `var.onepassword_token`. This is the **only place the raw 1Password Connect
   token is ever handled**; everything downstream references the Secret by
   name.[^tf-eso] See [secret management](/platform/secret-management.md).
2. **Cloudflare DNS** — `modules/cloudflare` creates records `for_each` over
   `var.cloudflare_records` (data lives in gitignored `terraform.tfvars`;
   documented set: `argocd`, `grafana`, `prometheus`, and a `*` wildcard, all
   A-records → 10.0.4.205). Static records only; per-service DNS is
   external-dns's job with `txtOwnerId: the-cluster` to avoid ownership fights.
3. **ArgoCD** — external module
   `terraform-helm-gitops-bridge` (branch-pinned `?ref=homelab`), ArgoCD chart
   9.0.3. Installs ArgoCD, creates the in-cluster **cluster Secret** whose
   labels (`cluster_role: control-plane`, `environment: production`, and ~14
   `enable_*` flags) drive all ApplicationSet targeting, and applies the two
   bootstrap ApplicationSets.
4. **GHCR pull secret** — `ghcr-login-secret` in `argocd`, built from a
   gitignored `dockerconfig.json` (ordered after the ArgoCD module).

# Bootstrap ApplicationSets

Both live in `terraform/cluster/bootstrap/` and generate one child
ApplicationSet per addon via the
[application-sets Helm chart](/addons/addon-system.md):

- `cluster-addons-control-plane` — clusters generator matching
  `cluster_role In [control-plane]`, destination `{{name}}`, five layered
  `valueFiles` (environment → environment common → cluster-role → cluster-role
  common → cluster), `ignoreMissingValueFiles: true`, `appsetPrefix: bootstrap-`.
- `cluster-addons-vcluster` — same shape for `cluster_role In [vcluster]`, but
  its destination is hardcoded to `the-cluster`: vcluster addon ApplicationSets
  are rendered *onto the host*, and each generated Application then targets the
  vcluster.[^tf-bootstrap]

These correspond to the only Helm releases visible on the cluster
(`argo-cd`, `addons-control-plane`, `addons-vcluster`, installed 2026-01-15 /
2026-02-17) — everything else is ArgoCD-rendered manifests with no Helm
release object.

# Known gotchas

- `main.tf` reads `dockerconfig.json` at *plan* time; on a clean checkout the
  file is absent and `tofu plan` fails before printing a plan.
- Several variable defaults are dead/stale (`gitops_addons_repo` defaults to
  `gitops-homelab`; the real values must come from `terraform.tfvars`).
- The ArgoCD module is pinned to a branch, not a SHA (self-flagged in the
  terraform README).

Related: [architecture](/platform/architecture.md),
[addon system](/addons/addon-system.md), [known issues](/cluster/known-issues.md).

[^tf-main]: terraform/cluster/main.tf
[^tf-eso]: external_secrets_operator.tf
[^tf-bootstrap]: Bootstrap ApplicationSet manifests
