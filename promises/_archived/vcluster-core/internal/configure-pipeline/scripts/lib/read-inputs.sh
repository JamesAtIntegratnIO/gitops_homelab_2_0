#!/usr/bin/env bash

INPUT_FILE="/kratix/input/object.yaml"

PROMISE_NAME="vcluster-core"
RESOURCE_NAME=$(yq eval '.metadata.name' "${INPUT_FILE}")

NAME=$(yq eval '.spec.name' "${INPUT_FILE}")
TARGET_NAMESPACE=$(yq eval '.spec.targetNamespace' "${INPUT_FILE}")
VALUES_YAML=$(yq eval '.spec.valuesYaml' "${INPUT_FILE}")
