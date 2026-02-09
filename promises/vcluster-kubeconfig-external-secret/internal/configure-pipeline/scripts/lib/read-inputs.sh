#!/usr/bin/env bash

INPUT_FILE="/kratix/input/object.yaml"

PROMISE_NAME="vcluster-kubeconfig-external-secret"
RESOURCE_NAME=$(yq eval '.metadata.name' "${INPUT_FILE}")

NAME=$(yq eval '.spec.name' "${INPUT_FILE}")
TARGET_NAMESPACE=$(yq eval '.spec.targetNamespace' "${INPUT_FILE}")
ONEPASSWORD_ITEM=$(yq eval '.spec.onepasswordItem' "${INPUT_FILE}")
