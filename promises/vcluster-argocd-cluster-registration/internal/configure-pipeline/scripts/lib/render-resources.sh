#!/usr/bin/env bash

build_label_and_annotation_blocks() {
  LABELS_FILE="/tmp/argocd-cluster-labels.yaml"
  ANNOTATIONS_FILE="/tmp/argocd-cluster-annotations.yaml"

  mkdir -p /tmp

  yq eval '.spec.labels // {}' "${INPUT_FILE}" > "${LABELS_FILE}"
  yq eval '.spec.annotations // {}' "${INPUT_FILE}" > "${ANNOTATIONS_FILE}"

  LABELS_LENGTH=$(yq eval 'length' "${LABELS_FILE}")
  ANNOTATIONS_LENGTH=$(yq eval 'length' "${ANNOTATIONS_FILE}")

  if [ "${LABELS_LENGTH}" -eq 0 ]; then
    LABELS="            {}"
  else
    LABELS=$(sed 's/^/            /' "${LABELS_FILE}")
  fi

  if [ "${ANNOTATIONS_LENGTH}" -eq 0 ]; then
    ANNOTATIONS="            {}"
  else
    ANNOTATIONS=$(sed 's/^/            /' "${ANNOTATIONS_FILE}")
  fi
}

write_argocd_cluster_secret() {
  cat > /kratix/output/argocd-cluster-secret.yaml <<EOF
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: vcluster-${NAME}-argocd-cluster
  namespace: ${ARGOCD_NAMESPACE}
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
${LABELS}
        annotations:
${ANNOTATIONS}
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
}

render_all_resources() {
  build_label_and_annotation_blocks
  write_argocd_cluster_secret
}
