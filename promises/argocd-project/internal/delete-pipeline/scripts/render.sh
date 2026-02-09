#!/usr/bin/env bash
set -euo pipefail

INPUT_FILE="/kratix/input/object.yaml"

if [ ! -s "${INPUT_FILE}" ]; then
	echo "Input file missing; skipping ArgoCD AppProject cleanup."
	echo "---" > /kratix/output/cleanup.yaml
	exit 0
fi

PROJECT_NAME=$(yq eval '.spec.name // ""' "${INPUT_FILE}")
NAMESPACE=$(yq eval '.spec.namespace // ""' "${INPUT_FILE}")

if [ -z "${PROJECT_NAME}" ] || [ "${PROJECT_NAME}" = "null" ] || \
	 [ -z "${NAMESPACE}" ] || [ "${NAMESPACE}" = "null" ]; then
	echo "Missing required fields; skipping ArgoCD AppProject cleanup."
	echo "---" > /kratix/output/cleanup.yaml
	exit 0
fi

cat > /kratix/output/cleanup.yaml <<EOF
apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
	name: ${PROJECT_NAME}
	namespace: ${NAMESPACE}
EOF
