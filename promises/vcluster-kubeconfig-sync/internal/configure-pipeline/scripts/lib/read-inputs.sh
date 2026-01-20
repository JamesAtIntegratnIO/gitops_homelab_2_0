#!/usr/bin/env bash

INPUT_FILE="/kratix/input/object.yaml"

NAME=$(yq eval '.spec.name' "${INPUT_FILE}")
TARGET_NAMESPACE=$(yq eval '.spec.targetNamespace' "${INPUT_FILE}")
KUBECONFIG_SYNC_JOB_NAME=$(yq eval '.spec.kubeconfigSyncJobName' "${INPUT_FILE}")
ONEPASSWORD_ITEM=$(yq eval '.spec.onepasswordItem' "${INPUT_FILE}")
HOSTNAME=$(yq eval '.spec.hostname // ""' "${INPUT_FILE}")
API_PORT=$(yq eval '.spec.apiPort // 8443' "${INPUT_FILE}")
SERVER_URL=$(yq eval '.spec.serverUrl // ""' "${INPUT_FILE}")
