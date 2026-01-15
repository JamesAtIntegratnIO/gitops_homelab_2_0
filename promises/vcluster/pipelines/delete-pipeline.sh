#!/usr/bin/env bash
set -euo pipefail

# Read values from ResourceRequest
NAME=$(yq eval '.spec.name' /kratix/input/object.yaml)
NAMESPACE=$(yq eval '.metadata.namespace' /kratix/input/object.yaml)

echo "Deleting vcluster resources for: ${NAME}"

# Output empty files to signal deletion - Kratix will prune resources
# The HelmRelease deletion will trigger vcluster cleanup
echo "---" > /kratix/output/cleanup.yaml

echo "Deletion pipeline complete for vcluster: ${NAME}"
