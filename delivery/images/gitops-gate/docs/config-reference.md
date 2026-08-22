# `.gitops-gate.yaml`

The gate binary knows nothing about any particular repository. This file is the
whole of that knowledge, and it lives at the root of the repository being gated.

```yaml
clusters: .gitops-gate/clusters.yaml
valuesRef: values

bootstraps:
  - name: control-plane
    path: bootstrap/addons-control-plane.yaml
  - name: tenant
    path: bootstrap/addons-tenant.yaml

clustersExport:
  ignoreKeys:
    - platform.example.com/reconcile-at

validate:
  enabled: true
  ignoreMissingSchemas: true
  schemaLocations:
    - default
    - 'https://raw.githubusercontent.com/datreeio/CRDs-catalog/main/{{.Group}}/{{.ResourceKind}}_{{.ResourceAPIVersion}}.json'
  skipKinds: []
```

## `clusters`

Path to the cluster inventory, relative to the repository root. Generate it with
`gitops-gate clusters export`.

## `valuesRef`

The `ref:` name your bootstrap ApplicationSet gives its values source. Multi-source
Applications refer to it as `$values/…` in `valueFiles`, and the gate has to strip
that prefix to find the file on disk. Defaults to `values`.

## `bootstraps`

The ApplicationSets that generate the Applications that render the ApplicationSets
that generate everything else. Two levels is the usual "app of apps of addons"
shape; the gate walks both.

`name` is cosmetic, used in output. It defaults to the file's base name.

## `clustersExport.ignoreKeys`

Labels and annotations to drop from the exported inventory because they churn
without affecting any selector or template — a resync timestamp, a content hash.
A trailing `*` matches by prefix.

This matters more than it looks. `clusters export -check` compares a fresh export
against the checked-in file; if a timestamp annotation is included, that check
fails every single time, and a check that always fails gets switched off.

## `validate`

`ignoreMissingSchemas` is effectively mandatory rather than a convenience. CRDs
outside the large projects appear in no published schema catalogue, and without
this one unknown kind fails a run that had nothing wrong with it.

Be clear-eyed about the cost: those kinds are then **not validated at all**. The
gate reports how many kinds it skipped so the gap is visible rather than assumed
away.

## The cluster inventory

```yaml
generatedAt: "2026-08-22T12:00:00Z"
clusters:
  - name: hub
    server: https://kubernetes.default.svc
    labels:
      argocd.argoproj.io/secret-type: cluster
      cluster_role: control-plane
      environment: production
    annotations:
      addons_repo_path: charts/application-sets
```

This is the gate's weakest joint, and it is worth being blunt about why.

Generators resolve selectors against **live** cluster labels. CI has no cluster
access, so the inventory is a checked-in snapshot. If a cluster's labels change,
or a cluster is added, the gate keeps answering confidently and wrongly — it will
report "no targeting change" for a change that does move targeting, because it is
comparing against a world that no longer exists.

There is no way to detect that from CI. The mitigation has to run somewhere with
cluster access:

```bash
gitops-gate clusters export -out .gitops-gate/clusters.yaml -check
```

Exit non-zero means the snapshot has drifted. Wire it into whatever already runs
against your cluster — a scheduled job, an operator's health command — and treat
a drifted inventory as a broken gate rather than a stale file.
