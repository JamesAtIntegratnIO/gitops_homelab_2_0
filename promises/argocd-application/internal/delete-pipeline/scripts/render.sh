#!/usr/bin/env bash
set -euo pipefail

INPUT_FILE="/kratix/input/object.yaml"

if [ ! -s "${INPUT_FILE}" ]; then
	echo "Input file missing; skipping ArgoCD Application cleanup."
	echo "---" > /kratix/output/cleanup.yaml
	exit 0
fi

APP_NAME=$(yq eval '.spec.name // ""' "${INPUT_FILE}")
NAMESPACE=$(yq eval '.spec.namespace // ""' "${INPUT_FILE}")

if [ -z "${APP_NAME}" ] || [ "${APP_NAME}" = "null" ] || \
	 [ -z "${NAMESPACE}" ] || [ "${NAMESPACE}" = "null" ]; then
	echo "Missing required fields; skipping ArgoCD Application cleanup."
	echo "---" > /kratix/output/cleanup.yaml
	exit 0
fi

cat > /kratix/output/cleanup.yaml <<EOF
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
	name: ${APP_NAME}
	namespace: ${NAMESPACE}
EOF
