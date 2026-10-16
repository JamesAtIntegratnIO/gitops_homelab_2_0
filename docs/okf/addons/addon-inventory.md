---
type: Inventory
title: Addon inventory
description: Every addon defined under addons/ — enabled state, namespace, chart/version, defining layer, and purpose — as of 2026-08-20, plus the kargo/kargo-projects pair added 2026-08-21, on 2026-08-30 the removal of vcluster-coredns-config and the addition of the coredns-host-config / coredns-workload pair, and on 2026-10-06 the specmarshal addon, on 2026-10-11 the umami and cloudflared pair and the homepage landing page, and on 2026-10-16 host-exporters.
tags: [addons, inventory]
status: stable
generated: { by: claude-code, at: 2026-10-11T23:55:00Z }
stale_after: 2026-11-20
sources:
  - id: env-prod
    resource: ../../../addons/environments/production/addons/addons.yaml
    title: environments/production addons.yaml
  - id: role-cp
    resource: ../../../addons/cluster-roles/control-plane/addons/addons.yaml
    title: cluster-roles/control-plane addons.yaml
  - id: role-vc
    resource: ../../../addons/cluster-roles/vcluster/addons/addons.yaml
    title: cluster-roles/vcluster addons.yaml
  - id: cluster-tc
    resource: ../../../addons/clusters/the-cluster/addons.yaml
    title: clusters/the-cluster addons.yaml
---

Layer key: **env** = environments/production, **cp** = cluster-roles/control-plane,
**vc** = cluster-roles/vcluster, **tc** = clusters/the-cluster overrides.

# Enabled — production environment layer (control-plane clusters)

| Addon | Namespace | Chart @ version | Purpose |
|---|---|---|---|
| argocd | argocd | argo-cd @ 10.4.0 (+tc: OIDC secret manifests, syncPolicy, ignoreDiffs) | ArgoCD, self-managed |
| argocd-projects | argocd | manifest | AppProjects: platform, platform-services, appteam-global |
| external-secrets | external-secrets | external-secrets @ 0.10.3 | ESO + onepassword-store ClusterSecretStore (wave -3) |
| metrics-server | kube-system | @ 3.13.0 | Metrics (Talos needs kubelet-insecure-tls) |
| nfs-subdir-external-provisioner | nfs-provisioner | @ 4.0.18 (+2 StorageClasses/Deployments) | [NFS storage](/platform/storage.md) |
| external-dns | external-dns | @ 1.19.0 (+tc: CF secret) | Cloudflare DNS |
| cert-manager | cert-manager | @ v1.16.2 | TLS (wave -1) |
| kratix | kratix-platform-system | @ 0.0.1 (also in cp layer: syncPolicy override, Destination, state-reconciler app, delete-RBAC) | [Kratix](/promises/kratix.md) |
| metallb + metallb-config | metallb-system | @ 0.16.1 + manifests | L2 LB pool 10.0.4.200-253 |
| gateway-api-crds | kube-system | manifest from kubernetes-sigs/gateway-api @ v1.6.3 | Gateway API CRDs |
| nginx-gateway-fabric | nginx-gateway | OCI @ 2.6.7 (+tc: wildcard cert, CF secret, ReferenceGrant) | [Gateway](/platform/networking.md) |
| kyverno | kyverno | @ 3.2.8 | [Policy engine](/platform/security-posture.md) |
| cilium | kube-system | @ 1.17.3 | CNI + Hubble |

Disabled at this layer (AWS-era leftovers): iam-chart, ack-eks, ack-acm,
route53-chart, aws-load-balancer-controller, karpenter, aws_efs_csi_driver,
kro, kro-resource-groups.[^env-prod]

# Enabled — control-plane role layer

| Addon | Namespace | Chart @ version | Purpose |
|---|---|---|---|
| kratix-promises | default | manifestSource: this repo `promises/` @ main | Installs all 7 promises |
| platform-vclusters / platform-http-services | platform-requests | manifestSource: `platform/…` @ main | Applies ResourceRequests |
| observability-secrets | monitoring | manifest | Grafana/matrix ExternalSecrets + 11 HTTPRoutes |
| matrix-alertmanager-receiver | monitoring | manifest | Alertmanager → Matrix bridge |
| vcluster/argocd/loki/kratix/trivy dashboards + platform-landing-zone | monitoring | manifests | Grafana dashboard ConfigMaps |
| network-policies | default | manifest (22 files) | ~100 NetPols + 26 CNPs + Kyverno policies |
| authentik | authentik | @ 2026.8.0 (+tc: blueprints, redis, ExternalSecrets, route) | SSO/OIDC |
| kube-prometheus-stack (+extras) | monitoring | @ 88.5.3 + etcd-cert Job | [Monitoring hub](/platform/observability.md) |
| loki / promtail | loki / promtail | @ 18.11.0 (grafana-community) / 6.17.1 | Logging |
| goldilocks | goldilocks | @ 11.0.0 | VPA right-sizing (Auto mode) |
| trivy-operator / trivy-explorer (+route, dashboard) | trivy-system | @ 0.35.0 / 0.4.6 | Vulnerability scanning |
| qdrant / open-webui | ai | @ 1.17.0 / 12.5.0 | [AI stack](/platform/ai-stack.md) |
| platform-status-reconciler | platform-status-reconciler | manifest | Custom status controller |
| kargo | kargo | kargo @ 1.11.2 (OCI, +kargo-extras: ExternalSecrets, HTTPRoute) | [Kargo](/platform/kargo.md) — version-bump bot; control-plane only |
| argo-rollouts | argo-rollouts | argo-rollouts @ 2.41.1 (controller only, no dashboard) | Executes Kargo's verification AnalysisRuns; no Rollout objects exist |
| kargo-projects | kargo (resources land in `addons`, `promises`, `workloads`) | local chart `addons/charts/kargo-projects` | 3 Projects, 48 Warehouse/Stage pairs from one target list |
| coredns-host-config | kube-system | manifest (`addons/.../coredns/`) | Owns the host Corefile so the `auth.cluster.integratn.tech` rewrite survives; `selfHeal: true`, `prune: false` |
| coredns-workload | kube-system | manifest (`addons/.../coredns-workload/`) | SA, RBAC, Deployment, `kube-dns` Service — Talos's own render, adopted 2026-08-30; `prune: false` |
| specmarshal | specmarshal | `registry.integratn.io/integratn/charts/specmarshal` 0.2.0-rc.9 (licensed, private) + manifests (`addons/.../specmarshal/`) | Specmarshal's orchestrator from its published chart; each agent iteration is a Job beside it that it `kubectl exec`s into. Preconditions and first-run steps in the addon's README.md. Added 2026-10-06 |
| homepage | homepage | manifest (`addons/.../homepage/`): Glance `docker.io/glanceapp/glance` v0.8.6, digest-pinned, Kargo-tracked (target `homepage`, `autoMerge: patch`). Replaced Homepage (gethomepage.dev) v2.5.0 on 2026-10-11 | The cluster landing page at `home.cluster.integratn.tech`, behind authentik, in the IntegratnIO brand (specmarshal DESIGN.md): status verdict and chips that open a Needs you tab (each alert, app, promotion and stuck pod with what it means and a direct link; alerts from Alertmanager, unsilenced), node cards and sparklines, a Media tab, media, Umami, Plex/Proxmox/Tower, releases, launcher. Cluster data from Prometheus and Alertmanager; config is the `glance-config` ConfigMap, keys from 1Password `homepage-secret`. Added 2026-10-11 |
| host-exporters | host-exporters | manifest (`addons/.../host-exporters/`): `docker.io/prompve/prometheus-pve-exporter` 3.10.1 and `ghcr.io/unpoller/unpoller` v5.5.1, digest-pinned, Kargo-tracked (targets `pve-exporter`, `unpoller`), plus a ScrapeConfig in `monitoring` for Tower's node_exporter | The machines outside the cluster as metrics: Proxmox (192.168.0.10, API token from `homepage-secret`), UniFi (192.168.0.1, login `unifi-mcp-login`) and Tower (10.0.0.12:9100, `job="host-node"`, `host="tower"`). holocron and pitdroid push the same job from nix_dots_2. Alerts in `platform-alerts/host-rules.yaml`; shown on the landing page's Hosts tab. Added 2026-10-16 |
| umami | umami | manifest (`addons/.../umami/`): `ghcr.io/umami-software/umami` 3.4.0 + `redis:7.4.11-alpine` (idle: the Deployment sets `REDIS_URL` empty until a release fixes umami-software/umami#4552), digest-pinned, Kargo-tracked | Cookieless web analytics for integratn.io and bosun.integratn.io; Postgres on 10.0.3.1. Dashboard `stats.cluster.integratn.tech` behind authentik, MCP at `mcp.cluster.integratn.tech/umami/mcp`, tracker public only through the tunnel. Added 2026-10-11 |
| cloudflared | cloudflared | manifest (`addons/.../cloudflared/`): `cloudflare/cloudflared` 2026.10.0, digest-pinned, Kargo-tracked | Two connectors of a remotely-managed Cloudflare Tunnel — the cluster's only public ingress. Hostname and path rules are OpenTofu in IntegratnIO/infrastructure. Added 2026-10-11 |

Disabled: ai-platform (superseded by mcp-system; its path no longer exists).[^role-cp]

# Enabled — vcluster role layer (rendered per vcluster, onto the host)

| Addon | Purpose |
|---|---|
| argocd-vcluster (argo-cd @ 10.4.0) | Per-vcluster ArgoCD; valuesObject also injects HTTPRoutes, in-cluster secret, and the nested **workloads ApplicationSet** |
| external-dns-vcluster (+resources) | Per-vcluster DNS, own txtOwnerId |
| kube-prometheus-stack-agent (@ 88.5.3) | Agent-mode metrics → hub |
| gateway-api-crds-vcluster | CRDs (non-production vclusters only) |
| cert-manager-vcluster (@ v1.21.1) | cert-manager + letsencrypt-prod issuer + wildcard cert |
| nginx-gateway-fabric-vcluster (OCI @ 2.6.7) | Per-vcluster gateway on `*.<cluster>.<domain>` |

Disabled: promtail-vcluster (host promtail covers vcluster logs).[^role-vc]

# Cluster overrides

- **the-cluster**: adds manifests to argocd/nginx-gateway-fabric/external-dns/
  authentik/qdrant; **disabled**: llmkube (commit 604c506) and the four
  legacy `mcp-*` stakater addons (superseded by the out-of-band
  `mcp-system-app` Application).[^cluster-tc]
- **vcluster-media**: empty addons.yaml — takes the vcluster role set
  verbatim, with one values override (kube-prometheus-stack-agent).

# Not in the addon framework

`mcp-system-app.yaml` (plain Application, applied out-of-band) — see
[AI stack](/platform/ai-stack.md). Staging/development environment files
exist (byte-identical to each other) but no cluster carries those labels —
latent config.

[^env-prod]: environments/production addons.yaml
[^role-cp]: cluster-roles/control-plane addons.yaml
[^role-vc]: cluster-roles/vcluster addons.yaml
[^cluster-tc]: clusters/the-cluster addons.yaml
