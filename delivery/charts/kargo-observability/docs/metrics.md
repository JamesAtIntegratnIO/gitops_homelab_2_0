# Metric contract

Every metric here comes from kube-state-metrics reading Kargo's CRDs through a
`CustomResourceState` config this chart supplies. No exporter, no polling, no state
of its own.

Names are prefixed with `metricPrefix` (default `kargo`). Changing that after
the fact orphans your dashboards and alerts, so pick once.

## Promotion

| Metric | Type | Labels |
|---|---|---|
| `kargo_promotion_phase` | StateSet | `namespace`, `promotion`, `stage`, `phase` |
| `kargo_promotion_created` | Gauge (unix time) | `namespace`, `promotion`, `stage` |

`phase` is one of `Pending`, `Running`, `Succeeded`, `Failed`, `Errored`,
`Aborted`. The distinction that matters operationally:

- **`Failed`** — the pipeline ran and did not get what it wanted. A pull request
  closed unmerged, or a merge step that timed out against a red check.
- **`Errored`** — a step could not run at all. A credential, a network fault, a
  parse path that no longer resolves.

They want different reactions, which is why they are separate alerts.

## Stage

| Metric | Type | Labels |
|---|---|---|
| `kargo_stage_condition` | Gauge (`1`/`0`/`-1`) | `namespace`, `stage`, `condition`, `reason` |
| `kargo_stage_last_promotion_phase` | StateSet | `namespace`, `stage`, `phase` |
| `kargo_stage_verification_phase` | StateSet | `namespace`, `stage`, `phase` |
| `kargo_stage_current_freight` | Info | `namespace`, `stage`, `freight` |

`kargo_stage_condition` carries the most signal in the whole set. `condition` is
`Ready`, `Healthy` or `Verified`, and `reason` explains a `0`. A stage sitting at
`Verified=0, reason=VerificationError` is a post-merge check that failed — which
nothing else in Kargo reports anywhere a human would see.

In a promotion chain that same condition is load-bearing: Kargo only makes
verified Freight available downstream, so an unverified stage silently holds
back everything behind it.

## Warehouse

| Metric | Type | Labels |
|---|---|---|
| `kargo_warehouse_condition` | Gauge (`1`/`0`/`-1`) | `namespace`, `warehouse`, `condition`, `reason` |
| `kargo_warehouse_last_freight` | Info | `namespace`, `warehouse`, `freight` |

A warehouse with `Healthy=0` has stopped discovering versions. Nothing breaks,
which is the problem — everything it feeds is quietly frozen at whatever it last
found.

## Freight

| Metric | Type | Labels |
|---|---|---|
| `kargo_freight_discovered` | Gauge (unix time) | `namespace`, `warehouse` |

Age since last discovery:

```promql
time() - max by (namespace, warehouse) (kargo_freight_discovered)
```

## Cardinality

This is a design constraint, not an afterthought. **Promotions accumulate** —
one object per attempt, forever, unless something prunes them.

Two decisions follow:

- Phase is a `StateSet`, not one `Info` series per promotion. `count by (stage,
  phase)` then costs the same whether there have been fifty promotions or fifty
  thousand.
- `kargo_freight_discovered` carries no freight name. There is one series per
  warehouse rather than one per freight object, which is the difference between
  bounded and unbounded growth.

`kargo_promotion_phase` and `kargo_promotion_created` do carry a `promotion`
label, and are therefore the two series that grow with history. If that becomes
a problem, prune old Promotion objects — the metric follows the object.
