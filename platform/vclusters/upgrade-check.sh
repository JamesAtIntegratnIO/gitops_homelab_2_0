#!/usr/bin/env bash
# Gate one rung of the vcluster-media upgrade ladder. See upgrades.md.
#
#   ./platform/vclusters/upgrade-check.sh snapshot                # manual etcd snapshot, prints the file
#   ./platform/vclusters/upgrade-check.sh baseline                # record what is true before the merge
#   ./platform/vclusters/upgrade-check.sh verify <chart> <k8s>    # compare after the merge
#
# `verify` needs the rung's targets, e.g. `verify 0.32.3 v1.34.3`, so that a rung
# that silently did nothing fails instead of comparing green against itself.
# Run it once ArgoCD has had time to roll; the first thing it checks is whether
# that happened. Only `snapshot` writes anything (one Job from the CronJob).
#
# Exit: 0 pass, 1 a check failed, 2 usage.

set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=/dev/null
source "$ROOT/.claude/skills/homelab-toolchain/scripts/tools.sh" 2>/dev/null || true
CTX="${CTX:-admin@the-cluster}"
VCTX="${VCTX:-admin@media-cluster}"
NS=vcluster-media
STATE="${XDG_CACHE_HOME:-$HOME/.cache}/vcluster-ladder"; mkdir -p "$STATE"
BASE="$STATE/baseline.json"

h() { "$KUBECTL" --context "$CTX" "$@"; }
v() { "$KUBECTL" --context "$VCTX" --request-timeout=30s "$@" 2>/dev/null; }
FAILS=0
pass() { printf 'PASS  %s\n' "$*"; }
warn() { printf 'WARN  %s\n' "$*"; }
fail() { printf 'FAIL  %s\n' "$*"; FAILS=$((FAILS+1)); }
check() { # check <label> <want> <got>
  if [ "$2" = "$3" ]; then pass "$1: $3"; else fail "$1: want $2, got ${3:-<empty>}"; fi
}

# Pods in `media` are the workloads the ladder must not touch: their identity is
# the check. Elsewhere HPAs and CronJobs replace pods on their own schedule, so
# there only the restart total and the not-running set mean anything.
snapshot_state() {
  # Files, not --argjson: a full pod list is over the kernel's argument limit.
  local tmp rc
  tmp="$(mktemp -d)"
  h -n "$NS" get pods -o json > "$tmp/h.json" &&
    v get pods -A -o json > "$tmp/v.json" &&
    h -n argocd get applications -o json > "$tmp/a.json" &&
    "$JQ" -n --slurpfile h "$tmp/h.json" --slurpfile v "$tmp/v.json" --slurpfile a "$tmp/a.json" '{
    media: [$v[0].items[] | select(.metadata.namespace=="media" and .status.phase=="Running") | {key: .metadata.name, value: .metadata.uid}] | from_entries,
    tenantRestarts: ([$v[0].items[].status.containerStatuses[]?.restartCount] | add // 0),
    etcdRestarts: ([$h[0].items[] | select(.metadata.labels.app=="vcluster-etcd") | .status.containerStatuses[]?.restartCount] | add // 0),
    notRunning: [$v[0].items[] | select(.status.phase!="Running" and .status.phase!="Succeeded") | "\(.metadata.namespace)/\(.metadata.name)"] | sort,
    unhealthyApps: [$a[0].items[] | select(.metadata.name | test("vcluster-media")) | select(.status.sync.status!="Synced" or .status.health.status!="Healthy") | .metadata.name] | sort
  }'
  rc=$?
  rm -rf "$tmp"
  return $rc
}

check_addons() {
  # A server-side dry run passes real admission, so a Kyverno, cert-manager or
  # ESO webhook that no longer answers fails here rather than at the next write.
  if v create namespace ladder-dryrun --dry-run=server >/dev/null; then pass "tenant admission: dry-run write accepted"
  else fail "tenant admission: dry-run write refused (webhooks?)"; fi
  local certs es
  certs="$(v get certificates -A -o json | "$JQ" -r '[.items[] | select(any(.status.conditions[]?; .type=="Ready" and .status=="True") | not)] | length')"
  check "cert-manager: Certificates not Ready" 0 "$certs"
  es="$(v get externalsecrets -A -o json | "$JQ" -r '[.items[] | select(any(.status.conditions[]?; .type=="Ready" and .status=="True") | not)] | length')"
  check "external-secrets: ExternalSecrets not Ready" 0 "$es"
  check "external-secrets: store synced from host" 1 "$(v get clustersecretstore --no-headers | grep -c onepassword-store)"
  check "cert-manager: issuer synced from host" 1 "$(v get clusterissuer --no-headers | grep -c letsencrypt-prod)"
  if [ "$(v top pods -n media --no-headers | wc -l | tr -d ' ')" -gt 0 ]; then pass "metrics.k8s.io answers in the tenant"
  else fail "metrics.k8s.io: kubectl top returned nothing"; fi
}

check_etcd() {
  local out
  out="$(h -n "$NS" exec vcluster-media-etcd-0 -- etcdctl --endpoints=https://localhost:2379 \
    --cacert=/run/config/pki/etcd-ca.crt --cert=/run/config/pki/etcd-server.crt --key=/run/config/pki/etcd-server.key \
    endpoint status --cluster -w json 2>/dev/null)"
  check "etcd members answering" 3 "$(printf '%s' "$out" | "$JQ" 'length')"
  check "etcd distinct leaders" 1 "$(printf '%s' "$out" | "$JQ" '[.[].Status.leader] | unique | length')"
}

case "${1:-}" in
  snapshot)
    job="pre-ladder-$(date -u +%Y%m%d%H%M)"
    h -n "$NS" create job --from=cronjob/vcluster-media-etcd-snapshot "$job" >/dev/null || exit 1
    h -n "$NS" wait --for=condition=complete "job/$job" --timeout=5m >/dev/null || { echo "snapshot did not complete" >&2; exit 1; }
    h -n "$NS" logs "job/$job" -c finalize | grep -E '^wrote ' || { echo "snapshot job logged no file" >&2; exit 1; }
    ;;
  baseline)
    snapshot_state > "$BASE" || { echo "could not read cluster state" >&2; exit 1; }
    "$JQ" '{mediaPods: (.media | length), tenantRestarts, etcdRestarts, notRunning, unhealthyApps}' "$BASE"
    check_etcd; check_addons
    echo "baseline: $BASE"
    [ "$FAILS" -eq 0 ] || exit 1
    ;;
  verify)
    [ $# -eq 3 ] || { echo "usage: $0 verify <chart-version> <k8s-version>" >&2; exit 2; }
    [ -f "$BASE" ] || { echo "no baseline: run '$0 baseline' before the merge" >&2; exit 2; }
    chart="$2"; k8s="$3"
    check "ArgoCD targetRevision" "$chart" "$(h -n argocd get application vcluster-vcluster-media -o jsonpath='{.spec.source.targetRevision}')"
    check "ArgoCD sync/health" "Synced/Healthy" "$(h -n argocd get application vcluster-vcluster-media -o jsonpath='{.status.sync.status}/{.status.health.status}')"
    check "promise request Ready" "True" "$(h -n platform-requests get vclusterorchestratorv2 vcluster-media -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}')"
    images="$(h -n "$NS" get statefulset vcluster-media -o jsonpath='{.spec.template.spec.containers[?(@.name=="syncer")].image} {.spec.template.spec.initContainers[?(@.name=="kubernetes")].image}{.spec.template.spec.containers[?(@.name=="kubernetes")].image}')"
    case "$images" in *"vcluster-oss:$chart"*) pass "syncer image: $chart" ;; *) fail "syncer image: want $chart in: $images" ;; esac
    case "$images" in *"kubernetes:$k8s"*) pass "kubernetes image: $k8s" ;; *) fail "kubernetes image: want $k8s in: $images" ;; esac
    for sts in vcluster-media vcluster-media-etcd; do
      check "$sts replicas ready" 3 "$(h -n "$NS" get statefulset "$sts" -o jsonpath='{.status.readyReplicas}')"
      check "$sts rollout finished" "$(h -n "$NS" get statefulset "$sts" -o jsonpath='{.status.updateRevision}')" "$(h -n "$NS" get statefulset "$sts" -o jsonpath='{.status.currentRevision}')"
    done
    check "tenant apiserver" "$k8s" "$(v get --raw /version | "$JQ" -r .gitVersion)"
    check_etcd
    now="$(snapshot_state)" || { fail "could not read cluster state"; exit 1; }
    replaced="$("$JQ" -n --argjson a "$(cat "$BASE")" --argjson b "$now" -r '[$a.media | to_entries[] | select($b.media[.key] != .value) | .key] | join(" ")')"
    check "media pods replaced" "" "$replaced"
    newbad="$("$JQ" -n --argjson a "$(cat "$BASE")" --argjson b "$now" -r '($b.notRunning - $a.notRunning) | join(" ")')"
    check "newly not-running tenant pods" "" "$newbad"
    newapps="$("$JQ" -n --argjson a "$(cat "$BASE")" --argjson b "$now" -r '($b.unhealthyApps - $a.unhealthyApps) | join(" ")')"
    check "newly unhealthy ArgoCD apps" "" "$newapps"
    d="$("$JQ" -n --argjson a "$(cat "$BASE")" --argjson b "$now" '$b.tenantRestarts - $a.tenantRestarts')"
    if [ "$d" -gt 0 ]; then warn "tenant container restarts up by $d since baseline (controllers can restart on an API blip; check they settled)"; else pass "tenant container restarts: no change"; fi
    # Above zero only: a rung that rolls etcd replaces the pods and resets the count.
    d="$("$JQ" -n --argjson a "$(cat "$BASE")" --argjson b "$now" '$b.etcdRestarts - $a.etcdRestarts')"
    if [ "$d" -gt 0 ]; then fail "etcd restarts up by $d since baseline"; else pass "etcd restarts: none since baseline"; fi
    check_addons
    [ "$FAILS" -eq 0 ] && { echo "rung $chart / $k8s: PASS"; exit 0; }
    echo "rung $chart / $k8s: $FAILS failed"; exit 1
    ;;
  *) echo "usage: $0 snapshot | baseline | verify <chart> <k8s>" >&2; exit 2 ;;
esac
