---
type: Platform Capability
title: The delivery gate and Bosun — judging and repairing version-bump PRs
description: What stands between Kargo opening a pull request and the merge — the Bosun agent running the gitops-gate render-diff in-cluster against the live ArgoCD inventory, publishing the required addons-gate check itself, then deterministically repairing dropped-API-version reds, proposing policy-checked fixes, explaining green gates, and escalating decisions as handoffs.
tags: [delivery, gate, bosun, kargo, triage, llm]
status: stable
generated: { by: claude-code/claude-opus-5, at: 2026-08-25T04:40:00Z }
stale_after: 2026-09-25
sources:
  - id: gate-config
    resource: ../../../.gitops-gate.yaml
    title: what the gate renders — no cluster inventory, it is read live
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

**The gate** runs *inside the cluster*, as the Bosun agent itself
(`gate.mode: cluster`, the chart default since bosun 0.16.0). It polls the open
pull requests every 30s and, for each, renders every bootstrap ApplicationSet
([what to render][gate-config]) for every cluster in the **live** ArgoCD
inventory — read from the cluster Secrets on every run, so there is no
checked-in snapshot to go stale — at base and at head, diffs what actually
deploys, chart-diffs every moved version down to the field, and schema-validates
the result. It publishes a report comment (marker
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
permission — keeps it from ever touching CI or the merge policy. It cannot
change which gate judges it either, though the mechanism moved: the gate is
now the agent's own image, pinned by `bosun.defaultVersion`, and that key sits
in `addons/` which the agent *may* edit — so the protection there is
`autoMerge: never` and a human reading the bump, not a path deny.

Deep-dives: [docs/delivery.md](../../delivery.md) (this repository's wiring),
and [Bosun's the-loop.md][upstream] (the full narrative). Merge policy and
verification live with [kargo](kargo.md).

## Sharp edges

- The gate validates the rendered *bootstrap ApplicationSets*; addon values
  are an opaque block inside them, so a bad value inside a chart's values
  renders green unless chart-diff surfaces its consequence.
- **A bump that stops declaring a values key takes the setting with it, and
  helm does not complain.** The gate finds these from bosun 0.17.0 and blocks:
  the chart's declared surface is read at both versions (its own `values.yaml`,
  plus a helm-docs README table when there is one) and anything this repository
  SETS that the new version no longer declares is reported under *Settings this
  bump stops reading*. Measured on kyverno 3.2.8 → 3.9.0: **48 of the 77 values
  we set**, six of them keys Kargo rewrites on every promotion — and that bump
  gated green before the check existed. If a dropped key is in the
  `kargo-projects` target list, the pin becomes silently useless; fix both.
- A clean no-op gets the `addons-gate` status and **no report comment** —
  the comment is posted only when there is something to read. Absence of a
  comment is not absence of a verdict; read the status.
- **One report comment per pull request, rewritten in place**, with earlier
  verdicts kept in a collapsed history. A repaired pull request shows the green
  verdict and, under it, the red it used to be.
- If the render fails, `addons-gate` goes to `error`, not `failure` — "the
  gate is broken" versus "this change is bad", deliberately distinguishable
  and worth a page. A required check that cannot report needs a human bypass
  to exist *before* it is needed.
- `clustersExport.knownAbsentLabels` in `.gitops-gate.yaml` is read by the
  **render**, despite the key's name suggesting it belongs to the deleted
  export path. Deleting it exits the gate 2 on `aws_cluster_name` — an
  `error` on every PR, repo-wide, while looking like tidying up.
- `kargo-pipelines` and `bosun` are `autoMerge: never`: they judge everything
  else and their failure mode is silence. Bosun triages its own bumps; the
  deny-list holds even there, and its own version pin is guarded by the
  never-merge policy rather than by a path rule.
