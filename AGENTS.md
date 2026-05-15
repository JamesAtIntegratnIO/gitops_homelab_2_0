# AGENTS.md - GitOps Homelab 2.0

## Project Overview

This is a **self-service internal developer platform (IDP)** built on Kubernetes, using a GitOps workflow with ArgoCD and Kratix (Syntasso) for platform engineering. The platform runs on **Talos Linux** with **Kubernetes 1.34.1**, managed entirely through git.

**Repository**: https://github.com/jamesatintegratnio/gitops_homelab_2_0
**Domain**: integratn.tech
**Cluster**: 3 control-plane nodes (10.0.4.101-103), MetalLB L2 (10.0.4.200-253)

## Core Architecture

```
GitHub Repo (gitops_homelab_2_0)
  |
  +-- Terraform/Tofu --> Creates ArgoCD bootstrap apps + Cloudflare DNS
  |
  +-- ArgoCD ApplicationSets --> Deploys addons to clusters
  |     |
  |     +-- addons/environments/{production,staging,development}/addons/
  |     +-- addons/cluster-roles/{control-plane,vcluster}/addons/
  |     +-- addons/clusters/{cluster-name}/addons/
  |     +-- addons/charts/application-sets/ (Helm chart rendering ApplicationSets)
  |
  +-- Kratix Promises --> Self-service platform capabilities
  |     |
  |     +-- promises/vcluster-orchestrator-v2/ (create virtual clusters)
  |     +-- promises/http-service/ (deploy HTTP applications)
  |     +-- promises/argocd-application/ (create ArgoCD apps)
  |     +-- promises/argocd-project/ (create ArgoCD projects)
  |     +-- promises/argocd-cluster-registration/ (register clusters in ArgoCD)
  |     +-- promises/gateway-route/ (create Gateway API routes)
  |     +-- promises/external-secret/ (create 1Password-backed secrets)
  |     +-- promises/_shared/kratixutil/ (shared Go library for pipelines)
  |
  +-- hctl CLI --> Day-to-day operations tool
  |
  +-- platform/ --> Platform resource requests (vClusters, HTTP services)
  +-- workloads/ --> Workload definitions per vCluster
```

## Directory Structure

```
.
├── addons/                          # ArgoCD addon definitions
│   ├── charts/application-sets/     # Helm chart rendering ApplicationSets
│   ├── environments/                # Per-environment addon configs
│   │   ├── production/addons/
│   │   ├── staging/addons/
│   │   └── development/addons/
│   ├── cluster-roles/               # Per-cluster-role addon configs
│   │   ├── control-plane/addons/
│   │   └── vcluster/addons/
│   └── clusters/                    # Per-cluster addon overrides
├── cli/                             # hctl CLI (Go, Cobra + Bubbletea TUI)
│   ├── cmd/                         # Command definitions
│   ├── internal/
│   │   ├── config/                  # Config management
│   │   ├── tui/                     # Terminal UI (charm.sh/bubbletea)
│   │   ├── platform/                # Platform operations (vClusters, diagnostics)
│   │   ├── deploy/                  # Score workload deployment
│   │   ├── kube/                    # Kubernetes client helpers
│   │   └── score/                   # Score manifest types
│   └── go.mod                       # Go 1.25.0
├── docs/                            # Documentation
│   ├── architecture.md              # High-level architecture
│   ├── addons.md                    # Addon system guide
│   ├── promises.md                  # Kratix Promise development guide
│   ├── vclusters.md                 # Virtual cluster guide
│   ├── terraform.md                 # Infrastructure provisioning
│   └── observability.md             # Monitoring/observability setup
├── promises/                        # Kratix Promises (platform capabilities)
│   ├── _shared/kratixutil/          # Shared Go library (types, helpers, writers)
│   ├── vcluster-orchestrator-v2/    # Main: create/manage virtual clusters
│   ├── http-service/                # Deploy HTTP applications via Score
│   ├── argocd-application/          # Create ArgoCD Applications
│   ├── argocd-project/              # Create ArgoCD Projects
│   ├── argocd-cluster-registration/ # Register clusters in ArgoCD
│   ├── gateway-route/               # Gateway API HTTPRoute creation
│   └── external-secret/             # 1Password ExternalSecret creation
├── platform/                        # Platform resource requests
│   ├── vclusters/                   # VClusterOrchestratorV2 resources
│   └── http-services/               # HTTPService resources
├── terraform/                       # Infrastructure as Code
│   ├── cluster/                     # Main cluster provisioning
│   │   ├── main.tf                  # ArgoCD bootstrap + Cloudflare
│   │   ├── variables.tf             # Input variables
│   │   ├── bootstrap/               # ArgoCD bootstrap Application manifests
│   │   └── external_secrets_operator.tf
│   └── modules/
│       └── cloudflare/              # Cloudflare DNS module
├── workloads/                       # Per-vCluster workload definitions
│   └── vcluster-media/              # Media stack (Radarr, Sonarr, etc.)
├── matchbox/                        # Talos machine configs (Proxmox provisioning)
│   └── assets/talos/1.11.5/
├── .github/
│   ├── copilot-instructions.md      # AI assistant guidelines
│   └── workflows/                   # CI: promise validation, image builds
└── .gitignore
```

## Key Technologies

| Component | Technology | Purpose |
|-----------|-----------|---------|
| OS | Talos Linux 1.11.5 | Immutable Kubernetes OS |
| K8s | Kubernetes 1.34.1 | Container orchestration |
| GitOps | ArgoCD 9.4.3 | Declarative continuous delivery |
| Platform | Kratix (Syntasso) | Self-service internal developer platform |
| Infra | Terraform/OpenTofu | Infrastructure provisioning |
| DNS | Cloudflare + ExternalDNS | Dynamic DNS management |
| Secrets | External Secrets Operator + 1Password | Secret management |
| Certs | cert-manager + Let's Encrypt | TLS certificate automation |
| Networking | NGINX Gateway Fabric + MetalLB | Ingress and LoadBalancer IPs |
| Storage | NFS Subdir External Provisioner | Dynamic PVC provisioning from NFS |
| Monitoring | kube-prometheus-stack | Prometheus + Grafana + Alertmanager |
| CLI | hctl (Go/Cobra/Bubbletea) | Day-to-day operations |
| Virtual Clusters | vcluster (Loft) | Multi-tenant isolation |
| CI | GitHub Actions | Promise validation, image builds |

## Kratix Promises - How They Work

Promises are Kratix's concept for self-service platform capabilities. Each promise has:
1. **promise.yaml** - CRD definition + workflow pipeline configuration
2. **workflows/resource/configure/** - Go pipeline that processes ResourceRequests
3. **Pipeline execution** - Runs as a container in Kratix work queue, reads ResourceRequest, outputs YAML to git state repo

### Shared Library: `promises/_shared/kratixutil/`
All promises import this shared Go module. Key files:
- `types.go` - Kubernetes resource types, ArgoCD types, secret types
- `builders.go` - Resource construction helpers
- `writers.go` - YAML output writers for Kratix pipeline output
- `helpers.go` - Value extraction, deep merge utilities

### Pipeline Pattern (Go)
```
main.go:
  1. Initialize Kratix SDK: sdk := kratix.New()
  2. Read resource input: sdk.ReadResourceInput()
  3. Build config from resource spec: buildConfig(sdk, resource)
  4. Branch on workflow action:
     - "configure": create resources, write YAML outputs
     - "delete": write delete manifests to cleanup resources
  5. Write outputs to Kratix state: sdk.WriteOutput(path, resource)
```

### Secret Management Rule (NON-NEGOTIABLE)
- **NEVER** generate `kind: Secret` in promise pipelines
- **ALWAYS** use `ExternalSecret` resources referencing 1Password via `ClusterSecretStore`
- The kratix-platform-state repo is PUBLIC - secrets in git = breach
- CI validates: blocks any `kind: Secret` in promise directories

## hctl CLI - Command Reference

Binary name: `hctl` (Homelab Control)

| Command | Description |
|---------|-------------|
| `hctl status` | Platform health dashboard (nodes, apps, promises, vClusters) |
| `hctl status -w` | Watch mode - continuously refresh status |
| `hctl doctor` | Check prerequisites and environment |
| `hctl diagnose <resource>` | Automated troubleshooting of resource lifecycle |
| `hctl reconcile <resource>` | Force re-reconciliation via annotation |
| `hctl trace <resource>` | Show delivery chain for a resource |
| `hctl alerts` | Show active platform alerts |
| `hctl context` | Show current platform context |
| `hctl init` | Initialize config at `~/.config/hctl/config.yaml` |
| `hctl version` | Print version |

### Sub-command groups
| Command | Description |
|---------|-------------|
| `hctl vcluster <create\|list\|delete\|status\|sync\|apps\|kubeconfig>` | Virtual cluster management |
| `hctl deploy <init\|run>` | Score-based workload deployment |
| `hctl addon <list\|enable\|disable>` | Addon management |
| `hctl scale <up\|down>` | Workload scaling |
| `hctl secret <get\|set>` | Secret operations |
| `hctl ai <...>` | AI-assisted operations |

### Convenience shortcuts
| Command | Description |
|---------|-------------|
| `hctl up <app>` | Scale up application |
| `hctl down <app>` | Scale down application |
| `hctl logs <app> -f` | Stream application logs |
| `hctl open <app>` | Open application in browser |

### Config file: `~/.config/hctl/config.yaml`
```yaml
repoPath: /path/to/gitops_homelab_2_0
gitMode: prompt          # auto | prompt | generate | stage-only
argocdURL: https://argocd.cluster.integratn.tech
interactive: true
outputFormat: text       # text | json | yaml
verbose: false
quiet: false
platform:
  domain: cluster.integratn.tech
  clusterSubnet: 10.0.4.0/24
  metalLBPool: 10.0.4.200-253
  platformNamespace: platform-requests
```

## Addon System

Addons are defined as YAML entries in environment-specific files. Each addon with `enabled: true` renders an ArgoCD ApplicationSet.

### Addon YAML Schema
```yaml
addon-name:
  enabled: true                          # Required: toggle
  namespace: target-namespace             # Required: destination namespace
  chartName: helm-chart-name             # Helm chart name
  chartRepository: https://repo.url      # Helm repo URL
  defaultVersion: "1.0.0"                # Chart version
  project: platform-services             # ArgoCD project
  selector:                              # Cluster selector (matchExpressions)
    matchExpressions:
      - key: enable_addon_name
        operator: In
        values: ['true']
  valuesObject:                          # Helm values injected directly
    key: value
  additionalResources:                   # Extra manifest files
    type: manifest
    path: addons/environments/.../addon-name
  syncPolicy:                            # Override default sync policy
  ignoreDifferences:                     # Ignore ArgoCD diff on specific fields
```

### Value Files Resolution Order
1. `environments/{environment}/addons/`
2. `cluster-roles/{cluster_role}/addons/`
3. `clusters/{cluster_name}/addons/`

### ApplicationSet Template
Located at `addons/charts/application-sets/`. Uses goTemplate with cluster generators. Supports:
- Helm chart sources with valueFiles
- Manifest sources (non-Helm git paths)
- Per-environment overrides via merge generators
- Generator values injection

## Terraform / OpenTofu

Working directory: `terraform/cluster/`

### What it provisions
1. **ArgoCD bootstrap** - Creates initial ArgoCD Applications pointing to addon paths
2. **Cloudflare DNS** - Creates DNS records from variable definitions
3. **GHCR login secret** - Docker registry credentials in ArgoCD namespace

### Key variables (variables.tf)
- `cluster_name` - Cluster identifier
- `gitops_addons_*` - Repo URL, basepath, path, revision for addons
- `onepassword_token` - 1Password Connect token (sensitive)
- `cloudflare_api_key` - Cloudflare API key (sensitive)
- `cloudflare_zone_name` - DNS zone
- `cloudflare_records` - Map of DNS records to create

### Bootstrap Applications
- `addons-control-plane.yaml` - Points to `bootstrap/control-plane/addons` path
- `addons-vcluster.yaml` - Points to `bootstrap/vcluster/addons` path

## Virtual Clusters (vClusters)

Managed via `VClusterOrchestratorV2` Kratix promise. Creates isolated virtual Kubernetes clusters on top of the physical cluster.

### Presets
- **dev**: 1 replica, 768Mi-1536Mi memory, SQLite backing store, no persistence
- **prod**: 3 replicas, 2Gi memory, etcd backing store, 10Gi persistence

### Key features
- Automatic ArgoCD cluster registration
- MetalLB LoadBalancer for API endpoint
- ExternalDNS for hostname -> VIP mapping
- cert-manager ClusterIssuer sync into vCluster
- External Secrets ClusterStore sync into vCluster
- Network policies (NFS egress, custom egress rules)
- Workload ApplicationSet pointing to `workloads/{vcluster-name}/`

### Creating a vCluster
```yaml
apiVersion: platform.integratn.tech/v1alpha1
kind: VClusterOrchestratorV2
metadata:
  name: my-vcluster
  namespace: platform-requests
spec:
  name: my-vcluster
  vcluster:
    preset: dev
  exposure:
    hostname: my-vcluster.integratn.tech
    subnet: 10.0.4.0/24
```

## HTTP Services

Managed via `HTTPService` Kratix promise. Deploys containerized HTTP applications with networking, secrets, and monitoring.

### Key features
- Automatic Deployment + Service + HTTPRoute creation
- 1Password-backed ExternalSecrets for credentials
- Prometheus ServiceMonitor for metrics scraping
- Persistent volume support
- Health check probes
- Security context hardening

### Creating an HTTP Service
```yaml
apiVersion: platform.integratn.tech/v1alpha1
kind: HTTPService
metadata:
  name: my-app
  namespace: platform-requests
spec:
  name: my-app
  image:
    repository: docker.io/myimage
    tag: "latest"
  port: 8080
  ingress:
    enabled: true
    hostname: my-app.cluster.integratn.tech
```

## Workflows and CI

### GitHub Actions
- **validate-promises.yaml**: Scans promise YAML for forbidden `kind: Secret` resources + validates YAML syntax
- **build-*-image.yaml**: Builds pipeline container images for promises (pushed to GHCR)

### Image Registry
All pipeline images are hosted on `ghcr.io/jamesatintegratnio/`

## Important Conventions

1. **Git is source of truth** - All changes go through git, ArgoCD syncs
2. **No secrets in git** - Use ExternalSecret + 1Password exclusively
3. **Atomic commits** - One logical change per commit
4. **Read before edit** - Always understand current state before modifying
5. **Test incrementally** - Validate changes step by step
6. **Preset defaults** - Use dev/prod presets for vClusters unless specific overrides needed
7. **Namespace targeting** - Always specify namespace explicitly, never rely on `default`
8. **Resource limits** - Always set requests/limits for workloads
9. **Labels** - Use consistent labeling: `kratix.io/promise-name`, `app.kubernetes.io/managed-by: kratix`

## Known IPs and Endpoints

| Resource | Address |
|----------|---------|
| Control plane nodes | 10.0.4.101-103 |
| MetalLB pool | 10.0.4.200-253 |
| Gateway LoadBalancer | 10.0.4.205 |
| NFS server | 10.0.0.12 |
| NFS storage path | /mnt/user/kube_storage |
| ArgoCD URL | https://argocd.cluster.integratn.tech |
| 1Password Connect | https://connect.integratn.tech |
| Base domain | integratn.tech |
| Cluster domain | cluster.integratn.tech |

## Cluster Labels Used

| Label | Values | Purpose |
|-------|--------|---------|
| `cluster_role` | control-plane, vcluster | Cluster type routing |
| `environment` | production, staging, development | Environment targeting |
| `enable_*` | "true"/"false" | Addon feature flags |
| `capability.vcluster` | "true" | Kratix destination selector for vCluster-capable clusters |
| `integratn.tech/cluster-issuer` | letsencrypt-prod | cert-manager issuer selector |
| `integratn.tech/cluster-secret-store` | onepassword-store | External Secrets store selector |

## Building and Developing

### CLI (hctl)
```bash
cd cli/
go build -o hctl .
# or with version info:
go build -ldflags="-X github.com/jamesatintegratnio/hctl/cmd.Version=1.0.0 \
  -X github.com/jamesatintegratnio/hctl/cmd.Commit=abc123" -o hctl .
```

### Promise Pipelines
```bash
cd promises/vcluster-orchestrator-v2/workflows/resource/configure/
docker build -t ghcr.io/jamesatintegratnio/vcluster-orchestrator-v2-configure:latest .
docker push ghcr.io/jamesatintegratnio/vcluster-orchestrator-v2-configure:latest
```

### Testing Promises
1. Create a sample ResourceRequest YAML
2. Apply to the platform namespace
3. Check Kratix work queue for pipeline execution
4. Verify outputs in kratix-platform-state repo

## Troubleshooting Checklist

1. **ArgoCD sync issues**: Check ArgoCD app status, events, and sync logs
2. **Promise not fulfilling**: Check Kratix work queue, pipeline pod logs
3. **vCluster not starting**: Check namespace pods, MetalLB assignment, DNS records
4. **Secret not available**: Verify ExternalSecret status, 1Password Connect connectivity
5. **DNS not resolving**: Check ExternalDNS controller logs, Cloudflare records
6. **Certificate issues**: Check cert-manager Certificate and Order resources
7. **hctl diagnose**: Run automated troubleshooting for any resource
