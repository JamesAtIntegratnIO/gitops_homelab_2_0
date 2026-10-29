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

# Every version of a package present in the store, for an error message that
# tells you what you DO have rather than only what you are missing.
nixbin_versions() { # nixbin_versions <store-pkg>
  find /nix/store -maxdepth 1 -type d -name "*-${1}-*" 2>/dev/null |
    sed -E "s|^/nix/store/[^-]+-${1}-||" |
    # Drop the -man/-doc/-bin companion outputs: they are the same version and
    # listing them makes the error look like four choices when there is one.
    grep -E '^[0-9]+(\.[0-9]+)*$' | sort -V | uniq
}

# kubectl is pinned HARD to the server minor. The store also carries 1.36/1.37,
# which are outside the +/-1 skew a 1.34 apiserver supports, and a client two
# minors ahead is worse than no client: reads mostly work, so you trust it, and
# then a write uses a field the apiserver will not honour.
#
# A silent fallback was tried here on 2026-10-29 and reverted the same day. It
# was written in the belief that no 1.34 build existed, which was a bad store
# scan -- the flake pins 1.34.1 and it was present all along. Falling back to
# 1.37 with only a stderr warning turns a loud stop into a soft one, which is
# the wrong direction for this repo.
#
# What WAS a real bug is the symptom when the pin matches nothing: the export
# became the empty string and every later call failed as "command not found",
# which names the wrong problem. So the pin stays hard and says so.
export KUBECTL_WANT_MINOR="${KUBECTL_WANT_MINOR:-1.34}"
export KUBECTL="${KUBECTL:-$(nixbin kubectl kubectl "$KUBECTL_WANT_MINOR")}"
if [ -z "$KUBECTL" ]; then
  echo "tools.sh: no kubectl ${KUBECTL_WANT_MINOR}.x in /nix/store." >&2
  echo "tools.sh: available: $(nixbin_versions kubectl | tr '\n' ' ')" >&2
  echo "tools.sh: run 'nix develop' (or 'nix build .#') to realise the pinned build," >&2
  echo "tools.sh: or set KUBECTL_WANT_MINOR to match the apiserver if the cluster moved." >&2
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
