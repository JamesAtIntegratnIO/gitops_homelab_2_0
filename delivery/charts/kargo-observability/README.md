# kargo-observability

Kargo's own state as Prometheus metrics, plus alerts and a dashboard.

Kargo exposes controller-runtime counters and nothing else — no promotion,
freight, stage or verification state. So a promotion that fails, a verification
that errors, or a target that has been broken since the day it was written sits
in the API and appears nowhere. This chart closes that gap without an exporter,
by configuring kube-state-metrics' `CustomResourceState` over Kargo's CRDs.

> **Status: not yet implemented.** This README is the contract.

## What it ships

- **Metrics** — promotion phase, stage last-promotion and verification phase,
  warehouse freight age, per-object creation timestamps. Phase is modelled as a
  `StateSet` rather than a per-object `Info`, so `count by (stage, phase)` works
  without one series per promotion name. Promotions accumulate; cardinality is
  a design constraint, not an afterthought.
- **Alerts** — promotion errored, promotion failed, promotion running long,
  verification failed, warehouse stale, stage behind. Plus `KargoMetricsMissing`,
  an `absent()` check on the metrics themselves: kube-state-metrics is usually
  a tracked artifact in its own right, and a subchart bump that invalidates the
  config would otherwise silence every other alert here.
- **A dashboard** — promotions by phase, failures with age, time-to-merge,
  stale stages, verification pass rate, and how much is parked on a human.

## Requirements

kube-state-metrics, with `customResourceState.enabled` and the matching
`rbac.extraRules`. Both are set by this chart; the host chart must allow them
to be passed through.

## Reference

- [`docs/metrics.md`](docs/metrics.md) — the metric contract: names, labels, cardinality
- [`docs/alerts.md`](docs/alerts.md) — every rule, what it means, how to tune it
