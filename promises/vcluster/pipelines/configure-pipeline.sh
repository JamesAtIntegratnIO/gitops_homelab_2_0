#!/usr/bin/env bash
set -euo pipefail

# Read values from ResourceRequest
NAME=$(yq eval '.spec.name' /kratix/input/object.yaml)
K8S_VERSION=$(yq eval '.spec.k8sVersion // "v1.34.3"' /kratix/input/object.yaml)
ISOLATION_MODE=$(yq eval '.spec.isolationMode // "standard"' /kratix/input/object.yaml)
PRESET=$(yq eval '.spec.preset // "dev"' /kratix/input/object.yaml)
REPLICAS_OVERRIDE=$(yq eval '.spec.replicas' /kratix/input/object.yaml)
COREDNS_REPLICAS_OVERRIDE=$(yq eval '.spec.coredns.replicas' /kratix/input/object.yaml)
CPU_REQUEST_RAW=$(yq eval '.spec.resources.requests.cpu' /kratix/input/object.yaml)
MEMORY_REQUEST_RAW=$(yq eval '.spec.resources.requests.memory' /kratix/input/object.yaml)
CPU_LIMIT_RAW=$(yq eval '.spec.resources.limits.cpu' /kratix/input/object.yaml)
MEMORY_LIMIT_RAW=$(yq eval '.spec.resources.limits.memory' /kratix/input/object.yaml)
PROJECT_NAME=$(yq eval '.spec.projectName // ""' /kratix/input/object.yaml)
CLUSTER_DOMAIN=$(yq eval '.spec.networking.clusterDomain // "cluster.local"' /kratix/input/object.yaml)
HOSTNAME=$(yq eval '.spec.hostname // ""' /kratix/input/object.yaml)
SUBNET=$(yq eval '.spec.subnet // ""' /kratix/input/object.yaml)
VIP=$(yq eval '.spec.vip // ""' /kratix/input/object.yaml)
API_PORT=$(yq eval '.spec.apiPort // 8443' /kratix/input/object.yaml)
PERSISTENCE_ENABLED_RAW=$(yq eval '.spec.persistence.enabled' /kratix/input/object.yaml)
PERSISTENCE_SIZE_RAW=$(yq eval '.spec.persistence.size' /kratix/input/object.yaml)
PERSISTENCE_STORAGE_CLASS=$(yq eval '.spec.persistence.storageClass // ""' /kratix/input/object.yaml)
CERT_MANAGER_CLUSTER_ISSUER_LABELS_RAW=$(yq eval -o=yaml '.spec.integrations.certManager.clusterIssuerSelectorLabels' /kratix/input/object.yaml)
EXTERNAL_SECRETS_CLUSTER_STORE_LABELS_RAW=$(yq eval -o=yaml '.spec.integrations.externalSecrets.clusterStoreSelectorLabels' /kratix/input/object.yaml)
ARGOCD_ENVIRONMENT_RAW=$(yq eval '.spec.integrations.argocd.environment // ""' /kratix/input/object.yaml)
ARGOCD_CLUSTER_LABELS_RAW=$(yq eval -o=yaml '.spec.integrations.argocd.clusterLabels' /kratix/input/object.yaml)
ARGOCD_CLUSTER_ANNOTATIONS_RAW=$(yq eval -o=yaml '.spec.integrations.argocd.clusterAnnotations' /kratix/input/object.yaml)
RECONCILE_AT_RAW=$(yq eval '.metadata.annotations."platform.integratn.tech/reconcile-at" // ""' /kratix/input/object.yaml)

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

# Get namespaces from ResourceRequest
REQUEST_NAMESPACE=$(yq eval '.metadata.namespace' /kratix/input/object.yaml)
NAMESPACE=$(yq eval '.spec.targetNamespace // ""' /kratix/input/object.yaml)

if [ -z "${NAMESPACE}" ] || [ "${NAMESPACE}" = "null" ]; then
  NAMESPACE="${REQUEST_NAMESPACE}"
fi

if [ -z "${PROJECT_NAME}" ] || [ "${PROJECT_NAME}" = "null" ]; then
  PROJECT_NAME="vcluster-${NAME}"
fi

# Generate 1Password item name for kubeconfig
ONEPASSWORD_ITEM="vcluster-${NAME}-kubeconfig"

echo "Generating vcluster resources for: ${NAME}"
echo "Request namespace: ${REQUEST_NAMESPACE}"
echo "Target namespace: ${NAMESPACE}"
echo "ArgoCD project: ${PROJECT_NAME}"

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
cluster_role: vcluster
environment: ${ARGOCD_ENVIRONMENT}
EOF
)

ARGOCD_CLUSTER_ANNOTATIONS_BASE=$(cat <<EOF
addons_repo_url: https://github.com/jamesatintegratnio/gitops_homelab_2_0
addons_repo_revision: main
addons_repo_basepath: addons/
addons_repo_path: charts/application-sets
managed-by: argocd.argoproj.io
cluster_name: ${NAME}
environment: ${ARGOCD_ENVIRONMENT}
EOF
)

if [ -n "${ARGOCD_CLUSTER_LABELS_RAW}" ]; then
  ARGOCD_CLUSTER_LABELS=$(printf "%s\n%s" "${ARGOCD_CLUSTER_LABELS_BASE}" "${ARGOCD_CLUSTER_LABELS_RAW}")
else
  ARGOCD_CLUSTER_LABELS="${ARGOCD_CLUSTER_LABELS_BASE}"
fi

if [ -n "${ARGOCD_CLUSTER_ANNOTATIONS_RAW}" ]; then
  ARGOCD_CLUSTER_ANNOTATIONS=$(printf "%s\n%s" "${ARGOCD_CLUSTER_ANNOTATIONS_BASE}" "${ARGOCD_CLUSTER_ANNOTATIONS_RAW}")
else
  ARGOCD_CLUSTER_ANNOTATIONS="${ARGOCD_CLUSTER_ANNOTATIONS_BASE}"
fi

ARGOCD_CLUSTER_LABELS_INDENTED=$(echo "${ARGOCD_CLUSTER_LABELS}" | sed 's/^/          /')
ARGOCD_CLUSTER_ANNOTATIONS_INDENTED=$(echo "${ARGOCD_CLUSTER_ANNOTATIONS}" | sed 's/^/          /')

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

CERT_MANAGER_CLUSTER_ISSUER_LABELS=$(echo "${CERT_MANAGER_CLUSTER_ISSUER_LABELS_RAW}" | sed 's/^/              /')
EXTERNAL_SECRETS_CLUSTER_STORE_LABELS=$(echo "${EXTERNAL_SECRETS_CLUSTER_STORE_LABELS_RAW}" | sed 's/^/              /')

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
if [ -n "${HOSTNAME}" ] || [ -n "${SUBNET}" ] || [ -n "${VIP}" ]; then
  if [ -z "${HOSTNAME}" ] || [ "${HOSTNAME}" = "null" ] || [ -z "${SUBNET}" ] || [ "${SUBNET}" = "null" ]; then
    echo "hostname and subnet are required when exposing a VIP"
    exit 1
  fi
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

  SERVICE_VALUES=$(cat <<EOF
  service:
    enabled: true
    annotations:
      external-dns.alpha.kubernetes.io/hostname: "${HOSTNAME}"
    spec:
      type: LoadBalancer
      loadBalancerIP: "${VIP}"
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
PERSISTENCE_STORAGE_CLASS_APP_LINE=""
if [ -n "${PERSISTENCE_STORAGE_CLASS}" ] && [ "${PERSISTENCE_STORAGE_CLASS}" != "null" ]; then
  PERSISTENCE_STORAGE_CLASS_CM_LINE="            storageClass: \"${PERSISTENCE_STORAGE_CLASS}\""
  PERSISTENCE_STORAGE_CLASS_APP_LINE="                storageClass: \"${PERSISTENCE_STORAGE_CLASS}\""
fi

# Create namespace for vcluster
cat > /kratix/output/namespace.yaml <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: ${NAMESPACE}
  labels:
    app: vcluster
    instance: ${NAME}
EOF

# Create ArgoCD Project for the vcluster
cat > /kratix/output/argocd-project.yaml <<EOF
apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: ${PROJECT_NAME}
  namespace: argocd
  annotations:
    argocd.argoproj.io/sync-wave: "-1"
  labels:
    app.kubernetes.io/managed-by: kratix
    kratix.io/promise-name: vcluster
    kratix.io/resource-request: ${NAME}
    argocd.argoproj.io/project-group: appteam
spec:
  description: VCluster project for ${NAME}
  sourceRepos:
    - https://charts.loft.sh
  destinations:
    - server: https://kubernetes.default.svc
      namespace: ${NAMESPACE}
  clusterResourceWhitelist:
    - group: '*'
      kind: '*'
  namespaceResourceWhitelist:
    - group: '*'
      kind: '*'
EOF

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
EOF

yq eval '.spec.helmOverrides // {}' /kratix/input/object.yaml > "${VALUES_OVERRIDES_FILE}"
yq eval-all 'select(fileIndex==0) * select(fileIndex==1)' "${VALUES_BASE_FILE}" "${VALUES_OVERRIDES_FILE}" > "${VALUES_MERGED_FILE}"

VALUES_CONFIGMAP=$(sed 's/^/    /' "${VALUES_MERGED_FILE}")
VALUES_OBJECT=$(sed 's/^/        /' "${VALUES_MERGED_FILE}")

# Create Helm values ConfigMap
cat > /kratix/output/helm-values.yaml <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: ${NAME}-vcluster-values
  namespace: ${NAMESPACE}
data:
  values.yaml: |
${VALUES_CONFIGMAP}
EOF

# Create ArgoCD Application for vcluster
cat > /kratix/output/argocd-application.yaml <<EOF
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: vcluster-${NAME}
  namespace: argocd
  annotations:
    argocd.argoproj.io/sync-wave: "0"
  finalizers:
    - resources-finalizer.argocd.argoproj.io
spec:
  project: ${PROJECT_NAME}
  source:
    repoURL: https://charts.loft.sh
    chart: vcluster
    targetRevision: 0.30.4
    helm:
      releaseName: ${NAME}
      valuesObject:
${VALUES_OBJECT}
  destination:
    server: https://kubernetes.default.svc
    namespace: ${NAMESPACE}
  syncPolicy:
    automated:
      selfHeal: true
      prune: true
    syncOptions:
      - CreateNamespace=true
EOF

# Create Job to sync kubeconfig to 1Password after vcluster is ready
cat > /kratix/output/kubeconfig-sync-job.yaml <<EOF
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: vcluster-${NAME}-onepassword-token
  namespace: ${NAMESPACE}
spec:
  refreshInterval: 1h
  secretStoreRef:
    name: onepassword-store
    kind: ClusterSecretStore
  target:
    name: vcluster-${NAME}-onepassword-token
    creationPolicy: Owner
  data:
    - secretKey: token
      remoteRef:
        key: onepassword-access-token
        property: credential
    - secretKey: vault
      remoteRef:
        key: onepassword-access-token
        property: vault
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: vcluster-${NAME}-kubeconfig-sync
  namespace: ${NAMESPACE}
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: vcluster-${NAME}-kubeconfig-reader
  namespace: ${NAMESPACE}
rules:
  - apiGroups: [""]
    resources: ["secrets"]
    resourceNames: ["vc-${NAME}", "vcluster-${NAME}-onepassword-token"]
    verbs: ["get"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: vcluster-${NAME}-kubeconfig-sync
  namespace: ${NAMESPACE}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: vcluster-${NAME}-kubeconfig-reader
subjects:
  - kind: ServiceAccount
    name: vcluster-${NAME}-kubeconfig-sync
    namespace: ${NAMESPACE}
---
apiVersion: batch/v1
kind: Job
metadata:
  name: ${KUBECONFIG_SYNC_JOB_NAME}
  namespace: ${NAMESPACE}
  labels:
    app: vcluster
    instance: ${NAME}
    component: kubeconfig-sync
spec:
  ttlSecondsAfterFinished: 300
  template:
    metadata:
      labels:
        app: vcluster
        instance: ${NAME}
    spec:
      serviceAccountName: vcluster-${NAME}-kubeconfig-sync
      restartPolicy: OnFailure
      volumes:
        - name: sync-data
          emptyDir: {}
      initContainers:
        # Wait for vcluster kubeconfig secret to be created
        - name: wait-for-kubeconfig
          image: bitnami/kubectl:latest
          volumeMounts:
            - name: sync-data
              mountPath: /shared
          command:
            - /bin/bash
            - -c
            - |
              echo "Waiting for vcluster kubeconfig secret vc-${NAME}..."
              until kubectl get secret vc-${NAME} -n ${NAMESPACE} 2>/dev/null; do
                echo "Secret not found, waiting..."
                sleep 10
              done
              echo "Writing kubeconfig to shared volume..."
              kubectl get secret vc-${NAME} -n ${NAMESPACE} -o jsonpath='{.data.config}' | base64 -d > /shared/kubeconfig
              echo "Secret found!"
      containers:
        - name: sync-to-onepassword
          image: alpine:3.19
          env:
            - name: OP_CONNECT_HOST
              value: "https://connect.integratn.tech"
            - name: OP_CONNECT_TOKEN
              valueFrom:
                secretKeyRef:
                  name: vcluster-${NAME}-onepassword-token
                  key: token
            - name: OP_VAULT_ID
              valueFrom:
                secretKeyRef:
                  name: vcluster-${NAME}-onepassword-token
                  key: vault
            - name: NAMESPACE
              value: "${NAMESPACE}"
            - name: VCLUSTER_NAME
              value: "${NAME}"
            - name: HOSTNAME
              value: "${HOSTNAME}"
            - name: API_PORT
              value: "${API_PORT}"
            - name: SERVER_URL
              value: "${EXTERNAL_SERVER_URL}"
            - name: OP_ITEM_NAME
              value: "${ONEPASSWORD_ITEM}"
          volumeMounts:
            - name: sync-data
              mountPath: /shared
          command:
            - /bin/sh
            - -c
            - |
              set -e
              apk add --no-cache ca-certificates curl jq yq

              if [ -n "\${SERVER_URL}" ]; then
                echo "Rewriting kubeconfig server to \${SERVER_URL}"
                awk -v new_server="\${SERVER_URL}" '
                  !done && \$1=="server:" {print "    server: " new_server; done=1; next}
                  {print}
                ' /shared/kubeconfig > /shared/kubeconfig.rewritten
                mv /shared/kubeconfig.rewritten /shared/kubeconfig
              fi

              ARGOCD_CLUSTER_NAME="vcluster-\${VCLUSTER_NAME}"
              KUBECONFIG_SERVER=\$(yq -r '.clusters[0].cluster.server // ""' /shared/kubeconfig)
              KUBECONFIG_CA_DATA=\$(yq -r '.clusters[0].cluster."certificate-authority-data" // ""' /shared/kubeconfig)
              KUBECONFIG_CERT_DATA=\$(yq -r '.users[0].user."client-certificate-data" // ""' /shared/kubeconfig)
              KUBECONFIG_KEY_DATA=\$(yq -r '.users[0].user."client-key-data" // ""' /shared/kubeconfig)
              KUBECONFIG_TOKEN=\$(yq -r '.users[0].user.token // ""' /shared/kubeconfig)

              if [ -z "\${KUBECONFIG_SERVER}" ]; then
                echo "Failed to extract server from kubeconfig"
                exit 1
              fi

              if [ -n "\${KUBECONFIG_TOKEN}" ]; then
                ARGOCD_CONFIG_JSON=\$(jq -n --arg token "\${KUBECONFIG_TOKEN}" --arg ca "\${KUBECONFIG_CA_DATA}" '{bearerToken:\$token,tlsClientConfig:{insecure:false,caData:\$ca}}')
              else
                if [ -z "\${KUBECONFIG_CERT_DATA}" ] || [ -z "\${KUBECONFIG_KEY_DATA}" ]; then
                  echo "Failed to extract client cert/key from kubeconfig"
                  exit 1
                fi
                ARGOCD_CONFIG_JSON=\$(jq -n --arg ca "\${KUBECONFIG_CA_DATA}" --arg cert "\${KUBECONFIG_CERT_DATA}" --arg key "\${KUBECONFIG_KEY_DATA}" '{tlsClientConfig:{insecure:false,caData:\$ca,certData:\$cert,keyData:\$key}}')
              fi

              KUBECONFIG_CONTENT=\$(cat /shared/kubeconfig)
              KUBECONFIG_BYTES=\$(printf '%s' "\${KUBECONFIG_CONTENT}" | wc -c | tr -d ' ')
              if [ "\${KUBECONFIG_BYTES}" -eq 0 ]; then
                echo "Kubeconfig content is empty; aborting sync"
                exit 1
              fi

              VAULT_NAME="homelab"
              OP_CONNECT_HOST_CLEAN="\$(printf '%s' "\${OP_CONNECT_HOST}" | tr -d '\r\n')"
              OP_CONNECT_TOKEN_CLEAN="\$(printf '%s' "\${OP_CONNECT_TOKEN}" | tr -d '\r\n')"
              API_BASE="\${OP_CONNECT_HOST_CLEAN%/}/v1"
              AUTH_HEADER="Authorization: Bearer \${OP_CONNECT_TOKEN_CLEAN}"

              echo "Syncing kubeconfig to 1Password item via Connect API: \${OP_ITEM_NAME}"

              VAULT_ID="\${OP_VAULT_ID:-}"
              VAULT_ID="\$(printf '%s' "\${VAULT_ID}" | tr -d '\r\n')"
              if [ -z "\${VAULT_ID}" ]; then
                VAULT_ID=\$(curl -fsS -H "\${AUTH_HEADER}" "\${API_BASE}/vaults" | jq -r --arg name "\${VAULT_NAME}" '.[] | select(.name==\$name) | .id' | head -n1)
              fi
              VAULT_ID="\$(printf '%s' "\${VAULT_ID}" | tr -d '\r\n')"
              if [ -z "\${VAULT_ID}" ]; then
                echo "Vault not found: \${VAULT_NAME}"
                exit 1
              fi

              ITEM_ID=\$(curl -fsS -H "\${AUTH_HEADER}" "\${API_BASE}/vaults/\${VAULT_ID}/items" | jq -r --arg title "\${OP_ITEM_NAME}" '.[] | select(.title==\$title) | .id' | head -n1)

              if [ -n "\${ITEM_ID}" ]; then
                echo "Item exists, replacing..."
                ITEM_PAYLOAD=\$(jq -n --arg id "\${ITEM_ID}" --arg title "\${OP_ITEM_NAME}" --arg vault "\${VAULT_ID}" --arg notes "\${KUBECONFIG_CONTENT}" --arg argocdName "\${ARGOCD_CLUSTER_NAME}" --arg argocdServer "\${KUBECONFIG_SERVER}" --arg argocdConfig "\${ARGOCD_CONFIG_JSON}" '{id:\$id,title:\$title,vault:{id:\$vault},category:"SECURE_NOTE",notesPlain:\$notes,fields:[{label:"kubeconfig",type:"CONCEALED",value:\$notes},{label:"argocd-name",type:"STRING",value:\$argocdName},{label:"argocd-server",type:"STRING",value:\$argocdServer},{label:"argocd-config",type:"CONCEALED",value:\$argocdConfig}]}')
                curl -fsS -X PUT -H "\${AUTH_HEADER}" -H "Content-Type: application/json" "\${API_BASE}/vaults/\${VAULT_ID}/items/\${ITEM_ID}" -d "\${ITEM_PAYLOAD}" >/dev/null
              else
                echo "Item not found, creating..."
                ITEM_PAYLOAD=\$(jq -n --arg title "\${OP_ITEM_NAME}" --arg vault "\${VAULT_ID}" --arg notes "\${KUBECONFIG_CONTENT}" --arg argocdName "\${ARGOCD_CLUSTER_NAME}" --arg argocdServer "\${KUBECONFIG_SERVER}" --arg argocdConfig "\${ARGOCD_CONFIG_JSON}" '{title:\$title,vault:{id:\$vault},category:"SECURE_NOTE",notesPlain:\$notes,fields:[{label:"kubeconfig",type:"CONCEALED",value:\$notes},{label:"argocd-name",type:"STRING",value:\$argocdName},{label:"argocd-server",type:"STRING",value:\$argocdServer},{label:"argocd-config",type:"CONCEALED",value:\$argocdConfig}]}')
                ITEM_ID=\$(curl -fsS -X POST -H "\${AUTH_HEADER}" -H "Content-Type: application/json" "\${API_BASE}/vaults/\${VAULT_ID}/items" -d "\${ITEM_PAYLOAD}" | jq -r '.id')
                if [ -z "\${ITEM_ID}" ] || [ "\${ITEM_ID}" = "null" ]; then
                  echo "Failed to create item in 1Password"
                  exit 1
                fi
              fi

              ITEM_JSON=\$(curl -fsS -H "\${AUTH_HEADER}" "\${API_BASE}/vaults/\${VAULT_ID}/items/\${ITEM_ID}")
              NOTES_LEN=\$(echo "\${ITEM_JSON}" | jq -r '.notesPlain // "" | length')
              FIELD_LEN=\$(echo "\${ITEM_JSON}" | jq -r '.fields[]? | select(.label=="kubeconfig") | .value | length' | head -n1)
              FIELD_LEN=\${FIELD_LEN:-0}
              echo "1Password item lengths: notesPlain=\${NOTES_LEN} kubeconfigField=\${FIELD_LEN}"

              echo "Kubeconfig synced successfully to 1Password"
EOF

# Create ExternalSecret to reference the kubeconfig from 1Password
cat > /kratix/output/external-secret.yaml <<EOF
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: vcluster-${NAME}-kubeconfig
  namespace: ${NAMESPACE}
  labels:
    app: vcluster
    instance: ${NAME}
spec:
  refreshInterval: 5m
  secretStoreRef:
    name: onepassword-store
    kind: ClusterSecretStore
  target:
    name: vcluster-${NAME}-kubeconfig-external
    creationPolicy: Owner
  data:
    - secretKey: kubeconfig
      remoteRef:
        key: ${ONEPASSWORD_ITEM}
        property: kubeconfig
EOF

# Create ExternalSecret to register vcluster as an ArgoCD cluster
cat > /kratix/output/argocd-cluster-secret.yaml <<EOF
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: vcluster-${NAME}-argocd-cluster
  namespace: argocd
  labels:
    app: vcluster
    instance: ${NAME}
spec:
  refreshInterval: 5m
  secretStoreRef:
    name: onepassword-store
    kind: ClusterSecretStore
  target:
    name: vcluster-${NAME}
    creationPolicy: Owner
    template:
      metadata:
        labels:
${ARGOCD_CLUSTER_LABELS_INDENTED}
        annotations:
${ARGOCD_CLUSTER_ANNOTATIONS_INDENTED}
      type: Opaque
  data:
    - secretKey: name
      remoteRef:
        key: ${ONEPASSWORD_ITEM}
        property: argocd-name
    - secretKey: server
      remoteRef:
        key: ${ONEPASSWORD_ITEM}
        property: argocd-server
    - secretKey: config
      remoteRef:
        key: ${ONEPASSWORD_ITEM}
        property: argocd-config
EOF

echo "Resources generated successfully for vcluster: ${NAME}"
