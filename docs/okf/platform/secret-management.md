---
type: Security Design
title: Secret management (1Password → ExternalSecrets)
description: The zero-secrets-in-git design — one seeded Connect token, one ClusterSecretStore, ExternalSecrets everywhere, and the enforcement layers around it.
tags: [secrets, 1password, external-secrets, security]
status: stable
generated: { by: claude-code/claude-fable-5, at: 2026-08-20T23:40:00Z }
stale_after: 2027-02-20
sources:
  - id: eso-addon
    resource: ../../../addons/environments/production/addons/addons.yaml
    title: external-secrets addon + ClusterSecretStore definition
  - id: live-eso
    resource: kubectl get clustersecretstores,externalsecrets -A on the-cluster, 2026-08-20
    title: Live ExternalSecrets state
  - id: arch-doc
    resource: ../../architecture.md
    title: docs/architecture.md (ADR-004, secret flow)
---

# Design

**No secret values ever live in git** — not even encrypted. 1Password (vault
`homelab`) is the single source of truth, reached through a self-hosted
**1Password Connect** at `https://connect.integratn.tech` (runs outside the
cluster; egress rules point at `10.0.1.139:443`).

The chain:

1. Terraform seeds exactly one Secret: `eso-onepassword-token` in
   `external-secrets` (the Connect token). The only raw credential Terraform
   ever touches.
2. The external-secrets addon (sync-wave -3) installs ESO v0.10.3 and a
   **ClusterSecretStore `onepassword-store`** (ReadOnly) whose auth references
   that token Secret by name.[^eso-addon]
3. Everything else declares `ExternalSecret` resources naming a 1Password
   item + property. ESO materializes real Secrets in-cluster and refreshes
   them (typically 15m; the ArgoCD cluster registration secret refreshes
   every 1m).

Live state 2026-08-20: the store is Valid/Ready and all **24 ExternalSecrets
are Ready=True**, spanning: ArgoCD OIDC, Authentik (bootstrap admin, OIDC
pairs for grafana/argocd/open-webui, Google OAuth), Cloudflare API key
(cert-manager, external-dns, nginx-gateway), Grafana admin + OIDC, Matrix
receiver, GitHub credentials for Kratix, MCP server tokens, and the
vcluster-media kubeconfig pair.[^live-eso]

# vclusters

The host's store syncs *into* each vcluster (the vcluster syncs the
`eso-onepassword-token` secret from host and runs its own ESO), selected by
label `integratn.tech/cluster-secret-store: onepassword-store`. Kubeconfigs
round-trip through 1Password via the
[argocd-cluster-registration promise](/promises/argocd-cluster-registration.md).

# Enforcement layers

1. CI: `validate-promises.yaml` fails on `^kind: Secret$` under `promises/`
   (the Kratix state repo is public — this is the hard gate).
2. Pre-commit hook (currently broken — wrong path filter; see
   [known issues](/cluster/known-issues.md)).
3. Promise convention: pipelines emit only
   [PlatformExternalSecret](/promises/external-secret.md)/ExternalSecret.
4. git-indexer redacts `kind: Secret` docs + token patterns before RAG
   embedding.
5. `.gitignore` keeps every credential-bearing artifact out: rendered Talos
   configs, talosconfig, kubeconfigs, `terraform.tfvars`, `backend.hcl`,
   `dockerconfig.json`, `secrets.env`.

# Operational notes

- Rotate in 1Password → ESO syncs on next refresh; force with an annotation
  (`force-sync=<ts>`) or by deleting the ExternalSecret.
- If everything breaks at once, check 1Password Connect reachability first —
  it is the platform's single external secret dependency.
- `hctl secret get/list` and the Nix `get_secret_data` helper print decoded
  values to the terminal; treat with the same care as any credential.

[^eso-addon]: external-secrets addon + ClusterSecretStore definition
[^live-eso]: Live ExternalSecrets state
[^arch-doc]: docs/architecture.md (ADR-004, secret flow)
