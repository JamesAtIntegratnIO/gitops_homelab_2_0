#!/usr/bin/env bash

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
BACKING_STORE_RAW=$(yq eval -o=yaml '.spec.vcluster.backingStore // {}' "${INPUT_FILE}")
EXPORT_KUBECONFIG_RAW=$(yq eval -o=yaml '.spec.vcluster.exportKubeConfig // {}' "${INPUT_FILE}")

HOSTNAME=$(yq eval '.spec.exposure.hostname // ""' "${INPUT_FILE}")
BASE_DOMAIN=$(yq eval '.metadata.annotations."platform.integratn.tech/base-domain" // "integratn.tech"' "${INPUT_FILE}")
SUBNET=$(yq eval '.spec.exposure.subnet // ""' "${INPUT_FILE}")
VIP=$(yq eval '.spec.exposure.vip // ""' "${INPUT_FILE}")
API_PORT=$(yq eval '.spec.exposure.apiPort // 443' "${INPUT_FILE}")

CERT_MANAGER_CLUSTER_ISSUER_LABELS_RAW=$(yq eval -o=yaml '.spec.integrations.certManager.clusterIssuerSelectorLabels' "${INPUT_FILE}")
EXTERNAL_SECRETS_CLUSTER_STORE_LABELS_RAW=$(yq eval -o=yaml '.spec.integrations.externalSecrets.clusterStoreSelectorLabels' "${INPUT_FILE}")
ARGOCD_ENVIRONMENT_RAW=$(yq eval '.spec.integrations.argocd.environment // ""' "${INPUT_FILE}")
ARGOCD_CLUSTER_LABELS_RAW=$(yq eval -o=yaml '.spec.integrations.argocd.clusterLabels' "${INPUT_FILE}")
ARGOCD_CLUSTER_ANNOTATIONS_RAW=$(yq eval -o=yaml '.spec.integrations.argocd.clusterAnnotations' "${INPUT_FILE}")
WORKLOAD_REPO_URL_RAW=$(yq eval '.spec.integrations.argocd.workloadRepo.url // ""' "${INPUT_FILE}")
WORKLOAD_REPO_BASEPATH_RAW=$(yq eval '.spec.integrations.argocd.workloadRepo.basePath // ""' "${INPUT_FILE}")
WORKLOAD_REPO_PATH_RAW=$(yq eval '.spec.integrations.argocd.workloadRepo.path // ""' "${INPUT_FILE}")
WORKLOAD_REPO_REVISION_RAW=$(yq eval '.spec.integrations.argocd.workloadRepo.revision // ""' "${INPUT_FILE}")

ARGOCD_REPO_URL=$(yq eval '.spec.argocdApplication.repoURL // "https://charts.loft.sh"' "${INPUT_FILE}")
ARGOCD_CHART=$(yq eval '.spec.argocdApplication.chart // "vcluster"' "${INPUT_FILE}")
ARGOCD_TARGET_REVISION=$(yq eval '.spec.argocdApplication.targetRevision // "0.30.4"' "${INPUT_FILE}")
ARGOCD_DEST_SERVER=$(yq eval '.spec.argocdApplication.destinationServer // "https://kubernetes.default.svc"' "${INPUT_FILE}")

RECONCILE_AT_RAW=$(yq eval '.metadata.annotations."platform.integratn.tech/reconcile-at" // ""' "${INPUT_FILE}")
