#!/usr/bin/env bash
set -euo pipefail

# Read values from ResourceRequest
NAME=$(yq eval '.spec.name' /kratix/input/object.yaml)
REQUEST_NAMESPACE=$(yq eval '.metadata.namespace' /kratix/input/object.yaml)
NAMESPACE=$(yq eval '.spec.targetNamespace // ""' /kratix/input/object.yaml)

if [ -z "${NAMESPACE}" ] || [ "${NAMESPACE}" = "null" ]; then
	NAMESPACE="${REQUEST_NAMESPACE}"
fi

# Generate 1Password item name for kubeconfig
ONEPASSWORD_ITEM="vcluster-${NAME}-kubeconfig"

echo "Deleting vcluster resources for: ${NAME}"

# Create Job to delete the 1Password item
cat > /kratix/output/onepassword-delete-job.yaml <<EOF
apiVersion: batch/v1
kind: Job
metadata:
  name: vcluster-${NAME}-onepassword-delete
  namespace: ${NAMESPACE}
  labels:
    app: vcluster
    instance: ${NAME}
    component: onepassword-delete
spec:
  ttlSecondsAfterFinished: 300
  template:
    metadata:
      labels:
        app: vcluster
        instance: ${NAME}
    spec:
      restartPolicy: OnFailure
      containers:
        - name: delete-item
          image: alpine:3.19
          env:
            - name: OP_CONNECT_HOST
              value: "https://connect.integratn.tech"
            - name: OP_CONNECT_TOKEN
              valueFrom:
                secretKeyRef:
                  name: vcluster-${NAME}-onepassword-token
                  key: token
            - name: OP_VAULT_ID
              valueFrom:
                secretKeyRef:
                  name: vcluster-${NAME}-onepassword-token
                  key: vault
            - name: OP_ITEM_NAME
              value: "${ONEPASSWORD_ITEM}"
          command:
            - /bin/sh
            - -c
            - |
              set -e
              apk add --no-cache ca-certificates curl jq
              VAULT_NAME="homelab"
              OP_CONNECT_HOST_CLEAN="\$(printf '%s' "\${OP_CONNECT_HOST}" | tr -d '\r\n')"
              OP_CONNECT_TOKEN_CLEAN="\$(printf '%s' "\${OP_CONNECT_TOKEN}" | tr -d '\r\n')"
              API_BASE="\${OP_CONNECT_HOST_CLEAN%/}/v1"
              AUTH_HEADER="Authorization: Bearer \${OP_CONNECT_TOKEN_CLEAN}"

              VAULT_ID="\${OP_VAULT_ID:-}"
              VAULT_ID="\$(printf '%s' "\${VAULT_ID}" | tr -d '\r\n')"
              if [ -z "\${VAULT_ID}" ]; then
                VAULT_ID=\$(curl -fsS -H "\${AUTH_HEADER}" "\${API_BASE}/vaults" | jq -r --arg name "\${VAULT_NAME}" '.[] | select(.name==$name) | .id' | head -n1)
              fi
              VAULT_ID="\$(printf '%s' "\${VAULT_ID}" | tr -d '\r\n')"
              if [ -z "\${VAULT_ID}" ]; then
                echo "Vault not found: \${VAULT_NAME}"
                exit 1
              fi

              ITEM_ID=\$(curl -fsS -H "\${AUTH_HEADER}" "\${API_BASE}/vaults/\${VAULT_ID}/items" | jq -r --arg title "\${OP_ITEM_NAME}" '.[] | select(.title==$title) | .id' | head -n1)
              if [ -n "\${ITEM_ID}" ]; then
                echo "Deleting 1Password item \${OP_ITEM_NAME} (\${ITEM_ID})"
                curl -fsS -X DELETE -H "\${AUTH_HEADER}" "\${API_BASE}/vaults/\${VAULT_ID}/items/\${ITEM_ID}" >/dev/null
              else
                echo "1Password item not found: \${OP_ITEM_NAME}"
              fi
EOF

# Output empty file to signal deletion - Kratix will prune resources
# The HelmRelease deletion will trigger vcluster cleanup
echo "---" > /kratix/output/cleanup.yaml

echo "Deletion pipeline complete for vcluster: ${NAME}"
