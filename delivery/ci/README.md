# CI adapters

The gate is a container. An adapter's whole job is:

1. Check out the pull request **and** its merge base — the gate compares two
   revisions, so a shallow single-revision checkout is not enough.
2. Run `gitops-gate` against both.
3. Turn the exit code into a commit status the branch protection rule can require.
4. **Post `report.md` to the pull request as a comment**, verbatim, before the
   step that fails the build.

That is roughly fifteen lines. Everything opinionated lives in the image, on
purpose — see [ADR 0002, *deterministic checks in CI, judgement in the cluster*](https://github.com/JamesAtIntegratnIO/bosun/blob/main/adr/0002-triage-in-cluster-not-ci.md).

## Step 4 is not optional, and it used to say the wrong thing

Until 2026-08-23 this list said *"publish `render-diff.json` where the agent
can fetch it"*. Nothing fetches that file. A triage agent reads the gate's
verdict by listing the pull request's **comments** and taking the most recent
one that begins with the gate's marker:

```
<!-- gitops-gate -->
```

Every adapter here implemented the contract as written, so none of them posted
a comment, so the verdict was reachable only by a human with the job summary
open. The gate was green-and-doing-nothing for its most important consumer,
and nothing failed — which is the failure mode this whole package exists to
make visible.

The marker is emitted by the **binary**, as the first line of `-report` output.
An adapter posts the file verbatim and is correct by construction; it does not
need to know the string. Do not prepend your own — you will get two.

Three details worth copying from [`github/`](github):

- **Post before you fail.** The step that turns exit `1` into a failed build
  must come *after* the comment, or the report is published only when the gate
  is green, which is precisely when nobody needs it.
- **Update in place.** Find the existing comment by marker and edit it. A gate
  that appends one comment per push is a gate people collapse and stop reading.
- **Post on green too.** A green report says "no change to what gets deployed",
  which is worth reading — and it means the publishing path runs on every pull
  request rather than for the first time during an incident.

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

## Never filter the required check by path

The trap that costs the most to discover late.

A required status check that has **never reported** blocks a pull request —
that is the mechanism by which the gate protects anything. But a workflow
skipped by a `paths:` filter never reports *at all*. It is not "passed" and it
is not "failed"; it sits at "expected" forever, and the pull request becomes
permanently unmergeable with no way to clear it short of editing the
protection rule.

So a docs-only change, or a change to an unrelated directory, is bricked by a
gate that was never meant to apply to it.

The fix is structural: the workflow runs on **every** pull request, a cheap
first job decides whether the expensive work is needed, and the aggregate job
reports either way.

```
scope         always runs; outputs relevant=true|false
render        if relevant
diff          if relevant
validate      if relevant
addons-gate   always runs; treats `skipped` as a pass
```

Two details make it work. The aggregate needs `if: always()`, or it inherits
the same never-reports problem from its own dependencies. And it must treat a
`skipped` dependency as a pass while still failing on `cancelled` — skipped
means "this change cannot affect what gets deployed", cancelled means the
answer is unknown.

## Enabling the gate on pull requests that already exist

Merging the workflow does **not** retroactively run it on open pull requests;
that needs a new event. Two things follow:

- Turning on branch protection blocks every open pull request immediately,
  because none of them has reported the required check yet. That is correct,
  and it is the protection.
- To give them a verdict, push to each branch — a rebase is the tidy way, and
  it makes the diff current at the same time. Workflows for `pull_request` are
  read from the merge commit, so a rebased branch picks up the gate from the
  base even though the branch predates it.

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
