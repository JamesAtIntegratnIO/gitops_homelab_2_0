#!/usr/bin/env bash
# Nine real incidents, replayed against the LIVE agent, on real pull requests.
#
# These are not invented scenarios. Each one is something that actually
# happened to this platform -- MetalLB swapping its FRR sidecars for a
# DaemonSet, argo-cd 10.0.0 flipping NetworkPolicy creation on, NGF requiring a
# newer Gateway API, authentik refusing to skip a version -- and each is
# already written down once as an eval fixture. This reads those same fixtures
# so the thing the eval measures and the thing you watch cannot drift apart.
#
# HONEST ABOUT WHAT IS REPLAYED: the gate's REPORT is the recorded one from
# each incident, posted as the gate would post it, because reproducing
# fourteen upstream chart versions locally would prove nothing extra. The
# agent, the model, the pull requests, the reasoning and every commit it
# pushes are live.
#
#   usage: 60-demo-scenarios.sh [case-name-substring]
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

load_credentials
FILTER="${1:-}"
CASES_JSON="$(mktemp)"; trap 'rm -f "$CASES_JSON"' EXIT
(cd "$ROOT/../images/delivery-agent" && GOTOOLCHAIN=auto go run ./evals/export) > "$CASES_JSON"
TOTAL=$(python3 -c "import json;print(len(json.load(open('$CASES_JSON'))))")
say "$TOTAL recorded incidents; agent is live"

AGENT_POD="$(kc -n delivery-agent get pod -l app.kubernetes.io/name=delivery-agent -o name | head -1)"
[ -n "$AGENT_POD" ] || bad "no agent pod"
MODEL="$(kc -n delivery-agent get deploy delivery-agent-delivery-agent \
  -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="LLM_MODEL")].value}')"
step "model: ${MODEL}"

CLONE="https://${GITEA_OWNER}:${GITEA_TOKEN}@gitea.${IDP_HOST}:${IDP_PORT}/${GITEA_OWNER}/${SAMPLE_REPO_NAME}.git"
RESULTS="$(mktemp)"

for i in $(seq 0 $((TOTAL - 1))); do
  NAME=$(python3 -c "import json;print(json.load(open('$CASES_JSON'))[$i]['Name'])")
  [ -n "$FILTER" ] && case "$NAME" in *"$FILTER"*) ;; *) continue ;; esac

  WANT=$(python3 -c "import json;print(json.load(open('$CASES_JSON'))[$i]['WantClass'])")
  SUBJECT=$(python3 -c "import json;print(json.load(open('$CASES_JSON'))[$i]['Subject'])")
  say "${NAME}  (expected: ${WANT})"
  step "$SUBJECT"

  # --- a branch carrying this incident's repository fixture ---
  WORK="$(mktemp -d)"
  GIT_SSL_NO_VERIFY=true git clone -q "$CLONE" "$WORK/repo"
  git -C "$WORK/repo" config http.sslVerify false
  git -C "$WORK/repo" config user.name "kargo"
  git -C "$WORK/repo" config user.email "kargo@localtest.me"
  BRANCH="scenario/${NAME}"
  git -C "$WORK/repo" checkout -q -B "$BRANCH"
  python3 - "$CASES_JSON" "$i" "$WORK/repo" <<'PY'
import json, os, sys
case = json.load(open(sys.argv[1]))[int(sys.argv[2])]
root = sys.argv[3]
for path, content in (case.get("Files") or {}).items():
    full = os.path.join(root, path)
    os.makedirs(os.path.dirname(full), exist_ok=True)
    open(full, "w").write(content)
PY
  git -C "$WORK/repo" add -A
  git -C "$WORK/repo" commit -q -m "$SUBJECT"
  git -C "$WORK/repo" push -q --force "$CLONE" "$BRANCH"

  PR=$(gitea_api POST "/repos/${GITEA_OWNER}/${SAMPLE_REPO_NAME}/pulls" \
        -d "$(python3 -c "
import json,sys; print(json.dumps({'head':sys.argv[1],'base':'main','title':sys.argv[2]}))" "$BRANCH" "$SUBJECT")" \
      | python3 -c 'import json,sys; print(json.load(sys.stdin).get("number",""))')
  if [ -z "$PR" ]; then
    PR=$(gitea_api GET "/repos/${GITEA_OWNER}/${SAMPLE_REPO_NAME}/pulls?state=open" \
        | python3 -c "import json,sys;d=[p for p in json.load(sys.stdin) if p['head']['ref']=='$BRANCH'];print(d[0]['number'] if d else '')")
  fi
  [ -n "$PR" ] || { bad "could not open a pull request for ${NAME}"; rm -rf "$WORK"; continue; }
  step "pull request #${PR}"

  HEAD_SHA=$(gitea_api GET "/repos/${GITEA_OWNER}/${SAMPLE_REPO_NAME}/pulls/${PR}" \
    | python3 -c 'import json,sys;print(json.load(sys.stdin)["head"]["sha"])')

  # --- the gate's recorded verdict, published the way the gate publishes it ---
  python3 - "$CASES_JSON" "$i" > "$WORK/comment.json" <<'PY'
import json, sys
case = json.load(open(sys.argv[1]))[int(sys.argv[2])]
print(json.dumps({"body": "<!-- gitops-gate -->\n" + case["GateReport"]}))
PY
  gitea_api POST "/repos/${GITEA_OWNER}/${SAMPLE_REPO_NAME}/issues/${PR}/comments" \
    --data-binary @"$WORK/comment.json" >/dev/null
  gitea_api POST "/repos/${GITEA_OWNER}/${SAMPLE_REPO_NAME}/statuses/${HEAD_SHA}" \
    -d '{"context":"gate","state":"failure","description":"blocking change"}' >/dev/null
  step "gate report posted, status gate=failure"

  # --- the live agent ---
  BEFORE=$(kc -n delivery-agent logs "$AGENT_POD" 2>/dev/null | wc -l | tr -d ' ')
  BODY=$(python3 - "$CASES_JSON" "$i" "$PR" "$BRANCH" <<'PY'
import json, sys
case = json.load(open(sys.argv[1]))[int(sys.argv[2])]
print(json.dumps({
  "project": "delivery", "stage": "scenario", "promotion": case["Name"],
  "artifact": case["Subject"], "from": "", "to": "", "autoMerge": "never",
  "prNumber": int(sys.argv[3]), "branch": sys.argv[4],
  "files": sorted((case.get("Files") or {}).keys()),
  "verifyApps": [],
}))
PY
)
  kc -n delivery-agent exec -i "$AGENT_POD" -- \
    wget -q -O- --post-data "$BODY" --header 'Content-Type: application/json' \
    http://localhost:8080/v1/promotion-opened >/dev/null 2>&1 || true

  for _ in $(seq 1 60); do
    kc -n delivery-agent logs "$AGENT_POD" 2>/dev/null | tail -n +$((BEFORE + 1)) \
      | grep -qE "PR ${PR}: triage done" && break
    sleep 5
  done

  # --- what it did ---
  AFTER_SHA=$(gitea_api GET "/repos/${GITEA_OWNER}/${SAMPLE_REPO_NAME}/pulls/${PR}" \
    | python3 -c 'import json,sys;print(json.load(sys.stdin)["head"]["sha"])')
  GOT="escalate/no_action"
  if [ "$HEAD_SHA" != "$AFTER_SHA" ]; then
    GOT="mechanical"
    ok "pushed a fix: ${HEAD_SHA:0:7} -> ${AFTER_SHA:0:7}"
    GIT_SSL_NO_VERIFY=true git -C "$WORK/repo" fetch -q "$CLONE" "$BRANCH"
    git -C "$WORK/repo" diff --unified=0 "$HEAD_SHA" "$AFTER_SHA" \
      | grep -E '^[-+][^-+]' | sed 's/^/      /'
  else
    step "pushed nothing"
  fi
  gitea_api GET "/repos/${GITEA_OWNER}/${SAMPLE_REPO_NAME}/issues/${PR}/comments?limit=50" \
    | python3 -c '
import json,sys
cs=[c for c in json.load(sys.stdin) if not c["body"].startswith("<!-- gitops-gate -->")]
if cs:
    print("      " + cs[-1]["body"].strip().splitlines()[0][:150])'

  printf '%s\t%s\t%s\t%s\n' "$NAME" "$WANT" "$GOT" "$PR" >> "$RESULTS"
  rm -rf "$WORK"
done

say "summary"
printf '  %-38s %-11s %-11s %s\n' CASE EXPECTED OBSERVED PR
while IFS=$'\t' read -r n w g p; do
  mark=" "; [ "$w" = "$g" ] && mark="+"
  [ "$w" != mechanical ] && [ "$g" != mechanical ] && mark="+"
  printf '%s %-38s %-11s %-11s #%s\n' "$mark" "$n" "$w" "$g" "$p"
done < "$RESULTS"
echo
echo "  + means the agent's ACTION matched the case's class."
echo "  This shows whether it edited, not whether the edit was right --"
echo "  the eval suite checks the exact scalars. Run: go test ./evals/..."
rm -f "$RESULTS"
