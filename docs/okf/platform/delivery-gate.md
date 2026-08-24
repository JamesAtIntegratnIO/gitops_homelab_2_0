---
type: Platform Capability
title: The delivery gate and Bosun — judging and repairing version-bump PRs
description: What stands between Kargo opening a pull request and the merge — the gitops-gate render-diff in CI publishing the required addons-gate check, and the in-cluster Bosun agent that deterministically repairs dropped-API-version reds, proposes policy-checked fixes, explains green gates, and escalates decisions as handoffs.
tags: [delivery, gate, bosun, kargo, ci, triage, llm]
status: stable
generated: { by: claude-code/claude-fable-5, at: 2026-08-24T02:10:00Z }
stale_after: 2026-09-24
sources:
  - id: workflow
    resource: ../../../.github/workflows/validate-addons.yaml
    title: the gate's CI adapter — five jobs, the report comment, the addons-gate check
  - id: gate-config
    resource: ../../../.gitops-gate.yaml
    title: what the gate renders, and against which cluster inventory
  - id: gate-pin
    resource: ../../../.github/gate-image.yaml
    title: the digest-pinned gate image, outside workflows/ so Kargo can bump it
  - id: addon
    resource: ../../../addons/cluster-roles/control-plane/addons/bosun/values.yaml
    title: the Bosun addon values — model endpoint, gate coupling, safety settings
  - id: hook
    resource: ../../../addons/cluster-roles/control-plane/addons/kargo-projects/values.yaml
    title: "the triage hook (`triage:` with `when: gated`) and every target's merge policy"
  - id: upstream
    resource: https://github.com/JamesAtIntegratnIO/bosun
    title: the Bosun repository — gate source, agent source, charts, evals, and docs/the-loop.md
---

# The delivery gate and Bosun

Kargo opens a pull request for every version its Warehouses discover
([kargo](kargo.md)). Two things then stand between that PR and the cluster.

**The gate** runs in CI on every pull request
([validate-addons.yaml][workflow]): it renders every bootstrap ApplicationSet
for every cluster in [the inventory][gate-config], at base and at head, diffs
what actually deploys, chart-diffs every moved version down to the field, and
schema-validates the result. It publishes a report comment (marker
`<!-- gitops-gate -->`, one per PR, updated in place) and the aggregate check
**`addons-gate`** — which a repository ruleset (`validating protection`,
created 2026-08-23) makes required on `main` with **no bypass actors**, so a
blocking finding is an unmergeable pull request, for Kargo too. Blocking
findings: a cluster-targeting change, a source/project/namespace change, an
apiVersion migration, and a CRD that stops serving a version **while
manifests in this repository still declare it** — that last count is the
gate's own scan, which is what lets a repair turn the red green.

**Bosun** runs in-cluster (the [bosun addon][addon], comments as
`bosun-mate[bot]`, status context `bosun`, model `qwen/qwen3.8-27b` on the
workstation via LM Studio). Kargo's promotion POSTs it every **gated** PR —
the [`triage:` hook][hook] with `when: gated`, so auto-merging patch bumps
never reach it. On a red gate it repairs dropped-served-version findings
deterministically (rewrites the declaring manifests to the version the
report names, **no model call**, verified by the gate's recount), applies
model-proposed scalar fixes behind a deny-list/scope/corroboration policy,
and escalates everything else as a handoff naming the file, key and decision.
On a green gate it explains what the bump actually changed and flags renders
that still warrant eyes. It never fails a check, never closes a PR, and its
deny-list — enforced in code, plus a GitHub App with **no** `workflows`
permission — keeps it from ever touching CI, [the gate's pin][gate-pin], or
the merge policy.

Deep-dives: [docs/delivery.md](../../delivery.md) (this repository's wiring),
and [Bosun's the-loop.md][upstream] (the full narrative). Merge policy and
verification live with [kargo](kargo.md).

## Sharp edges

- The gate validates the rendered *bootstrap ApplicationSets*; addon values
  are an opaque block inside them, so a bad value inside a chart's values
  renders green unless chart-diff surfaces its consequence.
- If the `render` job fails, the report comment is never posted and Bosun
  reports a red gate with no explanation — the gate-is-broken case, exit
  code 2, distinct from a bad change on purpose.
- `gitops-gate`, `kargo-pipelines` and `bosun` are `autoMerge: never`: they
  judge everything else and their failure mode is silence. Bosun triages its
  own bumps; the deny-list holds even there.
