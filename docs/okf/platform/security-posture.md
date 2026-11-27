---
type: Security Posture
title: Security posture
description: Defense-in-depth as actually deployed — Talos hardening, enforced Cilium/NetworkPolicies, Kyverno policies, Trivy scanning, Authentik SSO, VPA governance.
tags: [security, kyverno, cilium, network-policy, trivy, authentik]
status: stable
generated: { by: claude-code/claude-opus-5-5, at: 2026-11-26T02:00:00Z }
stale_after: 2027-02-20
sources:
  - id: live-kyverno-1126
    resource: kubectl get validatingpolicies,mutatingpolicies,generatingpolicies,deletingpolicies,policyexceptions -A on the-cluster, 2026-11-26
    title: Live Kyverno CEL policy state after the 1.20 migration
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

**Admission (Kyverno v1.20, chart 3.10.0, since 2026-11-26)** — CEL policy
types (`policies.kyverno.io/v1`) only. Kyverno 1.20 no longer executes
`ClusterPolicy`, `ClusterCleanupPolicy` or the `kyverno.io` PolicyException,
so every policy was migrated (#894, #902, #907) before the bump (#788).
Files: `addons/cluster-roles/control-plane/addons/network-policies/kyverno-*.yaml`.

| Policy | Kind | Mode |
|---|---|---|
| `disallow-default-namespace` | ValidatingPolicy | **Deny** (Kratix pipeline pods/Jobs exempt) |
| `disallow-privileged-containers` | ValidatingPolicy | **Deny** (kube-system excluded) |
| `restrict-pushsecret` | ValidatingPolicy | **Deny** |
| `require-default-deny-netpol` | ValidatingPolicy | Audit |
| `restrict-image-registries` | ValidatingPolicy | Audit (discovery mode) |
| `audit-arm64-images` | ValidatingPolicy | Audit |
| `generate-default-deny-netpol` | GeneratingPolicy | generate + synchronize, `orphanDownstreamOnPolicyDelete` (creates and owns `default-deny-all` in every namespace) |
| `mutate-restrict-escalation` | MutatingPolicy | mutate, JSONPatch, `failurePolicy: Ignore` (drops capabilities — the reason root-requiring images fail; see [http-service](/promises/http-service.md)) |
| `mutate-seccomp-default` | MutatingPolicy | mutate |
| `schedule-arm64-images` | MutatingPolicy | mutate, `failurePolicy: Ignore` |
| 4 cleanup policies | DeletingPolicy | hourly / 15-min schedules |

PolicyExceptions are enabled but **only honoured in `monitoring`**
(`features.policyExceptions` in the kyverno values; the chart's default is
off, which silently ignores every exception). The one exception is
`etcd-cert-extractor-privileged-cel` for the etcd-secret-writer Job.

Writing CEL policies here — what bit us on 2026-11-26 (see
[known issues](/cluster/known-issues.md)):

- Dry-run every new or changed policy server-side first
  (`kubectl apply --dry-run=server`). Kyverno's `validate-policy.kyverno.svc`
  webhook compiles it and rejects CEL errors. That does **not** run a mutation.
- A MutatingPolicy `ApplyConfiguration` may not set an atomic list
  (`capabilities.drop`, …): it fails at runtime, per pod. Use JSONPatch.
- CEL map literals need one value type: wrap mixed values in `dyn()`.
  On 1.19, `resource.List(...).items` is typed `any`; wrap the call in `dyn()`
  before `exists()`.
- Give mutations `failurePolicy: Ignore` unless failing closed is the point.

Plus `mutate-ns-vpa-auto-mode`, which labels new namespaces for VPA.

**Scanning**: Trivy operator scans workloads (kube-system excluded), with a
Grafana dashboard, explorer UI (`trivy.cluster.integratn.tech`), and CVE
alerts. Chart pinned at 0.35.0. Caveat: **scan coverage is not itself
monitored**, and on 2026-09-01 it was found to have fallen to 2
VulnerabilityReports cluster-wide with every health signal green — one report
4% over the 2 MiB apiserver limit had wedged the queue since 2026-08-28. Fixed
by `trivy.ignoreUnfixed: true`; the missing coverage alert is still open. Since 2026-10-18 scans run in ClientServer mode against the chart's built-in Trivy server (DB on an NFS PVC), with failed scans retried hourly. See
[known issues](/cluster/known-issues.md).

**Identity**: Authentik 2026.2.3 (`auth.cluster.integratn.tech`; the 2026.8.0
jump was reverted on 2026-08-22, see the comment in `addons.yaml`) provides
OIDC for ArgoCD, Grafana, Open WebUI and Kargo, and proxy auth for the
landing page, Bosun, Specmarshal, Umami and Qdrant (client secrets via
1Password; Google OAuth federated). It wears the IntegratnIO brand (logo,
favicon, CSS, a background per flow) from the generated blueprint and assets
in `hack/authentik-theme/`. Prometheus/Alertmanager/Loki endpoints have **no
auth** (flagged as future work in the docs).

Access model (since 2026-11-25, blueprints `03`, `11`, `12`, `13`, `14` in
`authentik-blueprints-configmap.yaml`):

- **Invite-only.** No public sign-up anywhere. New accounts come only from
  invitations on flow `integratn-invitation`, used one of two ways: the
  invitation link (username + password + MFA), or a Google button on the
  sign-in page with the Google account whose email is in the invite's Fixed
  data (flow `integratn-source-enrollment`, gated by an expression policy
  because authentik loses the invite token across the Google redirect and its
  Source stage is enterprise-only). A Google account with no matching invite
  is refused; an existing account's Google sign-in links to it by email.
- **MFA required.** The brand signs in through `integratn-authentication`
  (password, then TOTP/WebAuthn, enrolment forced if missing); Google sign-ins
  run `integratn-source-authentication`, which trusts Google's own 2FA (no
  second prompt, James's call). Password sign-in is home-network only: a deny
  stage before the password refuses non-private client IPs and anything that
  came through Cloudflare (`Cf-Ray`/`Cf-Connecting-Ip`), so from outside the
  only way in is Google. The stock
  `default-authentication-flow` is bound to a deny-all policy.
- **Default-deny apps.** Every application is bound to a group: `homelab`
  (ArgoCD, Kargo, Grafana, Bosun, Specmarshal, Qdrant, Umami, landing page,
  Open WebUI), `open-webui-users`, `mealie-users`, `authentik Admins` (hermes,
  cq), `home-family` (bound wherever media-viewer, open-webui-users or
  mealie-users are). `media-management` and `media-viewer` have no apps bound
  yet. Invited accounts start in `media-viewer`.
- **Admin UI LAN-only.** `/if/admin` on `auth.` redirects to
  `auth-admin.cluster.integratn.tech`, which only private client IPs may use
  (pod range 10.244.0.0/16 and anything with Cloudflare headers refused first,
  since the cloudflared connectors are pods) and which must never be published
  on a tunnel.
- **Superusers sign in at home only.** Google sign-in has no authentik MFA, so
  a Google sign-in that came through Cloudflare is refused for superusers
  (deny stage on `integratn-source-authentication`). Policy-gated deny stages
  fail closed (`failure_result: true`). `auth.` sends HSTS.
- **Public name `auth.integratn.tech`** (since 2026-11-25): Cloudflare tunnel
  straight to the `authentik-server` Service, never the LAN gateway, with
  `/if/admin` and `/api/v3/admin` answering 404 at the edge. Through it only
  Google sign-in of non-admins and invitation sign-up work (password and
  superusers are refused on Cloudflare traffic). On the LAN the name stays
  `auth.cluster.integratn.tech`. Tunnel and DNS: IntegratnIO/infrastructure
  `sites`.
- **Sonarr and Radarr away from home** (PR open 2026-11-27, not live until
  this repo's change merges AND IntegratnIO/infrastructure `sites` is
  applied): `sonarr.integratn.tech` / `radarr.integratn.tech` through the
  tunnel, straight to the apps' synced Services (never the vcluster gateway,
  whose LAN rule leaves `/api` open to key holders). Cloudflare Access (free
  plan) in front, signing in with authentik as OIDC (client `cloudflare-access`,
  blueprint `authentik-blueprints-cloudflare-access`), `media-management` only,
  checked by authentik's binding AND the Access policy's `groups` claim;
  cloudflared re-checks the Access token. Paths that return the apps' API keys,
  backups, logs, restart/shutdown and other systems' credentials answer 404 at
  the edge. Bazarr and SABnzbd stay LAN-only (Bazarr's UI loads its full
  settings, keys and provider passwords included).
- No bootstrap API token. `akadmin` is to be retired in favour of a named admin
  (see the authentik addon README).

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
