#!/usr/bin/env bash
# Runs the gate against one Gitea pull request and reports the result the way
# CI would: a report comment, and a commit status the agent can read.
#
# This is a CI adapter, the same shape as the ones in ../../ci, with the
# scheduler replaced by "you ran it". idpbuilder ships no Actions runner, so
# rather than pretend, the demo calls this directly -- same binary, same
# inputs, same two artifacts.
#
#   usage: gate-run.sh <pr-number>
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

PR="${1:?usage: gate-run.sh <pr-number>}"
load_credentials

: "${GATE_BIN:=/tmp/gitops-gate}"
if [ ! -x "$GATE_BIN" ]; then
  step "building the gate"
  # GOTOOLCHAIN=auto lets an older local Go fetch the one go.mod requires,
  # which is exactly the mismatch that broke the published image.
  (cd "$ROOT/../images/gitops-gate" && GOTOOLCHAIN=auto go build -o "$GATE_BIN" .)
fi

say "gate: pull request #${PR}"
PR_JSON="$(gitea_api GET "/repos/${GITEA_OWNER}/${SAMPLE_REPO_NAME}/pulls/${PR}")"
HEAD_SHA="$(printf '%s' "$PR_JSON" | python3 -c 'import json,sys; print(json.load(sys.stdin)["head"]["sha"])')"
BASE_SHA="$(printf '%s' "$PR_JSON" | python3 -c 'import json,sys; print(json.load(sys.stdin)["base"]["sha"])')"
step "base ${BASE_SHA:0:7}  head ${HEAD_SHA:0:7}"

WORK="$(mktemp -d)"; trap 'rm -rf "$WORK"' EXIT
CLONE="https://${GITEA_OWNER}:${GITEA_TOKEN}@gitea.${IDP_HOST}:${IDP_PORT}/${GITEA_OWNER}/${SAMPLE_REPO_NAME}.git"
GIT_SSL_NO_VERIFY=true git clone -q "$CLONE" "$WORK/repo"
git -C "$WORK/repo" config http.sslVerify false
git -C "$WORK/repo" fetch -q origin "$HEAD_SHA" "$BASE_SHA"

# Render both sides. The config and inventory come from the HEAD revision at
# both checkouts: they describe how to render, not what to render, and the
# base may predate them entirely.
for side in base head; do
  sha="$BASE_SHA"; [ "$side" = head ] && sha="$HEAD_SHA"
  git -C "$WORK/repo" checkout -q --detach "$sha"
  git -C "$WORK/repo" checkout -q "$HEAD_SHA" -- .gitops-gate.yaml .gitops-gate 2>/dev/null || true
  "$GATE_BIN" render -repo "$WORK/repo" -out "$WORK/targets-$side.json" >/dev/null
  step "rendered $side"
done

git -C "$WORK/repo" checkout -q --detach "$HEAD_SHA"
set +e
"$GATE_BIN" diff \
  -base "$WORK/targets-base.json" -head "$WORK/targets-head.json" \
  -repo "$WORK/repo" \
  -report "$WORK/report.md" -json "$WORK/render-diff.json"
GATE_EXIT=$?
set -e

case "$GATE_EXIT" in
  0) STATE=success; SUMMARY="no blocking change" ;;
  1) STATE=failure; SUMMARY="blocking change" ;;
  *) STATE=error;   SUMMARY="the gate could not run" ;;
esac
step "gate exit ${GATE_EXIT} -- ${SUMMARY}"

# The report goes up as a comment because that is the only artifact surface
# every git host has, and it is where the agent looks for it.
#
# Posted VERBATIM. The gate binary leads its own report with the marker the
# agent searches for, so nothing here has to know the magic string -- this
# script used to be the only thing in the package that did, which is precisely
# why no CI adapter ever published a report the agent could find.
python3 - "$WORK/report.md" <<'PY' > "$WORK/comment.json"
import json,sys
body = open(sys.argv[1]).read() if len(sys.argv)>1 else ""
print(json.dumps({"body": body}))
PY
gitea_api POST "/repos/${GITEA_OWNER}/${SAMPLE_REPO_NAME}/issues/${PR}/comments" \
  --data-binary @"$WORK/comment.json" >/dev/null
ok "report posted as a comment"

# And a commit status, because that is what the agent's CheckStatus reads and
# what a branch protection rule would gate on.
gitea_api POST "/repos/${GITEA_OWNER}/${SAMPLE_REPO_NAME}/statuses/${HEAD_SHA}" \
  -d "{\"context\":\"gate\",\"state\":\"${STATE}\",\"description\":\"${SUMMARY}\"}" >/dev/null
ok "commit status gate=${STATE}"

cp "$WORK/report.md" /tmp/gate-report-${PR}.md 2>/dev/null || true
cp "$WORK/render-diff.json" /tmp/render-diff-${PR}.json 2>/dev/null || true
exit "$GATE_EXIT"
