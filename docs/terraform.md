# Terraform/OpenTofu

> **Official Documentation References:**
> - [OpenTofu Documentation](https://opentofu.org/docs/) - Open-source Terraform alternative
> - [Terraform Kubernetes Provider](https://registry.terraform.io/providers/hashicorp/kubernetes/latest/docs) - Kubernetes resource management
> - [Terraform Helm Provider](https://registry.terraform.io/providers/hashicorp/helm/latest/docs) - Helm chart deployment
> - [Cloudflare Provider](https://registry.terraform.io/providers/cloudflare/cloudflare/latest/docs) - DNS management

## Overview

Terraform (via OpenTofu) handles **cluster bootstrap** and **foundational infrastructure** that must exist before GitOps takes over. This includes:

1. **ArgoCD installation** - GitOps engine deployment
2. **Bootstrap ApplicationSets** - Point ArgoCD at this repo
3. **DNS records** - Cloudflare A/CNAME records for services
4. **Secrets** - Initial credentials (1Password Connect token, GHCR pull secrets)
5. **Cluster registration** - ArgoCD Cluster Secret with labels

**Why Terraform:**
- ✅ **Chicken-and-egg solver**: Deploys ArgoCD before GitOps can work
- ✅ **State management**: Tracks external resources (DNS, cloud APIs)
- ✅ **Idempotent**: Safe to re-run (applies only changes)
- ✅ **Declarative**: Infrastructure as code
- ❌ **Not for everything**: GitOps handles 95% of cluster config after bootstrap

**GitOps vs Terraform Boundary:**
```
Terraform Territory:
- ArgoCD Helm release
- Bootstrap ApplicationSets (point ArgoCD at Git)
- DNS records (Cloudflare)
- Initial secrets (1Password Connect token)
- Cluster Secret with labels (for ApplicationSet selectors)

GitOps Territory (ArgoCD manages):
- All addons (cert-manager, external-secrets, gateways, etc.)
- Kratix Promises
- vCluster ResourceRequests
- Application manifests
- TLS certificates
- Monitoring dashboards
```

## Architecture Diagram

```mermaid
flowchart TB
    subgraph Workstation["💻 Workstation (Developer/CI)"]
        TFVars["📄 terraform.tfvars<br/>(git-ignored)<br/>cluster_name<br/>cloudflare_api_key<br/>onepassword_token"]
        Backend["🗄️ Remote Backend<br/>S3/Consul<br/>terraform.tfstate<br/>State locking"]
        
        TFVars -->|"tofu init<br/>-backend-config=backend.hcl"| Backend
    end
    
    subgraph APIs["🌐 External APIs"]
        CF["☁️ Cloudflare API<br/>A/CNAME/TXT records"]
        K8s["☸️ Kubernetes API<br/>Namespaces, Secrets<br/>ArgoCD Helm<br/>ApplicationSets"]
        GH["🐙 GitHub API<br/>Repos, Webhooks<br/>(Future use)"]
    end
    
    Backend -->|"tofu apply"| CF
    Backend --> K8s
    Backend --> GH
    
    style Workstation fill:#bbdefb,stroke:#1976d2,stroke-width:2px
    style APIs fill:#c8e6c9,stroke:#388e3c,stroke-width:2px
    style TFVars fill:#ff9800,stroke:#fff,stroke-width:2px,color:#fff
    style Backend fill:#42a5f5,stroke:#fff,stroke-width:2px,color:#fff
```

## Directory Structure

```
terraform/
├── cluster/                          # Main deployment stack
│   ├── backend.hcl                   # Remote state config (git-ignored)
│   ├── backend.hcl.example           # Template for backend.hcl
│   ├── main.tf                       # Root module
│   ├── variables.tf                  # Input variable definitions
│   ├── terraform.tfvars              # Variable values (git-ignored)
│   ├── versions.tf                   # Provider version constraints
│   ├── external_secrets_operator.tf  # ESO namespace and initial secret
│   ├── dockerconfig.json             # GHCR pull secret (git-ignored)
│   ├── README.md                     # Setup instructions
│   └── bootstrap/                    # Bootstrap ApplicationSets
│       ├── addons-control-plane.yaml # Control-plane addons ApplicationSet
│       └── addons-vcluster.yaml      # vCluster addons ApplicationSet
│
└── modules/                          # Reusable modules
    └── cloudflare/                   # DNS record management
        ├── main.tf
        ├── variables.tf
        └── outputs.tf
```

## Bootstrap Flow (Step-by-Step)

### Step 1: Prerequisites

**Install OpenTofu:**
```bash
# macOS
brew install opentofu

# Linux (snap)
snap install opentofu --classic

# Verify installation
tofu version
# OpenTofu v1.6.2
```

**Obtain Required Credentials:**
1. **Cloudflare API Token:**
   ```bash
   # Log in to Cloudflare Dashboard
   # My Profile → API Tokens → Create Token
   # Template: Edit zone DNS
   # Zone Resources: Include → Specific zone → integratn.tech
   # Copy token to terraform.tfvars
   ```

2. **1Password Connect Token:**
   ```bash
   # Already deployed via Talos/initial setup
   # Token stored in 1Password vault: homelab
   # Item: 1password-connect-token
   op item get 1password-connect-token --vault homelab --fields token
   ```

3. **GitHub Container Registry (GHCR) Credentials:**
   ```bash
   # Create Personal Access Token
   # GitHub Settings → Developer settings → Personal access tokens → Tokens (classic)
   # Scopes: read:packages
   # Generate dockerconfig.json:
   echo -n "your-github-username:your-pat-token" | base64
   # ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx

   cat > terraform/cluster/dockerconfig.json <<EOF
   {
     "auths": {
       "ghcr.io": {
         "auth": "base64-string-from-above"
       }
     }
   }
   EOF
   ```

### Step 2: Configure Backend

**Create backend.hcl:**
```bash
cd terraform/cluster/
cp backend.hcl.example backend.hcl

# Edit backend.hcl
cat > backend.hcl <<EOF
# S3-compatible backend (MinIO, AWS S3, etc.)
bucket = "terraform-state"
key    = "homelab/the-cluster/terraform.tfstate"
region = "us-east-1"

# For MinIO/self-hosted S3:
endpoint                    = "https://minio.integratn.tech"
skip_credentials_validation = true
skip_metadata_api_check     = true
skip_region_validation      = true
force_path_style            = true

# Credentials from environment variables:
# export AWS_ACCESS_KEY_ID=minioadmin
# export AWS_SECRET_ACCESS_KEY=minioadmin
EOF
```

**Alternative Backends:**

**Consul:**
```hcl
# backend.hcl
address = "consul.integratn.tech:8500"
scheme  = "https"
path    = "terraform/the-cluster"
```

**Kubernetes:**
```hcl
# backend.hcl
secret_suffix    = "the-cluster"
namespace        = "terraform-state"
in_cluster_config = false  # Use kubeconfig
```

### Step 3: Configure Variables

**Create terraform.tfvars:**
```bash
cp terraform.tfvars.example terraform.tfvars

# Edit terraform.tfvars
cat > terraform.tfvars <<EOF
cluster_name = "the-cluster"

# Cloudflare Configuration
cloudflare_api_key = "your-cloudflare-api-token"
cloudflare_zone_name = "integratn.tech"

cloudflare_records = {
  "argocd" = {
    name    = "argocd"
    content = "10.0.4.205"  # Gateway LoadBalancer IP
    type    = "A"
    ttl     = 300
    proxied = false
  }
  "grafana" = {
    name    = "grafana"
    content = "10.0.4.205"
    type    = "A"
    ttl     = 300
    proxied = false
  }
  "prometheus" = {
    name    = "prometheus"
    content = "10.0.4.205"
    type    = "A"
    ttl     = 300
    proxied = false
  }
  "wildcard-cluster" = {
    name    = "*"
    content = "10.0.4.205"
    type    = "A"
    ttl     = 300
    proxied = false
  }
}

# 1Password Configuration
onepassword_token = "your-1password-connect-token"

# GitOps Repository
gitops_addons_org      = "https://github.com/jamesatintegratnio"
gitops_addons_repo     = "gitops_homelab_2_0"
gitops_addons_revision = "main"
gitops_addons_basepath = ""
gitops_addons_path     = "addons/charts/application-sets"
EOF
```

### Step 4: Initialize Terraform

```bash
cd terraform/cluster/

# Initialize providers and backend
tofu init -backend-config=backend.hcl

# Output:
# Initializing the backend...
# Successfully configured the backend "s3"!
#
# Initializing provider plugins...
# - Finding hashicorp/kubernetes versions matching "~> 2.35.0"...
# - Finding hashicorp/helm versions matching "~> 2.17.0"...
# - Finding cloudflare/cloudflare versions matching "~> 4.0"...
# - Installing hashicorp/kubernetes v2.35.1...
# - Installing hashicorp/helm v2.17.0...
# - Installing cloudflare/cloudflare v4.45.0...
#
# Terraform has been successfully initialized!
```

### Step 5: Review Planned Changes

```bash
tofu plan

# Sample output:
# Terraform will perform the following actions:
#
#   # kubernetes_namespace.argocd will be created
#   + resource "kubernetes_namespace" "argocd" {
#       + id = (known after apply)
#       metadata {
#           + name = "argocd"
#         }
#     }
#
#   # helm_release.argocd will be created
#   + resource "helm_release" "argocd" {
#       + name       = "argocd"
#       + namespace  = "argocd"
#       + repository = "https://argoproj.github.io/argo-helm"
#       + chart      = "argo-cd"
#       + version    = "9.0.3"
#       + values = [
#           + <<-EOT
#               server:
#                 extraArgs:
#                   - --insecure
#               ...
#             EOT,
#         ]
#     }
#
#   # module.cloudflare.cloudflare_record.records["argocd"] will be created
#   + resource "cloudflare_record" "records" {
#       + id       = (known after apply)
#       + name     = "argocd"
#       + type     = "A"
#       + content  = "10.0.4.205"
#       + proxied  = false
#       + zone_id  = "abc123..."
#     }
#
# Plan: 15 to add, 0 to change, 0 to destroy.
```

### Step 6: Apply Bootstrap

```bash
tofu apply

# Review plan output
# Type 'yes' to confirm

# Output:
# kubernetes_namespace.argocd: Creating...
# kubernetes_namespace.argocd: Creation complete after 1s
# helm_release.argocd: Creating...
# helm_release.argocd: Still creating... [10s elapsed]
# helm_release.argocd: Still creating... [20s elapsed]
# helm_release.argocd: Creation complete after 25s
# module.cloudflare.cloudflare_record.records["argocd"]: Creating...
# module.cloudflare.cloudflare_record.records["argocd"]: Creation complete after 2s
#
# Apply complete! Resources: 15 added, 0 changed, 0 destroyed.
```

### Step 7: Verify ArgoCD Deployment

```bash
# Check ArgoCD pods
kubectl get pods -n argocd

# Expected output:
# NAME                                  READY   STATUS    RESTARTS   AGE
# argocd-server-xyz                     1/1     Running   0          2m
# argocd-repo-server-abc                1/1     Running   0          2m
# argocd-applicationset-controller-def  1/1     Running   0          2m
# argocd-redis-ghi                      1/1     Running   0          2m
# argocd-application-controller-jkl     1/1     Running   0          2m

# Get initial admin password
kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 -d
# Output: randomly-generated-password

# Port-forward to access UI
kubectl port-forward svc/argocd-server -n argocd 8080:443

# Open browser: https://localhost:8080
# Username: admin
# Password: <from secret above>
```

### Step 8: Verify Bootstrap ApplicationSets

```bash
# Check ApplicationSets created
kubectl get applicationset -n argocd

# Expected output:
# NAME                         AGE
# application-sets-control-plane   2m
# application-sets-vcluster         2m

# Check generated Applications
kubectl get application -n argocd

# Expected output (will grow as addons sync):
# NAME                  SYNC STATUS   HEALTH STATUS
# argocd                Synced        Healthy
# cert-manager          Synced        Healthy
# external-secrets      Synced        Progressing
# ...
```

## Key Terraform Resources

### ArgoCD Helm Release

**Source:** [terraform/cluster/main.tf](../terraform/cluster/main.tf) (via gitops-bridge module)

```terraform
module "argocd" {
  source = "git::https://github.com/jamesAtIntegratnIO/terraform-helm-gitops-bridge.git?ref=homelab"

  argocd = {
    chart_version = "9.0.3"
    values = {
      server = {
        extraArgs = ["--insecure"]  # TLS termination at Gateway
        service = {
          type = "ClusterIP"
        }
      }
      configs = {
        params = {
          "server.insecure" = true
        }
      }
    }
  }

  cluster = {
    cluster_name = var.cluster_name
    environment  = "production"
    metadata     = local.addons_metadata
    addons       = local.addons
  }

  apps = {
    addons-control-plane = file("${path.module}/bootstrap/addons-control-plane.yaml")
    addons-vcluster      = file("${path.module}/bootstrap/addons-vcluster.yaml")
  }
}
```

**What This Does:**
1. Installs ArgoCD Helm chart from https://argoproj.github.io/argo-helm
2. Configures insecure mode (TLS at Gateway instead of ArgoCD)
3. Creates ArgoCD Cluster Secret with labels (for ApplicationSet selectors)
4. Deploys bootstrap ApplicationSets from `bootstrap/` directory

### Bootstrap ApplicationSet (Control Plane)

**Source:** [terraform/cluster/bootstrap/addons-control-plane.yaml](../terraform/cluster/bootstrap/addons-control-plane.yaml)

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: application-sets-control-plane
  namespace: argocd
spec:
  goTemplate: true
  goTemplateOptions: ["missingkey=error"]
  
  generators:
    - clusters:
        selector:
          matchLabels:
            cluster_role: control-plane  # Only control-plane clusters

  template:
    metadata:
      name: 'application-sets-{{.name}}'
      labels:
        environment: '{{.metadata.labels.environment}}'
    spec:
      project: default
      source:
        repoURL: https://github.com/jamesatintegratnio/gitops_homelab_2_0
        path: addons/charts/application-sets
        targetRevision: main
        helm:
          valueFiles:
            - ../../environments/production/addons/addons.yaml
      destination:
        server: '{{.server}}'
        namespace: argocd
      syncPolicy:
        automated:
          prune: true
          selfHeal: true
```

**How It Works:**
1. Cluster generator finds all Clusters with `cluster_role: control-plane`
2. For each matching cluster, generates an Application
3. Application deploys `application-sets` Helm chart
4. Helm chart reads `addons.yaml` and creates ApplicationSets for each addon
5. Those ApplicationSets generate Applications for each enabled addon

### Cloudflare DNS Records

**Source:** [terraform/modules/cloudflare/main.tf](../terraform/modules/cloudflare/main.tf)

```terraform
data "cloudflare_zone" "zone" {
  name = var.cloudflare_zone_name
}

resource "cloudflare_record" "records" {
  for_each = var.cloudflare_records
  
  zone_id = data.cloudflare_zone.zone.id
  name    = each.value.name
  content = each.value.content
  type    = each.value.type
  ttl     = each.value.ttl
  proxied = each.value.proxied
}
```

**Usage:**
```terraform
module "cloudflare" {
  source = "../modules/cloudflare"

  cloudflare_zone_name = var.cloudflare_zone_name
  cloudflare_records   = var.cloudflare_records
}
```

**Why Module:**
- Reusable across multiple zones
- Encapsulates Cloudflare API logic
- Makes adding/removing DNS records simple (edit tfvars only)

### External Secrets Operator Bootstrap

**Source:** [terraform/cluster/external_secrets_operator.tf](../terraform/cluster/external_secrets_operator.tf)

```terraform
resource "kubernetes_namespace" "external_secrets" {
  metadata {
    name = "external-secrets"
  }
}

resource "kubernetes_secret" "eso_onepassword_token" {
  metadata {
    name      = "eso-onepassword-token"
    namespace = kubernetes_namespace.external_secrets.metadata.0.name
  }

  data = {
    token = var.onepassword_token
  }

  depends_on = [kubernetes_namespace.external_secrets]
}
```

**Why Terraform:**
- Chicken-and-egg: ESO needs token to start, but we use ESO for secrets
- Initial token must come from outside GitOps
- After ESO running, use ExternalSecrets for everything else

### GHCR Pull Secret

**Source:** [terraform/cluster/main.tf](../terraform/cluster/main.tf)

```terraform
resource "kubernetes_secret" "docker-config" {
  metadata {
    name      = "ghcr-login-secret"
    namespace = "argocd"
  }

  data = {
    ".dockerconfigjson" = file("${path.module}/dockerconfig.json")
  }

  type = "kubernetes.io/dockerconfigjson"

  depends_on = [module.argocd]
}
```

**Why Needed:**
- Kratix promise pipeline images stored in GHCR (private repo)
- ArgoCD needs credentials to pull images
- Alternative: Use ImagePullSecrets per namespace (more complex)

## Operational Workflows

### Adding a New DNS Record

**Edit terraform.tfvars:**
```hcl
cloudflare_records = {
  # ... existing records ...
  
  "new-service" = {
    name    = "new-service"
    content = "10.0.4.205"
    type    = "A"
    ttl     = 300
    proxied = false
  }
}
```

**Apply:**
```bash
cd terraform/cluster/
tofu plan -target=module.cloudflare
tofu apply -target=module.cloudflare
```

**Verify:**
```bash
dig new-service.integratn.tech +short
# 10.0.4.205
```

### Updating ArgoCD Version

**Edit main.tf:**
```terraform
module "argocd" {
  # ...
  argocd = {
    chart_version = "9.1.0"  # Changed from 9.0.3
  }
}
```

**Apply with caution:**
```bash
tofu plan
# Review changes carefully - this will upgrade ArgoCD

tofu apply

# Watch upgrade progress
kubectl get pods -n argocd -w

# Verify ArgoCD still functional
kubectl get application -n argocd
```

### Changing Backend Location

**Scenario:** Moving from MinIO to AWS S3

**Steps:**
1. **Update backend.hcl:**
   ```hcl
   bucket = "my-terraform-state"
   key    = "homelab/the-cluster/terraform.tfstate"
   region = "us-east-1"
   # Remove MinIO-specific settings
   ```

2. **Reinitialize with migration:**
   ```bash
   tofu init -backend-config=backend.hcl -migrate-state
   
   # Prompt:
   # Do you want to copy existing state to the new backend?
   #   Enter a value: yes
   
   # State migrated to new backend
   ```

3. **Verify state accessible:**
   ```bash
   tofu plan
   # Should show: No changes. Your infrastructure matches the configuration.
   ```

### Recovering from Lost State

**⚠️ DANGEROUS - Avoid if possible**

**Scenario:** State file deleted/corrupted, resources still exist in cluster.

**Option 1: Import Resources (Recommended)**
```bash
# Recreate state by importing existing resources
tofu import kubernetes_namespace.argocd argocd
tofu import helm_release.argocd argocd/argocd
tofu import module.cloudflare.cloudflare_record.records["argocd"] <zone-id>/<record-id>

# Continue importing all resources...

# Verify state matches reality
tofu plan
# Should show: No changes
```

**Option 2: Destroy and Recreate (DESTRUCTIVE)**
```bash
# ⚠️ This will DELETE and RECREATE all resources
# All ArgoCD data (applications, secrets) will be lost

tofu apply
# Terraform will create new resources (because it thinks none exist)

# Result: Duplicate resources or conflicts
```

**Best Practice:** Always keep state backups and use remote backend with versioning.

## Troubleshooting

### "Error: Kubernetes cluster unreachable"

**Symptom:**
```
Error: Get "https://10.0.4.100:6443/api/v1/namespaces/argocd": dial tcp 10.0.4.100:6443: i/o timeout
```

**Diagnosis:**
```bash
# Verify kubeconfig is valid
kubectl get nodes

# Check kubeconfig path
echo $KUBECONFIG
# /home/user/.kube/config-the-cluster

# Verify Terraform using correct context
grep current-context ~/.kube/config-the-cluster
```

**Fix:**
```bash
# Update kubeconfig in provider config (main.tf)
provider "kubernetes" {
  config_path = "~/.kube/config-the-cluster"
}

# Or use explicit cluster connection
provider "kubernetes" {
  host                   = "https://10.0.4.100:6443"
  cluster_ca_certificate = file("~/.kube/ca.crt")
  client_certificate     = file("~/.kube/client.crt")
  client_key             = file("~/.kube/client.key")
}
```

### "Error: Backend initialization required"

**Symptom:**
```
Error: Backend configuration changed

A change in the backend configuration has been detected, which may require
migrating existing state.
```

**Diagnosis:**
```bash
# Check if backend.hcl changed
git diff backend.hcl

# Check current backend config
cat .terraform/terraform.tfstate | jq '.backend'
```

**Fix:**
```bash
# Reinitialize with new backend config
tofu init -backend-config=backend.hcl -reconfigure

# If migrating state from old backend
tofu init -backend-config=backend.hcl -migrate-state
```

### "Error: Helm release already exists"

**Symptom:**
```
Error: cannot re-use a name that is still in use
```

**Diagnosis:**
```bash
# Check if Helm release exists
helm list -n argocd

# NAME    NAMESPACE  REVISION  UPDATED                                STATUS    CHART         APP VERSION
# argocd  argocd     1         2024-01-23 10:30:00.123456789 -0500    deployed  argo-cd-9.0.3 v2.13.0
```

**Fix Option 1: Import Existing Release**
```bash
# Import into Terraform state
tofu import module.argocd.helm_release.argocd argocd/argocd

# Verify import successful
tofu plan
# Should show: No changes
```

**Fix Option 2: Delete and Recreate**
```bash
# ⚠️ DESTRUCTIVE - ArgoCD data will be lost
helm uninstall argocd -n argocd

# Re-run Terraform
tofu apply
```

### "Error: Secret already exists"

**Symptom:**
```
Error: secrets "eso-onepassword-token" already exists
```

**Diagnosis:**
```bash
kubectl get secret -n external-secrets eso-onepassword-token
```

**Fix:**
```bash
# Import secret into state
tofu import kubernetes_secret.eso_onepassword_token external-secrets/eso-onepassword-token

# Verify
tofu plan
```

### Cloudflare API Rate Limiting

**Symptom:**
```
Error: error creating DNS record: HTTP status 429: Rate limit reached
```

**Diagnosis:**
```bash
# Check Cloudflare API rate limit headers
curl -H "Authorization: Bearer $CLOUDFLARE_API_TOKEN" \
  https://api.cloudflare.com/client/v4/zones | jq -r '.headers'
```

**Fix:**
```bash
# Add retry logic to provider (main.tf)
provider "cloudflare" {
  api_token = var.cloudflare_api_key
  retries   = 3
}

# Or batch DNS record creation (use count/for_each)
```

## Best Practices

### State Management
- ✅ Always use remote backend (S3, Consul, etc.)
- ✅ Enable state locking (prevent concurrent modifications)
- ✅ Enable state versioning (rollback capability)
- ✅ Never commit terraform.tfstate to Git
- ❌ Don't share state files manually (use remote backend)

### Secret Management
- ✅ Use environment variables for sensitive values:
  ```bash
  export TF_VAR_cloudflare_api_key="xxx"
  export TF_VAR_onepassword_token="xxx"
  ```
- ✅ Never commit terraform.tfvars with secrets
- ✅ Use .gitignore for sensitive files
- ❌ Don't hardcode secrets in .tf files

### Module Usage
- ✅ Create modules for reusable infrastructure patterns
- ✅ Version modules with Git tags
- ✅ Document module inputs/outputs
- ❌ Don't create overly complex modules (keep simple)

### Resource Tagging
- ✅ Add labels/tags to all resources:
  ```terraform
  resource "kubernetes_namespace" "app" {
    metadata {
      name = "my-app"
      labels = {
        managed-by = "terraform"
        environment = "production"
        team = "platform"
      }
    }
  }
  ```

### Plan Before Apply
- ✅ Always run `tofu plan` before `apply`
- ✅ Review planned changes carefully
- ✅ Use `-target` for surgical changes when needed
- ❌ Never blindly run `tofu apply -auto-approve` in production

## Key Files Reference

- **Main configuration**: [terraform/cluster/main.tf](../terraform/cluster/main.tf)
- **Variables definition**: [terraform/cluster/variables.tf](../terraform/cluster/variables.tf)
- **Provider versions**: [terraform/cluster/versions.tf](../terraform/cluster/versions.tf)
- **Bootstrap ApplicationSets**: [terraform/cluster/bootstrap/](../terraform/cluster/bootstrap/)
- **Cloudflare module**: [terraform/modules/cloudflare/](../terraform/modules/cloudflare/)
- **ESO bootstrap**: [terraform/cluster/external_secrets_operator.tf](../terraform/cluster/external_secrets_operator.tf)
- **Change log**: [TERRAFORM_CHANGES.md](../TERRAFORM_CHANGES.md)
