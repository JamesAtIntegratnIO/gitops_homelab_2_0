#!/usr/bin/env bash
# The CNOE local cluster: kind + ArgoCD + Gitea + ingress, via idpbuilder.
#
# Two deliberate departures from the documented quickstart:
#
# 1. NOT --use-path-routing, which the CNOE docs lead with. Path routing serves
#    Gitea at https://cnoe.localtest.me:8443/gitea/<owner>/<repo>, and Kargo's
#    Gitea provider cannot parse that -- it reads owner and repo positionally
#    and fails with "could not extract repository owner and name from URL".
#    Host-based routing gives https://gitea.cnoe.localtest.me:8443/<owner>/<repo>,
#    which every client parses. Costs nothing; both are one hostname on :8443.
#
# 2. NOT the full cnoe-io/stacks//ref-implementation. That stack adds Backstage,
#    Crossplane, Keycloak, Argo Workflows and Spark -- several GB and some
#    minutes for components this flow never touches, and Keycloak cannot be
#    removed from it. Set STACK=ref if you want it anyway.
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

: "${STACK:=lean}"

say "idpbuilder cluster"
if kubectl config get-contexts -o name 2>/dev/null | grep -qx "$CLUSTER_CONTEXT"; then
  ok "cluster already exists ($CLUSTER_CONTEXT)"
else
  args=()
  if [ "$STACK" = ref ]; then
    step "including the upstream CNOE reference implementation"
    args+=(--package https://github.com/cnoe-io/stacks//ref-implementation)
  fi
  idpbuilder create "${args[@]}"
fi

wait_for "argocd answers" 300 bash -c "curl -sk -o /dev/null -w '%{http_code}' https://argocd.${IDP_HOST}:${IDP_PORT} | grep -q 200"
wait_for "gitea answers"  300 bash -c "curl -sk -o /dev/null -w '%{http_code}' ${GITEA_URL} | grep -q 200"
