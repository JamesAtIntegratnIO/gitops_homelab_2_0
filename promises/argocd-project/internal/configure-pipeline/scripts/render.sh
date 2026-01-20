#!/usr/bin/env bash
set -euo pipefail

INPUT_FILE="/kratix/input/object.yaml"

PROJECT_NAME=$(yq eval '.spec.name' "${INPUT_FILE}")
NAMESPACE=$(yq eval '.spec.namespace' "${INPUT_FILE}")
DESCRIPTION=$(yq eval '.spec.description // ""' "${INPUT_FILE}")

ANNOTATIONS_FILE="/tmp/argocd-project-annotations.yaml"
LABELS_FILE="/tmp/argocd-project-labels.yaml"
SOURCEREPOS_FILE="/tmp/argocd-project-sourcerepos.yaml"
DESTINATIONS_FILE="/tmp/argocd-project-destinations.yaml"
CLUSTER_WHITELIST_FILE="/tmp/argocd-project-cluster-whitelist.yaml"
NAMESPACE_WHITELIST_FILE="/tmp/argocd-project-namespace-whitelist.yaml"

mkdir -p /tmp

yq eval '.spec.annotations // {}' "${INPUT_FILE}" > "${ANNOTATIONS_FILE}"
yq eval '.spec.labels // {}' "${INPUT_FILE}" > "${LABELS_FILE}"
yq eval '.spec.sourceRepos // []' "${INPUT_FILE}" > "${SOURCEREPOS_FILE}"
yq eval '.spec.destinations // []' "${INPUT_FILE}" > "${DESTINATIONS_FILE}"
yq eval '.spec.clusterResourceWhitelist // []' "${INPUT_FILE}" > "${CLUSTER_WHITELIST_FILE}"
yq eval '.spec.namespaceResourceWhitelist // []' "${INPUT_FILE}" > "${NAMESPACE_WHITELIST_FILE}"

ANNOTATIONS_LENGTH=$(yq eval 'length' "${ANNOTATIONS_FILE}")
LABELS_LENGTH=$(yq eval 'length' "${LABELS_FILE}")

if [ "${ANNOTATIONS_LENGTH}" -eq 0 ]; then
  ANNOTATIONS="    {}"
else
  ANNOTATIONS=$(sed 's/^/    /' "${ANNOTATIONS_FILE}")
fi

if [ "${LABELS_LENGTH}" -eq 0 ]; then
  LABELS="    {}"
else
  LABELS=$(sed 's/^/    /' "${LABELS_FILE}")
fi

SOURCEREPOS=$(sed 's/^/    /' "${SOURCEREPOS_FILE}")
DESTINATIONS=$(sed 's/^/    /' "${DESTINATIONS_FILE}")
CLUSTER_WHITELIST=$(sed 's/^/    /' "${CLUSTER_WHITELIST_FILE}")
NAMESPACE_WHITELIST=$(sed 's/^/    /' "${NAMESPACE_WHITELIST_FILE}")

cat > /kratix/output/argocd-project.yaml <<EOF
apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: ${PROJECT_NAME}
  namespace: ${NAMESPACE}
  annotations:
${ANNOTATIONS}
  labels:
${LABELS}
spec:
  description: ${DESCRIPTION}
  sourceRepos:
${SOURCEREPOS}
  destinations:
${DESTINATIONS}
  clusterResourceWhitelist:
${CLUSTER_WHITELIST}
  namespaceResourceWhitelist:
${NAMESPACE_WHITELIST}
EOF
