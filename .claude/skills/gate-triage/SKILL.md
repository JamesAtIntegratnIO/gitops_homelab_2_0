---
name: gate-triage
description: Read and act on the `addons-gate` check and Bosun's report on a pull request in this repo. Use when a PR is blocked or red, when the gate posts "could not render / NOT covered", when a PR has no gate verdict at all, when Bosun escalates or proposes a repair, or when deciding whether a green gate actually proves anything.
---

# The gate and Bosun

Since **2026-08-25 the gate is not CI.** Bosun runs in-cluster (`bosun`
namespace, `GATE_MODE=cluster`), polls open PRs every 30s, renders base and head
against the **live** ArgoCD inventory, and posts `addons-gate` itself as
`bosun-mate[bot]` (GitHub App id `4694792`). Branch protection was never
changed — same check name, different reporter. Consumer-side narrative:
[docs/delivery.md](../../../docs/delivery.md).

```bash
source .claude/skills/homelab-toolchain/scripts/tools.sh
k -n bosun logs deploy/bosun --tail=200
k -n bosun logs deploy/bosun | grep outbound     # every egress, query string redacted
k -n bosun get deploy bosun -o jsonpath='{.spec.template.spec.containers[0].image}'
```

## Reading a verdict

- Every report leads with a headline (`## 🔴 Blocking — …` / `## ✅ …`) and a
  machine-readable breakdown comment
  (`<!-- gitops-gate:blockers targeting=… valuesDropped=… -->`). Post it
  verbatim; never strip either.
- **One report comment per PR, rewritten in place**, earlier verdicts kept in a
  collapsed history.
- **A clean no-op posts the status and no comment.** Absence of a comment is not
  absence of a verdict.
- `(attempt N of M)` appears only on an actual retry.

## "Not covered" is two different things — read the reason

- `matched no cluster` — benign and permanent.
- anything naming **the filesystem, the network or a missing binary** — the gate
  is *degraded*, not the repo explained. The verdict still goes green, so the
  symptom is reduced coverage on a version bump, in a section people skim.
  Nothing alerts on this distinction. The precedent: `chartdiff.go` wrote to
  `/tmp` under `readOnlyRootFilesystem`, which silenced the chart-diff checks —
  and the CRD-dropped-version check *is* chart-diff. The same
  external-secrets 2.9.0 bump gated `success` with the check silenced and
  `failure` with it working, against 27 manifests that would have broken at
  apply. Fixed by an emptyDir at `/tmp` in chart 0.16.1.

**A green gate proves only what the gate could actually run.**

## A PR with no verdict at all

The sweep skips any head whose `addons-gate` already reads success/failure — *a
verdict that already stands is not re-litigated* — and `CheckStatus` reads
check-runs first. So a stale check-run from the old CI wins and Bosun never
posts. No status you can post dislodges it; check-runs cannot be deleted via the
API. **Only a new head commit clears it**: `gh pr update-branch`, a rebase, or
close-and-re-promote. GitHub also reuses check runs per head SHA, so re-opening
a PR on an unchanged branch does not re-run the gate — push an empty commit to
force a fresh revision when replaying.

The gate derives its sources from the Applications and ApplicationSets ArgoCD
serves (bosun ADR 0012), so a PR whose head predates the config file still
renders — derivation supplies the sources, and the Terraform-applied bootstraps
are found by scanning the head for their manifests. The old "no
.gitops-gate.yaml at the head revision" `error` is history.

## When the gate blocks

The blocking classes and where the fix lives:

| Verdict names | Fix |
|---|---|
| targeting change | this repo — see the `addon-change` skill's target matrix |
| "settings this bump stops reading" | this repo — delete/rename those keys, and check whether any is in the Kargo target list (a tracked key that no longer exists makes the pin silently useless) |
| CRD version removed while consumers exist | migrate the manifests; Bosun can do this deterministically |
| removed ClusterRoleBinding orphaning a ServiceAccount | re-bind or remove the SA |

A red whose every cause is chart-rendered — nothing in the repo declares it —
is escalated with no model call. A red the rendered diff proves is repaired
deterministically (27 manifests in 29s on the ESO case). The model never writes:
it returns a structured proposal and the process applies it behind an allowlist,
a from-value check and a corroboration check.

**Escalations become gate rules.** The loop worth preserving: the model catches
a class by heuristic, then it is crystallised into a deterministic gate rule,
then into a deterministic repair. The system gets *more* deterministic over
time. When you fix something the gate missed, ask what rule it becomes.

## Do not

- **Delete `.bosun.yaml`.** Its `roots:` list is the one fact derivation
  cannot supply: a bootstrap ApplicationSet this PR *introduces* has no live
  object to be found by, so nothing would render it at all — and the named
  files are what save the head scan from missing an existing one.
- **Rebuild an egress allow-list for Bosun.** Egress is deliberately open since
  0.15.0 (`toFQDNs: "*"` on 443, every request logged); naming hosts was a
  full-time job manufacturing exactly the silent failure Bosun exists to end.
  Forbid a host by name with `triage.egressDeny`.
- **Rebuild `@bosun` comment commands or a metrics scoreboard.** Both dropped by
  James on purpose.

## Working in the bosun repo

`~/Projects/bosun` (plus `-b`…`-e` worktrees), PolyForm Internal Use 1.0.0.
Releases are automatic: bump `charts/bosun` `appVersion`, merge, CI tags and
publishes. Never cut a tag by hand.

Two traps that have cost real time there:

- **A stacked PR merges into its parent branch, not `main`** — and if the parent
  already merged, GitHub marks the child "merged" while its commit never reaches
  `main`. Chart 0.13.0 never released that way. The tell is
  `git log --oneline origin/main..origin/<branch>` returning commits on a PR
  that says merged. Merge a stack bottom-up, re-targeting each remaining PR to
  `main` after the one below lands.
- **Merging with `gh` does not update your local `origin/main`.** Always
  `git fetch` before `git checkout -B <branch> origin/main`.

And the rule that came out of drip-feeding fixes one release at a time: **when a
bug's shape is "reality differs from my fixtures", do not fix the instance —
enumerate the real inputs and run them all.** Pointing the upstream resolver at
every artifact in the live Kargo target list at once took resolution from 17/41
to 34/41 and surfaced five distinct failure classes in one pass. Apply it to
every feature in the change, not just the one being complained about.
