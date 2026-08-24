# Delivery — the gate, the agent, and the loop they close

How a version bump travels from "a new release exists" to "verified running on
the cluster", and what stands between those two points. This is the
consumer-side companion to
[Bosun's own end-to-end narrative](https://github.com/JamesAtIntegratnIO/bosun/blob/main/docs/the-loop.md);
that page tells the story, this one names the wiring in *this* repository.

[docs/kargo.md](kargo.md) covers the promotion machinery in depth. This page
is about what judges and repairs the pull requests it opens.

## The pieces, and where they live here

| Piece | Where | What it does |
|---|---|---|
| The gate (CI) | [.github/workflows/validate-addons.yaml](../.github/workflows/validate-addons.yaml) | Renders every bootstrap ApplicationSet at base and head, diffs what actually deploys, chart-diffs every moved version, schema-validates. Publishes the report comment and the **`addons-gate`** check |
| Gate configuration | [.gitops-gate.yaml](../.gitops-gate.yaml), [.gitops-gate/clusters.yaml](../.gitops-gate/clusters.yaml) | The repo layout the gate binary knows nothing about, and the cluster inventory it expands ApplicationSets against |
| Gate image pin | [.github/gate-image.yaml](../.github/gate-image.yaml) | Digest-pinned, **outside** `workflows/` so Kargo's token can bump it — and inside Bosun's deny-list so the agent can never change which gate judges it |
| Bosun (in-cluster) | [bosun addon](../addons/cluster-roles/control-plane/addons/bosun/values.yaml) | The agent: repairs, fixes, explains, escalates. Comments as `bosun-mate[bot]`, status context `bosun` |
| The triage hook | `triage:` in [kargo-projects/values.yaml](../addons/cluster-roles/control-plane/addons/kargo-projects/values.yaml) | Kargo POSTs every **gated** promotion's pull request to Bosun — only PRs the merge policy will not auto-merge |
| Branch protection | repository ruleset `validating protection` | `addons-gate` is a required check on `main`, no bypass actors. A blocking finding is an unmergeable pull request — for Kargo too |

## The walk

1. **Kargo opens the PR** (see [kargo.md](kargo.md)). The diff is one pinned
   line. The merge policy decides whether anyone is asked: `patch` bumps of
   trusted charts self-merge once the gate is green; held PRs are exactly the
   ones the triage hook also hands to Bosun.

2. **`validate-addons.yaml` runs on every PR** — deliberately no `paths`
   filter, because a required check that never reports blocks the PR forever;
   a cheap `scope` job answers "does this change what gets deployed?" instead.
   The `targets-diff` job posts the report comment (marker `<!-- gitops-gate -->`,
   one comment per PR, updated in place) and the aggregate **`addons-gate`**
   job is the single check the ruleset requires. Exit codes distinguish "this
   change is bad" from "the gate is broken"; the report publishes *before* the
   verdict, because a report only published when green is only published when
   nobody needs it.

3. **Bosun reads the verdict.** Its `bosun` commit status is `pending` while
   it works. On a red gate: a dropped-served-version finding whose consumers
   the report names gets a **deterministic migration pushed to the PR branch —
   no model involved** — and the re-run gate recounts the consumers to verify
   it; a red the render proves gets a proposed scalar fix applied behind the
   deny-list, scope and corroboration checks; everything else gets a handoff
   comment (`needs-human` label) naming the file, the key and the decision.
   On a green gate it explains what the bump actually changed — grounded in
   the render diff and upstream release notes — and flags **Worth a look
   before merging** when a green render still warrants eyes. It never fails a
   check, never closes a PR, never touches the cluster.

4. **Merge and reconcile.** Kargo merges what policy allows; a human merges
   the rest — and nothing blocking merges at all, because the ruleset has no
   bypass. ArgoCD reconciles main.

5. **Verify.** The promotion's `verify.apps` AnalysisRun asks Prometheus
   whether the named Applications are Synced and Healthy — and in a promotion
   chain, that answer is what unlocks the next stage.

## Traps specific to this repository

- **The gate renders the bootstrap ApplicationSets, not each addon's chart
  output**, except where chart-diff kicks in (any row whose helm version
  moved). Schema validation of an addon's *values* is therefore shallow — the
  values are an opaque block inside the ApplicationSet.
- **`vcluster-coredns-config` and `gateway-api-crds-vcluster` match no
  cluster** and appear under "Not covered" in every report. Known; not an
  error.
- **The gate's own pin is `autoMerge: never`** — it judges every other
  promotion, and a gate that passes everything is Healthy by every signal
  there is. Read its bumps.
- **Bosun triages its own bump** (`bosun` is `never`, so its upgrade PRs are
  gated). The deny-list keeps it from touching the gate, CI or the merge
  policy even there.

## Verifying a change to any of this

```bash
# the gate, exactly as CI runs it (image pin from .github/gate-image.yaml)
docker run --rm -v "$PWD:/repo" -w /repo <gate-image> render -repo . -out targets.json

# who Bosun is and what it may touch
kubectl -n bosun get deploy bosun-bosun -o yaml | grep -A2 ALLOW_PATHS

# what the last gated PRs looked like
gh pr list --state all --limit 10
```

A change is verified the way everything here is: on `main`, reconciled, and
the thing it was supposed to fix stopped happening — for this system that
means a real promotion judged, repaired or escalated correctly on a live PR.
