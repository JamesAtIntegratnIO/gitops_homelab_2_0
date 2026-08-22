#!/usr/bin/env bash
# Puts the sample repository into Gitea and gives Kargo and the agent the
# credentials they need to act on it.
#
# Idempotent: re-running replaces the repository contents and re-applies the
# secrets rather than failing on "already exists".
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

load_credentials
[ -n "${GITEA_TOKEN:-}" ] || bad "no Gitea token -- is the cluster up?"

say "gitea repository ${GITEA_OWNER}/${SAMPLE_REPO_NAME}"
if gitea_api GET "/repos/${GITEA_OWNER}/${SAMPLE_REPO_NAME}" -o /dev/null -w '%{http_code}' | grep -q 200; then
  step "already exists"
else
  gitea_api POST "/user/repos" \
    -d "{\"name\":\"${SAMPLE_REPO_NAME}\",\"private\":false,\"auto_init\":false}" >/dev/null
  step "created"
fi

say "pushing the sample repository"
# A fresh clone every time: the working copy is disposable, and reusing one
# across runs is how you end up demonstrating yesterday's state.
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
cp -R "$ROOT/sample-repo/." "$WORK/"
PUSH_URL="https://${GITEA_OWNER}:${GITEA_TOKEN}@gitea.${IDP_HOST}:${IDP_PORT}/${GITEA_OWNER}/${SAMPLE_REPO_NAME}.git"

git -C "$WORK" init -q -b main
git -C "$WORK" config http.sslVerify false
git -C "$WORK" config user.name "local proving ground"
git -C "$WORK" config user.email "local@localtest.me"
git -C "$WORK" add -A
git -C "$WORK" commit -q -m "the sample repository under test"
git -C "$WORK" push -q --force "$PUSH_URL" main
ok "pushed $(git -C "$WORK" rev-parse --short main)"

say "argocd credentials for the same repository"
# ArgoCD reconciles from inside the cluster, so it takes the Service address
# over plain HTTP -- no certificate, nothing to trust. (Kargo cannot: it
# refuses to send credentials to a plain-HTTP endpoint, so 30-kit.sh gives it
# the ingress address instead.)
REPO_URL="${GITEA_SVC}/${GITEA_OWNER}/${SAMPLE_REPO_NAME}.git"
kc -n argocd delete secret sample-repo-creds >/dev/null 2>&1 || true
kc -n argocd create secret generic sample-repo-creds \
  --from-literal=type=git \
  --from-literal=url="$REPO_URL" \
  --from-literal=username="$GITEA_OWNER" \
  --from-literal=password="$GITEA_TOKEN" \
  --from-literal=insecure=true >/dev/null
kc -n argocd label secret sample-repo-creds \
  argocd.argoproj.io/secret-type=repository --overwrite >/dev/null
ok "argocd repository credentials"

say "app-of-apps"
# Without this nothing applies the sample repo's Applications, and the
# reconcile step of the demo has nothing to watch -- the pin lands on main and
# the cluster never hears about it. One root Application pointed at apps/ is
# the smallest thing that closes the loop.
cat <<YAML | kc apply -f - >/dev/null
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: sample-apps
  namespace: argocd
spec:
  project: default
  destination:
    server: https://kubernetes.default.svc
    namespace: argocd
  source:
    repoURL: ${REPO_URL}
    targetRevision: main
    path: apps
  syncPolicy:
    automated: {prune: true, selfHeal: true}
YAML
ok "sample-apps root Application"

say "seeded"
step "kargo credentials are created by 30-kit.sh, after the chart owns the namespace"
printf '  gitea    %s/%s/%s\n' "$GITEA_URL" "$GITEA_OWNER" "$SAMPLE_REPO_NAME"
printf '  argocd   https://%s:%s/argocd\n' "$IDP_HOST" "$IDP_PORT"
