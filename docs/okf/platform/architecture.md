---
type: Architecture Overview
title: Platform architecture
description: The big picture — Talos bare-metal cluster, ArgoCD ApplicationSets, Kratix promises, vclusters, and the design decisions behind them.
tags: [architecture, gitops, talos, argocd, kratix, vcluster]
status: stable
generated: { by: claude-code/claude-fable-5, at: 2026-08-20T23:40:00Z }
stale_after: 2027-02-20
sources:
  - id: arch-doc
    resource: ../../architecture.md
    title: docs/architecture.md (ADRs, data flows, security model)
  - id: readme
    resource: ../../../README.md
    title: Repository README
  - id: live-cluster
    resource: kubectl sweep of the-cluster (admin@the-cluster), 2026-08-20
    title: Live cluster observation
---

# What this platform is

A **self-service internal developer platform** at homelab scale:
three bare-metal [Talos Linux nodes](/infrastructure/talos-nodes.md) run
Kubernetes 1.34.1 as [the-cluster](/cluster/the-cluster.md); **ArgoCD**
continuously reconciles everything from this git repo; **Kratix** provides
platform APIs (request a vcluster or an HTTP service as a CR, get a fully
integrated deployment); **vcluster** gives tenants isolated virtual clusters.
Domain: `integratn.tech` (platform services under `*.cluster.integratn.tech`).

```
git (this repo) ──tofu apply once──▶ ArgoCD ──ApplicationSets──▶ addons on the-cluster & vclusters
      │                                ▲
      └─ platform/*.yaml CRs ──▶ Kratix pipelines ──▶ kratix-platform-state (public repo) ──┘
workloads/<vcluster>/ ──▶ per-vcluster ArgoCD ──▶ apps inside the vcluster
```

# The three declarative layers

| Layer | Directory | Engine | Examples |
|---|---|---|---|
| [Addons](/addons/addon-system.md) | `addons/` | ArgoCD ApplicationSets | cert-manager, Cilium, monitoring, Kratix itself |
| [Platform](/promises/kratix.md) | `platform/` | Kratix promises | `VClusterOrchestratorV2`, `HTTPService` |
| Workloads | `workloads/<vcluster>/` | ArgoCD **inside** each vcluster | media stack (sonarr/radarr/sabnzbd/otterwiki) |

Plus the one imperative step:
[Terraform/OpenTofu bootstrap](/infrastructure/terraform-bootstrap.md)
(ArgoCD install + cluster secret + bootstrap ApplicationSets + Cloudflare DNS).

# Key architecture decisions (from the ADRs)

- **Kratix over Crossplane/Backstage** — promise pipelines are plain
  containers writing YAML to a git state repo; GitOps-native, no provider CRD
  sprawl.[^arch-doc]
- **Gateway API over Ingress** (nginx-gateway-fabric) — typed routing,
  explicit TLS refs, role separation. See [networking](/platform/networking.md).
- **Talos over general-purpose Linux** — immutable, API-only, no SSH.
- **1Password + ExternalSecrets over SealedSecrets/SOPS** — zero secrets in
  git, even encrypted. See [secret management](/platform/secret-management.md).
- **Cilium as CNI** (kube-proxy replacement, Hubble observability) — a
  post-README addition; the top-level README/AGENTS.md still don't mention it.
- **Hub-and-spoke observability** — full Prometheus/Loki/Grafana on the host,
  agent-mode Prometheus in vclusters remote-writing to the hub.
  See [observability](/platform/observability.md).

# What runs where (2026-08-20)

The host cluster runs ~28 namespaces of platform services: ArgoCD, Cilium +
Hubble, cert-manager, external-dns, external-secrets, nginx-gateway-fabric,
MetalLB, Kratix, kube-prometheus-stack, Loki + Promtail, Kyverno, Trivy,
Goldilocks/VPA, Authentik (SSO), an [AI stack](/platform/ai-stack.md)
(Open WebUI, Qdrant, MCP servers), and one tenant:
[vcluster-media](/cluster/vcluster-media.md). Full listing:
[workload inventory](/cluster/workload-inventory.md);
versions: [component versions](/cluster/component-versions.md).[^live-cluster]

Deeper dives: [GitOps layers](/platform/gitops-layers.md) ·
[security posture](/platform/security-posture.md) ·
[storage](/platform/storage.md) · [known issues](/cluster/known-issues.md)

[^arch-doc]: docs/architecture.md (ADRs, data flows, security model)
[^live-cluster]: Live cluster observation
