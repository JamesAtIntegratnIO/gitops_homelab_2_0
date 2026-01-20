#!/usr/bin/env bash

INPUT_FILE="/kratix/input/object.yaml"

# Read values from ResourceRequest
NAME=$(yq eval '.spec.name' "${INPUT_FILE}")
K8S_VERSION=$(yq eval '.spec.k8sVersion // "v1.34.3"' "${INPUT_FILE}")
ISOLATION_MODE=$(yq eval '.spec.isolationMode // "standard"' "${INPUT_FILE}")
PRESET=$(yq eval '.spec.preset // "dev"' "${INPUT_FILE}")
REPLICAS_OVERRIDE=$(yq eval '.spec.replicas' "${INPUT_FILE}")
COREDNS_REPLICAS_OVERRIDE=$(yq eval '.spec.coredns.replicas' "${INPUT_FILE}")
CPU_REQUEST_RAW=$(yq eval '.spec.resources.requests.cpu' "${INPUT_FILE}")
MEMORY_REQUEST_RAW=$(yq eval '.spec.resources.requests.memory' "${INPUT_FILE}")
CPU_LIMIT_RAW=$(yq eval '.spec.resources.limits.cpu' "${INPUT_FILE}")
MEMORY_LIMIT_RAW=$(yq eval '.spec.resources.limits.memory' "${INPUT_FILE}")
PROJECT_NAME=$(yq eval '.spec.projectName // ""' "${INPUT_FILE}")
CLUSTER_DOMAIN=$(yq eval '.spec.networking.clusterDomain // "cluster.local"' "${INPUT_FILE}")
HOSTNAME=$(yq eval '.spec.hostname // ""' "${INPUT_FILE}")
BASE_DOMAIN=$(yq eval '.metadata.annotations."platform.integratn.tech/base-domain" // "integratn.tech"' "${INPUT_FILE}")
SUBNET=$(yq eval '.spec.subnet // ""' "${INPUT_FILE}")
VIP=$(yq eval '.spec.vip // ""' "${INPUT_FILE}")
API_PORT=$(yq eval '.spec.apiPort // 8443' "${INPUT_FILE}")
PERSISTENCE_ENABLED_RAW=$(yq eval '.spec.persistence.enabled' "${INPUT_FILE}")
PERSISTENCE_SIZE_RAW=$(yq eval '.spec.persistence.size' "${INPUT_FILE}")
PERSISTENCE_STORAGE_CLASS=$(yq eval '.spec.persistence.storageClass // ""' "${INPUT_FILE}")
CERT_MANAGER_CLUSTER_ISSUER_LABELS_RAW=$(yq eval -o=yaml '.spec.integrations.certManager.clusterIssuerSelectorLabels' "${INPUT_FILE}")
EXTERNAL_SECRETS_CLUSTER_STORE_LABELS_RAW=$(yq eval -o=yaml '.spec.integrations.externalSecrets.clusterStoreSelectorLabels' "${INPUT_FILE}")
ARGOCD_ENVIRONMENT_RAW=$(yq eval '.spec.integrations.argocd.environment // ""' "${INPUT_FILE}")
ARGOCD_CLUSTER_LABELS_RAW=$(yq eval -o=yaml '.spec.integrations.argocd.clusterLabels' "${INPUT_FILE}")
ARGOCD_CLUSTER_ANNOTATIONS_RAW=$(yq eval -o=yaml '.spec.integrations.argocd.clusterAnnotations' "${INPUT_FILE}")
RECONCILE_AT_RAW=$(yq eval '.metadata.annotations."platform.integratn.tech/reconcile-at" // ""' "${INPUT_FILE}")

# Get namespaces from ResourceRequest
REQUEST_NAMESPACE=$(yq eval '.metadata.namespace' "${INPUT_FILE}")
NAMESPACE=$(yq eval '.spec.targetNamespace // ""' "${INPUT_FILE}")
