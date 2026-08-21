---
type: Deployment Mechanism
title: Addon system (ApplicationSet factory)
description: How addons/charts/application-sets turns layered values files into one ApplicationSet per addon, and the merge/templating rules that govern it.
tags: [addons, argocd, applicationset, helm]
status: stable
generated: { by: claude-code/claude-fable-5, at: 2026-08-20T23:40:00Z }
stale_after: 2027-02-20
sources:
  - id: appset-chart
    resource: ../../../addons/charts/application-sets/templates/application-set.yaml
    title: application-sets chart template
  - id: bootstrap-appsets
    resource: ../../../terraform/cluster/bootstrap/addons-control-plane.yaml
    title: Bootstrap ApplicationSets
  - id: addons-doc
    resource: ../../addons.md
    title: docs/addons.md
---

# The factory pattern

Two Terraform-installed bootstrap ApplicationSets render the
`addons/charts/application-sets` Helm chart, which loops over its merged
values: **any top-level map with an `enabled` key is an addon**, and each
enabled one becomes its own ApplicationSet (`bootstrap-<addon>`), whose
clusters generator then produces one Application per matching cluster.[^appset-chart]

## Value layering (deep merge, later wins)

```
environments/{env}/addons/addons.yaml      # control-plane bootstrap only
environments/{env}/addons/common.yaml
cluster-roles/{role}/addons/addons.yaml
cluster-roles/{role}/addons/common.yaml
clusters/{name}/addons.yaml
+ the bootstrap valuesObject (appsetPrefix, repoURLGit)   # always wins
```

The **vcluster bootstrap deliberately omits the environment addons.yaml
layer** — a vcluster's addon inventory comes purely from the vcluster role +
per-cluster files. It also renders its ApplicationSets *onto the host*
(destination hardcoded to the-cluster); each generated Application then
targets the vcluster.[^bootstrap-appsets]

Merging is per-addon-key and deep: a cluster layer can add
`additionalResources`/`syncPolicy` to an addon whose `enabled: true` came
from the environment layer.

## Two-phase templating

Helm passes `{{ … }}` strings through untouched; the ApplicationSet
controller evaluates them per cluster (`goTemplate: true`,
`missingkey=error`). That's how one addon definition resolves per-cluster
values like `{{.metadata.annotations.cert_manager_namespace}}`. Consequence:
manifest addons **must** define `defaultVersion` (or `path`) or the
`addonVersion` label template errors out.

## Cluster targeting

Base generator always matches `argocd.argoproj.io/secret-type: cluster` +
`globalSelectors` from common.yaml (e.g. `environment: production`) +
addon-level `selector` matchExpressions (typically `cluster_role In […]` or
`enable_<addon> In [true]`). The labels live on the ArgoCD cluster Secrets —
the-cluster's from Terraform, each vcluster's from the
[cluster-registration promise](/promises/argocd-cluster-registration.md).

## Per-addon knobs (the schema)

`enabled`, `namespace`, `chartName`/`chartRepository`/`defaultVersion` (or
`type: manifest` + `manifestSource`/`path`), `project`, `selector`,
`valuesObject` (inlined into the Application — beats every values file),
`valueFiles` resolution via `valuesFolderName | chartName | addon key`,
`additionalResources` (extra git source; **silently ignored on
`type: manifest` addons**), `syncPolicy` (wholly *replaces* the global
default — addons must restate `syncOptions`), `ignoreDifferences`,
`environments[]` (merge generator for per-env chart versions),
`generatorValues`, `annotationsApp`.

## Sharp edges to remember

- Addon-level `syncPolicy` **deep-merges over** the default (changed 2026-08-21).
  A partial override — `automated: {}`, or a bare `syncOptions` list — keeps the
  default selfHeal/prune/retry instead of silently dropping them. Lists are still
  replaced wholesale, so an addon that sets `syncOptions` still replaces them.
  Before the change, 16 of 47 Applications had lost `selfHeal` this way.
- `additionalResources` on manifest-type addons is a no-op.
- Values folders contribute **only `values.yaml`** — any other manifest files
  placed there are inert (several exist; see [known issues](/cluster/known-issues.md)).
- `valuesObject` always wins over file layers.
- Addon keys with `_` are normalized to `-` in resource names.
- `addons/README.md` predates the current layout (documents `default/` and
  `clusters/in-cluster/` layers that no longer exist).

Full addon listing: [addon inventory](/addons/addon-inventory.md).

[^appset-chart]: application-sets chart template
[^bootstrap-appsets]: Bootstrap ApplicationSets
[^addons-doc]: docs/addons.md
