# Changelog

All notable changes to `gitops-gate`. Format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versioning is semver.

## [Unreleased]

### Added

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
