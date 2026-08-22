# 2. Deterministic checks in CI; judgement in the cluster

- **Status:** accepted
- **Date:** 2026-08-22

## Context

The gate has two halves. One is deterministic: render both sides, diff the
targeting, diff the chart, validate schemas — same input, same answer, every
time. The other is judgement: read a red gate, decide whether it is mechanical,
propose a fix.

Both could live in CI. Both could live in the cluster. They could be split.

## Decision

**Deterministic checks run in CI. Judgement runs in the cluster.**

CI is where the checkout already is, and the gate needs a working tree at two
revisions. Its output is an exit code and a commit status, which is what branch
protection consumes.

The agent runs in-cluster, triggered by a Kargo `http` promotion step when the
pull request opens — event-driven, not scheduled.

## Consequences

**Good.** Porting to another CI system means writing a thin adapter — pass a
workspace in, turn an exit code into a status — and nothing else. The judgement
half, which is the larger and more opinionated body of code, is written once
and is CI-agnostic by construction.

The agent also gets something CI structurally cannot have: live cluster and
Prometheus access. It can say "the app this bump touches is already Degraded,
this failure predates your change", which is often the correct triage and is
invisible from a checkout.

Kargo hands it exact context — artifact, from, to, semver difference, merge
policy, pull request number — rather than the agent reconstructing it from a
diff.

**Bad.** Two runtimes. The agent needs an image, RBAC, a network policy, a
credential and a model key — real operational surface that a CI job would not
have. An operator who wants only the deterministic gate can run just that, but
an operator who wants triage takes on a workload.

The agent also needs a checkout of its own to author edits, which CI would have
had for free.

## Alternatives rejected

- **Everything in CI.** Simplest, no new cluster workload — but the triage
  logic becomes provider-native, and porting means rewriting the expensive half.
  Also gives up live-state correlation permanently.
- **Everything in the cluster,** including the gate. Removes the CI dependency
  and the cluster-inventory fixture, but makes a pull-request check depend on
  cluster reachability, and leaves contributors without a local way to run it.
