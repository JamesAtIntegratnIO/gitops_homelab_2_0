# GitOps Homelab 2.0

A production-grade GitOps platform running on bare-metal, built for learning and demonstrating enterprise Kubernetes patterns at homelab scale.

**Stack**: Talos Linux · Kubernetes 1.34 · ArgoCD · Kratix · vcluster · ExternalSecrets · 1Password

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Git (this repo)                          │
│  addons/ · platform/ · promises/ · workloads/ · terraform/ │
└──────┬──────────────────────────────┬───────────────────────┘
       │                              │
       ▼                              ▼
┌──────────────┐            ┌──────────────────┐
│   Terraform  │            │     ArgoCD       │
│  Bootstrap   │───────────▶│  ApplicationSets │
└──────────────┘            └────────┬─────────┘
                                     │
              ┌──────────────────────┼──────────────────────┐
              ▼                      ▼                      ▼
     ┌────────────────┐   ┌──────────────────┐   ┌────────────────┐
     │  Host Cluster   │   │  Kratix Promises │   │   vclusters    │
     │  (control-plane)│   │  & Pipelines     │   │  (workloads)   │
     │                 │   │                  │   │                │
     │ cert-manager    │   │ VClusterOrch v2  │   │ nginx-gateway  │
     │ external-secrets│   │ (Go SDK)         │   │ cert-manager   │
     │ nginx-gateway   │   │                  │   │ external-dns   │
     │ metallb         │   │  ┌────────────┐  │   │ argocd         │
     │ kratix          │   │  │ Pipelines  │  │   │                │
     │ prometheus      │   │  │ render to  │──┼──▶│ sonarr, radarr │
     │ loki            │   │  │ state repo │  │   │ sabnzbd, wiki  │
     └────────────────┘   │  └────────────┘  │   └────────────────┘
                           └──────────────────┘
```

Three control-plane nodes (Talos Linux, PXE-booted via Matchbox) run the host cluster. ArgoCD manages everything declaratively. Kratix promises provide platform APIs — a `VClusterOrchestratorV2` request provisions an entire tenant cluster with its own ArgoCD, networking, TLS, DNS, and observability.

Secrets never live in Git. All credentials flow through 1Password → ExternalSecrets.

For the full architecture deep-dive, see [docs/architecture.md](docs/architecture.md).

## Repository Structure

```
.
├── addons/                        # ArgoCD addon definitions (the "what to deploy" layer)
│   ├── charts/application-sets/   #   Helm chart that renders one ApplicationSet per addon
│   ├── cluster-roles/             #   Addons by role: control-plane, vcluster
│   ├── clusters/                  #   Per-cluster overrides (the-cluster, vcluster-media)
│   └── environments/              #   Per-environment config (production, staging, development)
│
├── platform/                      # Kratix ResourceRequests (the "what to provision" layer)
│   └── vclusters/                 #   vcluster provisioning requests
│
├── promises/                      # Kratix Promise definitions (the "how to provision" layer)
│   ├── vcluster-orchestrator-v2/  #   Active: Go SDK pipeline for full vcluster lifecycle
│   └── _archived/                 #   Superseded v1 bash promises (kept for reference)
│
├── workloads/                     # Application definitions deployed inside vclusters
│   └── vcluster-media/            #   Media stack: sonarr, radarr, sabnzbd, otterwiki
│
├── terraform/                     # Infrastructure as Code
│   ├── cluster/                   #   ArgoCD bootstrap, ExternalSecrets operator, Cloudflare
│   └── modules/cloudflare/        #   DNS zone management
│
├── matchbox/                      # PXE/iPXE bare-metal provisioning for Talos Linux
│   ├── groups/                    #   MAC-address to profile mappings
│   ├── profiles/                  #   Boot profiles (kernel + initramfs + machine config)
│   └── talos-machineconfigs/      #   Talos machine configuration patches
│
├── docs/                          # Architecture, operations, troubleshooting guides
├── hack/                          # Development and testing utilities
├── images/                        # Container image sources (kubectl)
├── scripts/                       # Git hooks setup
└── flake.nix                      # Nix dev environment (kubectl, tofu, helm, talosctl, k9s)
```

## GitOps Layers

The platform uses three declarative layers, each driven by Git:

| Layer | Directory | Engine | Purpose |
|-------|-----------|--------|---------|
| **Addons** | `addons/` | ArgoCD ApplicationSets | Cluster services (cert-manager, monitoring, networking) |
| **Platform** | `platform/` | Kratix Promises | Infrastructure provisioning (vclusters, future: databases) |
| **Workloads** | `workloads/` | ArgoCD (inside vcluster) | Application deployments (media stack) |

Addon value files are layered with precedence: `environments/` → `cluster-roles/` → `clusters/`. See [addons/README.md](addons/README.md) for details.

## Quick Start

```bash
# 1. Enter the development environment (provides all CLI tools)
nix develop

# 2. Bootstrap the cluster (after Talos nodes are running)
cd terraform/cluster
tofu init && tofu apply

# 3. Access ArgoCD
argocd login argocd.cluster.integratn.tech
```

## Key Workflows

### Provision a vcluster
```bash
# Create a resource request
cp platform/vclusters/vcluster-media.yaml platform/vclusters/vcluster-new.yaml
# Edit the spec, commit, push — Kratix handles the rest
```

### Add a workload to a vcluster
```bash
# Add app definition under workloads/<cluster-name>/addons/
# The vcluster's ArgoCD picks it up automatically
```

### Update a promise pipeline
```bash
# Modify Go code in promises/vcluster-orchestrator-v2/workflows/
# Push — GitHub Actions builds and publishes the new image
# Refresh the kratix-promises ArgoCD app to pick up the change
```

See [docs/operations.md](docs/operations.md) for full runbooks.

## Infrastructure

| Component | Details |
|-----------|---------|
| **Nodes** | 3× control-plane (Talos 1.11.3), PXE-booted |
| **Network** | 10.0.4.0/24 cluster, MetalLB L2 (10.0.4.200-253) |
| **Ingress** | nginx-gateway-fabric, Gateway API |
| **TLS** | cert-manager, Let's Encrypt (Cloudflare DNS-01) |
| **DNS** | external-dns → Cloudflare |
| **Secrets** | 1Password Connect → ExternalSecrets operator |
| **Monitoring** | kube-prometheus-stack, Loki, Promtail |
| **Storage** | NFS (Unraid), config-nfs-client / data-nfs-client |

## Documentation

| Guide | Description |
|-------|-------------|
| [Architecture](docs/architecture.md) | Full architecture, ADRs, data flows, security model |
| [Bootstrap](docs/bootstrap.md) | PXE boot, Talos setup, initial cluster creation |
| [Addons](docs/addons.md) | ApplicationSet mechanics, value file precedence |
| [Promises](docs/promises.md) | Kratix promise development and pipeline design |
| [vclusters](docs/vclusters.md) | vcluster lifecycle, networking, storage |
| [Observability](docs/observability.md) | Metrics, logs, dashboards |
| [Operations](docs/operations.md) | Runbooks, troubleshooting, common tasks |
| [Terraform](docs/terraform.md) | IaC workflow for cluster bootstrap |

## CI/CD

| Workflow | Trigger | Purpose |
|----------|---------|---------|
| [Build Go SDK Promises](.github/workflows/build-go-sdk-promises.yaml) | `promises/*/workflows/**` | Build and publish Go promise pipeline images |
| [Build kubectl Image](.github/workflows/build-kubectl-image.yaml) | `images/kubectl/**` | Multi-arch kubectl container image |
| [Validate Promises](.github/workflows/validate-promises.yaml) | `promises/**/*.yaml` | Block `kind: Secret` in promise output |
