# Changelog

All notable changes to `gitops-gate`. Format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versioning is semver.

## [Unreleased]

### Added

- `ReportMarker` — `diff -report` now leads its output with
  `<!-- gitops-gate -->`, and a test asserts it on both a blocking report and a
  green one.

  This is a contract, not decoration: a triage agent finds the gate's verdict
  by searching a pull request's comments for that string. It previously lived
  in one shell script in the local proving ground and in **no CI adapter at
  all**, so a report published by CI was one no agent could locate. Emitting it
  from the binary makes any adapter that posts the report verbatim correct by
  construction, and makes the same bug unavailable to the GitLab and Bitbucket
  adapters.

  Adapters must no longer prepend it themselves or they will publish two.

### Fixed

- Chart diff no longer reports a chart's whole resource set as removed and
  re-added when the two versions disagree about stamping
  `metadata.namespace`. Whether a chart sets it varies between versions of the
  same chart -- podinfo omits it at 6.7.0 and sets it at 6.14.1 -- and a
  namespaced resource without it lands in the Application's destination
  namespace anyway, so that is now its identity. On a real 6.7.0 -> 6.14.1
  bump the report went from 5 added + 5 removed to the 2 resources that
  actually changed.
- Helm **test** hooks are excluded. They are never applied by a sync, and they
  are the one place charts routinely generate a random name, so all three of
  podinfo's test pods appeared as added AND removed on every render. Other
  hooks are applied and are still reported.

### Added

- **chart-diff** (`diff -repo <path>`) — every chart whose version moved is
  rendered at BOTH versions, with that Application's own value files and
  inline `valuesObject`, and the resources are compared. Turns "cert-manager
  moved to v1.22.0" into "adds two RBAC objects, changes six CRDs and three
  Deployments". Helm's per-object version stamps are excluded from the
  comparison: hashing them reported 101 of 105 resources as changed on one
  bump, burying the 15 that had.

- **`type: rendered`** — reads manifests already committed to git and diffs
  them at RESOURCE level: added, removed, changed, and `apiVersion` changed
  called out separately as the one that blocks. Supports ArgoCD's source
  hydrator output, Kargo's rendered promotion branches, or any CI job that
  commits its render. See docs/rendered-manifests.md.

- **Source model.** A repository's manifests are obtained through a list of
  sources -- `manifests`, `helm`, `kustomize`, `argocd-bootstrap` -- which can
  be combined. The previous version understood exactly one topology (an
  app-of-apps ApplicationSet rendering a chart) and was silently blind to every
  other, including committed ApplicationSets and plain Applications, which are
  the most common ArgoCD layouts there are.
- `argocd-bootstrap` resolves its source path the way ArgoCD does: a directory
  with `Chart.yaml` is a chart, anything else is read recursively as manifests.
  The canonical gitops-bridge bootstrap is the second kind.
- Both the singular `source:` and multi-source `sources:` Application template
  forms are read. gitops-bridge uses the singular.
- Plain `Application` manifests are read, with `destination.server` resolved
  against the inventory so they key the same way generated ones do.
- Concurrent rendering, and `argocd:` on sources and clusters for fleets
  running more than one ArgoCD.
- `scope: cluster | fleet` for per-cluster renders, because whether an
  ApplicationSet expands fleet-wide depends on hub-and-spoke versus
  per-cluster ArgoCD, and guessing is silent.
- Topology fixtures covering each shape, plus a 50-cluster fleet.

- `render` — expands both levels of the ApplicationSet hierarchy into the flat
  set of Applications a cluster would end up with, including the bootstrap
  Applications themselves.
- `diff` — compares two renders. Blocks on cluster-targeting changes and on a
  source changing underneath an unchanged Application; reports version changes
  without blocking.
- `diff` separates a brand-new addon (`introduced`, non-blocking) from an
  existing addon gaining or losing a cluster (`targeting`, blocking). Only the
  second is the leak; blocking on the first would make every new-addon pull
  request red for no reason and train people to override the check.
- `validate` — schema validation of every rendered stream via kubeconform.
- `clusters export` — regenerates the cluster inventory from live ArgoCD
  cluster Secrets, with `-check` for drift detection.
- Support for both ApplicationSet templating dialects, chosen from the
  ApplicationSet's own `goTemplate` field rather than guessed.
- Generators that cannot be expanded (git, matrix, list) produce an explicit
  "not covered" warning rather than silently reporting full coverage.
