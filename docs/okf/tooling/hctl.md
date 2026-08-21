---
type: CLI Tool
title: hctl — Homelab Control CLI
description: The Go/Cobra/Bubbletea CLI in cli/ that wraps day-to-day platform operations — vcluster lifecycle, Score-based deploys, addon management, diagnostics.
tags: [hctl, cli, go, bubbletea, score]
status: stable
generated: { by: claude-code/claude-fable-5, at: 2026-08-20T23:40:00Z }
stale_after: 2027-02-20
sources:
  - id: cli-src
    resource: ../../../cli/
    title: cli/ source tree (cmd/, internal/, pkg/provisioners)
  - id: agents-md
    resource: ../../../AGENTS.md
    title: AGENTS.md command reference
---

# What it is

`hctl` ("Homelab Control") is a ~12k-LoC Go CLI (`cli/`, module
`github.com/jamesatintegratnio/hctl`) built with Cobra + Bubbletea/Lipgloss.
It is built by the [Nix flake](/tooling/nix-dev-shell.md) as the default
package. Design invariant: **every mutating operation is GitOps-mediated** —
hctl writes YAML into this repo and commits (per `gitMode`), it never
`kubectl apply`s platform resources directly.[^cli-src]

# Command surface

| Group | Commands | Notes |
|---|---|---|
| Core | `init`, `doctor`, `context`, `version`, `status [-w]`, `alerts` | `status` is a 6-tab Bubbletea dashboard; `alerts` queries Prometheus via the apiserver service proxy (no port-forward) |
| Diagnostics | `diagnose <res> [--bundle]`, `trace <res>`, `reconcile <res>` | `diagnose` walks a 9-step Kratix→ArgoCD→pods chain; encodes tribal knowledge (e.g. "WorkPlacement can show Failing despite successful deployment") |
| vcluster | `create`, `list`, `status`, `kubeconfig`, `connect`, `delete`, `apps`, `sync` | `create` has a ~25-flag CLI + interactive wizard; writes `platform/vclusters/<name>.yaml`; `sync` syncs CRD-providing apps first |
| deploy | `init`, `run`, `render`, `diff`, `status`, `remove`, `list` | Score-based; see below |
| addon | `list`, `status`, `enable`, `disable` | edits addons.yaml layers |
| scale / convenience | `scale up/down <ns>`, `up`, `down`, `logs -f`, `open` | scale disables ArgoCD auto-sync before scaling to 0 |
| secret | `get <ns> <name>`, `list <ns>` | prints **decoded** secret values to the terminal — shoulder-surfing hazard by design |
| ai | `reindex [-w]` | creates a one-off Job from the `ai/git-indexer` CronJob (note: git-indexer was later removed from the repo — see [known issues](/cluster/known-issues.md)) |

# How it talks to things

- **Kubernetes**: client-go typed + dynamic clients from the standard
  kubeconfig loading rules (honors `$KUBECONFIG`, optional `kubeContext`
  config). Hardcoded GVRs for `vclusterorchestratorv2s`, ArgoCD `applications`,
  Kratix `promises`/`works`/`workplacements`.
- **ArgoCD**: only via the Kubernetes API (CR patches in the `argocd`
  namespace) — never the ArgoCD REST/gRPC API. Sync = merge-patching an
  `operation` block; autosync toggles = patching `spec.syncPolicy`.
- **Git**: shells out to `git`; all writes funnel through a `gitMode` switch —
  `auto` (commit+push), `prompt`, `generate` (commit only), `stage-only`.
- **Prometheus**: `GET .../services/http:kube-prometheus-stack-prometheus:9090/proxy/api/v1/query`.

# Score deploy flow

`hctl deploy run` reads a [Score](https://score.dev) `score.yaml` (v1b1; see
the template at [score.yaml](../../../score.yaml)), translates it to a
**Stakater `application` chart** values file plus provisioner-emitted
manifests, and writes:

- `workloads/<cluster>/addons/<workload>/values.yaml`
- an entry in `workloads/<cluster>/addons.yaml`

The target vcluster's ArgoCD then deploys it. Provisioners map Score resource
types to platform primitives: `postgres`/`redis` → ExternalSecret referencing a
1Password item, `route` → HTTPRoute (+ cert), `volume` → PVC, `dns` → handled
by external-dns.

# Config

`~/.config/hctl/config.yaml`: `repoPath`, `defaultCluster`, `gitMode` (default
`prompt`), `argocdURL`, `kubeContext`, `outputFormat`, and a `platform` block
(domain `cluster.integratn.tech`, subnet `10.0.4.0/24`, MetalLB pool
`10.0.4.200-253`, namespace `platform-requests`).

# Known quirks (as of 2026-08-20)

- `deploy run`'s git step captures the written-paths slice before it is
  populated, so its `git add` receives no paths (other flows unaffected).
- A Score `route` resource renders **two** HTTPRoutes (provisioner manifest +
  chart `httpRoute` block) with disagreeing parentRefs.
- The Nix build injects only `Version=0.1.0` (no commit), so `hctl version`
  reports `commit: none`.
- `internal/errors` exit-code taxonomy and `internal/tui/log.go` leveled
  logging are wired but effectively unused.

[^cli-src]: cli/ source tree (cmd/, internal/, pkg/provisioners)
[^agents-md]: AGENTS.md command reference
