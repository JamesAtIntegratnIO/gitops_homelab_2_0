---
name: kargo-pins
description: Work with Kargo — the repo's version-bump bot. Use when adding or moving an image/chart pin, when a Warehouse stops discovering, when a Stage is stuck Ready=False or a Promotion is Errored/Failed/Aborted/Running-forever, when triaging Kargo's bump PRs, or when nothing has bumped in days and every dashboard is green.
---

# Kargo

48 Warehouse/Stage pairs across 3 Projects (`addons`, `promises`, `workloads`),
rendered by `addons/charts/kargo-projects` from
[kargo-projects/values.yaml](../../../addons/cluster-roles/control-plane/addons/kargo-projects/values.yaml).
Background: [docs/kargo.md](../../../docs/kargo.md). Kargo merging its own
version-bump PRs is the one sanctioned exception to "never push to main".

## Kargo's failure mode is silence

Every signal stays green, because what you have to observe is the **absence of
an event**. Every individual object really is fine. Start any Kargo question by
looking for the absences:

```bash
source .claude/skills/homelab-toolchain/scripts/tools.sh
k get stages -A -o json | $JQ -r '.items[]
  | {ns:.metadata.namespace, n:.metadata.name,
     ready:([.status.conditions[]?|select(.type=="Ready")|.status]|first)}
  | select(.ready!="True") | "\(.ns)/\(.n) ready=\(.ready)"'
k get promotions -A -o json | $JQ -r '.items[]
  | select(.status.phase!="Succeeded")
  | "\(.metadata.namespace)/\(.metadata.name) \(.status.phase)"'
```

Three semantics that make the silence permanent:

- **A terminal promotion is FINAL.** Auto-promotion never re-runs one — from
  the controller's view that Freight *has* been promoted, the attempt merely
  failed. One transient DNS blip stops a Stage forever.
- **A failed AnalysisRun does not fail a promotion.** The Stage goes
  `Ready=False`, declines to start the next promotion, and nothing else changes.
- **A failed verification is ALSO terminal.** Fixing the cause does nothing;
  Kargo does not re-run it. Proved by merging a NetworkPolicy fix and watching
  three Stages not move until they were asked again.

Kargo does **not** execute AnalysisRuns itself — the Argo Rollouts controller
does (installed as the `argo-rollouts` addon). Verification checks
`argocd_app_info` Synced/Healthy for the apps a target names in `verify.apps`.
The vcluster's ArgoCD is not in host Prometheus, so `workloads` has no
verification.

## The four incantations, none guessable

```bash
# re-run a terminal VERIFICATION -- the id is at
# status.freightHistory[0].verificationHistory[0].id, three levels deeper than anyone looks
k -n <ns> annotate stage <name> 'kargo.akuity.io/reverify={"id":"<id>"}' --overwrite

# abort a promotion -- abort=true is SILENTLY IGNORED (parsed as a request object)
k -n <ns> annotate promotion <name> 'kargo.akuity.io/abort={"action":"terminate"}' --overwrite

# force artifact discovery (does NOT re-run a promotion)
k -n <ns> annotate warehouse <name> kargo.akuity.io/refresh="$(date -u +%Y-%m-%dT%H:%M:%SZ)" --overwrite

# re-promote Freight that already carries a terminal Promotion (what `kargo promote` does).
# generateName must NOT end in a dot -- RFC1123.
k create -f - <<'EOF'
apiVersion: kargo.akuity.io/v1alpha1
kind: Promotion
metadata: {generateName: <stage>, namespace: addons}
spec: {stage: <stage>, freight: <freight-name>}
EOF
```

A refresh is not a re-promotion. A promotion wedged on `git-wait-for-pr` for a
PR you closed clears itself.

The authoritative pin list is in the **Stage**, not only the repo:
`.spec.promotionTemplate.spec.steps[] | select(.uses=="yaml-update")` gives the
file and keys Kargo will actually write.

## Writing a pin Kargo can track

`yaml-update` parses only the **first YAML document** and rewrites the line in
place. A tracked key must:

- live in the first document of its file,
- be a scalar,
- carry no trailing `# comment`.

Renaming the file, reordering documents or commenting the line breaks the
target **silently** — the promotion fails and the pin goes stale. Add every new
pin to `kargo-projects/values.yaml`; do not let them rot.

Other spec details that cause an ArgoCD re-sync loop rather than an error:
Kargo's webhook stores durations canonically (`6h0m0s`) and defaults
`freightCreationPolicy` — write Warehouse specs exactly that way. Chart and git
subscriptions need a constraint (`>=0.0.0`) or prereleases win.

## Triaging Kargo's bump PRs

Expect a bump to need a config change riding along, because Kargo only moves the
version string:

- a chart default that flipped (`argo-cd` 10.x turns `global.networkPolicy.create`
  on; `metallb` 0.16 flips `speaker.frr.enabled` off and `frrk8s.enabled` on and
  moves metrics 7472 → 9120 on both controller and speaker),
- a coupled pin (NGF 2.6.7 needs Gateway API 1.5 → bump `gateway-api-crds`),
- a values surface that moved (see the values-drop check in `addon-change`),
- a new binding-time requirement (`argocd-mcp` v0.9.0 binds loopback and refuses
  to widen without `MCP_AUTH_TOKEN` or `--allow-unauthenticated`, while its
  tcpSocket probes keep reporting Healthy).

**authentik cannot skip major.minor releases.** `ensure_allowed_version()`
raises *before* `run_migrations()`, so it takes the DB lock, refuses, releases
and exits — nothing migrates and the old pods keep serving. Go one hop at a
time to the latest patch of each minor, widening the Kargo constraint after
each lands.

**Version distance is a real signal.** A one-line diff that is green everywhere
can still break every live object at apply — external-secrets 2.9.0 stops
serving `v1alpha1`/`v1beta1` while manifests here still declared them.

Related: `addon-change` for the values-drop and NetworkPolicy checks,
`gate-triage` for what the gate says about a bump.
