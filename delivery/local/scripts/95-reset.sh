#!/usr/bin/env bash
# Puts the flow back to its starting position so the demo can run again.
#
# Re-seeding the repository is NOT enough, and that is the whole reason this
# script exists. `20-seed.sh` force-pushes the sample repo back to its original
# pins, but Kargo's Stage still holds the Freight it already promoted -- so
# there is nothing left to promote, no Promotion is created, and the demo
# fails at step 2 with "a promotion exists (waited 240s)". The git side and the
# Kargo side both have to go back.
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

load_credentials

say "closing any open pull request"
for n in $(gitea_api GET "/repos/${GITEA_OWNER}/${SAMPLE_REPO_NAME}/pulls?state=open" \
           | python3 -c 'import json,sys;[print(p["number"]) for p in json.load(sys.stdin)]'); do
  gitea_api PATCH "/repos/${GITEA_OWNER}/${SAMPLE_REPO_NAME}/pulls/${n}" \
    -d '{"state":"closed"}' >/dev/null
  step "closed #${n}"
done

say "resetting the repository"
bash "$ROOT/scripts/20-seed.sh" >/dev/null
ok "sample repo back to its starting pins"

say "resetting Kargo"
# Uninstalling and reinstalling the chart is the reliable reset: it drops the
# Warehouse, the Stages and their accumulated status together. Clearing
# `status.freightHistory` by hand does not survive the next reconcile.
helm uninstall kargo-pipelines --kube-context "$CLUSTER_CONTEXT" -n kargo --wait >/dev/null 2>&1 || true
kc -n "$KARGO_PROJECT" delete promotions --all >/dev/null 2>&1 || true
kc -n "$KARGO_PROJECT" delete analysisruns --all >/dev/null 2>&1 || true
kc -n "$KARGO_PROJECT" delete freight --all >/dev/null 2>&1 || true
ok "stages, promotions, analysis runs and freight cleared"

say "reinstalling the pipelines"
bash "$ROOT/scripts/30-kit.sh" >/dev/null
ok "warehouse and stages recreated"

say "ready -- run: make demo"
