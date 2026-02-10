#!/usr/bin/env bash

INPUT_FILE="/kratix/input/object.yaml"

PROMISE_NAME="vcluster-kubeconfig-sync"
RESOURCE_NAME=$(yq eval '.metadata.name' "${INPUT_FILE}")

NAME=$(yq eval '.spec.name' "${INPUT_FILE}")
TARGET_NAMESPACE=$(yq eval '.spec.targetNamespace' "${INPUT_FILE}")
KUBECONFIG_SYNC_JOB_NAME=$(yq eval '.spec.kubeconfigSyncJobName' "${INPUT_FILE}")
ONEPASSWORD_ITEM=$(yq eval '.spec.onepasswordItem' "${INPUT_FILE}")
HOSTNAME=$(yq eval '.spec.hostname // ""' "${INPUT_FILE}")
API_PORT=$(yq eval '.spec.apiPort // 443' "${INPUT_FILE}")
SERVER_URL=$(yq eval '.spec.serverUrl // ""' "${INPUT_FILE}")

if [ -z "${SERVER_URL}" ] || [ "${SERVER_URL}" = "null" ]; then
	if [ -n "${HOSTNAME}" ] && [ "${HOSTNAME}" != "null" ]; then
		SERVER_URL="https://${HOSTNAME}:${API_PORT}"
	fi
fi
