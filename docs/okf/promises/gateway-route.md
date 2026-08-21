---
type: Kratix Promise
title: gateway-route
description: Leaf promise that renders a Gateway API HTTPRoute (plus optional HTTP→HTTPS redirect) from a GatewayRoute ResourceRequest.
tags: [kratix, promise, gateway-api, httproute]
status: stable
generated: { by: claude-code/claude-fable-5, at: 2026-08-20T23:40:00Z }
stale_after: 2027-02-20
sources:
  - id: promise-yaml
    resource: ../../../promises/gateway-route/promise.yaml
    title: promise.yaml
  - id: pipeline-src
    resource: ../../../promises/gateway-route/workflows/resource/configure/main.go
    title: Pipeline source
---

# API

`platform.integratn.tech/v1alpha1` **GatewayRoute** (shortname `gwr`).
Required: `name`, `namespace`, `hostname`, `backendRef` (`name` + `port`).
Defaults: `path: /`, gateway `nginx-gateway/nginx-gateway`, listener
`https`, `httpRedirect: true`.[^promise-yaml]

# Behavior

Writes an HTTPRoute (`hostnames: [hostname]`, PathPrefix match → backendRef)
and, when `httpRedirect`, a `{name}-http-redirect` route on the `http`
listener issuing a 301 RequestRedirect to https. Both sync-wave 10. This is
the same two-route pattern used hand-written across the cluster (grafana,
argocd, etc. — see [networking](/platform/networking.md)).[^pipeline-src]

Consumed by [http-service](/promises/http-service.md). Uses
`_shared/kratixutil`. No live instances as of 2026-08-21 — the `hello-world-route`
example went with the hello-world HTTPService.

[^promise-yaml]: promise.yaml
[^pipeline-src]: Pipeline source
