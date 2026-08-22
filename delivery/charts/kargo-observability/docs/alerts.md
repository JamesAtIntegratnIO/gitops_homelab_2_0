# Alerts

Every rule is individually toggleable and its `for` and `severity` are values,
so you can tune without forking. All carry `component: kargo`.

| Alert | Default | Fires when |
|---|---|---|
| `KargoPromotionErrored` | warning, 15m | A step could not run — credential, network, unresolvable path. |
| `KargoPromotionFailed` | warning, 15m | The pipeline ran and did not get what it wanted. |
| `KargoPromotionRunningLong` | warning, 24h | A promotion has been `Running` for a day. |
| `KargoVerificationFailed` | warning, 5m | Post-promotion verification reported `Failed` or `Error`. |
| `KargoStageNotVerified` | warning, 30m | A stage sits at `Verified=False`. |
| `KargoWarehouseUnhealthy` | warning, 30m | A warehouse has stopped discovering versions. |
| `KargoStageBehind` | info, 7d | A stage exists but its project has never promoted successfully. |
| `KargoMetricsMissing` | warning, 30m | The metrics themselves have gone. |

## The three worth understanding before you tune them

### `KargoPromotionRunningLong` is deliberately vague

A long-`Running` promotion is ambiguous *by construction*. It is either parked on
a human — normal, and can legitimately last days — or wedged on a merge that will
never happen. **No metric distinguishes them**, because from Kargo's side both
look like a step that has not returned.

So this fires late (24h) and its description names the two possibilities rather
than asserting one. Tightening the `for` will page you for pull requests that are
simply waiting for review. If that trade is wrong for you, the honest fix is
shortening the merge step's timeout so a genuinely stuck promotion becomes
`Failed` — which is unambiguous — rather than making this rule more sensitive.

### `KargoMetricsMissing` is the one that keeps the rest honest

Every other rule here fails *open*. If the metrics stop arriving, nothing fires,
and a silent board is indistinguishable from a healthy pipeline.

The realistic way that happens: kube-state-metrics is usually itself a tracked
artifact, so an automated bump upgrades it, the new version reads the
`CustomResourceState` config differently or not at all, and every Kargo alert
goes quiet without a single thing appearing broken.

Leave this one on. It is the only rule that notices.

### `KargoStageBehind` catches the failure with no symptoms

A target whose parse path no longer resolves — a file reordered, a key moved —
errors on every promotion and produces *nothing else*. No degraded app, no failed
sync, no unhealthy workload. The pin simply stops moving, and stays wrong until
someone happens to look.

Seven days at `info` is intentionally slow and quiet: it is a "this has been
broken for a week" signal, not a page.

## Routing

Rules carry `severity` and `component: kargo`; add anything else your routing
needs via `alerts.additionalLabels`. Set `alerts.kargoURL` and every alert gains
a `kargo_url` annotation pointing at the stage, so whoever is woken up can get
to the pipeline without hunting for it.
