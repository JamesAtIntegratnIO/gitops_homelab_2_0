---
type: Kratix Promise
title: external-secret (PlatformExternalSecret)
description: Leaf promise that renders external-secrets.io ExternalSecret resources bound to the 1Password ClusterSecretStore.
tags: [kratix, promise, external-secrets, 1password]
status: stable
generated: { by: claude-code/claude-fable-5, at: 2026-08-20T23:40:00Z }
stale_after: 2027-02-20
sources:
  - id: promise-yaml
    resource: ../../../promises/external-secret/promise.yaml
    title: promise.yaml
  - id: pipeline-src
    resource: ../../../promises/external-secret/workflows/resource/configure/main.go
    title: Pipeline source
---

# API

`platform.integratn.tech/v1alpha1` **PlatformExternalSecret** (shortname
`pes`; plural `externalsecrets` — deliberately distinct in kind from the
upstream `external-secrets.io` ExternalSecret). Required: `namespace` and at
least one `secrets[]` entry (`onePasswordItem` + `keys[]` of
`{secretKey, property}`). Defaults: store `onepassword-store`
(ClusterSecretStore); per-entry secret name defaults to
`{appName}-{onePasswordItem}`.[^promise-yaml]

# Behavior

Emits one `external-secrets.io/v1beta1 ExternalSecret` per entry into a single
multi-doc file, each labeled `kratix.io/promise-name: {ownerPromise}` for
attribution. Delete writes one stub per secret. This is the promise-shaped
face of the platform's [no-secrets-in-git rule](/platform/secret-management.md):
requesters name 1Password items and properties; values only ever materialize
in-cluster via ESO.[^pipeline-src]

Consumed by [http-service](/promises/http-service.md). Uses
`_shared/kratixutil`.

[^promise-yaml]: promise.yaml
[^pipeline-src]: Pipeline source
