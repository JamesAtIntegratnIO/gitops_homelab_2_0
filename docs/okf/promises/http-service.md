---
type: Kratix Promise
title: http-service
description: The "product" promise — deploy a containerized HTTP app with routing, 1Password secrets, monitoring, and network policies from one HTTPService CR.
tags: [kratix, promise, http, stakater]
status: stable
generated: { by: claude-code/claude-fable-5, at: 2026-08-20T23:40:00Z }
stale_after: 2027-02-20
sources:
  - id: promise-yaml
    resource: ../../../promises/http-service/promise.yaml
    title: promise.yaml
  - id: pipeline-src
    resource: ../../../promises/http-service/workflows/resource/configure/main.go
    title: Pipeline source
  - id: live-instance
    resource: ../../../platform/http-services/hello-world.yaml
    title: hello-world HTTPService request
---

# API

`platform.integratn.tech/v1alpha1` **HTTPService** (shortname `httpsvc`).
Required: `name`, `image` (`image.repository`). Defaults: namespace `{name}`,
tag `latest`, replicas 1 (max 10), port 8080, ingress **enabled** at
`{name}.cluster.integratn.tech`, health check on `/`, monitoring off,
persistence off. `secrets[]` maps 1Password items → secret keys;
`helmOverrides` is a deep-merged escape hatch.[^promise-yaml]

# Pipeline composition

The configure pipeline renders a Namespace directly, then delegates through
**three sub-ResourceRequests** (git-mediated, into `platform-requests`):[^pipeline-src]

| Output | Wave | Delegates to |
|---|---|---|
| Namespace (labeled `platform.integratn.tech/gateway-access: "true"`) | 0 | — |
| ArgoCDApplication — **Stakater `application` chart 6.16.1** with generated values | 10 | [argocd-application](/promises/argocd-application.md) |
| PlatformExternalSecret `{name}-secrets` (only if `secrets[]` set) | — | [external-secret](/promises/external-secret.md) |
| GatewayRoute `{name}-route` (only if ingress enabled) | — | [gateway-route](/promises/gateway-route.md) |
| NetworkPolicies (inline): allow-gateway, allow-monitoring, allow-dns | 5 | — |

Notably, the Stakater values explicitly disable ~20 chart sub-features
(httpRoute, ingress, externalSecret, networkPolicy, autoscaling, pdb…) because
those concerns are owned by the sub-promises. Default-deny is deliberately
*not* emitted — the Kyverno `generate-default-deny-netpol` ClusterPolicy
creates it per-namespace (see [security posture](/platform/security-posture.md)).

Status gives `phase: Configured` and the final `url`.

# Image compatibility gotcha

Kyverno's `mutate-restrict-escalation` policy drops all capabilities, so
root-requiring images (stock `nginx`, `nginxdemos/hello`) crash. Use
unprivileged variants (`nginxinc/nginx-unprivileged`) — the promise README has
a whole section on this, plus the `helmOverrides` escape hatch (Kyverno
mutation still wins).

# Live instance

`hello-world` (`nginx-unprivileged:latest`, hello-world.cluster.integratn.tech),
requested from [platform/http-services/hello-world.yaml](../../../platform/http-services/hello-world.yaml)
via the `platform-http-services` addon. Status on cluster: Reconciled.[^live-instance]

Delete: writes stubs removing the three sub-requests (children cascade); the
Namespace and inline NetworkPolicies get no delete stub — a known gap.

[^promise-yaml]: promise.yaml
[^pipeline-src]: Pipeline source
[^live-instance]: hello-world HTTPService request
