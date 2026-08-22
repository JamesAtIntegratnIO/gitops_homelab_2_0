# 3. Promotion chains are per-target stage lists, verified rather than timed

- **Status:** accepted
- **Date:** 2026-08-22

## Context

Kargo is built for multi-stage promotion, but a single-environment user has no
obvious use for it: there is one place for a version to land, so one Warehouse
and one Stage per artifact is the whole pipeline. That is how this started.

The gap it leaves is real. When the same chart runs in more than one place — a
tenant cluster and the platform cluster, a lab and production — a single pin
bump hits all of them at once. There is no first blast radius.

Meanwhile "canary" in most tooling means *time*: wait N minutes, then proceed.
Time is a poor proxy for health.

## Decision

A target declares an **ordered list of stages**. Each stage has its own
`updates` (which pins it moves) and its own `verify` (which apps must be healthy
afterwards). Downstream stages take `sources.stages` from the one before, with
`direct: false`.

Kargo then makes freight available downstream only once it is **verified**
upstream. Verification, not elapsed time, is the gate. `requiredSoakTime` is
available on top for cases where a problem takes a while to show up.

Crucially, each stage's "has this changed?" test reads **its own** first tracked
file, so a stage compares against the pin it is responsible for. Without that,
the second stage sees the first stage's already-updated pin and concludes there
is nothing to do.

## Consequences

**Good.** The gating comes free from Kargo — no orchestration code, no polling,
no state of our own. A stage that fails verification simply never makes its
freight available downstream. The topology is data: "vcluster then host",
"lab then prod", or a single stage, are the same chart with different values.

**Bad.** Each stage is a distinct Kargo object with its own freight and
verification history. Introducing a chain into an existing single-stage target
means the terminal stage should keep its original name, or it loses that
history and ArgoCD prunes and recreates it.

Chains also slow a bump down by design — a patch that used to land in one pull
request now lands in two, gated on a verification window in between.

**And a caveat worth stating loudly.** A canary is only as good as the thing
that is different about it. Verifying in a smaller cluster running a subset of
workloads proves less than it appears to. Any artifact with no presence in the
canary gets no staging at all, and the deployment must say so rather than
letting the chain imply coverage it does not have.

## Alternatives rejected

- **Time-based soak with no verification.** Simpler, and a worse signal.
  Available as an addition, not as the gate.
- **A separate Warehouse per stage.** Would decouple the stages entirely, and
  lose the thing that makes this work — one freight, traceable across stages.
