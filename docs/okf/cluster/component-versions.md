---
type: Version Snapshot
title: Component versions (live snapshot)
description: Running image versions for every platform component as observed 2026-08-22 after Kargo's day-one bumps, with chart pins where they differ.
tags: [versions, inventory, snapshot]
status: stable
generated: { by: claude-code/claude-fable-5, at: 2026-08-22T03:35:00Z }
stale_after: 2026-09-22
sources:
  - id: live-images
    resource: kubectl get deploy,sts,ds -A -o jsonpath (container images) on the-cluster, 2026-08-22
    title: Live container images
---

Observed running versions.[^live-images] Where a chart pin in `addons/`
differs meaningfully, it's noted — the running image is the truth.

| Component | Running version | Notes |
|---|---|---|
| Talos / Kubernetes | 1.11.5 / v1.34.1 | host; vcluster runs K8s v1.34.3 |
| Cilium (+ Hubble UI) | 1.17.3 (+0.13.2) | CNI, kube-proxy replacement |
| ArgoCD | v3.5.1 | chart 10.4.0; 10.0.0 turns chart NetworkPolicies on -- pinned off, this repo owns netpol. Trivy UI extension installs for the first time as of 2026-08-22 |
| Kratix | syntasso community image (sha-pinned) | chart 0.0.1 |
| cert-manager | v1.21.1 | host had been moved off v1.16.2 on 2026-08-21 (Cloudflare DNS01 cleanup bug blocked wildcard renewal) |
| external-secrets | v0.10.3 | the 2.9.0 bump is open and held: v1beta1 stops being served by default and 39 manifests still use it |
| external-dns | v0.19.0 | |
| MetalLB | v0.16.1 | L2-only: `speaker.frr.enabled` and `frrk8s.enabled` both pinned off, so the speaker is a single container. Metrics moved 7472 -> 9120 |
| nginx-gateway-fabric | 2.6.7 | Gateway API v1.5.1 CRDs -- the two move together, by hand |
| CoreDNS | v1.12.4 | |
| metrics-server | v0.8.0 | |
| Prometheus / operator | v3.14.0-distroless / v0.93.1 | kube-prometheus-stack chart 88.5.3; 85.x made images distroless by default |
| Alertmanager | v0.34.0 | |
| kube-state-metrics / node-exporter | v2.20.0 / v1.12.1-distroless | |
| Grafana | 13.2.0 (kiwigrid sidecar 2.10.1) | admin via 1Password, OIDC via Authentik |
| Loki / Promtail | 3.6.3 / 2.7.3 | promtail notably older than loki; the chart 7.x bump is held -- 7.x is Grafana Enterprise Logs only |
| Kyverno | v1.12.6 | chart 3.2.8; the 3.9.0 bump is open and held -- see [known issues](known-issues.md) |
| Trivy operator / explorer | 0.33.0 / v0.5.8 | chart pins 0.35.0 / 0.4.6; the explorer 0.5.1 bump is held |
| Goldilocks / VPA recommender | v4.16.1 / 1.6.0 | chart 11.0.0 |
| Authentik | 2026.8.0 | |
| Open WebUI / Qdrant | v0.11.0 / v1.19.0 | |
| vcluster (media) | vcluster-oss 0.31.1, etcd 3.6.8-0 | |
| llmkube controller | 0.7.6 (orphaned) | |
| MCP servers | argocd-mcp v0.9.0, github-mcp v1.10.1, kubernetes-mcp v0.0.66, grafana-mcp 1.1.0, supergateway 3.4.3 | only mcpo still floats (`:main`) -- it publishes no release tags |
| platform-status-reconciler | ghcr.io/jamesatintegratnio/gitops_homelab_2_0/platform-status-reconciler:latest | built from `images/` in this repo |
| ~~hello-world~~ | — | removed 2026-08-21 |

Dev-shell tool versions (Nix, same date): kubectl 1.34.1, helm 3.19.0,
talosctl 1.11.3, argocd CLI 3.1.9, opentofu 1.10.6, go 1.25.2.

[^live-images]: Live container images
