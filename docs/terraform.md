# Terraform

Terraform/OpenTofu is used for cluster bootstrap, DNS records, and platform prerequisites.

## Location
- `terraform/cluster/`: primary stack (providers, bootstrap, modules).
- `terraform/modules/`: reusable modules (Cloudflare, etc.).

## Prerequisites
- OpenTofu/Terraform 1.6.x (see `terraform/cluster/versions.tf`).
- Kubernetes access (kubeconfig).
- Cloudflare API token.
- 1Password Connect token.

## Backend
Backend configuration is in `terraform/cluster/backend.hcl` (git‑ignored). Copy from:
- `terraform/cluster/backend.hcl.example`

## Variables
Variables are in `terraform/cluster/variables.tf` and `terraform/cluster/terraform.tfvars` (git‑ignored). Required inputs include:
- `cluster_name`
- `cloudflare_api_key`
- `cloudflare_zone_name`
- `onepassword_token`
- `cloudflare_records`

## Modules
### Cloudflare module
Location: `terraform/modules/cloudflare`
Inputs:
- `cloudflare_zone_name`
- `cloudflare_records` (map of DNS records)

## Workflow
1. Initialize: `tofu init -backend-config=backend.hcl`
2. Validate: `tofu fmt -check -recursive` and `tofu validate`
3. Plan: `tofu plan`
4. Apply: `tofu apply`

## Notes
- State and tfvars are ignored by Git.
- Use the changes log: `TERRAFORM_CHANGES.md`.
