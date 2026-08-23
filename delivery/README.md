# Delivery Kit

Gating, triage and visibility for a GitOps pipeline built on **ArgoCD + Kargo**.

Kargo is very good at producing change. Left alone it will open more pull
requests than anyone can read, merge a good share of them on a version-shaped
policy that cannot see what is actually in the diff, and tell you nothing when
a promotion breaks. This kit closes those three gaps:

| Piece | What it does |
|---|---|
| [`images/gitops-gate`](images/gitops-gate) | Renders your ApplicationSets at base and at head, fails when an app's **cluster targeting** changes, diffs the old and new chart render, and schema-validates the result. Run it from any CI system — the exit code is the verdict. |
| [`images/bosun`](images/bosun) | **Bosun** — reads a red gate, explains it, and fixes the mechanical cases — a flipped chart default, a coupled pin, a metrics port a NetworkPolicy still names. Escalates everything else. |
| [`charts/kargo-pipelines`](charts/kargo-pipelines) | Warehouses and Stages from one target list, with **multi-stage promotion chains**, verification gating, bounded timeouts and a triage hook. |
| [`charts/kargo-observability`](charts/kargo-observability) | Kargo's own state as Prometheus metrics, plus alerts and a dashboard. Kargo ships no domain metrics of its own. |
| [`charts/bosun`](charts/bosun) | Runs Bosun in-cluster, triggered by Kargo rather than polled. |

Bosun was called `delivery-agent` until 2026-08-23 — the name changed, the job
did not. A bosun makes routine repairs on their own authority and reports
serious damage to the captain, which is the split it draws between a mechanical
fix and an escalation. It sits beside Argo (the ship) and Kargo (the cargo).

## What problem this actually solves

Three things go wrong once Kargo is merging its own pull requests, and none of
them are visible without this kit:

**A bump that renders fine and breaks at runtime.** A chart minor flips a
default you depend on, or starts requiring a CRD version you do not have, or
moves a metrics port that a NetworkPolicy names by number. The pull request
looks like a one-line version change. The gate diffs the *rendered* output, so
the real change is on the pull request before it merges.

**An addon quietly changing which clusters it targets.** ApplicationSet
generators resolve selectors against live cluster labels, so a values-layer
edit can add or drop an entire cluster without the diff showing it. The gate
expands the generators and fails on the targeting change itself.

**Failure with nowhere to appear.** Kargo exposes only controller-runtime
metrics — no promotion, freight or stage state. A promotion that errors, a
verification that fails, a target broken since the day it was written: all of
it sits in the API and nowhere else. The observability chart turns that state
into metrics, alerts and a dashboard.

## Requirements

- **ArgoCD**, with metrics scraped by Prometheus — `argocd_app_info` is what
  verification and the dashboards are built on.
- **Kargo** 1.11 or newer.
- **Argo Rollouts controller** if you use `verify`. Kargo *builds* AnalysisRuns
  but does not execute them; without the controller they sit with no phase
  forever. Installing only the CRDs is not enough.
- **kube-state-metrics** for `charts/kargo-observability`.
- A git host Kargo supports — `github`, `gitlab`, `bitbucket`, `gitea`, `azure`.
- A CI system that can run a container and report a commit status.

## Portability

Nothing here hardcodes a cluster, domain, namespace, CNI, secret manager, git
host or model provider. Those are values. The rules that keep it that way, and
the test that enforces them, are in [CONTRIBUTING.md](CONTRIBUTING.md).

The design decisions and their trade-offs are recorded in [`adr/`](adr).

## Status

Early. Built and dogfooded inside a single homelab platform repository, which
is the first consumer. Interfaces may move before a 1.0.

## License

Apache-2.0. See [LICENSE](LICENSE).
