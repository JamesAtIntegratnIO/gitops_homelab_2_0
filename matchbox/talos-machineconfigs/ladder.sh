#!/usr/bin/env bash
# Upgrade every control-plane node of the-cluster to ONE Talos version, one node
# at a time, behind gates. One rung of the ladder per invocation.
#
#   ./matchbox/talos-machineconfigs/ladder.sh --dry-run 1.12.12   # gates + plan, changes nothing
#   ./matchbox/talos-machineconfigs/ladder.sh 1.12.12             # do it
#
# Idempotent: nodes already on the target are skipped, so a rung that stopped
# half way is resumed by running the same command again.
#
# It never forces anything. No `talosctl upgrade --force` (skips etcd checks),
# no drain that bypasses a PodDisruptionBudget. Anything that does not pass
# stops the run, and a node it cordoned but had not yet upgraded is uncordoned.
#
# What it does, per rung:
#   - checks the target exists, is the next step (same minor, or minor + 1),
#     and that the Image Factory has a metal-installer for the nodes' schematic;
#   - uses a talosctl client matching the version the nodes RUN (Sidero's
#     upgrade FAQ), downloaded once and checked against the release checksums;
#   - on a fresh rung, requires preflight.sh <target> to pass, and takes a fresh
#     host etcd snapshot through the talos-etcd-snapshot CronJob;
# per node (others first, the etcd leader next, the VIP holder last):
#   - gates: nodes Ready, etcd 3 voting members / no learners / no alarms /
#     errors empty, core Talos services healthy, control-plane pods Ready,
#     nothing broken that was not broken at the start;
#   - cordons and drains the node itself with a 15-minute budget, so a PDB that
#     needs a replacement pod to come Ready elsewhere has time to. Talos's own
#     drain gives up after 5 minutes and aborts the upgrade;
#   - `talosctl upgrade --image <factory metal-installer for the schematic>`;
#   - waits for the new version, Ready, the same extensions as before, etcd
#     healthy and caught up, services and control-plane pods back;
#   - waits for the workloads to settle, then spreads the vcluster etcd members
#     back to one per node: a drain leaves two on one node (its anti-affinity is
#     only preferred), and then losing that node costs the tenant its quorum.
#
# Exit: 0 rung complete, 1 stopped by a gate (cluster left safe), 2 usage.

set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=/dev/null
source "$ROOT/.claude/skills/homelab-toolchain/scripts/tools.sh" 2>/dev/null || true

DRY=0
[ "${1:-}" = "--dry-run" ] && { DRY=1; shift; }
TARGET="${1:-}"
TARGET="${TARGET#v}"
[[ "$TARGET" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "usage: $0 [--dry-run] <talos-version x.y.z>" >&2; exit 2; }

TALOS_CTX="${TALOS_CTX:-admin@the-cluster}"
NODES=(10.0.4.101 10.0.4.102 10.0.4.103)
VIP=10.0.4.100
CACHE="${XDG_CACHE_HOME:-$HOME/.cache}/talos-ladder"
mkdir -p "$CACHE"
LOG="${LADDER_LOG:-$CACHE/ladder-$(date -u +%Y%m%dT%H%M%SZ)-v$TARGET.log}"
# Pods that are allowed to stay unschedulable: Trivy's scan Jobs are kept off
# .102 by affinity and ask for more CPU than a full node has spare.
ALLOW_BROKEN_RE='^trivy-system/Job/scan-vulnerabilityreport-'

log() { printf '[%s] %s\n' "$(date -u +%H:%M:%SZ)" "$*" | tee -a "$LOG"; }
CORDONED=""        # node cordoned by this run and not yet upgraded
die() {
  log "STOP: $*"
  if [ -n "$CORDONED" ]; then
    log "uncordoning $CORDONED -- it was cordoned by this run and not upgraded"
    kc uncordon "$CORDONED" >>"$LOG" 2>&1 || log "WARNING: could not uncordon $CORDONED; run: kubectl uncordon $CORDONED"
  fi
  log "log: $LOG"
  exit 1
}
trap 'die "interrupted"' INT TERM

# kubectl through the VIP, with retries: while the VIP holder reboots the
# address refuses connections for ~45s.
kc() {
  local i out
  for i in 1 2 3 4 5 6; do
    if out="$("$KUBECTL" --context "${CTX:-admin@the-cluster}" "$@" 2>&1)"; then printf '%s\n' "$out"; return 0; fi
    sleep 10
  done
  printf '%s\n' "$out" >&2
  return 1
}

# until_ok <timeout-seconds> <description> <command...>
until_ok() {
  local timeout="$1" desc="$2"; shift 2
  local start; start=$(date +%s)
  until "$@" >/dev/null 2>&1; do
    if [ $(( $(date +%s) - start )) -ge "$timeout" ]; then return 1; fi
    sleep 10
  done
  log "  ok: $desc ($(( $(date +%s) - start ))s)"
}

# --- talosctl clients ---------------------------------------------------------
client_for() { # client_for <version> -> path to a verified talosctl of that version
  local v="$1" os arch asset out want got
  case "$(uname -s)" in Darwin) os=darwin ;; Linux) os=linux ;; *) die "unsupported OS" ;; esac
  case "$(uname -m)" in arm64|aarch64) arch=arm64 ;; x86_64|amd64) arch=amd64 ;; *) die "unsupported arch" ;; esac
  asset="talosctl-$os-$arch"
  out="$CACHE/talosctl-$v-$os-$arch"
  if [ ! -x "$out" ]; then
    curl -fsSL -o "$out.tmp" "https://github.com/siderolabs/talos/releases/download/v$v/$asset" || die "cannot download talosctl $v"
    want="$(curl -fsSL "https://github.com/siderolabs/talos/releases/download/v$v/sha256sum.txt" | awk -v a="$asset" '$2 ~ a"$" {print $1}')"
    got="$(shasum -a 256 "$out.tmp" 2>/dev/null | cut -d' ' -f1 || sha256sum "$out.tmp" | cut -d' ' -f1)"
    [ -n "$want" ] && [ "$want" = "$got" ] || { rm -f "$out.tmp"; die "talosctl $v checksum mismatch (got $got, release says ${want:-nothing})"; }
    mv "$out.tmp" "$out"; chmod 755 "$out"
  fi
  printf '%s\n' "$out"
}
TC=""   # the client for this rung
tc() { "$TC" --context "$TALOS_CTX" "$@"; }

node_version() { tc -n "$1" version --short 2>/dev/null | awk '/Tag:/{sub(/^v/,"",$2); print $2; exit}'; }
node_name() { kc get nodes -o json | "$JQ" -r --arg ip "$1" '.items[] | select(any(.status.addresses[]; .type=="InternalIP" and .address==$ip)) | .metadata.name'; }
# Capture, then match. `talosctl ... | grep -q` under pipefail reports FAILURE
# exactly when it matches: grep exits early, talosctl takes SIGPIPE (141).
vip_holder() { local n a; for n in "${NODES[@]}"; do a="$(tc -n "$n" get addresses 2>/dev/null)"; [[ "$a" == *" $VIP/"* ]] && { echo "$n"; return; }; done; }
etcd_leader() { tc -n "$(IFS=,; echo "${NODES[*]}")" etcd status 2>/dev/null | awk 'NR>1 {id[$2]=$1; ld=$8} END{print id[ld]}'; }
minor_of() { local m="${1#*.}"; echo "${m%%.*}"; }   # 1.11.5 -> 11

# --- gates ----------------------------------------------------------------------
g_nodes_ready() {
  kc get nodes -o json | "$JQ" -e '[.items[] | .status.conditions[] | select(.type=="Ready") | .status] | (length == 3 and all(. == "True"))' >/dev/null
}
g_etcd() { # 3 voting members, no learners, no alarms, every member answering with no errors
  local up m s alarms
  for up in "${NODES[@]}"; do m="$(tc -n "$up" etcd members 2>/dev/null)" && break; done
  [ "$(printf '%s\n' "$m" | awk 'NR>1 && NF && $NF=="false"' | wc -l | tr -d ' ')" = 3 ] || return 1
  alarms="$(tc -n "$up" etcd alarm list 2>/dev/null | sed '/^$/d')" || return 1
  [ -z "$alarms" ] || return 1
  s="$(tc -n "$(IFS=,; echo "${NODES[*]}")" etcd status 2>/dev/null)" || return 1
  # columns: NODE MEMBER DB(2) INUSE(3) LEADER INDEX TERM APPLIED LEARNER PROTO STORAGE [ERRORS...]
  printf '%s\n' "$s" | awk 'NR>1 && NF>=14 {n++; if (NF>14) bad=1; if ($12!="false") bad=1; ix[n]=$9}
    END { if (n!=3 || bad) exit 1; mx=ix[1]; mn=ix[1]; for (i in ix) { if (ix[i]>mx) mx=ix[i]; if (ix[i]<mn) mn=ix[i] }; exit (mx-mn > 1000) }'
}
g_services() { # g_services <node-ip>
  tc -n "$1" services 2>/dev/null | awk 'NR>1 && $2 ~ /^(apid|containerd|cri|etcd|kubelet|machined|trustd)$/ { n++; if ($3!="Running" || $4!="OK") bad=1 } END { exit !(n==7 && !bad) }'
}
g_cp_pods() { # g_cp_pods <node-name>
  kc -n kube-system get pods -o json | "$JQ" -e --arg n "$1" '[.items[] | select(.spec.nodeName==$n) | select(.metadata.name | test("^kube-(apiserver|controller-manager|scheduler)-")) | ([.status.containerStatuses[]?.ready] | all)] | (length == 3 and all)' >/dev/null
}
broken_now() { # namespace/OwnerKind/ownerName of every pod not Running+Ready (minus the allowlist), and unhealthy ArgoCD apps
  {
  kc get pods -A -o json | "$JQ" -r '.items[] | select(.status.phase != "Succeeded")
    | select(.status.phase != "Running" or ([.status.containerStatuses[]?.ready] | all | not))
    | "\(.metadata.namespace)/\((.metadata.ownerReferences // [{}])[0].kind // "Pod")/\((.metadata.ownerReferences // [{}])[0].name // .metadata.name)"' \
    | grep -vE "$ALLOW_BROKEN_RE"
  # ArgoCD sees what pod readiness cannot: a workload that is up but wrong.
  # Progressing is not "broken" now, but it counts as already-unhealthy in the
  # baseline (BASELINE_BROKEN passes a wider ARGO_BAD_RE), so an app that was mid-recovery when the
  # run started and later settles as Degraded does not stop the ladder.
  kc -n argocd get applications -o json | "$JQ" -r --arg re "${ARGO_BAD_RE:-^(Degraded|Missing|Unknown)$}" \
    '.items[] | select((.status.health.status // "Unknown") | test($re)) | "argocd/Application/\(.metadata.name)"'
  } | sort -u
}
BASELINE_BROKEN=""
g_nothing_new_broken() { [ -z "$(comm -13 <(printf '%s\n' "$BASELINE_BROKEN" | sed '/^$/d') <(broken_now))" ]; }
new_broken() { comm -13 <(printf '%s\n' "$BASELINE_BROKEN" | sed '/^$/d') <(broken_now); }

# --- vcluster etcd: one member per node ----------------------------------------------
vc_etcd_by_node() { kc -n vcluster-media get pods -l app=vcluster-etcd -o json | "$JQ" -r '.items[] | "\(.spec.nodeName)\t\(.metadata.name)\t\([.status.containerStatuses[]?.ready] | all)"'; }
g_vc_etcd_ready() { [ "$(vc_etcd_by_node | awk -F'\t' '$3=="true"' | wc -l | tr -d ' ')" = 3 ]; }
g_vc_pdb_allows() { kc -n vcluster-media get pdb vcluster-media-etcd -o json | "$JQ" -e '.status.disruptionsAllowed >= 1' >/dev/null; }
spread_vc_etcd() {
  local attempt dup pod
  for attempt in 1 2 3; do
    dup="$(vc_etcd_by_node | awk -F'\t' '{c[$1]++; p[$1]=$2} END {for (n in c) if (c[n]>1) {print p[n]; exit}}')"
    [ -z "$dup" ] && { log "  ok: vcluster etcd spread one per node"; return 0; }
    until_ok 300 "vcluster etcd PDB allows a disruption" g_vc_pdb_allows || die "vcluster etcd PDB never allowed a disruption"
    log "  moving vcluster etcd member $dup off a node that holds two"
    [ "$DRY" = 1 ] && return 0
    kc -n vcluster-media delete pod "$dup" --wait=true >>"$LOG" 2>&1 || die "could not delete $dup"
    sleep 15
    until_ok 600 "vcluster etcd 3/3 Ready again" g_vc_etcd_ready || die "vcluster etcd did not recover after moving $dup"
  done
  log "  WARNING: vcluster etcd still has two members on one node after 3 moves (anti-affinity is only preferred)"
}

# --- the rung ---------------------------------------------------------------------
log "ladder: target Talos v$TARGET, context $TALOS_CTX$( [ "$DRY" = 1 ] && echo ', DRY RUN' )"

# Which nodes still need it, and what they run now. Reading `version` works with
# any recent client, so use the target's to look before choosing the real one.
# (client_for runs in a subshell: its die only ends that, hence the `|| exit 1`.)
TC="$(client_for "$TARGET")" || exit 1
PENDING=(); VERS=""; FROM=""
for n in "${NODES[@]}"; do
  v="$(node_version "$n")"
  [ -n "$v" ] || die "cannot read the Talos version of $n"
  VERS="$VERS$n=$v "
  if [ "$v" != "$TARGET" ]; then
    PENDING+=("$n")
    [ -z "$FROM" ] && FROM="$v"
    [ "$v" = "$FROM" ] || die "nodes still to upgrade run different versions ($v vs $FROM); finish that first"
  fi
done
log "running: $VERS"
[ "${#PENDING[@]}" -eq 0 ] && { log "every node already runs v$TARGET -- nothing to do"; exit 0; }

STEP=$(( $(minor_of "$TARGET") - $(minor_of "$FROM") ))
[ "$STEP" -ge 0 ] && [ "$STEP" -le 1 ] || die "v$FROM -> v$TARGET is not the next step (one minor at a time)"
printf '%s\n%s\n' "$FROM" "$TARGET" | sort -V -C || die "v$TARGET is older than the running v$FROM"

TC="$(client_for "$FROM")" || exit 1
log "client: talosctl $FROM (matches the nodes being upgraded) -- $TC"

# The image: from the schematic the nodes actually run.
SCHEMATIC="$(tc -n "$(IFS=,; echo "${NODES[*]}")" get extensions -o json 2>/dev/null | "$JQ" -r 'select(.spec.metadata.name=="schematic") | .spec.metadata.version' | sort -u)"
[ "$(printf '%s' "$SCHEMATIC" | grep -c .)" = 1 ] || die "nodes do not agree on one Image Factory schematic: '$SCHEMATIC'"
IMAGE="factory.talos.dev/metal-installer/$SCHEMATIC:v$TARGET"
curl -fsS --max-time 15 https://factory.talos.dev/versions | "$JQ" -e --arg v "v$TARGET" 'index($v) != null' >/dev/null || die "the Image Factory does not list v$TARGET"
for attempt in 1 2 3; do
  code="$(curl -s -o /dev/null -w '%{http_code}' --max-time 150 \
    -H 'Accept: application/vnd.oci.image.index.v1+json, application/vnd.oci.image.manifest.v1+json, application/vnd.docker.distribution.manifest.list.v2+json' \
    "https://factory.talos.dev/v2/metal-installer/$SCHEMATIC/manifests/v$TARGET")"
  case "$code" in 200|307) break ;; 404) die "the factory has no $IMAGE" ;; esac
  log "  factory answered HTTP $code for the installer (attempt $attempt); retrying"
done
case "$code" in 200|307) log "image: $IMAGE" ;; *) die "could not verify $IMAGE (HTTP $code)" ;; esac

# Extensions every node must still carry afterwards.
EXT_EXPECT="$(tc -n "${NODES[0]}" get extensions -o json 2>/dev/null | "$JQ" -r '.spec.metadata.name' | sort -u | tr '\n' ' ')"
log "extensions to preserve: $EXT_EXPECT"

# A fresh rung (every node on FROM) goes through the full pre-flight.
if [ "${#PENDING[@]}" -eq "${#NODES[@]}" ]; then
  log "fresh rung: running preflight.sh $TARGET"
  TALOSCTL="$TC" TALOS_CTX="$TALOS_CTX" "$ROOT/matchbox/talos-machineconfigs/preflight.sh" "$TARGET" >>"$LOG" 2>&1 \
    || die "preflight.sh $TARGET failed -- see the log"
  log "  ok: preflight passed"
else
  log "resuming a rung in progress (${#PENDING[@]} node(s) left); per-node gates apply"
fi

BASELINE_BROKEN="$(ARGO_BAD_RE='^(Degraded|Missing|Unknown|Progressing)$' broken_now)"
log "already not Ready before this run (not held against it): $(printf '%s' "$BASELINE_BROKEN" | tr '\n' ' ' | sed 's/ $//')"

if [ "$DRY" = 1 ]; then
  log "DRY RUN -- gates now:"
  g_nodes_ready && log "  ok: nodes Ready" || log "  FAIL: nodes Ready"
  g_etcd && log "  ok: etcd" || log "  FAIL: etcd"
  for n in "${NODES[@]}"; do g_services "$n" && log "  ok: services $n" || log "  FAIL: services $n"; done
  g_vc_etcd_ready && log "  ok: vcluster etcd 3/3" || log "  FAIL: vcluster etcd"
  log "  vcluster etcd placement: $(vc_etcd_by_node | awk -F'\t' '{print $1}' | sort | uniq -c | awk '{printf "%s x%s  ", $2, $1}')"
  log "  order (recomputed per node in a real run): leader=$(etcd_leader) vip=$(vip_holder); pending: ${PENDING[*]}"
  exit 0
fi

# Fresh snapshot of the host etcd before anything reboots, in the place the
# nightly ones go, rather than a copy of the cluster's secrets on a laptop.
JOB="talos-ladder-$(date -u +%Y%m%d%H%M%S)"
log "taking a fresh host etcd snapshot: talos-backup/$JOB"
kc -n talos-backup create job --from=cronjob/talos-etcd-snapshot "$JOB" >>"$LOG" 2>&1 || die "could not start the snapshot job"
# Not through kc: its retries would turn one 10-minute wait into an hour.
"$KUBECTL" --context "${CTX:-admin@the-cluster}" -n talos-backup wait --for=condition=complete "job/$JOB" --timeout=600s >>"$LOG" 2>&1 \
  || die "the etcd snapshot job did not complete -- not upgrading without a fresh snapshot"
log "  ok: snapshot job complete"

pick_next() { # others first, the etcd leader next, the VIP holder last
  local leader vip n best="" bestscore=9 s
  leader="$(etcd_leader)"; vip="$(vip_holder)"
  for n in "$@"; do
    s=0; [ "$n" = "$leader" ] && s=$((s+1)); [ "$n" = "$vip" ] && s=$((s+2))
    [ "$s" -lt "$bestscore" ] && { best="$n"; bestscore="$s"; }
  done
  echo "$best"
}

while :; do
  LEFT=()
  for n in "${NODES[@]}"; do [ "$(node_version "$n")" = "$TARGET" ] || LEFT+=("$n"); done
  [ "${#LEFT[@]}" -eq 0 ] && break
  IP="$(pick_next "${LEFT[@]}")"
  NAME="$(node_name "$IP")"
  [ -n "$NAME" ] || die "cannot map $IP to a Kubernetes node"
  T0=$(date +%s)
  log "=== $IP ($NAME): v$FROM -> v$TARGET  [leader=$(etcd_leader) vip=$(vip_holder)]"

  until_ok 300 "all nodes Ready" g_nodes_ready || die "nodes not all Ready"
  until_ok 300 "etcd healthy" g_etcd || die "etcd not healthy before touching $IP"
  for n in "${NODES[@]}"; do until_ok 300 "Talos services on $n" g_services "$n" || die "core Talos services unhealthy on $n"; done
  until_ok 600 "nothing broken that was not broken at the start" g_nothing_new_broken || die "newly broken before $IP: $(new_broken | tr '\n' ' ')"
  until_ok 300 "vcluster etcd 3/3" g_vc_etcd_ready || die "vcluster etcd not 3/3 Ready"
  spread_vc_etcd

  log "  cordon + drain $NAME (15m budget; PDB-respecting)"
  CORDONED="$NAME"
  kc cordon "$NAME" >>"$LOG" 2>&1 || die "cordon failed"
  "$KUBECTL" --context "${CTX:-admin@the-cluster}" drain "$NAME" --ignore-daemonsets --delete-emptydir-data --force --timeout=15m >>"$LOG" 2>&1 \
    || die "drain of $NAME did not finish within 15m (a PDB that could not be satisfied?) -- see the log"
  log "  ok: drained"

  log "  talosctl upgrade --image $IMAGE"
  CORDONED=""   # from here the node may be mid-reboot; never uncordon it blindly
  tc -n "$IP" upgrade --image "$IMAGE" --wait --timeout 30m >>"$LOG" 2>&1 \
    || { [ "$(node_version "$IP")" = "$FROM" ] && { CORDONED="$NAME"; die "upgrade of $IP failed before it rebooted -- still on v$FROM"; }; die "upgrade of $IP returned an error -- see the log"; }

  until_ok 1200 "$IP reports v$TARGET" sh -c "[ \"\$(\"$TC\" --context \"$TALOS_CTX\" -n $IP version --short 2>/dev/null | awk '/Tag:/{print \$2}')\" = v$TARGET ]" \
    || die "$IP did not come back on v$TARGET within 20m (if it hangs in kexec, retry with -m powercycle)"
  until_ok 600 "$NAME Ready" g_nodes_ready || die "$NAME did not become Ready"
  if [ "$(kc get node "$NAME" -o jsonpath='{.spec.unschedulable}')" = true ]; then
    kc uncordon "$NAME" >>"$LOG" 2>&1 || die "could not uncordon $NAME"
    log "  ok: uncordoned $NAME"
  fi
  got="$(tc -n "$IP" get extensions -o json 2>/dev/null | "$JQ" -r '.spec.metadata.name' | sort -u | tr '\n' ' ')"
  [ "$got" = "$EXT_EXPECT" ] || die "$IP extensions changed: had [$EXT_EXPECT], now [$got]"
  got="$(tc -n "$IP" get extensions -o json 2>/dev/null | "$JQ" -r 'select(.spec.metadata.name=="schematic") | .spec.metadata.version')"
  [ "$got" = "$SCHEMATIC" ] || die "$IP schematic changed: now '$got'"
  log "  ok: extensions and schematic preserved"
  until_ok 600 "etcd healthy and caught up" g_etcd || die "etcd not healthy after $IP"
  until_ok 300 "Talos services on $IP" g_services "$IP" || die "core services unhealthy on $IP"
  until_ok 600 "control-plane pods on $NAME" g_cp_pods "$NAME" || die "control-plane static pods not Ready on $NAME"
  until_ok 900 "workloads settled (nothing newly broken)" g_nothing_new_broken || die "still broken 15m after $IP: $(new_broken | tr '\n' ' ')"
  spread_vc_etcd
  sleep 60
  until_ok 300 "etcd healthy after a minute's soak" g_etcd || die "etcd unhealthy after soak"
  log "=== $IP done in $(( ($(date +%s) - T0) / 60 ))m"
done

log "rung complete: every node runs v$TARGET"
log "next: bump the talosctl overlay in flake.nix to $TARGET, then run the next rung"
log "log: $LOG"
