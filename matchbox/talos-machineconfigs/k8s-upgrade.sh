#!/usr/bin/env bash
# Upgrade the-cluster's Kubernetes ONE minor, behind gates.
#
#   ./matchbox/talos-machineconfigs/k8s-upgrade.sh --dry-run 1.35.10   # Talos's plan + gates, changes nothing
#   ./matchbox/talos-machineconfigs/k8s-upgrade.sh 1.35.10             # do it
#
# Run it only once the Talos ladder is finished: the Talos minor caps the
# Kubernetes maximum (see upgrades.md), so the nodes must already run a Talos
# that supports the target.
#
# `talosctl upgrade-k8s` drives the control plane from one node and restarts the
# kubelets in place; it does not drain. This wraps it with:
#   - a talosctl matching the Talos the nodes run, and a kubectl within one minor
#     of BOTH the old and new apiserver (the target's if the store has it, else
#     the current one's) -- never a client two minors off;
#   - gates before: nodes Ready, etcd healthy, the vcluster's API answering, and a
#     baseline of what is already unhealthy (pods and ArgoCD apps);
#   - Talos's own --dry-run, which must pass (it checks for removed APIs and
#     component flags), then the upgrade with:
#       --manifests-no-prune      never delete a previously applied object
#       --with-docs=false --with-examples=false
#                                 do not rewrite every machine config with
#                                 comments, which is churn in the most sensitive
#                                 config the cluster has
#   - checks after: every kubelet and the apiserver report the target, the
#     control-plane pods are Ready on every node, etcd is healthy, nothing is
#     newly broken after 15 minutes, a server-side dry-run write passes the
#     admission webhooks (Kyverno's are failurePolicy=Fail and v1.12 is far
#     outside its tested range), and the vcluster's API still answers.
#
# Exit: 0 done, 1 stopped by a gate, 2 usage.

set -uo pipefail

DRY=0
[ "${1:-}" = "--dry-run" ] && { DRY=1; shift; }
TARGET="${1:-}"; TARGET="${TARGET#v}"
[[ "$TARGET" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "usage: $0 [--dry-run] <kubernetes-version x.y.z>" >&2; exit 2; }
TMINOR="${TARGET%.*}"

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
TALOS_CTX="${TALOS_CTX:-admin@the-cluster}"
VCTX="${VCTX:-admin@media-cluster}"
NODES=(10.0.4.101 10.0.4.102 10.0.4.103)
CACHE="${XDG_CACHE_HOME:-$HOME/.cache}/talos-ladder"; mkdir -p "$CACHE"
LOG="${LADDER_LOG:-$CACHE/k8s-$(date -u +%Y%m%dT%H%M%SZ)-v$TARGET.log}"
ALLOW_BROKEN_RE='^trivy-system/Job/scan-vulnerabilityreport-'

log() { printf '[%s] %s\n' "$(date -u +%H:%M:%SZ)" "$*" | tee -a "$LOG"; }
die() { log "STOP: $*"; log "log: $LOG"; exit 1; }
trap 'die "interrupted"' INT TERM

# --- clients --------------------------------------------------------------------
# tools.sh hard-pins kubectl to a minor; choose the one within skew of both ends.
# shellcheck source=/dev/null
source "$ROOT/.claude/skills/homelab-toolchain/scripts/tools.sh" 2>/dev/null || true
CUR_K8S="$("$KUBECTL" --context "${CTX:-admin@the-cluster}" version -o json 2>/dev/null | "$JQ" -r '.serverVersion.gitVersion' | sed 's/^v//')"
[ -n "$CUR_K8S" ] || die "cannot read the apiserver version"
CUR_MINOR="${CUR_K8S%.*}"
STEP=$(( ${TMINOR#*.} - ${CUR_MINOR#*.} ))
[ "$STEP" -ge 0 ] && [ "$STEP" -le 1 ] || die "Kubernetes $CUR_K8S -> $TARGET is not one minor"
KUBECTL=""   # not tools.sh's pin: if neither minor is present this must stay empty and stop us
for want in "$TMINOR" "$CUR_MINOR"; do
  k="$(nixbin kubectl kubectl "$want")"
  [ -n "$k" ] && { KUBECTL="$k"; break; }
done
[ -n "${KUBECTL:-}" ] && [ -x "$KUBECTL" ] || die "no kubectl $TMINOR or $CUR_MINOR in the store (have: $(nixbin_versions kubectl | tr '\n' ' '))"

# deadline <seconds> <command...>: the same hard limit as ladder.sh, for the
# same reason -- a Talos API call against a node whose etcd is down waits
# forever, and Go ignores SIGALRM. 124 if it ran out.
kill_tree() { local c; for c in $(pgrep -P "$1" 2>/dev/null); do kill_tree "$c"; done; kill -TERM "$1" 2>/dev/null; }
deadline() {
  local end=$(( SECONDS + $1 )); shift
  "$@" &
  local pid=$!
  while kill -0 "$pid" 2>/dev/null; do
    if [ "$SECONDS" -ge "$end" ]; then
      kill_tree "$pid"; sleep 2; kill -KILL "$pid" 2>/dev/null
      wait "$pid" 2>/dev/null; return 124
    fi
    sleep 0.2
  done
  wait "$pid"
}

TALOS_RUNNING="$(deadline 60 "$TALOSCTL" --context "$TALOS_CTX" -n "${NODES[0]}" version --short 2>/dev/null | awk '/Tag:/{sub(/^v/,"",$2); print $2; exit}')"
[ -n "$TALOS_RUNNING" ] || die "cannot read the Talos version"
TC="$CACHE/talosctl-$TALOS_RUNNING-$(uname -s | tr A-Z a-z)-$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')"
[ -x "$TC" ] || die "no verified talosctl $TALOS_RUNNING in $CACHE -- run ladder.sh for that version first (it fetches and checks one)"
tc() { deadline "${TC_DEADLINE:-60}" "$TC" --context "$TALOS_CTX" "$@"; }

kc() {
  local i out
  for i in 1 2 3 4 5 6; do
    if out="$(deadline "${KC_DEADLINE:-120}" "$KUBECTL" --context "${CTX:-admin@the-cluster}" "$@" 2>&1)"; then printf '%s\n' "$out"; return 0; fi
    sleep 10
  done
  printf '%s\n' "$out" >&2; return 1
}
until_ok() {
  local timeout="$1" desc="$2"; shift 2
  local start; start=$(date +%s)
  until "$@" >/dev/null 2>&1; do
    [ $(( $(date +%s) - start )) -ge "$timeout" ] && return 1
    sleep 10
  done
  log "  ok: $desc ($(( $(date +%s) - start ))s)"
}

# --- gates (the same definitions as ladder.sh) -----------------------------------------
g_nodes_ready() {
  kc get nodes -o json | "$JQ" -e '[.items[] | .status.conditions[] | select(.type=="Ready") | .status] | (length == 3 and all(. == "True"))' >/dev/null
}
g_etcd() {
  local up m s alarms
  for up in "${NODES[@]}"; do m="$(tc -n "$up" etcd members 2>/dev/null)" && break; done
  [ "$(printf '%s\n' "$m" | awk 'NR>1 && NF && $NF=="false"' | wc -l | tr -d ' ')" = 3 ] || return 1
  alarms="$(tc -n "$up" etcd alarm list 2>/dev/null | sed '/^$/d')" || return 1
  [ -z "$alarms" ] || return 1
  s="$(tc -n "$(IFS=,; echo "${NODES[*]}")" etcd status 2>/dev/null)" || return 1
  printf '%s\n' "$s" | awk 'NR>1 && NF>=14 {n++; if (NF>14) bad=1; if ($12!="false") bad=1; ix[n]=$9}
    END { if (n!=3 || bad) exit 1; mx=ix[1]; mn=ix[1]; for (i in ix) { if (ix[i]>mx) mx=ix[i]; if (ix[i]<mn) mn=ix[i] }; exit (mx-mn > 1000) }'
}
broken_now() {
  {
  kc get pods -A -o json | "$JQ" -r '.items[] | select(.status.phase != "Succeeded")
    | select(.status.phase != "Running" or ([.status.containerStatuses[]?.ready] | all | not))
    | "\(.metadata.namespace)/\((.metadata.ownerReferences // [{}])[0].kind // "Pod")/\((.metadata.ownerReferences // [{}])[0].name // .metadata.name)"' \
    | grep -vE "$ALLOW_BROKEN_RE"
  kc -n argocd get applications -o json | "$JQ" -r --arg re "${ARGO_BAD_RE:-^(Degraded|Missing|Unknown)$}" \
    '.items[] | select((.status.health.status // "Unknown") | test($re)) | "argocd/Application/\(.metadata.name)"'
  } | sort -u
}
BASELINE_BROKEN=""
g_nothing_new_broken() { [ -z "$(comm -13 <(printf '%s\n' "$BASELINE_BROKEN" | sed '/^$/d') <(broken_now))" ]; }
new_broken() { comm -13 <(printf '%s\n' "$BASELINE_BROKEN" | sed '/^$/d') <(broken_now); }
g_vcluster_api() { deadline 30 "$KUBECTL" --context "$VCTX" get --raw /readyz >/dev/null 2>&1; }
g_kubelets_at() { kc get nodes -o json | "$JQ" -e --arg v "v$1" '[.items[].status.nodeInfo.kubeletVersion] | (length == 3 and all(. == $v))' >/dev/null; }
g_apiserver_at() { [ "$(deadline 30 "$KUBECTL" --context "${CTX:-admin@the-cluster}" version -o json 2>/dev/null | "$JQ" -r '.serverVersion.gitVersion')" = "v$1" ]; }
g_cp_pods_all() {
  kc -n kube-system get pods -o json | "$JQ" -e '[.items[] | select(.metadata.name | test("^kube-(apiserver|controller-manager|scheduler)-")) | ([.status.containerStatuses[]?.ready] | all)] | (length == 9 and all)' >/dev/null
}
# A write that goes through every admission webhook and persists nothing.
g_admission_write() { deadline 60 "$KUBECTL" --context "${CTX:-admin@the-cluster}" create namespace "k8s-upgrade-probe-$$" --dry-run=server -o name >/dev/null 2>&1; }

# --- run --------------------------------------------------------------------------
log "k8s-upgrade: $CUR_K8S -> $TARGET$( [ "$DRY" = 1 ] && echo ', DRY RUN' )"
log "clients: kubectl $("$KUBECTL" version --client -o json | "$JQ" -r .clientVersion.gitVersion), talosctl $TALOS_RUNNING (matches the nodes)"
[ "$CUR_K8S" = "$TARGET" ] && { log "already on $TARGET"; exit 0; }

until_ok 300 "all nodes Ready" g_nodes_ready || die "nodes not all Ready"
until_ok 300 "etcd healthy" g_etcd || die "etcd not healthy"
until_ok 120 "vcluster API answering" g_vcluster_api || die "the vcluster's API is not answering before we start"
until_ok 120 "admission webhooks accept a dry-run write" g_admission_write || die "a server-side dry-run write already fails -- fix that first"
BASELINE_BROKEN="$(ARGO_BAD_RE='^(Degraded|Missing|Unknown|Progressing)$' broken_now)"
log "already not Ready before this run (not held against it): $(printf '%s' "$BASELINE_BROKEN" | tr '\n' ' ')"

log "Talos's plan (--dry-run):"
TC_DEADLINE=900 tc -n "${NODES[0]}" upgrade-k8s --to "$TARGET" --dry-run --manifests-no-prune >>"$LOG" 2>&1 \
  || die "upgrade-k8s --dry-run failed -- see the log"
log "  ok: dry-run passed"
[ "$DRY" = 1 ] && { log "DRY RUN -- stopping here"; exit 0; }

T0=$(date +%s)
log "upgrade-k8s --to $TARGET (pre-pulls images, then control plane, then kubelets)"
# Driven from this client: a kill stops the roll part-way (inside skew; re-run
# to finish), so the limit is generous -- it only exists so a hang ends.
TC_DEADLINE=7200 tc -n "${NODES[0]}" upgrade-k8s --to "$TARGET" --manifests-no-prune --with-docs=false --with-examples=false >>"$LOG" 2>&1 \
  || die "upgrade-k8s returned an error -- see the log; the control plane may be part-way"
log "  ok: upgrade-k8s finished ($(( ($(date +%s) - T0) / 60 ))m)"

until_ok 600 "apiserver reports v$TARGET" g_apiserver_at "$TARGET" || die "apiserver is not on v$TARGET"
until_ok 900 "every kubelet reports v$TARGET" g_kubelets_at "$TARGET" || die "kubelets are not all on v$TARGET"
until_ok 600 "control-plane pods Ready on every node" g_cp_pods_all || die "control-plane static pods not all Ready"
until_ok 600 "all nodes Ready" g_nodes_ready || die "nodes not all Ready"
until_ok 300 "etcd healthy" g_etcd || die "etcd not healthy after the upgrade"
until_ok 300 "admission webhooks accept a dry-run write" g_admission_write || die "server-side dry-run writes fail after the upgrade -- check Kyverno first"
until_ok 300 "vcluster API answering" g_vcluster_api || die "the vcluster's API stopped answering"
until_ok 900 "workloads settled (nothing newly broken)" g_nothing_new_broken || die "newly broken after the upgrade: $(new_broken | tr '\n' ' ')"
log "done: Kubernetes v$TARGET in $(( ($(date +%s) - T0) / 60 ))m"
log "log: $LOG"
