#!/usr/bin/env bash

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
