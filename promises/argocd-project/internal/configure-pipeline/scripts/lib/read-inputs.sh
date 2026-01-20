#!/usr/bin/env bash

INPUT_FILE="/kratix/input/object.yaml"

PROJECT_NAME=$(yq eval '.spec.name' "${INPUT_FILE}")
NAMESPACE=$(yq eval '.spec.namespace' "${INPUT_FILE}")
DESCRIPTION=$(yq eval '.spec.description // ""' "${INPUT_FILE}")
