#!/usr/bin/env bash

INPUT_FILE="/kratix/input/object.yaml"

PROMISE_NAME="argocd-project"
RESOURCE_NAME=$(yq eval '.metadata.name' "${INPUT_FILE}")

PROJECT_NAME=$(yq eval '.spec.name' "${INPUT_FILE}")
NAMESPACE=$(yq eval '.spec.namespace' "${INPUT_FILE}")
DESCRIPTION=$(yq eval '.spec.description // ""' "${INPUT_FILE}")
