#!/usr/bin/env bash
set -euo pipefail

INPUT_FILE="/kratix/input/object.yaml"

NAME=$(yq eval '.spec.name' "${INPUT_FILE}")
REQUEST_NAMESPACE=$(yq eval '.metadata.namespace' "${INPUT_FILE}")
TARGET_NAMESPACE=$(yq eval '.spec.targetNamespace // ""' "${INPUT_FILE}")
PROJECT_NAME=$(yq eval '.spec.projectName // ""' "${INPUT_FILE}")

K8S_VERSION=$(yq eval '.spec.vcluster.k8sVersion // "v1.34.3"' "${INPUT_FILE}")
PRESET=$(yq eval '.spec.vcluster.preset // "dev"' "${INPUT_FILE}")
ISOLATION_MODE=$(yq eval '.spec.vcluster.isolationMode // "standard"' "${INPUT_FILE}")
REPLICAS_OVERRIDE=$(yq eval '.spec.vcluster.replicas' "${INPUT_FILE}")
COREDNS_REPLICAS_OVERRIDE=$(yq eval '.spec.vcluster.coredns.replicas' "${INPUT_FILE}")
CPU_REQUEST_RAW=$(yq eval '.spec.vcluster.resources.requests.cpu' "${INPUT_FILE}")
MEMORY_REQUEST_RAW=$(yq eval '.spec.vcluster.resources.requests.memory' "${INPUT_FILE}")
CPU_LIMIT_RAW=$(yq eval '.spec.vcluster.resources.limits.cpu' "${INPUT_FILE}")
MEMORY_LIMIT_RAW=$(yq eval '.spec.vcluster.resources.limits.memory' "${INPUT_FILE}")
PERSISTENCE_ENABLED_RAW=$(yq eval '.spec.vcluster.persistence.enabled' "${INPUT_FILE}")
PERSISTENCE_SIZE_RAW=$(yq eval '.spec.vcluster.persistence.size' "${INPUT_FILE}")
PERSISTENCE_STORAGE_CLASS=$(yq eval '.spec.vcluster.persistence.storageClass // ""' "${INPUT_FILE}")
CLUSTER_DOMAIN=$(yq eval '.spec.vcluster.networking.clusterDomain // "cluster.local"' "${INPUT_FILE}")

HOSTNAME=$(yq eval '.spec.exposure.hostname // ""' "${INPUT_FILE}")
BASE_DOMAIN=$(yq eval '.metadata.annotations."platform.integratn.tech/base-domain" // "integratn.tech"' "${INPUT_FILE}")
SUBNET=$(yq eval '.spec.exposure.subnet // ""' "${INPUT_FILE}")
VIP=$(yq eval '.spec.exposure.vip // ""' "${INPUT_FILE}")
API_PORT=$(yq eval '.spec.exposure.apiPort // 8443' "${INPUT_FILE}")

CERT_MANAGER_CLUSTER_ISSUER_LABELS_RAW=$(yq eval -o=yaml '.spec.integrations.certManager.clusterIssuerSelectorLabels' "${INPUT_FILE}")
EXTERNAL_SECRETS_CLUSTER_STORE_LABELS_RAW=$(yq eval -o=yaml '.spec.integrations.externalSecrets.clusterStoreSelectorLabels' "${INPUT_FILE}")
ARGOCD_ENVIRONMENT_RAW=$(yq eval '.spec.integrations.argocd.environment // ""' "${INPUT_FILE}")
ARGOCD_CLUSTER_LABELS_RAW=$(yq eval -o=yaml '.spec.integrations.argocd.clusterLabels' "${INPUT_FILE}")
ARGOCD_CLUSTER_ANNOTATIONS_RAW=$(yq eval -o=yaml '.spec.integrations.argocd.clusterAnnotations' "${INPUT_FILE}")

ARGOCD_REPO_URL=$(yq eval '.spec.argocdApplication.repoURL // "https://charts.loft.sh"' "${INPUT_FILE}")
ARGOCD_CHART=$(yq eval '.spec.argocdApplication.chart // "vcluster"' "${INPUT_FILE}")
ARGOCD_TARGET_REVISION=$(yq eval '.spec.argocdApplication.targetRevision // "0.30.4"' "${INPUT_FILE}")
ARGOCD_DEST_SERVER=$(yq eval '.spec.argocdApplication.destinationServer // "https://kubernetes.default.svc"' "${INPUT_FILE}")

RECONCILE_AT_RAW=$(yq eval '.metadata.annotations."platform.integratn.tech/reconcile-at" // ""' "${INPUT_FILE}")

is_valid_ipv4() {
  local ip=$1
  IFS=. read -r o1 o2 o3 o4 <<<"${ip}"
  for octet in "${o1}" "${o2}" "${o3}" "${o4}"; do
    if ! [[ "${octet}" =~ ^[0-9]+$ ]] || [ "${octet}" -gt 255 ]; then
      return 1
    fi
  done
  return 0
}

ip_to_int() {
  IFS=. read -r o1 o2 o3 o4 <<<"$1"
  echo $(( (o1 << 24) + (o2 << 16) + (o3 << 8) + o4 ))
}

int_to_ip() {
  local ip_int=$1
  echo "$(( (ip_int >> 24) & 255 )).$(( (ip_int >> 16) & 255 )).$(( (ip_int >> 8) & 255 )).$(( ip_int & 255 ))"
}

ip_in_cidr() {
  local ip=$1
  local cidr=$2
  local cidr_ip prefix mask ip_int cidr_int

  IFS=/ read -r cidr_ip prefix <<<"${cidr}"
  if [ -z "${cidr_ip}" ] || [ -z "${prefix}" ] || ! [[ "${prefix}" =~ ^[0-9]+$ ]] || [ "${prefix}" -gt 32 ]; then
    return 1
  fi
  if ! is_valid_ipv4 "${ip}" || ! is_valid_ipv4 "${cidr_ip}"; then
    return 1
  fi

  ip_int=$(ip_to_int "${ip}")
  cidr_int=$(ip_to_int "${cidr_ip}")
  mask=$(( 0xFFFFFFFF << (32 - prefix) & 0xFFFFFFFF ))
  if [ $(( ip_int & mask )) -ne $(( cidr_int & mask )) ]; then
    return 1
  fi

  return 0
}

default_vip_from_cidr() {
  local cidr=$1
  local offset=$2
  local cidr_ip prefix mask network_int host_count vip_int

  IFS=/ read -r cidr_ip prefix <<<"${cidr}"
  if [ -z "${cidr_ip}" ] || [ -z "${prefix}" ]; then
    return 1
  fi
  if ! is_valid_ipv4 "${cidr_ip}"; then
    return 1
  fi
  mask=$(( 0xFFFFFFFF << (32 - prefix) & 0xFFFFFFFF ))
  network_int=$(( $(ip_to_int "${cidr_ip}") & mask ))
  host_count=$(( 1 << (32 - prefix) ))
  if [ "${offset}" -ge "${host_count}" ]; then
    return 1
  fi
  vip_int=$(( network_int + offset ))
  int_to_ip "${vip_int}"
}

if [ -z "${TARGET_NAMESPACE}" ] || [ "${TARGET_NAMESPACE}" = "null" ]; then
  TARGET_NAMESPACE="${REQUEST_NAMESPACE}"
fi

if [ -z "${PROJECT_NAME}" ] || [ "${PROJECT_NAME}" = "null" ]; then
  PROJECT_NAME="vcluster-${NAME}"
fi

if [ -z "${API_PORT}" ] || [ "${API_PORT}" = "null" ]; then
  API_PORT=8443
fi

RECONCILE_TOKEN=$(echo "${RECONCILE_AT_RAW}" | tr -cd '0-9')
if [ -n "${RECONCILE_TOKEN}" ]; then
  KUBECONFIG_SYNC_JOB_NAME="vcluster-${NAME}-kubeconfig-sync-${RECONCILE_TOKEN}"
else
  KUBECONFIG_SYNC_JOB_NAME="vcluster-${NAME}-kubeconfig-sync"
fi

EXTERNAL_SERVER_URL=""

if [ -z "${HOSTNAME}" ] || [ "${HOSTNAME}" = "null" ]; then
  HOSTNAME="${NAME}.${BASE_DOMAIN}"
fi

if [ -z "${PRESET}" ] || [ "${PRESET}" = "null" ]; then
  PRESET=dev
fi

if [ -n "${ARGOCD_ENVIRONMENT_RAW}" ] && [ "${ARGOCD_ENVIRONMENT_RAW}" != "null" ]; then
  ARGOCD_ENVIRONMENT="${ARGOCD_ENVIRONMENT_RAW}"
else
  if [ "${PRESET}" = "prod" ]; then
    ARGOCD_ENVIRONMENT="production"
  else
    ARGOCD_ENVIRONMENT="development"
  fi
fi

if [ -z "${ARGOCD_CLUSTER_LABELS_RAW}" ] || [ "${ARGOCD_CLUSTER_LABELS_RAW}" = "null" ] || [ "${ARGOCD_CLUSTER_LABELS_RAW}" = "{}" ]; then
  ARGOCD_CLUSTER_LABELS_RAW=""
fi

if [ -z "${ARGOCD_CLUSTER_ANNOTATIONS_RAW}" ] || [ "${ARGOCD_CLUSTER_ANNOTATIONS_RAW}" = "null" ] || [ "${ARGOCD_CLUSTER_ANNOTATIONS_RAW}" = "{}" ]; then
  ARGOCD_CLUSTER_ANNOTATIONS_RAW=""
fi

ARGOCD_CLUSTER_LABELS_BASE=$(cat <<EOF
argocd.argoproj.io/secret-type: cluster
cluster_name: ${NAME}
cluster_role: worker
environment: ${ARGOCD_ENVIRONMENT}
EOF
)

ARGOCD_CLUSTER_ANNOTATIONS_BASE=$(cat <<EOF
addons_repo_url: https://github.com/jamesatintegratnio/gitops_homelab_2_0
addons_repo_revision: main
addons_repo_basepath: addons/
addons_repo_path: charts/application-sets
managed-by: argocd.argoproj.io
cert_manager_namespace: cert-manager
external_dns_namespace: external-dns
nfs_subdir_external_provisioner_namespace: nfs-provisioner
cluster_name: ${NAME}
environment: ${ARGOCD_ENVIRONMENT}
EOF
)

ARGOCD_CLUSTER_LABELS="${ARGOCD_CLUSTER_LABELS_BASE}"
if [ -n "${ARGOCD_CLUSTER_LABELS_RAW}" ]; then
  ARGOCD_CLUSTER_LABELS=$(printf "%s\n%s" "${ARGOCD_CLUSTER_LABELS}" "${ARGOCD_CLUSTER_LABELS_RAW}")
fi

if [ -n "${ARGOCD_CLUSTER_ANNOTATIONS_RAW}" ]; then
  ARGOCD_CLUSTER_ANNOTATIONS=$(printf "%s\n%s" "${ARGOCD_CLUSTER_ANNOTATIONS_BASE}" "${ARGOCD_CLUSTER_ANNOTATIONS_RAW}")
else
  ARGOCD_CLUSTER_ANNOTATIONS="${ARGOCD_CLUSTER_ANNOTATIONS_BASE}"
fi

ARGOCD_CLUSTER_LABELS_INDENTED=$(echo "${ARGOCD_CLUSTER_LABELS}" | sed 's/^/    /')
ARGOCD_CLUSTER_ANNOTATIONS_INDENTED=$(echo "${ARGOCD_CLUSTER_ANNOTATIONS}" | sed 's/^/    /')

case "${PRESET}" in
  dev)
    PRESET_CPU_REQUEST="200m"
    PRESET_MEMORY_REQUEST="512Mi"
    PRESET_CPU_LIMIT="1000m"
    PRESET_MEMORY_LIMIT="1Gi"
    PRESET_PERSISTENCE_ENABLED=false
    PRESET_PERSISTENCE_SIZE="5Gi"
    PRESET_COREDNS_REPLICAS=1
    PRESET_REPLICAS=1
    ;;
  prod)
    PRESET_CPU_REQUEST="500m"
    PRESET_MEMORY_REQUEST="1Gi"
    PRESET_CPU_LIMIT="2"
    PRESET_MEMORY_LIMIT="2Gi"
    PRESET_PERSISTENCE_ENABLED=true
    PRESET_PERSISTENCE_SIZE="10Gi"
    PRESET_COREDNS_REPLICAS=2
    PRESET_REPLICAS=3
    ;;
  *)
    echo "Invalid preset: ${PRESET}. Allowed: dev, prod"
    exit 1
    ;;
esac

if [ -z "${CPU_REQUEST_RAW}" ] || [ "${CPU_REQUEST_RAW}" = "null" ]; then
  CPU_REQUEST=${PRESET_CPU_REQUEST}
else
  CPU_REQUEST=${CPU_REQUEST_RAW}
fi

if [ -z "${MEMORY_REQUEST_RAW}" ] || [ "${MEMORY_REQUEST_RAW}" = "null" ]; then
  MEMORY_REQUEST=${PRESET_MEMORY_REQUEST}
else
  MEMORY_REQUEST=${MEMORY_REQUEST_RAW}
fi

if [ -z "${CPU_LIMIT_RAW}" ] || [ "${CPU_LIMIT_RAW}" = "null" ]; then
  CPU_LIMIT=${PRESET_CPU_LIMIT}
else
  CPU_LIMIT=${CPU_LIMIT_RAW}
fi

if [ -z "${MEMORY_LIMIT_RAW}" ] || [ "${MEMORY_LIMIT_RAW}" = "null" ]; then
  MEMORY_LIMIT=${PRESET_MEMORY_LIMIT}
else
  MEMORY_LIMIT=${MEMORY_LIMIT_RAW}
fi

if [ -z "${PERSISTENCE_ENABLED_RAW}" ] || [ "${PERSISTENCE_ENABLED_RAW}" = "null" ]; then
  PERSISTENCE_ENABLED=${PRESET_PERSISTENCE_ENABLED}
else
  PERSISTENCE_ENABLED=${PERSISTENCE_ENABLED_RAW}
fi

if [ -z "${PERSISTENCE_SIZE_RAW}" ] || [ "${PERSISTENCE_SIZE_RAW}" = "null" ]; then
  PERSISTENCE_SIZE=${PRESET_PERSISTENCE_SIZE}
else
  PERSISTENCE_SIZE=${PERSISTENCE_SIZE_RAW}
fi

if [ -z "${CERT_MANAGER_CLUSTER_ISSUER_LABELS_RAW}" ] || [ "${CERT_MANAGER_CLUSTER_ISSUER_LABELS_RAW}" = "null" ] || [ "${CERT_MANAGER_CLUSTER_ISSUER_LABELS_RAW}" = "{}" ]; then
  CERT_MANAGER_CLUSTER_ISSUER_LABELS_RAW=$(cat <<EOF
integratn.tech/cluster-issuer: letsencrypt-prod
EOF
)
fi

if [ -z "${EXTERNAL_SECRETS_CLUSTER_STORE_LABELS_RAW}" ] || [ "${EXTERNAL_SECRETS_CLUSTER_STORE_LABELS_RAW}" = "null" ] || [ "${EXTERNAL_SECRETS_CLUSTER_STORE_LABELS_RAW}" = "{}" ]; then
  EXTERNAL_SECRETS_CLUSTER_STORE_LABELS_RAW=$(cat <<EOF
integratn.tech/cluster-secret-store: onepassword-store
EOF
)
fi

CERT_MANAGER_CLUSTER_ISSUER_LABELS=$(echo "${CERT_MANAGER_CLUSTER_ISSUER_LABELS_RAW}" | sed 's/^/        /')
EXTERNAL_SECRETS_CLUSTER_STORE_LABELS=$(echo "${EXTERNAL_SECRETS_CLUSTER_STORE_LABELS_RAW}" | sed 's/^/        /')

if [ -n "${REPLICAS_OVERRIDE}" ] && [ "${REPLICAS_OVERRIDE}" != "null" ]; then
  REPLICAS=${REPLICAS_OVERRIDE}
else
  REPLICAS=${PRESET_REPLICAS}
fi

if [ -n "${COREDNS_REPLICAS_OVERRIDE}" ] && [ "${COREDNS_REPLICAS_OVERRIDE}" != "null" ]; then
  COREDNS_REPLICAS=${COREDNS_REPLICAS_OVERRIDE}
else
  COREDNS_REPLICAS=${PRESET_COREDNS_REPLICAS}
fi

SERVICE_VALUES=""
if [ -n "${SUBNET}" ] && [ "${SUBNET}" != "null" ]; then
  if [ -z "${VIP}" ] || [ "${VIP}" = "null" ]; then
    VIP=$(default_vip_from_cidr "${SUBNET}" 100)
  fi
  if [ -z "${VIP}" ]; then
    echo "Failed to compute default VIP from subnet ${SUBNET}"
    exit 1
  fi
  if ! ip_in_cidr "${VIP}" "${SUBNET}"; then
    echo "VIP ${VIP} is not within subnet ${SUBNET}"
    exit 1
  fi
fi

SERVICE_LOADBALANCER_IP_LINE=""
if [ -n "${VIP}" ] && [ "${VIP}" != "null" ]; then
  SERVICE_LOADBALANCER_IP_LINE="      loadBalancerIP: \"${VIP}\""
fi

SERVICE_VALUES=$(cat <<EOF
  service:
    enabled: true
    annotations:
      external-dns.alpha.kubernetes.io/hostname: "${HOSTNAME}"
    spec:
      type: LoadBalancer
${SERVICE_LOADBALANCER_IP_LINE}
      ports:
        - name: https
          port: ${API_PORT}
          targetPort: 8443
          protocol: TCP
EOF
)

if [ -n "${HOSTNAME}" ] && [ "${HOSTNAME}" != "null" ]; then
  EXTERNAL_SERVER_URL="https://${HOSTNAME}:${API_PORT}"
elif [ -n "${VIP}" ] && [ "${VIP}" != "null" ]; then
  EXTERNAL_SERVER_URL="https://${VIP}:${API_PORT}"
fi

PROXY_EXTRA_SANS_VALUES=""
if [ -n "${HOSTNAME}" ] && [ "${HOSTNAME}" != "null" ]; then
  PROXY_EXTRA_SANS_VALUES=$(cat <<EOF
  proxy:
    extraSANs:
      - "${HOSTNAME}"
EOF
)
  if [ -n "${VIP}" ] && [ "${VIP}" != "null" ]; then
    PROXY_EXTRA_SANS_VALUES=$(cat <<EOF
${PROXY_EXTRA_SANS_VALUES}
      - "${VIP}"
EOF
)
  fi
elif [ -n "${VIP}" ] && [ "${VIP}" != "null" ]; then
  PROXY_EXTRA_SANS_VALUES=$(cat <<EOF
  proxy:
    extraSANs:
      - "${VIP}"
EOF
)
fi

PERSISTENCE_STORAGE_CLASS_CM_LINE=""
if [ -n "${PERSISTENCE_STORAGE_CLASS}" ] && [ "${PERSISTENCE_STORAGE_CLASS}" != "null" ]; then
  PERSISTENCE_STORAGE_CLASS_CM_LINE="            storageClass: \"${PERSISTENCE_STORAGE_CLASS}\""
fi

VALUES_BASE_FILE="/tmp/vcluster-values-base.yaml"
VALUES_OVERRIDES_FILE="/tmp/vcluster-values-overrides.yaml"
VALUES_MERGED_FILE="/tmp/vcluster-values-merged.yaml"

cat > "${VALUES_BASE_FILE}" <<EOF
controlPlane:
  distro:
    k8s:
      enabled: true
      version: "${K8S_VERSION}"
  statefulSet:
    highAvailability:
      replicas: ${REPLICAS}
    image:
      repository: "loft-sh/vcluster-oss"
    persistence:
      volumeClaim:
        enabled: ${PERSISTENCE_ENABLED}
        size: "${PERSISTENCE_SIZE}"
${PERSISTENCE_STORAGE_CLASS_CM_LINE}
    resources:
      requests:
        cpu: "${CPU_REQUEST}"
        memory: "${MEMORY_REQUEST}"
      limits:
        cpu: "${CPU_LIMIT}"
        memory: "${MEMORY_LIMIT}"
  coredns:
    enabled: true
    deployment:
      replicas: ${COREDNS_REPLICAS}
    overwriteConfig: |
      .:1053 {
        errors
        health
        ready
        kubernetes ${CLUSTER_DOMAIN} in-addr.arpa ip6.arpa {
          pods insecure
          fallthrough in-addr.arpa ip6.arpa
          ttl 30
        }
        prometheus 0.0.0.0:9153
        forward . /etc/resolv.conf
        cache 30
        loop
        reload
        loadbalance
      }
${PROXY_EXTRA_SANS_VALUES}
${SERVICE_VALUES}

deploy:
  metallb:
    enabled: true

integrations:
  externalSecrets:
    enabled: true
    webhook:
      enabled: true
    sync:
      fromHost:
        clusterStores:
          enabled: true
          selector:
            matchLabels:
${EXTERNAL_SECRETS_CLUSTER_STORE_LABELS}
  metricsServer:
    enabled: true
  certManager:
    enabled: true
    sync:
      fromHost:
        clusterIssuers:
          enabled: true
          selector:
            labels:
${CERT_MANAGER_CLUSTER_ISSUER_LABELS}

telemetry:
  enabled: false

networking:
  advanced:
    clusterDomain: "${CLUSTER_DOMAIN}"

sync:
  toHost:
    pods:
      enabled: true
  fromHost:
    secrets:
      enabled: true
      mappings:
        byName:
          "external-secrets/eso-onepassword-token": "external-secrets/eso-onepassword-token"

rbac:
  clusterRole:
    enabled: true
    extraRules:
      - apiGroups: [""]
        resources: ["secrets"]
        verbs: ["get", "list", "watch"]
        resourceNames:
          - "eso-onepassword-token"
EOF

yq eval '.spec.vcluster.helmOverrides // {}' "${INPUT_FILE}" > "${VALUES_OVERRIDES_FILE}"
yq eval-all 'select(fileIndex==0) * select(fileIndex==1)' "${VALUES_BASE_FILE}" "${VALUES_OVERRIDES_FILE}" > "${VALUES_MERGED_FILE}"

VALUES_CONFIGMAP=$(sed 's/^/    /' "${VALUES_MERGED_FILE}")
VALUES_OBJECT=$(sed 's/^/        /' "${VALUES_MERGED_FILE}")

SYNC_POLICY_DEFAULT="/tmp/argocd-sync-policy-default.yaml"
SYNC_POLICY_OVERRIDE="/tmp/argocd-sync-policy-override.yaml"
SYNC_POLICY_MERGED="/tmp/argocd-sync-policy.yaml"

cat > "${SYNC_POLICY_DEFAULT}" <<EOF
automated:
  selfHeal: true
  prune: true
syncOptions:
  - CreateNamespace=true
EOF

yq eval '.spec.argocdApplication.syncPolicy // {}' "${INPUT_FILE}" > "${SYNC_POLICY_OVERRIDE}"
yq eval-all 'select(fileIndex==0) * select(fileIndex==1)' "${SYNC_POLICY_DEFAULT}" "${SYNC_POLICY_OVERRIDE}" > "${SYNC_POLICY_MERGED}"

SYNC_POLICY_OBJECT=$(sed 's/^/    /' "${SYNC_POLICY_MERGED}")

ONEPASSWORD_ITEM="vcluster-${NAME}-kubeconfig"

cat > /kratix/output/vcluster-core-request.yaml <<EOF
apiVersion: platform.integratn.tech/v1alpha1
kind: VClusterCore
metadata:
  name: ${NAME}
  namespace: ${REQUEST_NAMESPACE}
spec:
  name: ${NAME}
  targetNamespace: ${TARGET_NAMESPACE}
  valuesYaml: |
${VALUES_CONFIGMAP}
EOF

cat > /kratix/output/vcluster-coredns-request.yaml <<EOF
apiVersion: platform.integratn.tech/v1alpha1
kind: VClusterCoredns
metadata:
  name: ${NAME}
  namespace: ${REQUEST_NAMESPACE}
spec:
  name: ${NAME}
  targetNamespace: ${TARGET_NAMESPACE}
  clusterDomain: ${CLUSTER_DOMAIN}
EOF

cat > /kratix/output/argocd-project-request.yaml <<EOF
apiVersion: platform.integratn.tech/v1alpha1
kind: ArgoCDProject
metadata:
  name: ${PROJECT_NAME}
  namespace: ${REQUEST_NAMESPACE}
spec:
  name: ${PROJECT_NAME}
  namespace: argocd
  description: VCluster project for ${NAME}
  annotations:
    argocd.argoproj.io/sync-wave: "-1"
  labels:
    app.kubernetes.io/managed-by: kratix
    kratix.io/promise-name: vcluster-orchestrator
    kratix.io/resource-request: ${NAME}
    argocd.argoproj.io/project-group: appteam
  sourceRepos:
    - https://charts.loft.sh
  destinations:
    - server: https://kubernetes.default.svc
      namespace: ${TARGET_NAMESPACE}
  clusterResourceWhitelist:
    - group: '*'
      kind: '*'
  namespaceResourceWhitelist:
    - group: '*'
      kind: '*'
EOF

cat > /kratix/output/argocd-application-request.yaml <<EOF
apiVersion: platform.integratn.tech/v1alpha1
kind: ArgoCDApplication
metadata:
  name: vcluster-${NAME}
  namespace: ${REQUEST_NAMESPACE}
spec:
  name: vcluster-${NAME}
  namespace: argocd
  annotations:
    argocd.argoproj.io/sync-wave: "0"
  finalizers:
    - resources-finalizer.argocd.argoproj.io
  project: ${PROJECT_NAME}
  source:
    repoURL: ${ARGOCD_REPO_URL}
    chart: ${ARGOCD_CHART}
    targetRevision: ${ARGOCD_TARGET_REVISION}
    helm:
      releaseName: ${NAME}
      valuesObject:
${VALUES_OBJECT}
  destination:
    server: ${ARGOCD_DEST_SERVER}
    namespace: ${TARGET_NAMESPACE}
  syncPolicy:
${SYNC_POLICY_OBJECT}
EOF

cat > /kratix/output/vcluster-kubeconfig-sync-request.yaml <<EOF
apiVersion: platform.integratn.tech/v1alpha1
kind: VClusterKubeconfigSync
metadata:
  name: ${NAME}
  namespace: ${REQUEST_NAMESPACE}
spec:
  name: ${NAME}
  targetNamespace: ${TARGET_NAMESPACE}
  kubeconfigSyncJobName: ${KUBECONFIG_SYNC_JOB_NAME}
  onepasswordItem: ${ONEPASSWORD_ITEM}
  hostname: ${HOSTNAME}
  apiPort: ${API_PORT}
  serverUrl: ${EXTERNAL_SERVER_URL}
EOF

cat > /kratix/output/vcluster-kubeconfig-external-secret-request.yaml <<EOF
apiVersion: platform.integratn.tech/v1alpha1
kind: VClusterKubeconfigExternalSecret
metadata:
  name: ${NAME}
  namespace: ${REQUEST_NAMESPACE}
spec:
  name: ${NAME}
  targetNamespace: ${TARGET_NAMESPACE}
  onepasswordItem: ${ONEPASSWORD_ITEM}
EOF

cat > /kratix/output/vcluster-argocd-cluster-request.yaml <<EOF
apiVersion: platform.integratn.tech/v1alpha1
kind: VClusterArgoCDClusterRegistration
metadata:
  name: ${NAME}
  namespace: ${REQUEST_NAMESPACE}
spec:
  name: ${NAME}
  argocdNamespace: argocd
  onepasswordItem: ${ONEPASSWORD_ITEM}
  labels:
${ARGOCD_CLUSTER_LABELS_INDENTED}
  annotations:
${ARGOCD_CLUSTER_ANNOTATIONS_INDENTED}
EOF

echo "Orchestrator outputs rendered for vcluster: ${NAME}"
