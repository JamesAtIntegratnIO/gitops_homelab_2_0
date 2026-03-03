#!/usr/bin/env bash
# restart-nfs-pods.sh — Rolling restart of all pods backed by NFS PVCs.
# Use after an NFS server reboot to re-establish stale mounts.
#
# Usage:
#   ./hack/restart-nfs-pods.sh          # restart all NFS-dependent workloads
#   ./hack/restart-nfs-pods.sh --dry-run  # show what would be restarted

set -euo pipefail

DRY_RUN=false
if [[ "${1:-}" == "--dry-run" ]]; then
  DRY_RUN=true
  echo "=== DRY RUN — no changes will be made ==="
  echo
fi

# ---------------------------------------------------------------------------
# Step 1: Discover all NFS-backed PVs and their bound PVCs
# ---------------------------------------------------------------------------
echo "Discovering NFS-backed PersistentVolumes..."

NFS_PVCS=$(kubectl get pv -o json | jq -r '
  .items[]
  | select(
      .spec.nfs != null
      or .spec.csi.driver == "nfs.csi.k8s.io"
    )
  | "\(.spec.claimRef.namespace)/\(.spec.claimRef.name)"
')

if [[ -z "$NFS_PVCS" ]]; then
  echo "No NFS-backed PVCs found. Nothing to do."
  exit 0
fi

echo "Found NFS PVCs:"
echo "$NFS_PVCS" | sed 's/^/  /'
echo

# ---------------------------------------------------------------------------
# Step 2: Map PVCs → owning controllers (Deployment / StatefulSet)
# ---------------------------------------------------------------------------
echo "Mapping PVCs to workload controllers..."

# Build a set of NFS PVC keys for fast lookup
declare -A NFS_PVC_SET
while IFS= read -r pvc; do
  NFS_PVC_SET["$pvc"]=1
done <<< "$NFS_PVCS"

# For each pod, check if any of its PVCs are in the NFS set.
# Resolve the pod's owner to the top-level controller (Deployment or StatefulSet).
declare -A CONTROLLERS   # key = "kind/namespace/name"

POD_JSON=$(kubectl get pods --all-namespaces -o json)

while IFS=$'\t' read -r ns pod_name pvc_name owner_kind owner_name; do
  pvc_key="${ns}/${pvc_name}"
  if [[ -n "${NFS_PVC_SET[$pvc_key]:-}" ]]; then
    # Resolve ReplicaSet → Deployment if needed
    if [[ "$owner_kind" == "ReplicaSet" ]]; then
      dep=$(kubectl get replicaset -n "$ns" "$owner_name" -o jsonpath='{.metadata.ownerReferences[0].name}' 2>/dev/null || true)
      if [[ -n "$dep" ]]; then
        owner_kind="Deployment"
        owner_name="$dep"
      fi
    fi
    key="${owner_kind}/${ns}/${owner_name}"
    CONTROLLERS["$key"]=1
  fi
done < <(echo "$POD_JSON" | jq -r '
  .items[]
  | . as $pod
  | ($pod.spec.volumes // [])[]
  | select(.persistentVolumeClaim != null)
  | [
      $pod.metadata.namespace,
      $pod.metadata.name,
      .persistentVolumeClaim.claimName,
      ($pod.metadata.ownerReferences[0].kind // "None"),
      ($pod.metadata.ownerReferences[0].name // "None")
    ]
  | @tsv
')

if [[ ${#CONTROLLERS[@]} -eq 0 ]]; then
  echo "No running pods found using NFS PVCs. Nothing to restart."
  exit 0
fi

echo
echo "Controllers to restart:"
for key in $(echo "${!CONTROLLERS[@]}" | tr ' ' '\n' | sort); do
  IFS='/' read -r kind ns name <<< "$key"
  echo "  ${kind} ${ns}/${name}"
done
echo

# ---------------------------------------------------------------------------
# Step 3: Rolling restart each controller
# ---------------------------------------------------------------------------
SUCCESS=0
FAILED=0

for key in $(echo "${!CONTROLLERS[@]}" | tr ' ' '\n' | sort); do
  IFS='/' read -r kind ns name <<< "$key"
  resource_type=$(echo "$kind" | tr '[:upper:]' '[:lower:]')

  # Only Deployments, StatefulSets, and DaemonSets support rollout restart.
  # vcluster-synced pods show "Service" as owner — they restart with the vcluster itself.
  if [[ "$resource_type" != "deployment" && "$resource_type" != "statefulset" && "$resource_type" != "daemonset" ]]; then
    echo "Skipping ${kind} ${ns}/${name} (not a rollout-restartable resource)"
    continue
  fi

  if [[ "$DRY_RUN" == true ]]; then
    echo "[dry-run] kubectl rollout restart ${resource_type} -n ${ns} ${name}"
  else
    echo -n "Restarting ${kind} ${ns}/${name}... "
    if kubectl rollout restart "${resource_type}" -n "${ns}" "${name}" > /dev/null 2>&1; then
      echo "✓"
      ((SUCCESS++))
    else
      echo "✗ (failed)"
      ((FAILED++))
    fi
  fi
done

echo
if [[ "$DRY_RUN" == true ]]; then
  echo "Dry run complete. ${#CONTROLLERS[@]} controller(s) would be restarted."
else
  echo "Done. ${SUCCESS} restarted, ${FAILED} failed out of ${#CONTROLLERS[@]} total."
fi
