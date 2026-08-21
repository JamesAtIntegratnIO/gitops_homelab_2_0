---
type: Version Snapshot
title: Component versions (live snapshot)
description: Running image versions for every platform component as observed 2026-08-20, with chart pins where they differ.
tags: [versions, inventory, snapshot]
status: stable
generated: { by: claude-code/claude-fable-5, at: 2026-08-20T23:40:00Z }
stale_after: 2026-09-20
sources:
  - id: live-images
    resource: kubectl get deploy,sts,ds -A -o jsonpath (container images) on the-cluster, 2026-08-20
    title: Live container images
---

Observed running versions.[^live-images] Where a chart pin in `addons/`
differs meaningfully, it's noted — the running image is the truth.

| Component | Running version | Notes |
|---|---|---|
| Talos / Kubernetes | 1.11.5 / v1.34.1 | host; vcluster runs K8s v1.34.3 |
| Cilium (+ Hubble UI) | 1.17.3 (+0.13.2) | CNI, kube-proxy replacement |
| ArgoCD | v3.3.1 | chart pin 9.4.3 in addons; docs mentioning 9.0.3/2.x are stale |
| Kratix | syntasso community image (sha-pinned) | chart 0.0.1 |
| cert-manager | v1.19.3 | host upgraded 2026-08-21 from v1.16.2 (Cloudflare DNS01 cleanup bug had blocked wildcard renewal); vclusters already v1.19.3 |
| external-secrets | v0.10.3 | |
| external-dns | v0.19.0 | |
| MetalLB | v0.15.3 | |
| nginx-gateway-fabric | 2.2.2 | Gateway API v1.4.0 CRDs |
| CoreDNS | v1.12.4 | |
| metrics-server | v0.8.0 | |
| Prometheus / operator | v3.9.1 / v0.89.0 | kube-prometheus-stack chart 82.1.1 |
| Alertmanager | v0.31.1 | |
| kube-state-metrics / node-exporter | v2.18.0 / v1.10.2 | |
| Grafana | (kiwigrid sidecar 2.5.0; check pod for core version) | admin via 1Password, OIDC via Authentik |
| Loki / Promtail | 3.6.3 / 2.7.3 | promtail notably older than loki |
| Kyverno | v1.12.6 | chart 3.2.8 |
| Trivy operator / explorer | 0.30.0 / v0.5.8 | chart pins 0.32.0 / 0.4.6 |
| Goldilocks / VPA recommender | v4.14.1 / 1.4.1 | |
| Authentik | 2025.12.4 | |
| Open WebUI / Qdrant | v0.8.6 / v1.17.0 | |
| vcluster (media) | vcluster-oss 0.31.1, etcd 3.6.8-0 | |
| llmkube controller | 0.7.6 (orphaned) | |
| MCP servers | argocd-mcp v0.5.0; github/grafana/kubernetes/supergateway/mcpo on floating tags | several `:latest`/`:main` |
| platform-status-reconciler | ghcr.io/jamesatintegratnio/gitops_homelab_2_0/platform-status-reconciler:latest | built from `images/` in this repo |
| hello-world | nginx-unprivileged:latest | |

Dev-shell tool versions (Nix, same date): kubectl 1.34.1, helm 3.19.0,
talosctl 1.11.3, argocd CLI 3.1.9, opentofu 1.10.6, go 1.25.2.

[^live-images]: Live container images
