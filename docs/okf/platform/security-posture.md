---
type: Security Posture
title: Security posture
description: Defense-in-depth as actually deployed — Talos hardening, enforced Cilium/NetworkPolicies, Kyverno policies, Trivy scanning, Authentik SSO, VPA governance.
tags: [security, kyverno, cilium, network-policy, trivy, authentik]
status: stable
generated: { by: claude-code/claude-opus-5, at: 2026-09-01T00:00:00Z }
stale_after: 2027-02-20
sources:
  - id: live-sec
    resource: kubectl get clusterpolicies,netpol,cnp -A on the-cluster, 2026-08-20
    title: Live policy state
  - id: alert-sweep-0901
    resource: Prometheus /api/v1/alerts + kubectl sweep of the-cluster, 2026-09-01
    title: Alert triage sweep 2026-09-01
  - id: prod-readiness
    resource: ../../production-readiness-plan.md
    title: docs/production-readiness-plan.md (phases + completion history)
  - id: netpol-addon
    resource: ../../../addons/cluster-roles/control-plane/addons/network-policies/
    title: network-policies addon (100 NetPols + 26 CNPs + Kyverno policies)
---

# Layers

**OS**: Talos — immutable, no SSH, API-only, kernel hardening boot flags
(`pti=on`, `init_on_alloc/free`, `slab_nomerge`).

**Network**: Cilium policy in **enforce mode** (since 2026-02-18, commit
ee491b2). Every namespace ships `default-deny-all` plus scoped allows —
~130 NetworkPolicies and 29 CiliumNetworkPolicies live today, delivered by
the `network-policies` manifest addon. CNPs handle what vanilla netpol can't:
`allow-kube-api` per namespace (`toEntities: kube-apiserver`), metrics-server
from apiserver, vcluster LB SNAT.[^live-sec]

**Admission (Kyverno v1.12.6)** — 7 ClusterPolicies:

| Policy | Mode |
|---|---|
| `disallow-default-namespace` | **Enforce** |
| `disallow-privileged-containers` | **Enforce** |
| `generate-default-deny-netpol` | Audit + generate (creates default-deny per namespace) |
| `require-default-deny-netpol` | Audit |
| `restrict-image-registries` | Audit (discovery mode) |
| `mutate-restrict-escalation` | mutate (drops capabilities — the reason root-requiring images fail; see [http-service](/promises/http-service.md)) |
| `mutate-seccomp-default` | mutate |

Plus `mutate-ns-vpa-auto-mode`, which labels new namespaces for VPA.

**Scanning**: Trivy operator scans workloads (kube-system excluded), with a
Grafana dashboard, explorer UI (`trivy.cluster.integratn.tech`), and CVE
alerts. Chart pinned at 0.35.0. Caveat: **scan coverage is not itself
monitored**, and on 2026-09-01 it was found to have fallen to 2
VulnerabilityReports cluster-wide with every health signal green — one report
4% over the 2 MiB apiserver limit had wedged the queue since 2026-08-28. Fixed
by `trivy.ignoreUnfixed: true`; the missing coverage alert is still open. See
[known issues](/cluster/known-issues.md).

**Identity**: Authentik 2026.8.0 (`auth.cluster.integratn.tech`) provides
OIDC for ArgoCD, Grafana, and Open WebUI (client secrets via 1Password;
Google OAuth federated). Prometheus/Alertmanager/Loki endpoints have **no
auth** (flagged as future work in the docs).

**Secrets**: see [secret management](/platform/secret-management.md) — the
no-Secrets-in-git chain.

**Resource governance**: Goldilocks + VPA in **Auto mode** across ~all
namespaces (107 VPA objects at rollout); ArgoCD globally ignores
container-resources drift so VPA and self-heal don't fight. ResourceQuota /
LimitRange remain open items (production-readiness Phase 3).[^prod-readiness]

**Supply chain**: promise pipeline pods run non-root 65532, read-only rootfs,
drop-ALL, seccomp RuntimeDefault; CI images push with ephemeral tokens.
Weak spots: several `:latest` image tags (pipelines, mcp servers, an etcd
merge job using `bitnami/kubectl:latest`), and the broken local pre-commit
hook (see [CI workflows](/tooling/ci-workflows.md)).

# Roadmap context

Production-readiness phases 1.1–1.3 (Cilium enforce, Trivy, Kyverno enforce)
and 2.5 (VPA/Goldilocks) are complete; open items include ResourceQuotas,
OpenCost, tracing (Tempo/OTel), long-term metrics, alert-channel redundancy,
CI hardening, and Falco as a stretch goal.[^prod-readiness]

[^live-sec]: Live policy state
[^prod-readiness]: docs/production-readiness-plan.md (phases + completion history)
[^netpol-addon]: network-policies addon (100 NetPols + 26 CNPs + Kyverno policies)
