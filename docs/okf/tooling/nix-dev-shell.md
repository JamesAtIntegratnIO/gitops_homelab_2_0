---
type: Dev Environment
title: Nix dev shell
description: The flake.nix development environment — pinned CLI toolchain, the hctl build, and helper scripts; activated via direnv (`use flake`).
tags: [nix, flake, direnv, tooling]
status: stable
generated: { by: claude-code/claude-fable-5, at: 2026-08-20T23:40:00Z }
stale_after: 2027-02-20
sources:
  - id: flake
    resource: ../../../flake.nix
    title: flake.nix
---

# What it provides

`nix develop` (auto-entered via `.envrc` → `use flake`, and by the VS Code
terminal profile) provides the full pinned toolchain:[^flake]

argocd, go, opentofu, tflint, terraform-docs, kubecm, kubectl, kustomize,
helm, krew, k9s, talosctl, cilium-cli, hubble, jq, yq, minio-client,
clusterctl, nodejs 22 — plus custom tools:

| Tool | Purpose |
|---|---|
| `hctl` | The [platform CLI](/tooling/hctl.md), built from `./cli` via `buildGoModule` |
| `scale-down-namespace <ns>` | Disables ArgoCD auto-sync per-app (via tracking-id annotation) *then* scales deployments to 0 — so self-heal doesn't fight the scale-down |
| `scale-up-namespace <ns>` | Reverse: scale 0→1 and re-enable auto-sync |
| `get_secret_data <ns> <name>` | Prints a secret's data base64-decoded (convenience; prints secrets to the terminal) |
| `yolo` | **Dead code** — targets `terraform/hub/` and `terraform/spokes/`, neither of which exists anymore |

Versions observed 2026-08-20: kubectl 1.34.1 (matches the cluster), helm
3.19.0, talosctl 1.11.3 (cluster runs Talos 1.11.5), argocd CLI 3.1.9,
opentofu 1.10.6, go 1.25.2.

# Shell hook behavior

The `shellHook` sources `./secrets.env` **unguarded** — the file is gitignored
(it carries local environment credentials), so `nix develop` errors on a fresh
clone until you create it. It also loads bash completions for kubectl, kubecm,
helm, argocd, kustomize, talosctl, and hctl.

Never wrap commands in `nix develop -c` inside the VS Code terminal — the
terminal is already inside the shell (this rule is codified in
`.github/copilot-instructions.md`).

Related: [hctl](/tooling/hctl.md), [CI workflows](/tooling/ci-workflows.md).

[^flake]: flake.nix
