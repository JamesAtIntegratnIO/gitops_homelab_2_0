#!/usr/bin/env bash

write_kubeconfig_sync_job() {
  cat > /kratix/output/kubeconfig-sync-job.yaml <<EOF
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: vcluster-${NAME}-onepassword-token
  namespace: ${TARGET_NAMESPACE}
spec:
  refreshInterval: 1h
  secretStoreRef:
    name: onepassword-store
    kind: ClusterSecretStore
  target:
    name: vcluster-${NAME}-onepassword-token
    creationPolicy: Owner
  data:
    - secretKey: token
      remoteRef:
        key: onepassword-access-token
        property: credential
    - secretKey: vault
      remoteRef:
        key: onepassword-access-token
        property: vault
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: vcluster-${NAME}-kubeconfig-sync
  namespace: ${TARGET_NAMESPACE}
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: vcluster-${NAME}-kubeconfig-reader
  namespace: ${TARGET_NAMESPACE}
rules:
  - apiGroups: [""]
    resources: ["secrets"]
    resourceNames: ["vc-${NAME}", "vcluster-${NAME}-onepassword-token"]
    verbs: ["get"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: vcluster-${NAME}-kubeconfig-sync
  namespace: ${TARGET_NAMESPACE}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: vcluster-${NAME}-kubeconfig-reader
subjects:
  - kind: ServiceAccount
    name: vcluster-${NAME}-kubeconfig-sync
    namespace: ${TARGET_NAMESPACE}
---
apiVersion: batch/v1
kind: Job
metadata:
  name: ${KUBECONFIG_SYNC_JOB_NAME}
  namespace: ${TARGET_NAMESPACE}
  labels:
    app: vcluster
    instance: ${NAME}
    component: kubeconfig-sync
spec:
  ttlSecondsAfterFinished: 300
  template:
    metadata:
      labels:
        app: vcluster
        instance: ${NAME}
    spec:
      serviceAccountName: vcluster-${NAME}-kubeconfig-sync
      restartPolicy: OnFailure
      volumes:
        - name: sync-data
          emptyDir: {}
      initContainers:
        - name: wait-for-kubeconfig
          image: bitnami/kubectl:latest
          volumeMounts:
            - name: sync-data
              mountPath: /shared
          command:
            - /bin/bash
            - -c
            - |
              echo "Waiting for vcluster kubeconfig secret vc-${NAME}..."
              until kubectl get secret vc-${NAME} -n ${TARGET_NAMESPACE} 2>/dev/null; do
                echo "Secret not found, waiting..."
                sleep 10
              done
              echo "Writing kubeconfig to shared volume..."
              kubectl get secret vc-${NAME} -n ${TARGET_NAMESPACE} -o jsonpath='{.data.config}' | base64 -d > /shared/kubeconfig
              echo "Secret found!"
      containers:
        - name: sync-to-onepassword
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
            - name: NAMESPACE
              value: "${TARGET_NAMESPACE}"
            - name: VCLUSTER_NAME
              value: "${NAME}"
            - name: HOSTNAME
              value: "${HOSTNAME}"
            - name: API_PORT
              value: "${API_PORT}"
            - name: SERVER_URL
              value: "${SERVER_URL}"
            - name: OP_ITEM_NAME
              value: "${ONEPASSWORD_ITEM}"
          volumeMounts:
            - name: sync-data
              mountPath: /shared
          command:
            - /bin/sh
            - -c
            - |
              set -e
              apk add --no-cache ca-certificates curl jq yq

              if [ -n "${SERVER_URL}" ]; then
                echo "Rewriting kubeconfig server to ${SERVER_URL}"
                awk -v new_server="${SERVER_URL}" '
                  !done && \$1=="server:" {print "    server: " new_server; done=1; next}
                  {print}
                ' /shared/kubeconfig > /shared/kubeconfig.rewritten
                mv /shared/kubeconfig.rewritten /shared/kubeconfig
              fi

              ARGOCD_CLUSTER_NAME="vcluster-${NAME}"
              KUBECONFIG_SERVER=$(yq -r '.clusters[0].cluster.server // ""' /shared/kubeconfig)
              KUBECONFIG_CA_DATA=$(yq -r '.clusters[0].cluster."certificate-authority-data" // ""' /shared/kubeconfig)
              KUBECONFIG_CERT_DATA=$(yq -r '.users[0].user."client-certificate-data" // ""' /shared/kubeconfig)
              KUBECONFIG_KEY_DATA=$(yq -r '.users[0].user."client-key-data" // ""' /shared/kubeconfig)
              KUBECONFIG_TOKEN=$(yq -r '.users[0].user.token // ""' /shared/kubeconfig)

              if [ -z "${KUBECONFIG_SERVER}" ]; then
                echo "Failed to extract server from kubeconfig"
                exit 1
              fi

              if [ -n "${KUBECONFIG_TOKEN}" ]; then
                ARGOCD_CONFIG_JSON=$(jq -n --arg token "${KUBECONFIG_TOKEN}" --arg ca "${KUBECONFIG_CA_DATA}" '{bearerToken:$token,tlsClientConfig:{insecure:false,caData:$ca}}')
              else
                if [ -z "${KUBECONFIG_CERT_DATA}" ] || [ -z "${KUBECONFIG_KEY_DATA}" ]; then
                  echo "Failed to extract client cert/key from kubeconfig"
                  exit 1
                fi
                ARGOCD_CONFIG_JSON=$(jq -n --arg ca "${KUBECONFIG_CA_DATA}" --arg cert "${KUBECONFIG_CERT_DATA}" --arg key "${KUBECONFIG_KEY_DATA}" '{tlsClientConfig:{insecure:false,caData:$ca,certData:$cert,keyData:$key}}')
              fi

              KUBECONFIG_CONTENT=$(cat /shared/kubeconfig)
              KUBECONFIG_BYTES=$(printf '%s' "${KUBECONFIG_CONTENT}" | wc -c | tr -d ' ')
              if [ "${KUBECONFIG_BYTES}" -eq 0 ]; then
                echo "Kubeconfig content is empty; aborting sync"
                exit 1
              fi

              VAULT_NAME="homelab"
              OP_CONNECT_HOST_CLEAN="$(printf '%s' "${OP_CONNECT_HOST}" | tr -d '\r\n')"
              OP_CONNECT_TOKEN_CLEAN="$(printf '%s' "${OP_CONNECT_TOKEN}" | tr -d '\r\n')"
              API_BASE="${OP_CONNECT_HOST_CLEAN%/}/v1"
              AUTH_HEADER="Authorization: Bearer ${OP_CONNECT_TOKEN_CLEAN}"

              echo "Syncing kubeconfig to 1Password item via Connect API: ${ONEPASSWORD_ITEM}"

              VAULT_ID="${OP_VAULT_ID:-}"
              VAULT_ID="$(printf '%s' "${VAULT_ID}" | tr -d '\r\n')"
              if [ -z "${VAULT_ID}" ]; then
                VAULT_ID=$(curl -fsS -H "${AUTH_HEADER}" "${API_BASE}/vaults" | jq -r --arg name "${VAULT_NAME}" '.[] | select(.name==$name) | .id' | head -n1)
              fi
              VAULT_ID="$(printf '%s' "${VAULT_ID}" | tr -d '\r\n')"
              if [ -z "${VAULT_ID}" ]; then
                echo "Vault not found: ${VAULT_NAME}"
                exit 1
              fi

              ITEM_ID=$(curl -fsS -H "${AUTH_HEADER}" "${API_BASE}/vaults/${VAULT_ID}/items" | jq -r --arg title "${ONEPASSWORD_ITEM}" '.[] | select(.title==$title) | .id' | head -n1)

              if [ -n "${ITEM_ID}" ]; then
                echo "Item exists, replacing..."
                ITEM_PAYLOAD=$(jq -n --arg id "${ITEM_ID}" --arg title "${ONEPASSWORD_ITEM}" --arg vault "${VAULT_ID}" --arg notes "${KUBECONFIG_CONTENT}" --arg argocdName "${ARGOCD_CLUSTER_NAME}" --arg argocdServer "${KUBECONFIG_SERVER}" --arg argocdConfig "${ARGOCD_CONFIG_JSON}" '{id:$id,title:$title,vault:{id:$vault},category:"SECURE_NOTE",notesPlain:$notes,fields:[{label:"kubeconfig",type:"CONCEALED",value:$notes},{label:"argocd-name",type:"STRING",value:$argocdName},{label:"argocd-server",type:"STRING",value:$argocdServer},{label:"argocd-config",type:"CONCEALED",value:$argocdConfig}]}')
                curl -fsS -X PUT -H "${AUTH_HEADER}" -H "Content-Type: application/json" "${API_BASE}/vaults/${VAULT_ID}/items/${ITEM_ID}" -d "${ITEM_PAYLOAD}" >/dev/null
              else
                echo "Item not found, creating..."
                ITEM_PAYLOAD=$(jq -n --arg title "${ONEPASSWORD_ITEM}" --arg vault "${VAULT_ID}" --arg notes "${KUBECONFIG_CONTENT}" --arg argocdName "${ARGOCD_CLUSTER_NAME}" --arg argocdServer "${KUBECONFIG_SERVER}" --arg argocdConfig "${ARGOCD_CONFIG_JSON}" '{title:$title,vault:{id:$vault},category:"SECURE_NOTE",notesPlain:$notes,fields:[{label:"kubeconfig",type:"CONCEALED",value:$notes},{label:"argocd-name",type:"STRING",value:$argocdName},{label:"argocd-server",type:"STRING",value:$argocdServer},{label:"argocd-config",type:"CONCEALED",value:$argocdConfig}]}')
                ITEM_ID=$(curl -fsS -X POST -H "${AUTH_HEADER}" -H "Content-Type: application/json" "${API_BASE}/vaults/${VAULT_ID}/items" -d "${ITEM_PAYLOAD}" | jq -r '.id')
                if [ -z "${ITEM_ID}" ] || [ "${ITEM_ID}" = "null" ]; then
                  echo "Failed to create item in 1Password"
                  exit 1
                fi
              fi

              ITEM_JSON=$(curl -fsS -H "${AUTH_HEADER}" "${API_BASE}/vaults/${VAULT_ID}/items/${ITEM_ID}")
              NOTES_LEN=$(echo "${ITEM_JSON}" | jq -r '.notesPlain // "" | length')
              FIELD_LEN=$(echo "${ITEM_JSON}" | jq -r '.fields[]? | select(.label=="kubeconfig") | .value | length' | head -n1)
              FIELD_LEN=${FIELD_LEN:-0}
              echo "1Password item lengths: notesPlain=${NOTES_LEN} kubeconfigField=${FIELD_LEN}"

              echo "Kubeconfig synced successfully to 1Password"
EOF
}

render_all_resources() {
  write_kubeconfig_sync_job
}
