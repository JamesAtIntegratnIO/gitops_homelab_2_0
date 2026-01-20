#!/usr/bin/env bash

build_service_values() {
  SERVICE_VALUES=""
  if [ -n "${SUBNET}" ] && [ "${SUBNET}" != "null" ]; then
    if [ -z "${VIP}" ] || [ "${VIP}" = "null" ]; then
      VIP=$(default_vip_from_cidr "${SUBNET}" 100)
    fi
    if [ -z "${VIP}" ]; then
      echo "Failed to compute default VIP from subnet ${SUBNET}"
      exit 1
    fi
    if ! ip_in_cidr "${VIP}" "${SUBNET}"; then
      echo "VIP ${VIP} is not within subnet ${SUBNET}"
      exit 1
    fi
  fi

  SERVICE_LOADBALANCER_IP_LINE=""
  if [ -n "${VIP}" ] && [ "${VIP}" != "null" ]; then
    SERVICE_LOADBALANCER_IP_LINE="      loadBalancerIP: \"${VIP}\""
  fi

  echo "Configuring LoadBalancer service for ${HOSTNAME}"

  SERVICE_VALUES=$(cat <<EOF
  service:
    enabled: true
    annotations:
      external-dns.alpha.kubernetes.io/hostname: "${HOSTNAME}"
    spec:
      type: LoadBalancer
${SERVICE_LOADBALANCER_IP_LINE}
      ports:
        - name: https
          port: ${API_PORT}
          targetPort: 8443
          protocol: TCP
EOF
)

  if [ -n "${HOSTNAME}" ] && [ "${HOSTNAME}" != "null" ]; then
    EXTERNAL_SERVER_URL="https://${HOSTNAME}:${API_PORT}"
  elif [ -n "${VIP}" ] && [ "${VIP}" != "null" ]; then
    EXTERNAL_SERVER_URL="https://${VIP}:${API_PORT}"
  fi
}

build_proxy_extra_sans() {
  PROXY_EXTRA_SANS_VALUES=""
  if [ -n "${HOSTNAME}" ] && [ "${HOSTNAME}" != "null" ]; then
    PROXY_EXTRA_SANS_VALUES=$(cat <<EOF
  proxy:
    extraSANs:
      - "${HOSTNAME}"
EOF
)
    if [ -n "${VIP}" ] && [ "${VIP}" != "null" ]; then
      PROXY_EXTRA_SANS_VALUES=$(cat <<EOF
${PROXY_EXTRA_SANS_VALUES}
      - "${VIP}"
EOF
)
    fi
  elif [ -n "${VIP}" ] && [ "${VIP}" != "null" ]; then
    PROXY_EXTRA_SANS_VALUES=$(cat <<EOF
  proxy:
    extraSANs:
      - "${VIP}"
EOF
)
  fi
}

build_persistence_storage_class_lines() {
  PERSISTENCE_STORAGE_CLASS_CM_LINE=""
  PERSISTENCE_STORAGE_CLASS_APP_LINE=""
  if [ -n "${PERSISTENCE_STORAGE_CLASS}" ] && [ "${PERSISTENCE_STORAGE_CLASS}" != "null" ]; then
    PERSISTENCE_STORAGE_CLASS_CM_LINE="            storageClass: \"${PERSISTENCE_STORAGE_CLASS}\""
    PERSISTENCE_STORAGE_CLASS_APP_LINE="                storageClass: \"${PERSISTENCE_STORAGE_CLASS}\""
  fi
}

write_namespace() {
  cat > /kratix/output/namespace.yaml <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: ${NAMESPACE}
  labels:
    app: vcluster
    instance: ${NAME}
EOF
}

write_coredns_configmap() {
  cat > /kratix/output/coredns-configmap.yaml <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: coredns-x-kube-system-x-${NAME}
  namespace: ${NAMESPACE}
  labels:
    app: vcluster
    instance: ${NAME}
data:
  Corefile: |
    .:1053 {
      errors
      health
      ready
      kubernetes ${CLUSTER_DOMAIN} in-addr.arpa ip6.arpa {
        pods insecure
        fallthrough in-addr.arpa ip6.arpa
        ttl 30
      }
      prometheus 0.0.0.0:9153
      forward . /etc/resolv.conf
      cache 30
      loop
      reload
      loadbalance
    }
  NodeHosts: ""
EOF
}

write_argocd_project() {
  cat > /kratix/output/argocd-project.yaml <<EOF
apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: ${PROJECT_NAME}
  namespace: argocd
  annotations:
    argocd.argoproj.io/sync-wave: "-1"
  labels:
    app.kubernetes.io/managed-by: kratix
    kratix.io/promise-name: vcluster
    kratix.io/resource-request: ${NAME}
    argocd.argoproj.io/project-group: appteam
spec:
  description: VCluster project for ${NAME}
  sourceRepos:
    - https://charts.loft.sh
  destinations:
    - server: https://kubernetes.default.svc
      namespace: ${NAMESPACE}
  clusterResourceWhitelist:
    - group: '*'
      kind: '*'
  namespaceResourceWhitelist:
    - group: '*'
      kind: '*'
EOF
}

build_values_files() {
  VALUES_BASE_FILE="/tmp/vcluster-values-base.yaml"
  VALUES_OVERRIDES_FILE="/tmp/vcluster-values-overrides.yaml"
  VALUES_MERGED_FILE="/tmp/vcluster-values-merged.yaml"

  cat > "${VALUES_BASE_FILE}" <<EOF
controlPlane:
  distro:
    k8s:
      enabled: true
      version: "${K8S_VERSION}"
  statefulSet:
    highAvailability:
      replicas: ${REPLICAS}
    image:
      repository: "loft-sh/vcluster-oss"
    persistence:
      volumeClaim:
        enabled: ${PERSISTENCE_ENABLED}
        size: "${PERSISTENCE_SIZE}"
${PERSISTENCE_STORAGE_CLASS_CM_LINE}
    resources:
      requests:
        cpu: "${CPU_REQUEST}"
        memory: "${MEMORY_REQUEST}"
      limits:
        cpu: "${CPU_LIMIT}"
        memory: "${MEMORY_LIMIT}"
  coredns:
    enabled: true
    deployment:
      replicas: ${COREDNS_REPLICAS}
    overwriteConfig: |
      .:1053 {
        errors
        health
        ready
        kubernetes ${CLUSTER_DOMAIN} in-addr.arpa ip6.arpa {
          pods insecure
          fallthrough in-addr.arpa ip6.arpa
          ttl 30
        }
        prometheus 0.0.0.0:9153
        forward . /etc/resolv.conf
        cache 30
        loop
        reload
        loadbalance
      }
${PROXY_EXTRA_SANS_VALUES}
${SERVICE_VALUES}

deploy:
  metallb:
    enabled: true

integrations:
  externalSecrets:
    enabled: true
    webhook:
      enabled: true
    sync:
      fromHost:
        clusterStores:
          enabled: true
          selector:
            matchLabels:
${EXTERNAL_SECRETS_CLUSTER_STORE_LABELS}
  metricsServer:
    enabled: true
  certManager:
    enabled: true
    sync:
      fromHost:
        clusterIssuers:
          enabled: true
          selector:
            labels:
${CERT_MANAGER_CLUSTER_ISSUER_LABELS}

telemetry:
  enabled: false

networking:
  advanced:
    clusterDomain: "${CLUSTER_DOMAIN}"

sync:
  toHost:
    pods:
      enabled: true
  fromHost:
    secrets:
      enabled: true
      mappings:
        byName:
          "external-secrets/eso-onepassword-token": "external-secrets/eso-onepassword-token"

rbac:
  clusterRole:
    enabled: true
    extraRules:
      - apiGroups: [""]
        resources: ["secrets"]
        verbs: ["get", "list", "watch"]
        resourceNames:
          - "eso-onepassword-token"
EOF

  yq eval '.spec.helmOverrides // {}' /kratix/input/object.yaml > "${VALUES_OVERRIDES_FILE}"
  yq eval-all 'select(fileIndex==0) * select(fileIndex==1)' "${VALUES_BASE_FILE}" "${VALUES_OVERRIDES_FILE}" > "${VALUES_MERGED_FILE}"

  VALUES_CONFIGMAP=$(sed 's/^/    /' "${VALUES_MERGED_FILE}")
  VALUES_OBJECT=$(sed 's/^/        /' "${VALUES_MERGED_FILE}")
}

write_helm_values_configmap() {
  cat > /kratix/output/helm-values.yaml <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: ${NAME}-vcluster-values
  namespace: ${NAMESPACE}
data:
  values.yaml: |
${VALUES_CONFIGMAP}
EOF
}

write_argocd_application() {
  cat > /kratix/output/argocd-application.yaml <<EOF
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: vcluster-${NAME}
  namespace: argocd
  annotations:
    argocd.argoproj.io/sync-wave: "0"
  finalizers:
    - resources-finalizer.argocd.argoproj.io
spec:
  project: ${PROJECT_NAME}
  source:
    repoURL: https://charts.loft.sh
    chart: vcluster
    targetRevision: 0.30.4
    helm:
      releaseName: ${NAME}
      valuesObject:
${VALUES_OBJECT}
  destination:
    server: https://kubernetes.default.svc
    namespace: ${NAMESPACE}
  syncPolicy:
    automated:
      selfHeal: true
      prune: true
    syncOptions:
      - CreateNamespace=true
EOF
}

write_kubeconfig_sync_job() {
  cat > /kratix/output/kubeconfig-sync-job.yaml <<EOF
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: vcluster-${NAME}-onepassword-token
  namespace: ${NAMESPACE}
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
  namespace: ${NAMESPACE}
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: vcluster-${NAME}-kubeconfig-reader
  namespace: ${NAMESPACE}
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
  namespace: ${NAMESPACE}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: vcluster-${NAME}-kubeconfig-reader
subjects:
  - kind: ServiceAccount
    name: vcluster-${NAME}-kubeconfig-sync
    namespace: ${NAMESPACE}
---
apiVersion: batch/v1
kind: Job
metadata:
  name: ${KUBECONFIG_SYNC_JOB_NAME}
  namespace: ${NAMESPACE}
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
        # Wait for vcluster kubeconfig secret to be created
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
              until kubectl get secret vc-${NAME} -n ${NAMESPACE} 2>/dev/null; do
                echo "Secret not found, waiting..."
                sleep 10
              done
              echo "Writing kubeconfig to shared volume..."
              kubectl get secret vc-${NAME} -n ${NAMESPACE} -o jsonpath='{.data.config}' | base64 -d > /shared/kubeconfig
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
              value: "${NAMESPACE}"
            - name: VCLUSTER_NAME
              value: "${NAME}"
            - name: HOSTNAME
              value: "${HOSTNAME}"
            - name: API_PORT
              value: "${API_PORT}"
            - name: SERVER_URL
              value: "${EXTERNAL_SERVER_URL}"
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
                  !done && $1=="server:" {print "    server: " new_server; done=1; next}
                  {print}
                ' /shared/kubeconfig > /shared/kubeconfig.rewritten
                mv /shared/kubeconfig.rewritten /shared/kubeconfig
              fi

              ARGOCD_CLUSTER_NAME="vcluster-${VCLUSTER_NAME}"
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

              echo "Syncing kubeconfig to 1Password item via Connect API: ${OP_ITEM_NAME}"

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

              ITEM_ID=$(curl -fsS -H "${AUTH_HEADER}" "${API_BASE}/vaults/${VAULT_ID}/items" | jq -r --arg title "${OP_ITEM_NAME}" '.[] | select(.title==$title) | .id' | head -n1)

              if [ -n "${ITEM_ID}" ]; then
                echo "Item exists, replacing..."
                ITEM_PAYLOAD=$(jq -n --arg id "${ITEM_ID}" --arg title "${OP_ITEM_NAME}" --arg vault "${VAULT_ID}" --arg notes "${KUBECONFIG_CONTENT}" --arg argocdName "${ARGOCD_CLUSTER_NAME}" --arg argocdServer "${KUBECONFIG_SERVER}" --arg argocdConfig "${ARGOCD_CONFIG_JSON}" '{id:$id,title:$title,vault:{id:$vault},category:"SECURE_NOTE",notesPlain:$notes,fields:[{label:"kubeconfig",type:"CONCEALED",value:$notes},{label:"argocd-name",type:"STRING",value:$argocdName},{label:"argocd-server",type:"STRING",value:$argocdServer},{label:"argocd-config",type:"CONCEALED",value:$argocdConfig}]}')
                curl -fsS -X PUT -H "${AUTH_HEADER}" -H "Content-Type: application/json" "${API_BASE}/vaults/${VAULT_ID}/items/${ITEM_ID}" -d "${ITEM_PAYLOAD}" >/dev/null
              else
                echo "Item not found, creating..."
                ITEM_PAYLOAD=$(jq -n --arg title "${OP_ITEM_NAME}" --arg vault "${VAULT_ID}" --arg notes "${KUBECONFIG_CONTENT}" --arg argocdName "${ARGOCD_CLUSTER_NAME}" --arg argocdServer "${KUBECONFIG_SERVER}" --arg argocdConfig "${ARGOCD_CONFIG_JSON}" '{title:$title,vault:{id:$vault},category:"SECURE_NOTE",notesPlain:$notes,fields:[{label:"kubeconfig",type:"CONCEALED",value:$notes},{label:"argocd-name",type:"STRING",value:$argocdName},{label:"argocd-server",type:"STRING",value:$argocdServer},{label:"argocd-config",type:"CONCEALED",value:$argocdConfig}]}')
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

write_external_secret() {
  cat > /kratix/output/external-secret.yaml <<EOF
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: vcluster-${NAME}-kubeconfig
  namespace: ${NAMESPACE}
  labels:
    app: vcluster
    instance: ${NAME}
spec:
  refreshInterval: 5m
  secretStoreRef:
    name: onepassword-store
    kind: ClusterSecretStore
  target:
    name: vcluster-${NAME}-kubeconfig-external
    creationPolicy: Owner
  data:
    - secretKey: kubeconfig
      remoteRef:
        key: ${ONEPASSWORD_ITEM}
        property: kubeconfig
EOF
}

write_argocd_cluster_secret() {
  cat > /kratix/output/argocd-cluster-secret.yaml <<EOF
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: vcluster-${NAME}-argocd-cluster
  namespace: argocd
  labels:
    app: vcluster
    instance: ${NAME}
spec:
  refreshInterval: 5m
  secretStoreRef:
    name: onepassword-store
    kind: ClusterSecretStore
  target:
    name: vcluster-${NAME}
    creationPolicy: Owner
    template:
      metadata:
        labels:
${ARGOCD_CLUSTER_LABELS_INDENTED}
        annotations:
${ARGOCD_CLUSTER_ANNOTATIONS_INDENTED}
      type: Opaque
  data:
    - secretKey: name
      remoteRef:
        key: ${ONEPASSWORD_ITEM}
        property: argocd-name
    - secretKey: server
      remoteRef:
        key: ${ONEPASSWORD_ITEM}
        property: argocd-server
    - secretKey: config
      remoteRef:
        key: ${ONEPASSWORD_ITEM}
        property: argocd-config
EOF
}

render_all_resources() {
  build_service_values
  build_proxy_extra_sans
  build_persistence_storage_class_lines

  write_namespace
  write_coredns_configmap
  write_argocd_project
  build_values_files
  write_helm_values_configmap
  write_argocd_application
  write_kubeconfig_sync_job
  write_external_secret
  write_argocd_cluster_secret

  echo "Resources generated successfully for vcluster: ${NAME}"
}
