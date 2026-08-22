# Promotion chains

A target with no `stages` is one Warehouse and one Stage: find a new version,
open a pull request, done. That is the right shape for a single environment and
stays the default.

`stages` turns that into an ordered chain where each step must prove itself
before the next one runs.

## What it looks like

```yaml
cert-manager:
  chart:
    repoURL: https://charts.jetstack.io
    name: cert-manager
  autoMerge: patch
  stages:
    - name: canary
      updates:
        - file: clusters/tenant/addons.yaml
          keys: [cert-manager.defaultVersion]
      verify:
        apps: [cert-manager-tenant]
      requiredSoakTime: 30m0s
    - name: prod
      updates:
        - file: clusters/hub/addons.yaml
          keys: [cert-manager.defaultVersion]
      verify:
        apps: [cert-manager-hub]
```

That renders two Stages:

| Stage | Freight source | Updates | Verifies |
|---|---|---|---|
| `cert-manager-canary` | `direct: true` | the tenant pin | the tenant app |
| `cert-manager` | `stages: [cert-manager-canary]`, soak 30m | the hub pin | the hub app |

## Why the gate is free

Kargo only makes Freight available to a downstream Stage once it has been
**verified** upstream. So `verify` on the canary plus auto-promotion on the
next stage already *is* "tenant first, hub only if the tenant came up healthy".
There is no orchestration code, nothing polls, and the chart keeps no state
of its own — a
canary that fails verification simply never offers its freight on.

`requiredSoakTime` adds "and it has to have been fine for a while", which
catches the failures that need traffic or a cron tick to show up. Write it on
the stage doing the soaking; the chart renders it where Kargo expects it, on
the downstream stage's `sources`.

## Two rules that are not obvious

**The last stage keeps the target's bare name.** `cert-manager-canary` and
`cert-manager`, not `cert-manager-prod`. This is deliberate: renaming the
terminal Stage discards its freight and verification history and makes ArgoCD
prune and recreate it. A repository adopting a chain keeps the object it
already had and gains a new upstream one.

**Each stage parses the first file of its own `updates`.** This is what makes a
chain correct rather than cosmetic. A stage decides whether to act by comparing
the Warehouse's version against what is currently pinned — so if the downstream
stage read the *canary's* file, it would see the version its upstream just
wrote, conclude there was nothing to do, and silently never promote. The chain
would look wired up and never move.

## A canary is only as good as its differences

Worth writing down wherever you deploy this, because the shape invites
over-trust.

A canary environment proves something only about the things it actually runs.
An artifact with no presence there gets **no staging at all** — it goes straight
to the terminal stage, exactly as it did before you built the chain. And a
smaller environment running a subset of workloads at a smaller scale can be
perfectly healthy while the same version breaks the real one.

State plainly which targets are chained and which are not. A chain that is
assumed to cover everything is worse than no chain, because it converts
"nobody checked" into "we thought it was checked".

## Per-stage merge policy

A stage may tighten the target's policy:

```yaml
stages:
  - name: canary
    autoMerge: minor      # let the canary move on its own
    ...
  - name: prod
    autoMerge: never      # a human decides when it reaches production
    ...
```

Omitted, a stage inherits the target's policy.
