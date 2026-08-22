# Changelog

All notable changes to `gitops-gate`. Format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versioning is semver.

## [Unreleased]

### Added

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
