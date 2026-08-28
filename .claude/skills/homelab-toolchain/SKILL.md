---
name: homelab-toolchain
description: Get working tools and cluster access in this repo without `nix develop`. Use before any kubectl, helm, argocd, yq, jq, talosctl or go command here, and whenever a command fails with "command not found", a stalled `nix develop`, an empty $var after word-splitting, or a kubectl version-skew warning.
---

# Toolchain and cluster access

## Get the tools

```bash
source .claude/skills/homelab-toolchain/scripts/tools.sh
k get nodes                     # $KUBECTL --context admin@the-cluster
kv -n media get pods            # the vcluster
```

Exports `KUBECTL HELM JQ YQ ARGOCD TALOSCTL KUBECM GO CTX VCTX` and the `k` /
`kv` helpers. Read the script before extending it — the two non-obvious bits are
commented there.

## Why not `nix develop`

Flake tooling is not on the default PATH, and in an agent worktree under
`.claude/worktrees/*` the gitignored `secrets.env` is absent, so `nix develop`
either warns from the shellHook or stalls building `hctl`. The store paths are
already there; use them directly.

## Traps this exists to avoid

- **`ls /nix/store/*-<pkg>-*/... | sort -V | tail -1` returns an arbitrary
  build.** `sort -V` on a full store path sorts by the *hash* first. Extract the
  version field before sorting — `tools.sh` does.
- **kubectl must be 1.34.** The store also holds 1.36.x; the apiserver is
  1.34.1 and the supported skew is ±1. `tools.sh` pins the prefix.
- **`yq` here is mikefarah v4** (store package `yq-go`). The bare glob
  `*-yq-*` also matches `python3.13-yq`, which is a different, jq-syntax tool.
- **The Bash tool runs zsh on macOS.** Unquoted `$var` does *not* word-split —
  use `${=var}` or an array. There is no `timeout` binary.
- **Never read or print `secrets.env`.** Gitignored, live credentials.

## Cluster identities

| | |
|---|---|
| host | `the-cluster`, context `admin@the-cluster`, API VIP `https://10.0.4.100:6443` |
| tenant | `vcluster-media`, context `admin@media-cluster` |
| reconciles from | `origin/main` — a branch commit changes nothing in the cluster |

Adding a kubeconfig: `$KUBECM add -cf <file> --context-name admin@<cluster>`.
The `-c/--cover` flag is required in a non-TTY session (without it kubecm opens
an interactive prompt and dies with "Prompt failed ^D"); it only means "write
the merge to ~/.kube/config", existing contexts are preserved. Test the incoming
file standalone first and back up `~/.kube/config`.

## Talos

`talosctl` read-only and `--dry-run` work from an agent session against
`~/.talos/config` (context `the-cluster`, **cert expires 2026-10-30**). A live
`talosctl patch mc` is blocked by the permission classifier — write the patch
into `matchbox/talos-machineconfigs/` and hand the command to James.

Once `watchdog.yaml` is applied the machine config is multi-document and JSON
6902 patches are refused; use the `live-*.yaml` strategic-merge patches (maps
only — a list-bearing fragment duplicates the block). Never `patch mc` with
`all.yaml`/`cp.yaml`: they append list items and duplicate the interface/VIP
block.
