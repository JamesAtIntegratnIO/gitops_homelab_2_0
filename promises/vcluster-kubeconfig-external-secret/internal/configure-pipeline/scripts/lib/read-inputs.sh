#!/usr/bin/env bash

INPUT_FILE="/kratix/input/object.yaml"

NAME=$(yq eval '.spec.name' "${INPUT_FILE}")
TARGET_NAMESPACE=$(yq eval '.spec.targetNamespace' "${INPUT_FILE}")
ONEPASSWORD_ITEM=$(yq eval '.spec.onepasswordItem' "${INPUT_FILE}")
