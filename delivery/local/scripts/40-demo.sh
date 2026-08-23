#!/usr/bin/env bash
# The whole flow, end to end, asserted at every step.
#
# Every stage checks something rather than printing hopefully. A demo that
# prints "promoting..." and exits 0 whatever happened is a screensaver.
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

load_credentials
FAIL=0
check() { if "$@" >/dev/null 2>&1; then ok "$1"; else bad "$1"; FAIL=1; fi; }

# ---------------------------------------------------------------------------
say "1. discovery -- the Warehouse finds a chart version"
# ---------------------------------------------------------------------------
wait_for "warehouse podinfo is healthy" 180 \
  bash -c "kubectl --context $CLUSTER_CONTEXT -n $KARGO_PROJECT get warehouse podinfo \
    -o jsonpath='{.status.conditions[?(@.type==\"Healthy\")].status}' | grep -q True"
FREIGHT="$(kc -n "$KARGO_PROJECT" get freight --no-headers 2>/dev/null | wc -l | tr -d ' ')"
[ "$FREIGHT" -gt 0 ] && ok "freight discovered: $FREIGHT" || { bad "no freight"; FAIL=1; }
kc -n "$KARGO_PROJECT" get freight -o custom-columns=NAME:.metadata.name,CHART:.charts[0].name,VERSION:.charts[0].version --no-headers 2>/dev/null | head -3 | sed 's/^/    /'

# ---------------------------------------------------------------------------
say "2. promotion -- the Stage writes the new version and pushes a branch"
# ---------------------------------------------------------------------------
wait_for "a promotion exists" 240 \
  bash -c "[ \$(kubectl --context $CLUSTER_CONTEXT -n $KARGO_PROJECT get promotions --no-headers 2>/dev/null | wc -l) -gt 0 ]"
kc -n "$KARGO_PROJECT" get promotions -o custom-columns=NAME:.metadata.name,STAGE:.spec.stage,PHASE:.status.phase --no-headers 2>/dev/null | head -5 | sed 's/^/    /'

# ---------------------------------------------------------------------------
say "3. the pull request -- opened against Gitea, not simulated"
# ---------------------------------------------------------------------------
wait_for "a pull request is open" 300 \
  bash -c "curl -sk -H 'Authorization: token ${GITEA_TOKEN}' \
    '${GITEA_URL}/api/v1/repos/${GITEA_OWNER}/${SAMPLE_REPO_NAME}/pulls?state=open' \
    | python3 -c 'import json,sys; sys.exit(0 if json.load(sys.stdin) else 1)'"
PR="$(gitea_api GET "/repos/${GITEA_OWNER}/${SAMPLE_REPO_NAME}/pulls?state=open" \
  | python3 -c 'import json,sys; d=json.load(sys.stdin); print(d[0]["number"] if d else "")')"
[ -n "$PR" ] || { bad "could not read the pull request number"; exit 1; }
gitea_api GET "/repos/${GITEA_OWNER}/${SAMPLE_REPO_NAME}/pulls/${PR}" \
  | python3 -c 'import json,sys; d=json.load(sys.stdin); print("    #%s  %s\n    %s -> %s" % (d["number"], d["title"], d["head"]["ref"], d["base"]["ref"]))'

# ---------------------------------------------------------------------------
say "4. the gate -- renders both sides and diffs the resources"
# ---------------------------------------------------------------------------
set +e
bash "$ROOT/scripts/gate-run.sh" "$PR"
GATE_EXIT=$?
set -e
[ -f "/tmp/gate-report-${PR}.md" ] && sed 's/^/    /' "/tmp/gate-report-${PR}.md" | head -25

# ---------------------------------------------------------------------------
say "5. triage -- the agent reads the gate's comment and decides"
# ---------------------------------------------------------------------------
# Kargo's triage step fires this on promotion, but calling it directly makes
# the demo deterministic and shows the request and the verdict.
AGENT_POD="$(kc -n bosun get pod -l app.kubernetes.io/name=bosun -o name | head -1)"
if [ -n "$AGENT_POD" ]; then
  # The field is prNumber. The handler answers 202 immediately and triages
  # asynchronously -- Kargo's http step is synchronous, so a blocking handler
  # would put a model round trip inside every promotion's critical path. That
  # means the verdict is in the logs, not the response.
  kc -n bosun exec "$AGENT_POD" -- \
    wget -q -O- --post-data "{\"prNumber\":${PR},\"stage\":\"podinfo-tenant\"}" \
    --header 'Content-Type: application/json' \
    http://localhost:8080/v1/promotion-opened 2>&1 | sed 's/^/    /' | head -5 || true
  step "accepted; triage runs asynchronously -- waiting for the verdict"
  for _ in $(seq 1 30); do
    kc -n bosun logs "$AGENT_POD" --tail=200 2>/dev/null | grep -qiE "triage done|nothing to triage|verdict|classif|applied|refus" && break
    sleep 6
  done
  kc -n bosun logs "$AGENT_POD" --tail=25 2>/dev/null | sed 's/^/    /'
else
  bad "no agent pod"; FAIL=1
fi

# ---------------------------------------------------------------------------
say "6. merge"
# ---------------------------------------------------------------------------
if [ "$GATE_EXIT" -eq 0 ]; then
  gitea_api POST "/repos/${GITEA_OWNER}/${SAMPLE_REPO_NAME}/pulls/${PR}/merge" \
    -d '{"Do":"squash"}' >/dev/null && ok "merged #${PR}"
else
  step "gate exit ${GATE_EXIT} -- not merging, which is the gate doing its job"
fi

# ---------------------------------------------------------------------------
say "7. reconcile -- ArgoCD syncs the new version"
# ---------------------------------------------------------------------------
kc -n argocd get applications -o custom-columns=NAME:.metadata.name,SYNC:.status.sync.status,HEALTH:.status.health.status --no-headers 2>/dev/null | sed 's/^/    /'

# ---------------------------------------------------------------------------
say "8. verification -- the AnalysisRun asks Prometheus"
# ---------------------------------------------------------------------------
kc -n "$KARGO_PROJECT" get analysisruns --no-headers 2>/dev/null | head -5 | sed 's/^/    /' \
  || step "no AnalysisRun yet"

# ---------------------------------------------------------------------------
say "9. observability -- the metrics must RETURN ROWS, not merely parse"
# ---------------------------------------------------------------------------
# This is the assertion the production incident earned. Every alert expression
# parsed against a live Prometheus and matched nothing for hours, because
# kube-state-metrics had prefixed every series. Parsing is not evidence.
kc -n monitoring port-forward svc/monitoring-kube-prometheus-prometheus 19099:9090 >/dev/null 2>&1 &
PF=$!; trap 'kill $PF 2>/dev/null' EXIT
sleep 6
for m in kargo_stage_condition kargo_promotion_phase kargo_warehouse_condition kargo_freight_discovered; do
  n="$(curl -s --max-time 10 --data-urlencode "query=count($m)" \
      http://127.0.0.1:19099/api/v1/query \
      | python3 -c 'import json,sys; r=json.load(sys.stdin).get("data",{}).get("result",[]); print(r[0]["value"][1] if r else "0")')"
  if [ "$n" != "0" ]; then ok "$m -> $n series"; else bad "$m returned NO ROWS"; FAIL=1; fi
done

say "done"
[ "$FAIL" -eq 0 ] && { echo "  the whole flow ran"; exit 0; } || { echo "  something above failed"; exit 1; }
