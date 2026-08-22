# CI adapters

The gate is a container. An adapter's whole job is:

1. Check out the pull request **and** its merge base — the gate compares two
   revisions, so a shallow single-revision checkout is not enough.
2. Run `gitops-gate` against both.
3. Turn the exit code into a commit status the branch protection rule can require.
4. Publish `render-diff.json` where the agent can fetch it.

That is roughly fifteen lines. Everything opinionated lives in the image, on
purpose — see [`../adr/0002-triage-in-cluster-not-ci.md`](../adr/0002-triage-in-cluster-not-ci.md).

| Adapter | Status |
|---|---|
| [`github/`](github) | Reference implementation. Exercised in anger. |
| [`gitlab/`](gitlab) | Written against documentation. Unproven — see the README. |
| [`bitbucket/`](bitbucket) | Written against documentation. Unproven — see the README. |

## Exit codes

| Code | Status to report | Meaning |
|---|---|---|
| `0` | success | No blocking change. |
| `1` | failure | Targeting moved, or validation failed. |
| `2` | error | The gate could not run. Distinct from `1` on purpose: "this change is bad" and "the gate is broken" want different reactions, and conflating them trains people to ignore the check. |

## One aggregate status

Report **one** status, aggregating every job. Branch protection then names a
single check, and adding or splitting jobs later never requires editing the
protection rule — a rule change that is easy to forget and silently drops your
gate.

## Two things that bite

**The gate must be fast.** Kargo polls a pull request it is waiting to merge on
a fixed interval, so gate latency is added to every automated merge. Skip the
expensive chart-render diff when no chart version changed — that is the common
case and it roughly halves the wall clock.

**A push made by CI's own token usually does not trigger CI.** Most hosts
suppress it to prevent loops. If the agent pushes a fix with that token, the
gate never re-runs, the status stays red at its previous conclusion, and the
promotion waits against a result that will never change. Use a separate
credential for agent pushes.
