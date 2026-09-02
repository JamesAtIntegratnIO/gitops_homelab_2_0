---
type: Inventory
title: Workload inventory (live snapshot)
description: Everything running on the-cluster by namespace — deployments, statefulsets, daemonsets — as observed 2026-08-20.
tags: [inventory, workloads, snapshot]
status: stable
generated: { by: claude-code/claude-opus-5, at: 2026-09-01T00:00:00Z }
stale_after: 2026-10-01
sources:
  - id: live-workloads
    resource: kubectl get deploy,sts,ds -A on the-cluster (admin@the-cluster), 2026-08-20
    title: Live workload listing
  - id: alert-sweep-0901
    resource: kubectl sweep of the-cluster, 2026-09-01 (trivy-system Jobs/Pods)
    title: Alert triage sweep 2026-09-01
---

All workloads were Ready at snapshot time except where noted.[^live-workloads]

# By namespace

| Namespace | Workloads |
|---|---|
| `argocd` | argo-cd server ×2, repo-server ×2, application-controller ×2, applicationset-controller ×2, notifications, redis |
| `kube-system` | Cilium DS + cilium-envoy DS + operator, CoreDNS ×2, Hubble relay + UI, metrics-server ×2 |
| `cert-manager` | controller, cainjector, webhook |
| `external-dns` | external-dns |
| `external-secrets` | controller, cert-controller, webhook |
| `nginx-gateway` | nginx-gateway-fabric (control plane), nginx-gateway-nginx (data plane) |
| `metallb-system` | controller, speaker DS |
| `kratix-platform-system` | kratix-platform-controller-manager |
| `platform-requests` | (pipeline Jobs only, periodic; daily cleanup CronJob at 04:00) |
| `platform-status-reconciler` | platform-status-reconciler |
| `monitoring` | prometheus (STS), alertmanager (STS), grafana, kube-state-metrics, prometheus-operator, node-exporter DS, matrix-alertmanager-receiver |
| `loki` | loki (STS), loki-gateway, chunks-cache + results-cache (STS), loki-canary DS |
| `promtail` | promtail DS |
| `kyverno` | admission, background, cleanup, reports controllers + 4 cleanup CronJobs (*/10m) |
| `trivy-system` | trivy-operator, trivy-operator-explorer (+ 13 stuck scan Jobs and 9 `Failed` Pods from 2026-08-28 — see [known issues](/cluster/known-issues.md)) |
| `goldilocks` | controller, dashboard, vpa-recommender |
| `authentik` | server ×2, worker, redis |
| `nfs-provisioner` | config-, data-, and plain nfs-subdir provisioners |
| `ai` | open-webui (STS), qdrant (STS), git-indexer CronJob (hourly, **failing** — orphaned) |
| `mcp-system` | argocd-mcp, github-mcp, grafana-mcp, kubernetes-mcp, mcpo, sequential-thinking |
| `llmkube-system` | llmkube-controller-manager, llama-3.1-8b (0/1, orphaned) |
| ~~`hello-world`~~ | *removed 2026-08-21* — was the http-service promise's smoke test; deleted because it cost three pipelines re-running every ~10h for nothing |
| `vcluster-media` | vcluster-media STS ×3 + etcd STS ×3 + 35 synced pods — see [vcluster-media](/cluster/vcluster-media.md) |

# Cross-cutting counts (2026-08-20)

- ~55 ArgoCD Applications, ~47 ApplicationSets, 5 AppProjects (+1 default)
- 7 Kratix promises, 7 Works, 1 Destination
- ~130 NetworkPolicies + 29 CiliumNetworkPolicies, 7 Kyverno ClusterPolicies
- 40 PrometheusRules, 34 ServiceMonitors, ~30 Grafana dashboards
- 24 ExternalSecrets (all Ready), 1 ClusterSecretStore
- 23 PVs on NFS, 2 StorageClasses
- 3 LoadBalancer services (10.0.4.205 main gateway, .200/.201 vcluster-media)

[^live-workloads]: Live workload listing
