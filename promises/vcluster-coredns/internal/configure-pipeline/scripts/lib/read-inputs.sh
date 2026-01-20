#!/usr/bin/env bash

INPUT_FILE="/kratix/input/object.yaml"

NAME=$(yq eval '.spec.name' "${INPUT_FILE}")
TARGET_NAMESPACE=$(yq eval '.spec.targetNamespace' "${INPUT_FILE}")
CLUSTER_DOMAIN=$(yq eval '.spec.clusterDomain' "${INPUT_FILE}")
