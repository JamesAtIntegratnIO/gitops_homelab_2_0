#!/usr/bin/env bash
# Installs the delivery kit itself: the agent, the pipelines, the observability.
#
# The agent image is built from the WORKING TREE rather than pulled. That is
# the point of a proving ground -- it exercises the code in front of you, not
# whatever was last published. It also sidesteps the architecture question
# entirely: the published images are amd64 and this machine may not be.
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

load_credentials
: "${LLM_BASE_URL:?set LLM_BASE_URL to an OpenAI-compatible endpoint the cluster can reach, e.g. http://<host>:1234/v1}"
: "${LLM_MODEL:=qwen/qwen3.5-9b}"
: "${KIND_CLUSTER:=localdev}"
: "${AGENT_IMAGE:=delivery-agent:local}"

# Two addresses for one repository, because two consumers disagree about what
# is acceptable:
#
#   ArgoCD and the agent take the Service over plain HTTP. No certificate, no
#   trust store, nothing to configure.
#
#   Kargo will not. It REFUSES to send credentials to a plain-HTTP endpoint
#   ("refused to get credentials for insecure HTTP endpoint"), which is a
#   defensible rule and one you only discover from a controller log -- the
#   promotion itself fails at `git push` with "could not read Username",
#   naming nothing. So Kargo gets the ingress address over HTTPS, and skips
#   verification of its self-signed certificate.
REPO_URL="${GITEA_SVC}/${GITEA_OWNER}/${SAMPLE_REPO_NAME}.git"
KARGO_REPO_URL="${GITEA_URL}/${GITEA_OWNER}/${SAMPLE_REPO_NAME}.git"
GITEA_ROOT="${GITEA_SVC}"

# The model endpoint is off-cluster, and the chart's allowPublicHTTPS rule
# excepts every RFC1918 range -- so a model on your LAN needs its own explicit
# ipBlock or the agent hangs with zero bytes and no error.
LLM_HOST="$(printf '%s' "$LLM_BASE_URL" | sed -E 's#^https?://##; s#[:/].*$##')"
LLM_PORT="$(printf '%s' "$LLM_BASE_URL" | sed -nE 's#^https?://[^:/]+:([0-9]+).*#\1#p')"
: "${LLM_PORT:=80}"

say "building the agent image from the working tree"
docker build -q -t "$AGENT_IMAGE" "$ROOT/../images/delivery-agent" >/dev/null
ok "built $AGENT_IMAGE"
# kind nodes have their own image store; a locally built image is invisible
# until it is loaded, and the pod would sit ImagePullBackOff against a
# registry that has never heard of this tag.
command -v kind >/dev/null 2>&1 || bad "kind is not on PATH -- idpbuilder embeds it but does not install it"
kind load docker-image "$AGENT_IMAGE" --name "$KIND_CLUSTER" 2>&1 | sed 's/^/    /'
ok "loaded into kind/$KIND_CLUSTER"

say "agent credentials"
kc get namespace delivery-agent >/dev/null 2>&1 || kc create namespace delivery-agent >/dev/null
kc -n delivery-agent delete secret agent-git >/dev/null 2>&1 || true
kc -n delivery-agent create secret generic agent-git \
  --from-literal=token="$GITEA_TOKEN" >/dev/null
ok "agent-git"

say "delivery-agent"
helm upgrade --install delivery-agent "$ROOT/../charts/delivery-agent" \
  --kube-context "$CLUSTER_CONTEXT" \
  --namespace delivery-agent \
  --set image.repository="${AGENT_IMAGE%%:*}" \
  --set image.tag="${AGENT_IMAGE##*:}" \
  --set image.pullPolicy=Never \
  --set git.provider=gitea \
  --set git.apiBase="$GITEA_ROOT" \
  --set git.owner="$GITEA_OWNER" \
  --set git.repo="$SAMPLE_REPO_NAME" \
  --set git.repoURL="$REPO_URL" \
  --set git.insecureSkipTLSVerify=false \
  --set "networkPolicy.egress.namespaces[0].name=${GITEA_NAMESPACE}" \
  --set "networkPolicy.egress.namespaces[0].ports[0]=${GITEA_SVC_PORT}" \
  --set "networkPolicy.egress.ipBlocks[0].cidr=${LLM_HOST}/32" \
  --set "networkPolicy.egress.ipBlocks[0].port=${LLM_PORT}" \
  --set git.existingSecret=agent-git \
  --set llm.provider=openai \
  --set llm.baseURL="$LLM_BASE_URL" \
  --set llm.model="$LLM_MODEL" \
  --set gate.checkName=gate \
  --set gate.wait=3m \
  --set gate.poll=10s \
  --set 'triage.allowPaths[0]=apps/**' \
  --wait --timeout 5m >/dev/null
ok "delivery-agent ready"

say "kargo-pipelines"
helm upgrade --install kargo-pipelines "$ROOT/../charts/kargo-pipelines" \
  --kube-context "$CLUSTER_CONTEXT" \
  --namespace kargo \
  -f "$ROOT/values/kargo-pipelines.yaml" \
  --set git.repoURL="$KARGO_REPO_URL" \
  --set git.insecureSkipTLSVerify=true \
  --wait --timeout 5m >/dev/null
ok "warehouse and stages created"

say "kargo git credentials"
# After the chart, never before: kargo-pipelines owns the Project namespace
# and helm refuses to adopt one that already exists without its ownership
# labels. Kargo also matches credentials to a repository by NORMALISED URL,
# so this repoURL and the Warehouse's must agree down to the trailing .git.
kc -n "$KARGO_PROJECT" delete secret sample-repo >/dev/null 2>&1 || true
kc -n "$KARGO_PROJECT" create secret generic sample-repo \
  --from-literal=repoURL="$KARGO_REPO_URL" \
  --from-literal=username="$GITEA_OWNER" \
  --from-literal=password="$GITEA_TOKEN" >/dev/null
kc -n "$KARGO_PROJECT" label secret sample-repo \
  kargo.akuity.io/cred-type=git --overwrite >/dev/null
ok "git credentials in ${KARGO_PROJECT}"

say "kargo-observability"
helm upgrade --install kargo-observability "$ROOT/../charts/kargo-observability" \
  --kube-context "$CLUSTER_CONTEXT" \
  --namespace monitoring \
  -f "$ROOT/values/kargo-observability.yaml" \
  --wait --timeout 5m >/dev/null
ok "custom-resource-state config and alerts installed"

say "pointing kube-state-metrics at the config"
# kube-state-metrics reads this file ONCE, at startup, and nothing watches it.
# In production this cost a rollout to notice: the ConfigMap was correct and
# the running pod kept emitting the old series.
helm upgrade --install monitoring prometheus-community/kube-prometheus-stack \
  --kube-context "$CLUSTER_CONTEXT" --namespace monitoring --reuse-values \
  --set kube-state-metrics.customResourceState.enabled=true \
  --set kube-state-metrics.customResourceState.create=false \
  --set kube-state-metrics.customResourceState.name=kargo-custom-resource-state \
  --set kube-state-metrics.rbac.extraRules[0].apiGroups[0]=kargo.akuity.io \
  --set kube-state-metrics.rbac.extraRules[0].resources[0]=* \
  --set kube-state-metrics.rbac.extraRules[0].verbs[0]=list \
  --set kube-state-metrics.rbac.extraRules[1].apiGroups[0]=kargo.akuity.io \
  --set kube-state-metrics.rbac.extraRules[1].resources[0]=* \
  --set kube-state-metrics.rbac.extraRules[1].verbs[0]=watch \
  --wait --timeout 8m >/dev/null
kc -n monitoring rollout restart deploy/monitoring-kube-state-metrics >/dev/null
kc -n monitoring rollout status deploy/monitoring-kube-state-metrics --timeout=180s >/dev/null
ok "kube-state-metrics restarted onto the new config"

say "kit installed"
kc -n kargo get warehouses,stages -A --no-headers 2>/dev/null | sed 's/^/  /' || true
