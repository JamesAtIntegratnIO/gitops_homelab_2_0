#!/usr/bin/env bash

INPUT_FILE="/kratix/input/object.yaml"

PROMISE_NAME="argocd-application"
RESOURCE_NAME=$(yq eval '.metadata.name' "${INPUT_FILE}")

APP_NAME=$(yq eval '.spec.name' "${INPUT_FILE}")
NAMESPACE=$(yq eval '.spec.namespace' "${INPUT_FILE}")
PROJECT=$(yq eval '.spec.project' "${INPUT_FILE}")

REPO_URL=$(yq eval '.spec.source.repoURL' "${INPUT_FILE}")
CHART=$(yq eval '.spec.source.chart' "${INPUT_FILE}")
TARGET_REVISION=$(yq eval '.spec.source.targetRevision' "${INPUT_FILE}")
RELEASE_NAME=$(yq eval '.spec.source.helm.releaseName // ""' "${INPUT_FILE}")

DEST_SERVER=$(yq eval '.spec.destination.server' "${INPUT_FILE}")
DEST_NAMESPACE=$(yq eval '.spec.destination.namespace' "${INPUT_FILE}")
