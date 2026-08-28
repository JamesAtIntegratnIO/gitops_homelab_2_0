#!/usr/bin/env bash
# Resolve the Nix-flake tooling without `nix develop`.
#
#   source .claude/skills/homelab-toolchain/scripts/tools.sh
#   $KUBECTL --context "$CTX" get nodes
#
# Why not `nix develop`: in an agent worktree under .claude/worktrees/* the
# gitignored secrets.env is absent, so the flake's shellHook warns or the shell
# stalls building hctl. These paths are already in the store; use them directly.

nixbin() { # nixbin <store-pkg> [exe] [version-prefix]
  local pkg="$1" exe="${2:-$1}" pref="${3:-}"
  ls -d /nix/store/*-"$pkg"-*/bin/"$exe" 2>/dev/null |
    # Prefix each path with its version field. Sorting the raw paths sorts by
    # the store HASH, not the version -- `sort -V | tail -1` on full paths
    # returns an arbitrary build.
    sed -E "s|^/nix/store/[^-]+-${pkg}-([^/]+)/|\1 &|" |
    { if [ -n "$pref" ]; then grep "^${pref}" || true; else cat; fi; } |
    sort -V -k1,1 | tail -1 | cut -d' ' -f2
}

# kubectl is pinned to the server minor on purpose: the store also carries
# 1.36.x, which is outside the +/-1 skew the 1.34 apiserver supports.
export KUBECTL="${KUBECTL:-$(nixbin kubectl kubectl 1.34)}"
export HELM="${HELM:-$(nixbin kubernetes-helm helm)}"
export JQ="${JQ:-$(nixbin jq jq)}"
export YQ="${YQ:-$(nixbin yq-go yq)}"          # mikefarah yq v4, NOT python-yq
export ARGOCD="${ARGOCD:-$(nixbin argocd argocd)}"
export TALOSCTL="${TALOSCTL:-$(nixbin talosctl talosctl)}"
export KUBECM="${KUBECM:-$(nixbin kubecm kubecm)}"
export GO="${GO:-$(nixbin go go)}"

export CTX="${CTX:-admin@the-cluster}"         # host cluster
export VCTX="${VCTX:-admin@media-cluster}"     # vcluster-media

k()  { "$KUBECTL" --context "$CTX" "$@"; }
kv() { "$KUBECTL" --context "$VCTX" "$@"; }
