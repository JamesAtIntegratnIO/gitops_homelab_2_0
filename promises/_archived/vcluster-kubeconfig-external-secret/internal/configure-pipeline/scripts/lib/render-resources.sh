#!/usr/bin/env bash

write_external_secret() {
  cat > /kratix/output/external-secret.yaml <<EOF
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: vcluster-${NAME}-kubeconfig
  namespace: ${TARGET_NAMESPACE}
  labels:
    app: vcluster
    instance: ${NAME}
    app.kubernetes.io/managed-by: kratix
    kratix.io/promise-name: ${PROMISE_NAME}
    kratix.io/resource-name: ${RESOURCE_NAME}
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
}

render_all_resources() {
  write_external_secret
}
