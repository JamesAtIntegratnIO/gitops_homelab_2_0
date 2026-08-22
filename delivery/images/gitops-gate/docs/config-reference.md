# `.gitops-gate.yaml`

The gate binary knows nothing about any particular repository. This file is the
whole of that knowledge, and it lives at the root of the repository being gated.

```yaml
clusters: .gitops-gate/clusters.yaml
valuesRef: values
concurrency: 8

sources:
  # Committed YAML -- Applications and ApplicationSets alike.
  - name: appsets
    type: manifests
    paths: ["clusters/*/appsets/*.yaml", "apps/**/*.yaml"]

  # An app-of-apps ApplicationSet: the gitops-bridge shape.
  - name: addons
    type: argocd-bootstrap
    path: bootstrap/addons.yaml

  # A chart, rendered once per cluster when its values depend on cluster
  # metadata.
  - name: platform
    type: helm
    chart: charts/platform
    valueFiles:
      - "envs/{{metadata.labels.environment}}/values.yaml"
    selector:
      matchLabels: {cluster_role: control-plane}
    scope: cluster        # or fleet (default)

  - name: overlays
    type: kustomize
    path: overlays/production

clustersExport:
  ignoreKeys: [platform.example.com/reconcile-at]
  knownAbsentLabels: [aws_cluster_name]

validate:
  enabled: true
  ignoreMissingSchemas: true
```

## `sources`

A repository is rarely one shape. After a few years it is ApplicationSets
committed as YAML, *plus* a chart that renders more of them, *plus* an overlay
somebody added. So this is a list of strategies, not a mode.

| `type` | Needs | Reads |
|---|---|---|
| `manifests` | `paths` (globs) | committed `Application` and `ApplicationSet` YAML |
| `argocd-bootstrap` | `path` | an app-of-apps ApplicationSet, following it to whatever it points at |
| `helm` | `chart`, optional `valueFiles` | a rendered chart |
| `kustomize` | `path` | `kustomize build`, falling back to `kubectl kustomize` |
| `rendered` | `paths` (globs) | manifests already rendered into git — diffed at resource level. See [rendered-manifests.md](rendered-manifests.md) |

`chart` and `valueFiles` may contain `{{metadata.labels.x}}` and
`{{metadata.annotations.y}}`, resolved per cluster — which is how a
per-environment values layout is expressed without enumerating every
combination. A value file whose placeholders do not resolve for a given cluster
is simply not that cluster's file, matching ArgoCD's `ignoreMissingValueFiles`.

`selector.matchLabels` limits which clusters a source renders for. `argocd`
scopes a source to one ArgoCD instance in a fleet that runs several.

### `scope`

Only meaningful for a source rendered per cluster.

- **`fleet`** (default) — ApplicationSets expand against the whole inventory.
  Correct for hub-and-spoke, where one ArgoCD holds every cluster and an
  ApplicationSet rendered under one cluster's values can still generate
  Applications for others.
- **`cluster`** — they expand only against the cluster they were rendered for.
  Correct where each cluster runs its own ArgoCD and only ever sees itself.

Getting this wrong is quiet rather than loud, which is why it is explicit:
under `fleet`, a chart rendered per cluster yields the same ApplicationSet name
several times with different contents, and whichever arrives first wins.

### `argocd-bootstrap` follows what it finds

The bootstrap's source path is resolved the way ArgoCD resolves it: a directory
containing `Chart.yaml` is rendered as a chart, anything else is read as a
directory of manifests. The canonical gitops-bridge bootstrap is the second
kind — it points at a directory and applies every ApplicationSet YAML in it
with `directory.recurse: true`.

Both the singular `source:` and the multi-source `sources:` template forms are
read. gitops-bridge uses the singular.

## `bootstraps` (older form)

```yaml
bootstraps:
  - {name: control-plane, path: bootstrap/addons.yaml}
```

Exactly equivalent to one `type: argocd-bootstrap` source each, and still
supported.

## `concurrency`

Parallel renders, default 8. Fleets are the reason: fifty clusters is fifty
chart renders per revision, and serial execution turns a ninety-second gate
into something people route around.

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
