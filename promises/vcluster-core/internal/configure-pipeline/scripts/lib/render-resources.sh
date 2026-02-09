#!/usr/bin/env bash

write_namespace() {
  cat > /kratix/output/namespace.yaml <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: ${TARGET_NAMESPACE}
  labels:
    app: vcluster
    instance: ${NAME}
    app.kubernetes.io/managed-by: kratix
    kratix.io/promise-name: ${PROMISE_NAME}
    kratix.io/resource-name: ${RESOURCE_NAME}
    pod-security.kubernetes.io/enforce: privileged
    pod-security.kubernetes.io/audit: privileged
    pod-security.kubernetes.io/warn: privileged
EOF
}

write_helm_values() {
  VALUES_CONFIGMAP=$(printf "%s" "${VALUES_YAML}" | sed 's/^/    /')

  cat > /kratix/output/helm-values.yaml <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: ${NAME}-vcluster-values
  namespace: ${TARGET_NAMESPACE}
  labels:
    app: vcluster
    instance: ${NAME}
    app.kubernetes.io/managed-by: kratix
    kratix.io/promise-name: ${PROMISE_NAME}
    kratix.io/resource-name: ${RESOURCE_NAME}
data:
  values.yaml: |
${VALUES_CONFIGMAP}
EOF
}

render_all_resources() {
  write_namespace
  write_helm_values
}
