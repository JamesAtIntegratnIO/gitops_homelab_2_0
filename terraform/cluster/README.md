# Terraform Setup

## Prerequisites

- OpenTofu or Terraform >= 1.6.0, < 2.0.0
- Access to Kubernetes cluster (kubeconfig)
- Cloudflare API token
- 1Password Connect instance

## Initial Setup

1. **Configure Backend**

   Copy the backend configuration template:
   ```bash
   cd terraform/cluster
   cp backend.hcl.example backend.hcl
   ```

   Edit `backend.hcl` with your Postgres connection details:
   ```hcl
   conn_str = "postgres://YOUR_HOST/YOUR_DATABASE"
   ```

   Note: `backend.hcl` is git-ignored for portability.

2. **Initialize Terraform**

   ```bash
   tofu init -backend-config=backend.hcl
   ```

3. **Configure Variables**

   Create or edit `terraform.tfvars` (git-ignored) with your environment values:
   ```hcl
   cluster_name         = "your-cluster-name"
   cloudflare_api_key   = "your-cloudflare-api-token"
   cloudflare_zone_name = "your-domain.com"
   onepassword_token    = "your-1password-token"
   cloudflare_records   = {
     # Your DNS records
   }
   ```

4. **Plan and Apply**

   ```bash
   tofu plan
   tofu apply
   ```

## Provider Versions

- Kubernetes: 2.31.0
- Helm: 2.10.1
- Cloudflare: 4.39.0
- 1Password: ~> 3.0.2

## Module Sources

- ArgoCD module: `git::https://github.com/jamesAtIntegratnIO/terraform-helm-gitops-bridge.git?ref=homelab`
  - Note: Consider pinning to a specific commit SHA for reproducibility
- Cloudflare module: local at `../modules/cloudflare`

## Ignored Files

The following patterns are ignored by git to prevent accidental commits:
- `.terraform/` - Terraform working directory
- `*.tfstate*` - State files (using Postgres backend)
- `*.tfvars` - Variable files (may contain secrets)
- `backend.hcl` - Backend configuration (environment-specific)
- `dockerconfig.json` - Docker credentials

## Validation

Run formatting and validation checks:
```bash
tofu fmt -check -recursive
tofu validate
```
