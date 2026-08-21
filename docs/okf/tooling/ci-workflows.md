---
type: CI Pipeline
title: CI workflows and repo guardrails
description: The five GitHub Actions workflows, the no-Secrets enforcement chain, and the local git hook / editor guardrails.
tags: [ci, github-actions, security, git-hooks]
status: stable
generated: { by: claude-code/claude-fable-5, at: 2026-08-20T23:40:00Z }
stale_after: 2027-02-20
sources:
  - id: workflows
    resource: ../../../.github/workflows/
    title: .github/workflows/*.yaml
  - id: prehook
    resource: ../../../.githooks/pre-commit
    title: pre-commit hook
  - id: copilot
    resource: ../../../.github/copilot-instructions.md
    title: copilot-instructions.md
---

# Workflows

| Workflow | Trigger | What it does |
|---|---|---|
| `build-go-sdk-promises.yaml` | push/PR on `promises/*/workflows/**` or `promises/_shared/**` | Change-detection job emits a promise matrix (a `_shared` change rebuilds **all** promises); builds `ghcr.io/jamesatintegratnio/<promise>-configure` (and `-delete` if present) with `./promises` as build context; `go build` validation job; PRs build without pushing |
| `validate-promises.yaml` | PR/push on `promises/**/*.yaml` | **The hard gate**: fails if any promise YAML contains `^kind: Secret$` (the kratix-platform-state repo is public); also yq-parses all YAML (yq fetched from `releases/latest`, unpinned) |
| `build-kubectl-image.yaml` | `images/kubectl/**` | Multi-arch kubectl+bash image (exists because upstream kubectl images are distroless/discontinued); Dockerfile default v1.34.4 vs workflow default v1.34.1 — mismatched |
| `build-git-indexer-image.yaml` | `images/git-indexer/**` | Multi-arch RAG indexer image (see [AI stack](/platform/ai-stack.md)) |
| `build-platform-status-reconciler-image.yaml` | `images/platform-status-reconciler/**` | The custom status controller image (see [Kratix](/promises/kratix.md)) |

All image pushes authenticate to GHCR with the ephemeral `GITHUB_TOKEN` — no
long-lived registry credentials in CI.[^workflows]

# The no-Secrets enforcement chain

Four independent layers exist to keep `kind: Secret` out of git (the Kratix
state repo is public):

1. `.githooks/pre-commit` — scans staged blobs… **but is currently a no-op**:
   it filters on `promises-v2/`, a directory that no longer exists. Local
   commits are unguarded.[^prehook] (See [known issues](/cluster/known-issues.md).)
2. `validate-promises.yaml` CI — the working gate, correctly scoped to `promises/`.
3. Pipeline convention — promises emit `ExternalSecret` referencing the
   `onepassword-store` ClusterSecretStore, never Secrets
   (see [secret management](/platform/secret-management.md)).
4. The git-indexer redacts `kind: Secret` docs and token patterns before
   embedding repo content for RAG.

Hooks are opt-in: `scripts/install-hooks.sh` sets `core.hooksPath .githooks`.

# Editor / agent guardrails

- `.github/copilot-instructions.md` tiers agent actions (auto / confirm /
  refuse), bans `kind: Secret` outright, and documents the Nix-terminal rule.
- `.vscode/settings.json` carries a large MCP + terminal-regex auto-approval
  policy (read-only allowed; mutating kubectl/git/tofu/talosctl/helm verbs
  denied). Note: `.vscode/mcp.json` and one settings path hardcode a stale
  absolute path from another machine (`/home/boboysdadda/...`).

# Container images built from this repo

`images/kubectl` (job/cronjob shell image), `images/git-indexer` (RAG
indexer; its `rag_pipeline.py`/`rag_tool.py` are Open WebUI plugins *not*
baked into the image), `images/platform-status-reconciler` (status
controller). All are published under `ghcr.io/jamesatintegratnio/`.

[^workflows]: .github/workflows/*.yaml
[^prehook]: pre-commit hook
