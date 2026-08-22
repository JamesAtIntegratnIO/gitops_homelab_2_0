# kargo-pipelines

Renders Kargo `Project`, `Warehouse` and `Stage` objects from one declarative
target list, so "keep this pin current" is a few lines of values rather than a
hand-written promotion pipeline per artifact.

> **Status: not yet implemented.** Generalized from a working single-cluster
> chart. This README is the contract.

## What it adds over hand-written Stages

- **One target, N ordered stages.** Chain them with `sources.stages` so a bump
  lands in a canary environment, is *verified* there, and only then becomes
  available to the next stage. Kargo does the gating; the chart makes it
  declarative. Optional `requiredSoakTime` holds freight in a stage for a
  minimum duration before it moves on.
- **A merge policy per target.** `always` / `minor` / `patch` / `never`,
  evaluated against the semver difference, so a patch can merge itself while a
  major waits for a human.
- **Bounded retries.** Kargo's default `errorThreshold` is 1 — a single
  transient API error fails the whole promotion. This sets a real policy, and
  caps how long a step that is waiting on a check may retry.
- **A triage hook.** An optional `http` step that hands the promotion's context
  to an agent when the pull request opens.

## Constraints you inherit from Kargo

`yaml-update` rewrites the matched line in place and parses **only the first
YAML document** of a file. So a tracked key must already exist, address a
scalar, sit in the first document, and carry no trailing comment. Multi-document
files need the tracked resource first — see [`docs/targets.md`](docs/targets.md).

Kargo's admission webhook also defaults and canonicalizes fields on write. If
the chart does not emit those fields explicitly and in canonical form, ArgoCD
reads the difference as permanent drift and re-syncs forever.

## Reference

- [`docs/targets.md`](docs/targets.md) — the target schema and the `yaml-update` rules
- [`docs/chaining.md`](docs/chaining.md) — multi-stage chains and verification gating
