---
type: Kratix Promise
title: vcluster-orchestrator-v2
description: The composite promise behind VClusterOrchestratorV2 — one Go pipeline that renders a full tenant vcluster (Helm app, ArgoCD registration, DNS/TLS/secrets integrations, network policies).
tags: [kratix, promise, vcluster, go]
status: stable
generated: { by: claude-code/claude-fable-5, at: 2026-08-20T23:40:00Z }
stale_after: 2027-02-20
sources:
  - id: promise-yaml
    resource: ../../../promises/vcluster-orchestrator-v2/promise.yaml
    title: promise.yaml (CRD schema + workflows)
  - id: pipeline-src
    resource: ../../../promises/vcluster-orchestrator-v2/workflows/resource/configure/main.go
    title: Pipeline source (main.go + builders_*.go)
  - id: vclusters-doc
    resource: ../../vclusters.md
    title: docs/vclusters.md (v1→v2 evolution)
---

# API

`platform.integratn.tech/v1alpha1` **VClusterOrchestratorV2** (namespaced,
requested in `platform-requests`). Only `spec.name` is required; everything
else defaults. Highlights:[^promise-yaml]

| Field | Default | Notes |
|---|---|---|
| `vcluster.preset` | `dev` | `dev`: 1 replica, 768Mi–1536Mi, no persistence, 1 CoreDNS. `prod`: 3 replicas, 1–2Gi, persistence 10Gi, 2 CoreDNS. (Authoritative source is the Go code — the READMEs' preset tables are stale.) |
| `vcluster.k8sVersion` | `v1.34.3` | vclusters may run newer K8s than the host |
| `vcluster.backingStore` / `helmOverrides` / `exportKubeConfig` | — | free-form pass-through, deep-merged over generated values (user wins) |
| `exposure.hostname` | `{name}.{baseDomain}` | baseDomain from annotation `platform.integratn.tech/base-domain`, else `integratn.tech` |
| `exposure.subnet` / `vip` | VIP = subnet network + **200** | code uses offset 200 to land in the MetalLB pool; promise.yaml/README still say ".100" (stale) |
| `integrations.certManager` / `externalSecrets` | letsencrypt-prod / onepassword-store selector labels | synced into the vcluster |
| `integrations.argocd.workloadRepo` | this repo, path `workloads`, rev `main` | drives the per-vcluster workloads ApplicationSet |
| `networkPolicies.enableNFS`, `extraEgress[]` | false / — | e.g. vcluster-media adds postgres egress `10.0.3.1/32:5432` |

# What the configure pipeline emits

Single pipeline `vco-v2-configure` (v2 replaced the archived v1's 6-sub-promise
decomposition; the `_archived/` tree was deleted in commit `1e5d7b0`). Outputs:[^pipeline-src]

1. **Three sub-ResourceRequests** into `platform-requests`:
   [ArgoCDProject](/promises/argocd-project.md) (sync-wave -1),
   [ArgoCDApplication](/promises/argocd-application.md) `vcluster-{name}`
   (the loft `vcluster` Helm chart with a fully generated valuesObject),
   [ArgoCDClusterRegistration](/promises/argocd-cluster-registration.md).
2. **Direct resources**: Namespace (wave -2), `vc-{name}-coredns` ConfigMap,
   optional 9-doc etcd cert chain (cert-manager CA/Issuer/server/peer certs +
   a kubectl merge Job producing `{name}-etcd-certs`), and 7+ network policies.

Generated Helm values worth knowing: LoadBalancer service with
`loadBalancerIP: {vip}` and an external-dns hostname annotation, port
`{apiPort}→8443`, ServiceMonitor enabled, JSON logging, sync-from-host of the
`external-secrets/eso-onepassword-token` secret (with a matching RBAC rule),
sync-to-host of pods/PVCs/ingresses/netpols.

The network-policy set includes Cilium-specific pieces: `allow-kube-api`
(CiliumNetworkPolicy `toEntities: kube-apiserver`), `allow-coredns-to-host-dns`
(to Talos node-local DNS `169.254.116.108/32`), and `allow-vcluster-lb-snat`
(`fromEntities: host/remote-node/world` → 8443 — works around Cilium
kube-proxy-replacement DNAT-before-policy behavior).

# Delete pipeline

`vco-v2-delete` is the only pipeline that touches the live API: it deletes
vcluster-managed PVs and the target namespace via in-cluster client-go
(RBAC granted by the kratix addon values), then writes ~20 delete stubs for
the state store. 

# Consumers and live instances

Live instance: `vcluster-media` (prod preset, 3+3 HA, external etcd 3.6.8-0,
`media.integratn.tech`) — see [vcluster-media](/cluster/vcluster-media.md).
`hctl vcluster create` is the paved-road authoring path.

Known doc drift (README preset/VIP/delete descriptions) is catalogued in
[known issues](/cluster/known-issues.md).

[^promise-yaml]: promise.yaml (CRD schema + workflows)
[^pipeline-src]: Pipeline source (main.go + builders_*.go)
