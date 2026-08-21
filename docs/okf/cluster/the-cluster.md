---
type: Kubernetes Cluster
title: the-cluster
description: The host Kubernetes cluster — identity, control plane, API access, and its role as both platform and Kratix destination.
tags: [cluster, kubernetes, talos]
resource: https://10.0.4.100:6443
status: stable
generated: { by: claude-code/claude-fable-5, at: 2026-08-20T23:40:00Z }
stale_after: 2026-11-20
sources:
  - id: live-cluster
    resource: kubectl (context admin@the-cluster) sweep, 2026-08-20
    title: Live cluster observation
---

# Identity

| Property | Value |
|---|---|
| Name | `the-cluster` |
| API endpoint | `https://10.0.4.100:6443` (Talos VIP; kubeconfig context `admin@the-cluster`) |
| Kubernetes | v1.34.1 (pinned by Talos 1.11.5) |
| Nodes | 3× control-plane, schedulable — see [Talos nodes](/infrastructure/talos-nodes.md) |
| Age | ~221 days (provisioned mid-January 2026) |
| ArgoCD cluster labels | `cluster_role: control-plane`, `environment: production`, ~16 `enable_*` flags |
| Kratix role | the single Destination (`capability.vcluster: "true"`) |
| Base domain | `cluster.integratn.tech` (services), `integratn.tech` (vcluster APIs) |

# Namespaces (28, by function)

- **Kubernetes/CNI**: kube-system (Cilium, CoreDNS, Hubble, metrics-server),
  cilium-secrets
- **GitOps/platform**: argocd, kratix-platform-system, kratix-worker-system,
  platform-requests (pipelines + CRs), platform-status-reconciler
- **Ingress/network**: nginx-gateway, metallb-system, external-dns
- **Security/identity**: cert-manager, external-secrets, kyverno,
  trivy-system, authentik
- **Observability**: monitoring, loki, promtail, goldilocks
- **Storage**: nfs-provisioner
- **AI**: ai, mcp-system, llmkube-system (orphaned)
- **Apps/tenants**: hello-world, vcluster-media, default (has llmkube
  leftovers), kube-public, kube-node-lease

# Access

Admin access via the local kubeconfig (single context). Web entry points:
ArgoCD `argocd.cluster.integratn.tech`, Grafana
`grafana.cluster.integratn.tech`, Authentik `auth.cluster.integratn.tech`.
Day-2 tooling: [hctl](/tooling/hctl.md), k9s, talosctl (all via the
[Nix dev shell](/tooling/nix-dev-shell.md)).

Health at snapshot time: all nodes Ready; ~55 ArgoCD apps — 51 Synced/Healthy,
4 OutOfSync (kratix-state-reconciler, kube-prometheus-stack-extras,
network-policies, mcp-system-app), 1 Progressing (nginx-gateway-fabric).
Details and explanations: [known issues](/cluster/known-issues.md).
Full workload map: [workload inventory](/cluster/workload-inventory.md).
Versions: [component versions](/cluster/component-versions.md).[^live-cluster]

[^live-cluster]: Live cluster observation
