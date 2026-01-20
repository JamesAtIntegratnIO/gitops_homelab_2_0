#!/usr/bin/env bash
set -euo pipefail

INPUT_FILE="/kratix/input/object.yaml"

NAME=$(yq eval '.spec.name' "${INPUT_FILE}")
TARGET_NAMESPACE=$(yq eval '.spec.targetNamespace' "${INPUT_FILE}")
VALUES_YAML=$(yq eval '.spec.valuesYaml' "${INPUT_FILE}")

VALUES_CONFIGMAP=$(printf "%s" "${VALUES_YAML}" | sed 's/^/    /')

cat > /kratix/output/namespace.yaml <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: ${TARGET_NAMESPACE}
  labels:
    app: vcluster
    instance: ${NAME}
EOF

cat > /kratix/output/helm-values.yaml <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: ${NAME}-vcluster-values
  namespace: ${TARGET_NAMESPACE}
data:
  values.yaml: |
${VALUES_CONFIGMAP}
EOF
