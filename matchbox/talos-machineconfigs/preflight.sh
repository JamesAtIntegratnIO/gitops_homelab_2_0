#!/usr/bin/env bash
# Pre-flight for a Talos or Kubernetes upgrade on the-cluster.
#
#   ./matchbox/talos-machineconfigs/preflight.sh            # just report
#   ./matchbox/talos-machineconfigs/preflight.sh 1.12.0      # and check k8s fits that Talos
#
# READ-ONLY. Every check here is a GET or a list; nothing is applied, patched or
# restarted. Run it before each node and again after, because "was this already
# broken?" is the question you cannot answer retroactively at 02:00.
#
# FAIL means stop -- the upgrade can make things worse or cannot finish.
# WARN means proceed knowingly -- usually it exists so that a pre-existing
# symptom does not get attributed to the upgrade afterwards.
#
# Exit status: 1 if any FAIL, else 0. WARNs never fail the run.

set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=/dev/null
source "$ROOT/.claude/skills/homelab-toolchain/scripts/tools.sh" 2>/dev/null || true

TARGET_TALOS="${1:-}"
NODES=(10.0.4.101 10.0.4.102 10.0.4.103)
FAILS=0
WARNS=0

red()  { printf '\033[31mFAIL\033[0m  %s\n' "$*"; FAILS=$((FAILS+1)); }
yel()  { printf '\033[33mWARN\033[0m  %s\n' "$*"; WARNS=$((WARNS+1)); }
grn()  { printf '\033[32m ok \033[0m  %s\n' "$*"; }
inf()  { printf '      %s\n' "$*"; }
hdr()  { printf '\n\033[1m%s\033[0m\n' "$*"; }

need() { [ -n "${1:-}" ] && [ -x "${1:-}" ]; }

# Talos -> supported Kubernetes minors. From the Sidero support matrix; update
# it when you add a column. Encoded here because the coupling is the whole
# reason this script exists: a Talos MINOR caps the Kubernetes maximum, so
# "upgrade Kubernetes" is gated on "upgrade Talos" and neither changelog says so.
talos_supports() { # talos_supports <talos-minor> -> "1.31 1.32 ..."
  case "$1" in
    1.11) echo "1.29 1.30 1.31 1.32 1.33 1.34" ;;
    1.12) echo "1.30 1.31 1.32 1.33 1.34 1.35" ;;
    1.13) echo "1.31 1.32 1.33 1.34 1.35 1.36" ;;
    1.14) echo "1.33 1.34 1.35 1.36 1.37" ;;
    *)    echo "" ;;
  esac
}

hdr "tooling"
if need "${KUBECTL:-}"; then grn "kubectl $("$KUBECTL" version --client -o json 2>/dev/null | "$JQ" -r .clientVersion.gitVersion)"
else red "no kubectl resolved -- source tools.sh and look at its warnings"; fi
if need "${TALOSCTL:-}"; then grn "talosctl $("$TALOSCTL" version --client --short 2>/dev/null | awk '/Talos/{print $2; exit}')"
else red "no talosctl resolved"; fi

# ---------------------------------------------------------------------------
hdr "talosconfig credential"
# This gates everything else: `talosctl upgrade` needs a valid os:admin cert,
# and if it lapses mid-ladder you cannot finish or roll back without
# regenerating from the CA in matchbox/assets/talos/<ver>/controlplane1.yaml.
# It expired-in-26-hours on 2026-10-29 and nothing was watching it.
CRT_B64="$(grep -m1 -E '^[[:space:]]*crt:' "$HOME/.talos/config" 2>/dev/null | sed -E 's/.*crt:[[:space:]]*//')"
if [ -n "$CRT_B64" ]; then
  END="$(printf '%s' "$CRT_B64" | base64 -d 2>/dev/null | openssl x509 -noout -enddate 2>/dev/null | cut -d= -f2)"
  if [ -n "$END" ]; then
    END_EPOCH="$(date -j -f '%b %d %T %Y %Z' "$END" +%s 2>/dev/null || date -d "$END" +%s 2>/dev/null)"
    if [ -n "$END_EPOCH" ]; then
      DAYS=$(( (END_EPOCH - $(date +%s)) / 86400 ))
      if   [ "$DAYS" -lt 0 ]; then red "os:admin cert EXPIRED ($END) -- regenerate from the CA before anything else"
      elif [ "$DAYS" -lt 7 ]; then red "os:admin cert expires in ${DAYS}d ($END) -- renew first: talosctl config new --roles os:admin --crt-ttl 8760h"
      elif [ "$DAYS" -lt 30 ]; then yel "os:admin cert expires in ${DAYS}d ($END)"
      else grn "os:admin cert valid for ${DAYS}d"; fi
    else yel "could not parse cert end date '$END'"; fi
  else yel "could not decode the os:admin cert"; fi
else yel "no crt in ~/.talos/config -- cannot check expiry"; fi

# ---------------------------------------------------------------------------
hdr "nodes"
NODE_JSON="$(k get nodes -o json 2>/dev/null)"
if [ -z "$NODE_JSON" ]; then
  red "cannot reach the apiserver"
else
  NOT_READY="$(printf '%s' "$NODE_JSON" | "$JQ" -r '[.items[]|select([.status.conditions[]|select(.type=="Ready")|.status]|index("True")|not)|.metadata.name]|join(" ")')"
  [ -n "$NOT_READY" ] && red "not Ready: $NOT_READY" || grn "all nodes Ready"

  # A mixed kubelet version means a previous upgrade did not finish. Starting a
  # new one on top of that is how you end up unable to say what broke.
  KVERS="$(printf '%s' "$NODE_JSON" | "$JQ" -r '[.items[].status.nodeInfo.kubeletVersion]|unique|join(" ")')"
  case "$KVERS" in
    *" "*) red "kubelet versions are mixed ($KVERS) -- a previous upgrade is unfinished" ;;
    *)     grn "kubelet uniform: $KVERS" ;;
  esac
  OSVERS="$(printf '%s' "$NODE_JSON" | "$JQ" -r '[.items[].status.nodeInfo.osImage]|unique|join(" ")')"
  case "$OSVERS" in
    *Talos*Talos*) red "Talos versions are mixed ($OSVERS) -- finish the ladder before starting another" ;;
    *)             grn "Talos uniform: $OSVERS" ;;
  esac
  CUR_K8S="$(printf '%s' "$NODE_JSON" | "$JQ" -r '.items[0].status.nodeInfo.kubeletVersion' | sed -E 's/^v?([0-9]+\.[0-9]+).*/\1/')"
  CUR_TALOS="$(printf '%s' "$NODE_JSON" | "$JQ" -r '.items[0].status.nodeInfo.osImage' | sed -E 's/.*v([0-9]+\.[0-9]+).*/\1/')"
fi

# ---------------------------------------------------------------------------
hdr "etcd"
# Upgrades go one node at a time and each one reboots. With 3 members quorum is
# 2, so you can afford exactly one down -- which means anything already
# degraded must be fixed BEFORE you take a node out deliberately.
if need "${TALOSCTL:-}"; then
  MEM="$("$TALOSCTL" --context the-cluster -n "${NODES[0]}" etcd members 2>/dev/null)"
  COUNT="$(printf '%s\n' "$MEM" | awk 'NR>1 && NF {n++} END{print n+0}')"
  LEARNERS="$(printf '%s\n' "$MEM" | awk 'NR>1 && $NF=="true" {n++} END{print n+0}')"
  if [ "$COUNT" -lt 3 ]; then red "etcd has $COUNT member(s); expected 3 -- no headroom to reboot a node"
  else grn "etcd has $COUNT voting members"; fi
  [ "$LEARNERS" -gt 0 ] && red "etcd has $LEARNERS learner(s) -- a learner does not count toward quorum"

  ALARM="$("$TALOSCTL" --context the-cluster -n "${NODES[0]}" etcd alarm list 2>/dev/null | sed '/^$/d')"
  [ -n "$ALARM" ] && red "etcd alarm(s) raised: $ALARM" || grn "no etcd alarms"
else
  yel "talosctl unavailable -- etcd not checked"
fi

# ---------------------------------------------------------------------------
hdr "backups"
# Do not upgrade a 3-node control plane without a recent etcd snapshot. This is
# the only check here whose failure is unrecoverable rather than inconvenient.
SNAP="$(k -n talos-backup get cronjob talos-etcd-snapshot -o json 2>/dev/null | "$JQ" -r '.status.lastSuccessfulTime // empty')"
if [ -n "$SNAP" ]; then
  S_EPOCH="$(date -j -f '%Y-%m-%dT%H:%M:%SZ' "$SNAP" +%s 2>/dev/null || date -d "$SNAP" +%s 2>/dev/null)"
  AGE_H=$(( ( $(date +%s) - ${S_EPOCH:-0} ) / 3600 ))
  if [ "$AGE_H" -gt 24 ]; then red "host etcd snapshot is ${AGE_H}h old ($SNAP) -- take one before upgrading"
  else grn "host etcd snapshot ${AGE_H}h old ($SNAP)"; fi
else
  red "no successful host etcd snapshot on record -- platform-backups-talos may be disabled"
fi
VSNAP="$(k -n vcluster-media get cronjob vcluster-media-etcd-snapshot -o json 2>/dev/null | "$JQ" -r '.status.lastSuccessfulTime // empty')"
[ -n "$VSNAP" ] && grn "vcluster etcd snapshot $VSNAP" || yel "no vcluster etcd snapshot on record"

# ---------------------------------------------------------------------------
hdr "target compatibility"
if [ -n "$TARGET_TALOS" ]; then
  TMINOR="$(printf '%s' "$TARGET_TALOS" | sed -E 's/^v?([0-9]+\.[0-9]+).*/\1/')"
  SUP="$(talos_supports "$TMINOR")"
  if [ -z "$SUP" ]; then
    yel "no support data for Talos $TMINOR -- add it to talos_supports() from the Sidero matrix"
  elif printf '%s' "$SUP" | tr ' ' '\n' | grep -qx "${CUR_K8S:-none}"; then
    grn "Talos $TMINOR supports the running Kubernetes $CUR_K8S (range: $SUP)"
  else
    red "Talos $TMINOR does NOT support Kubernetes $CUR_K8S (it supports: $SUP) -- move Kubernetes first or pick another Talos"
  fi
  # Skipping a minor is the other way this goes wrong.
  if [ -n "${CUR_TALOS:-}" ]; then
    CUR_MIN="${CUR_TALOS#*.}"; TGT_MIN="${TMINOR#*.}"
    if [ "$(( TGT_MIN - CUR_MIN ))" -gt 1 ]; then
      red "Talos $CUR_TALOS -> $TMINOR skips a minor -- upgrade one minor at a time"
    fi
  fi
else
  inf "no target given; pass one (e.g. 1.12.0) to check Kubernetes compatibility"
fi
inf "running: Talos ${CUR_TALOS:-?} / Kubernetes ${CUR_K8S:-?}"

# ---------------------------------------------------------------------------
hdr "baseline noise"
# Everything below is a WARN on purpose. It exists so that after the upgrade you
# can tell a new problem from one that was already there -- which is the single
# most useful thing a pre-flight produces.
BAD_APPS="$(k -n argocd get applications -o json 2>/dev/null | "$JQ" -r '[.items[]|select(.status.sync.status!="Synced" or .status.health.status!="Healthy")|"\(.metadata.name)(\(.status.sync.status)/\(.status.health.status))"]|join(" ")')"
[ -n "$BAD_APPS" ] && yel "ArgoCD not clean: $BAD_APPS" || grn "all ArgoCD applications Synced/Healthy"

BAD_PODS="$(k get pods -A -o json 2>/dev/null | "$JQ" -r '[.items[]|select(.status.phase!="Running" and .status.phase!="Succeeded")|"\(.metadata.namespace)/\(.metadata.name)(\(.status.phase))"]|length')"
[ "${BAD_PODS:-0}" -gt 0 ] && yel "${BAD_PODS} pod(s) not Running/Succeeded" || grn "no pods in a bad phase"

# etcd on these nodes shares a disk with the OS and containerd, so controllers
# drop leases for no local reason. If that is happening NOW, an upgrade will
# look like the cause. See CLAUDE.md and docs/okf/cluster/known-issues.md.
LEASE="$(k get events -A --field-selector reason=LeaderElection -o json 2>/dev/null | "$JQ" -r --arg t "$(date -u -v-2H +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u -d '2 hours ago' +%Y-%m-%dT%H:%M:%SZ)" '[.items[]|select((.lastTimestamp//"")>$t)]|length')"
[ "${LEASE:-0}" -gt 5 ] && yel "${LEASE} leader-election events in the last 2h -- etcd I/O starvation is active; expect noise" || grn "leader-election quiet"

# ---------------------------------------------------------------------------
hdr "result"
printf '%d fail, %d warn\n' "$FAILS" "$WARNS"
[ "$FAILS" -eq 0 ] || { echo "Do not start the upgrade."; exit 1; }
echo "Clear to proceed. Re-run after each node."
