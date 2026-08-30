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
| The gate (in-cluster) | `gate.mode: cluster` in the [bosun addon](../addons/cluster-roles/control-plane/addons/bosun/values.yaml) | The agent *is* the gate. It polls open PRs, renders every bootstrap ApplicationSet at base and head against the **live** ArgoCD inventory, diffs what actually deploys, chart-diffs every moved version, schema-validates, and posts the **`addons-gate`** status itself |
| Gate configuration | [.bosun.yaml](../.bosun.yaml) | Only what cannot be derived: the two Terraform-applied bootstrap roots. Everything else the gate renders is derived live from the Applications and ApplicationSets ArgoCD serves (bosun ADR 0012), and validate policy lives in the bosun addon's values |
| Bosun (in-cluster) | [bosun addon](../addons/cluster-roles/control-plane/addons/bosun/values.yaml) | The agent: repairs, fixes, explains, escalates. Comments as `bosun-mate[bot]`, status context `bosun` |
| The triage hook | `triage:` in [kargo-projects/values.yaml](../addons/cluster-roles/control-plane/addons/kargo-projects/values.yaml) | Kargo POSTs every **gated** promotion's pull request to Bosun — only PRs the merge policy will not auto-merge |
| Branch protection | repository ruleset `validating protection` | `addons-gate` is a required check on `main`, no bypass actors. A blocking finding is an unmergeable pull request — for Kargo too |

## The walk

1. **Kargo opens the PR** (see [kargo.md](kargo.md)). The diff is one pinned
   line. The merge policy decides whether anyone is asked: `patch` bumps of
   trusted charts self-merge once the gate is green; held PRs are exactly the
   ones the triage hook also hands to Bosun.

2. **The agent gates every PR**, on a 30s sweep. No paths filter and no
   workflow: every open PR is rendered, so "no change to what gets deployed"
   is an answer rather than a skipped check. It posts `addons-gate` directly —
   `pending` while rendering, then `success`/`failure`, and `error` for "the
   gate is broken", which is deliberately distinct from "this change is bad"
   and should be treated as a page.

   The report comment (marker `<!-- gitops-gate -->`, stamped
   `<!-- gitops-gate:head <sha> -->` so a restarted pod stays quiet) is posted
   **only when there is something to read** — a change or a blocking finding.
   A clean no-op gets the status and no comment. The verdict no longer travels
   through the comment at all: the gate runs in-process, so triage receives it
   as a value and `gate.reportAuthor` has nothing to check.

   Because the gate polls rather than triggers, an already-open PR gets its
   verdict on the next sweep without being rebased.

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
- **A bump can stop reading a value you set, silently.** Helm ignores an
  unknown value rather than failing on it, so a chart that renames or removes a
  key takes the setting with it and renders identically. From bosun 0.17.2 the
  gate compares the chart's declared surface at both versions and blocks on the
  difference, under *Settings this bump stops reading*. kyverno 3.2.8 → 3.9.0
  drops **48 of the 77 values this repository sets** and gated green before the
  check existed. **Cross-check every dropped key against the
  [kargo-projects target list](../addons/cluster-roles/control-plane/addons/kargo-projects/values.yaml)**
  — six of kyverno's seven tracked keys are in that set, and a tracked key the
  chart no longer reads is a pin that updates forever and does nothing.
- **`vcluster-coredns-config` and `gateway-api-crds-vcluster` match no
  cluster** and appear under "Not covered" in every report. Known; not an
  error.
- **The gate ships inside Bosun now**, so its version is `bosun.defaultVersion`
  and there is no separate gate image to pin or bump. That target is gone.
  `bosun` remains `autoMerge: never`: it judges every other promotion, and a
  gate that passes everything is Healthy by every signal there is.
- **`clustersExport.knownAbsentLabels` is gone, and staying gone is right.**
  It only ever guarded a gate bug: `DoesNotExist` selector keys were collected
  into the presence-demand list, and `aws_cluster_name` appears in a selector
  only under `DoesNotExist`. Bosun fixed the collection (in by 0.27.0), so the
  workaround left with the file — and putting it back would mask the
  stale-inventory refusal it piggybacked on for every other selector.
- **Bosun triages its own bump** (`bosun` is `never`, so its upgrade PRs are
  gated). The deny-list keeps it from touching the gate, CI or the merge
  policy even there.

## Verifying a change to any of this

```bash
# the gate is in the cluster: watch it work
kubectl -n bosun logs deploy/bosun -f | grep 'gate:'

# prove the grant cluster mode buys is the one you meant
kubectl auth can-i list secrets -n argocd --as=system:serviceaccount:bosun:bosun

# render locally against a snapshot (the CLI still takes `clusters:`)
go run ./gate/cmd/gitops-gate render -repo . -out targets.json   # in the bosun repo

# who Bosun is and what it may touch
kubectl -n bosun get deploy bosun -o yaml | grep -A2 ALLOW_PATHS

# what the last gated PRs looked like
gh pr list --state all --limit 10
```

A change is verified the way everything here is: on `main`, reconciled, and
the thing it was supposed to fix stopped happening — for this system that
means a real promotion judged, repaired or escalated correctly on a live PR.
