#!/usr/bin/env bash
set -euo pipefail

INPUT_FILE="/kratix/input/object.yaml"

NAME=$(yq eval '.spec.name' "${INPUT_FILE}")
TARGET_NAMESPACE=$(yq eval '.spec.targetNamespace' "${INPUT_FILE}")
ONEPASSWORD_ITEM=$(yq eval '.spec.onepasswordItem' "${INPUT_FILE}")

cat > /kratix/output/external-secret.yaml <<EOF
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: vcluster-${NAME}-kubeconfig
  namespace: ${TARGET_NAMESPACE}
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
