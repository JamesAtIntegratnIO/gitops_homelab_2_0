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
  # `find`, not a glob. This file is SOURCED, so the calling shell expands any
  # glob before `ls` runs -- and zsh (which the Claude Code Bash tool uses on
  # macOS) treats an unmatched glob as a FATAL error, so `2>/dev/null` does not
  # help: a single absent package aborted the whole file and exported nothing.
  # `find` does its own matching and simply prints nothing when there is no hit.
  find /nix/store -maxdepth 1 -type d -name "*-${pkg}-*" 2>/dev/null |
    sed "s|\$|/bin/${exe}|" |
    while IFS= read -r p; do [ -x "$p" ] && printf '%s\n' "$p"; done |
    # Prefix each path with its version field. Sorting the raw paths sorts by
    # the store HASH, not the version -- `sort -V | tail -1` on full paths
    # returns an arbitrary build.
    sed -E "s|^/nix/store/[^-]+-${pkg}-([^/]+)/|\1 &|" |
    { if [ -n "$pref" ]; then grep "^${pref}" || true; else cat; fi; } |
    sort -V -k1,1 | tail -1 | cut -d' ' -f2
}

# kubectl must stay within +/-1 minor of the apiserver. The pin is a PREFERENCE,
# not a hard pin, because the store moves with nixpkgs and the matching build
# gets garbage-collected: on 2026-10-29 the flake had only 1.36/1.37 while the
# apiserver was still 1.34.1, and a hard `1.34` pin resolved to the empty string
# -- which fails as a confusing "command not found" at the first call site
# rather than as a version problem. Falling back with a warning keeps the
# session usable and names the real issue.
export KUBECTL_WANT_MINOR="${KUBECTL_WANT_MINOR:-1.34}"
export KUBECTL="${KUBECTL:-$(nixbin kubectl kubectl "$KUBECTL_WANT_MINOR")}"
if [ -z "$KUBECTL" ]; then
  KUBECTL="$(nixbin kubectl kubectl)"
  export KUBECTL
  if [ -n "$KUBECTL" ]; then
    echo "tools.sh: no kubectl ${KUBECTL_WANT_MINOR}.x in the store; using $(basename "$(dirname "$(dirname "$KUBECTL")")")." >&2
    echo "tools.sh: that is outside the supported skew for a ${KUBECTL_WANT_MINOR} apiserver -- fine for reads, suspect for writes." >&2
  else
    echo "tools.sh: no kubectl in the store at all." >&2
  fi
fi
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
