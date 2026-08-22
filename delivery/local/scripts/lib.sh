#!/usr/bin/env bash
# Shared settings and helpers for the local proving ground.
#
# Sourced, never executed. Everything here is deliberately overridable from the
# environment so the same scripts work against a cluster you built by hand.

set -euo pipefail

: "${CLUSTER_CONTEXT:=kind-localdev}"
: "${IDP_HOST:=cnoe.localtest.me}"
: "${IDP_PORT:=8443}"
: "${GITEA_URL:=https://gitea.${IDP_HOST}:${IDP_PORT}}"
: "${ARGOCD_URL:=https://argocd.${IDP_HOST}:${IDP_PORT}}"
: "${GITEA_OWNER:=giteaAdmin}"
: "${SAMPLE_REPO_NAME:=delivery-sample}"
: "${KARGO_PROJECT:=delivery}"

# The same repository has two addresses, and which one you want depends on
# where you are standing.
#
#   GITEA_URL     through the ingress, TLS with a self-signed certificate.
#                 For anything running on the HOST -- the seed push, the gate.
#   GITEA_SVC     the Service, plain HTTP, no certificate involved.
#                 For anything running IN the cluster -- Kargo, ArgoCD, the
#                 agent. Kargo's git-clone fails on the ingress address with
#                 `SSL certificate ... self-signed certificate (18)`, and
#                 teaching every in-cluster component to trust a throwaway CA
#                 is a lot of plumbing to reach a service two hops away.
: "${GITEA_SVC:=http://my-gitea-http.gitea.svc.cluster.local:3000}"
: "${GITEA_NAMESPACE:=gitea}"
: "${GITEA_SVC_PORT:=3000}"

# The chart the demo keeps current. Public, tiny, and with enough published
# versions that a bump is always available.
: "${DEMO_CHART_REPO:=https://stefanprodan.github.io/podinfo}"
: "${DEMO_CHART:=podinfo}"

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

say()  { printf '\n\033[1m==> %s\033[0m\n' "$*"; }
step() { printf '  \033[36m%s\033[0m\n' "$*"; }
ok()   { printf '  \033[32mok\033[0m    %s\n' "$*"; }
bad()  { printf '  \033[31mFAIL\033[0m  %s\n' "$*" >&2; return 1; }

kc() { kubectl --context "$CLUSTER_CONTEXT" "$@"; }

# Wait for a condition rather than sleeping. Every wait in these scripts has a
# deadline: a demo that hangs forever is worse than one that fails, because
# nobody can tell the difference between slow and stuck.
wait_for() {
  local desc="$1" deadline="$2"; shift 2
  local end=$((SECONDS + deadline))
  while [ $SECONDS -lt $end ]; do
    if "$@" >/dev/null 2>&1; then ok "$desc"; return 0; fi
    sleep 3
  done
  bad "$desc (waited ${deadline}s)"
}

# Gitea's certificate is self-signed, so every call to it needs -k. Wrapped so
# that fact lives in one place rather than being copied into a dozen curls.
gitea_api() {
  local method="$1" path="$2"; shift 2
  curl -sk -X "$method" \
    -H "Authorization: token ${GITEA_TOKEN}" \
    -H "Content-Type: application/json" \
    "${GITEA_URL}/api/v1${path}" "$@"
}

load_credentials() {
  GITEA_TOKEN="$(idpbuilder get secrets -p gitea -o json 2>/dev/null \
    | python3 -c 'import json,sys; print(json.load(sys.stdin)[0]["token"])')"
  GITEA_PASSWORD="$(idpbuilder get secrets -p gitea -o json 2>/dev/null \
    | python3 -c 'import json,sys; print(json.load(sys.stdin)[0]["password"])')"
  export GITEA_TOKEN GITEA_PASSWORD
}
