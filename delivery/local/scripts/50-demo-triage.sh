#!/usr/bin/env bash
# Act two: a pull request the gate REFUSES, and an agent that has to earn it.
#
# The happy-path demo is not a demonstration of the agent. When the gate is
# green the agent correctly does nothing -- "gate is green, nothing to
# triage" -- which is the right behaviour and a dull thing to watch.
#
# This opens a pull request the gate blocks: a version bump that arrives
# together with a changed destination namespace -- one word, changed in
# passing, that silently moves everything the Application deploys.
#
# The agent reads the gate's report and ESCALATES, with its reasoning, as a
# comment on the pull request. That is the correct verdict here and not a
# limitation: measured against both qwen3.5-9b and qwen3.8-27b, each
# independently concluded that a namespace move cannot be proven accidental
# from the rendered diff, which is exactly the calibration the prompt asks
# for. What you are watching is the agent refusing to guess, in public,
# with an argument.
#
# See README.md ("What the agent will and will not fix") for why a
# mechanical FIX does not appear here: the gate blocks on structural changes,
# and the agent's mechanical class is values conflicts, which the gate
# reports without blocking. The two barely intersect today.
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

load_credentials
BRANCH="broken/bump-and-namespace"

say "1. a pull request a person might write"
WORK="$(mktemp -d)"; trap 'rm -rf "$WORK"' EXIT
CLONE="https://${GITEA_OWNER}:${GITEA_TOKEN}@gitea.${IDP_HOST}:${IDP_PORT}/${GITEA_OWNER}/${SAMPLE_REPO_NAME}.git"
GIT_SSL_NO_VERIFY=true git clone -q "$CLONE" "$WORK/repo"
git -C "$WORK/repo" config http.sslVerify false
git -C "$WORK/repo" config user.name "a hurried human"
git -C "$WORK/repo" config user.email "human@localtest.me"
git -C "$WORK/repo" checkout -q -B "$BRANCH"

# The bump is fine. The namespace is the defect: one word, changed in passing,
# and it silently moves everything the Application deploys into a namespace
# that already belongs to something else. A values diff shows a one-word
# change; only rendering shows what it does.
sed -i.bak -E 's/^( *targetRevision: ).*/\16.14.1/' "$WORK/repo/apps/podinfo-hub.yaml"
sed -i.bak -E 's/^( *namespace: )podinfo-hub$/\1podinfo-tenant/' "$WORK/repo/apps/podinfo-hub.yaml"
rm -f "$WORK/repo/apps/podinfo-hub.yaml.bak"
step "$(git -C "$WORK/repo" diff --stat | tail -1)"
git -C "$WORK/repo" diff | grep -E '^[-+] ' | sed 's/^/    /'

git -C "$WORK/repo" commit -qam "chore(podinfo): bump hub to 6.14.1"
git -C "$WORK/repo" push -q --force "$CLONE" "$BRANCH"
ok "pushed $BRANCH"

PR="$(gitea_api POST "/repos/${GITEA_OWNER}/${SAMPLE_REPO_NAME}/pulls" \
  -d "$(BR="$BRANCH" python3 -c 'import json,os; print(json.dumps({"head":os.environ["BR"],"base":"main","title":"chore(podinfo): bump hub to 6.14.1","body":"Routine version bump.\n\nOpened by the local proving ground to show what the gate refuses. The bump itself is fine; the destination namespace changed alongside it."}))')" \
  | python3 -c 'import json,sys; print(json.load(sys.stdin).get("number",""))')"
[ -n "$PR" ] || PR="$(gitea_api GET "/repos/${GITEA_OWNER}/${SAMPLE_REPO_NAME}/pulls?state=open" \
  | python3 -c 'import json,sys; d=[p for p in json.load(sys.stdin) if p["head"]["ref"]=="'"$BRANCH"'"]; print(d[0]["number"] if d else "")')"
ok "pull request #${PR}"
printf '    %s/%s/%s/pulls/%s\n' "$GITEA_URL" "$GITEA_OWNER" "$SAMPLE_REPO_NAME" "$PR"

say "2. the gate refuses it"
set +e
bash "$ROOT/scripts/gate-run.sh" "$PR"
GATE_EXIT=$?
set -e
if [ "$GATE_EXIT" -eq 0 ]; then
  bad "the gate passed a change it should have blocked"; exit 1
fi
ok "gate exit ${GATE_EXIT} -- blocked, as it should be"
sed 's/^/    /' "/tmp/gate-report-${PR}.md" | head -20

say "3. the agent triages"
AGENT_POD="$(kc -n bosun get pod -l app.kubernetes.io/name=bosun -o name | head -1)"
[ -n "$AGENT_POD" ] || { bad "no agent pod"; exit 1; }
BEFORE="$(kc -n bosun logs "$AGENT_POD" 2>/dev/null | wc -l | tr -d ' ')"
step "agent log is ${BEFORE} lines; only what follows belongs to this run"
HEAD_BEFORE="$(gitea_api GET "/repos/${GITEA_OWNER}/${SAMPLE_REPO_NAME}/pulls/${PR}" \
  | python3 -c 'import json,sys; print(json.load(sys.stdin)["head"]["sha"])')"

# The SAME body Kargo's triage step posts. `files` is the important field:
# the agent turns each one into a scalar inventory -- every key and its exact
# value -- and the model picks a key from that list rather than inventing a
# path. Send only prNumber and it has the gate's report, no keys to edit
# against, and nothing it can honestly propose.
BODY=$(python3 -c "
import json,sys
print(json.dumps({
  'project': 'delivery', 'stage': 'podinfo',
  'promotion': 'demo-triage', 'artifact': 'podinfo',
  'from': '6.7.0', 'to': '6.14.1', 'autoMerge': 'never',
  'prNumber': int(sys.argv[1]), 'branch': sys.argv[2],
  'files': ['apps/podinfo-hub.yaml', 'apps/podinfo-tenant.yaml'],
  'verifyApps': ['podinfo-hub'],
}))" "$PR" "$BRANCH")
kc -n bosun exec -i "$AGENT_POD" -- \
  wget -q -O- --post-data "$BODY" \
  --header 'Content-Type: application/json' \
  http://localhost:8080/v1/promotion-opened 2>&1 | sed 's/^/    /' || true

step "waiting for the verdict (a local model takes a moment)"
for _ in $(seq 1 60); do
  kc -n bosun logs "$AGENT_POD" 2>/dev/null | tail -n +"$BEFORE" \
    | grep -qiE "triage done|escalat|applied|no fix|refus" && break
  sleep 5
done
kc -n bosun logs "$AGENT_POD" 2>/dev/null | tail -n +"$BEFORE" | sed 's/^/    /'

say "4. what the agent actually did"
HEAD_AFTER="$(gitea_api GET "/repos/${GITEA_OWNER}/${SAMPLE_REPO_NAME}/pulls/${PR}" \
  | python3 -c 'import json,sys; print(json.load(sys.stdin)["head"]["sha"])')"
if [ "$HEAD_BEFORE" != "$HEAD_AFTER" ]; then
  ok "it pushed a fix: ${HEAD_BEFORE:0:7} -> ${HEAD_AFTER:0:7}"
  gitea_api GET "/repos/${GITEA_OWNER}/${SAMPLE_REPO_NAME}/git/commits/${HEAD_AFTER}" \
    | python3 -c 'import json,sys; d=json.load(sys.stdin); print("    commit: "+d["commit"]["message"].strip().splitlines()[0])' || true
else
  step "it pushed nothing -- so it escalated or found no fix"
fi

echo
step "comments on the pull request:"
gitea_api GET "/repos/${GITEA_OWNER}/${SAMPLE_REPO_NAME}/issues/${PR}/comments?limit=50" \
  | python3 -c '
import json,sys
for c in json.load(sys.stdin):
    body = c["body"]
    if body.startswith("<!-- gitops-gate -->"):
        continue          # the gate report, already shown above
    print("    --- %s ---" % c["user"]["login"])
    for line in body.strip().splitlines()[:20]:
        print("    " + line)
'

say "5. does the gate accept it now?"
if [ "$HEAD_BEFORE" != "$HEAD_AFTER" ]; then
  set +e
  bash "$ROOT/scripts/gate-run.sh" "$PR"
  RE=$?
  set -e
  [ "$RE" -eq 0 ] && ok "gate is green on the agent's fix" || step "gate still exit ${RE} -- the fix was not enough"
else
  step "nothing was pushed, so the gate would return the same verdict"
fi

say "done"
printf '  pull request  %s/%s/%s/pulls/%s\n' "$GITEA_URL" "$GITEA_OWNER" "$SAMPLE_REPO_NAME" "$PR"
