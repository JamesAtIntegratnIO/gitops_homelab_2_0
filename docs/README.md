# GitOps Homelab Documentation

This folder is the canonical, deep‑dive knowledge base for the homelab platform. It focuses on *how the system actually works* and *how to operate it safely*.

## Navigation
- [Architecture](architecture.md)
- [Bootstrap & Bare‑metal Talos](bootstrap.md)
- [Addons & ApplicationSets](addons.md)
- [vCluster Platform Requests](vclusters.md)
- [Kratix Promises & Pipelines](promises.md)
- [Observability](observability.md)
- [Terraform & Infrastructure](terraform.md)
- [Operations & Runbooks](operations.md)

## Repository Anchors
- Root overview: ../README.md
- Terraform changes: ../TERRAFORM_CHANGES.md

## Non‑Negotiables
- Git is the only source of truth.
- No secrets in Git: use ExternalSecrets + 1Password.
- Use Gateway API (nginx‑gateway‑fabric) for HTTP exposure.
- All changes are small, reviewed, and reversible.
