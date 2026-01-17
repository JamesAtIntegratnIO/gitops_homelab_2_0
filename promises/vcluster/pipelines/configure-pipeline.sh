#!/usr/bin/env bash
set -euo pipefail

# Read values from ResourceRequest
NAME=$(yq eval '.spec.name' /kratix/input/object.yaml)
K8S_VERSION=$(yq eval '.spec.k8sVersion // "1.34"' /kratix/input/object.yaml)
ISOLATION_MODE=$(yq eval '.spec.isolationMode // "standard"' /kratix/input/object.yaml)
CPU_REQUEST=$(yq eval '.spec.resources.requests.cpu // "200m"' /kratix/input/object.yaml)
MEMORY_REQUEST=$(yq eval '.spec.resources.requests.memory // "512Mi"' /kratix/input/object.yaml)
CPU_LIMIT=$(yq eval '.spec.resources.limits.cpu // "1000m"' /kratix/input/object.yaml)
MEMORY_LIMIT=$(yq eval '.spec.resources.limits.memory // "1Gi"' /kratix/input/object.yaml)
PROJECT_NAME=$(yq eval '.spec.projectName // ""' /kratix/input/object.yaml)

# Get namespaces from ResourceRequest
REQUEST_NAMESPACE=$(yq eval '.metadata.namespace' /kratix/input/object.yaml)
NAMESPACE=$(yq eval '.spec.targetNamespace // ""' /kratix/input/object.yaml)

if [ -z "${NAMESPACE}" ] || [ "${NAMESPACE}" = "null" ]; then
  NAMESPACE="${REQUEST_NAMESPACE}"
fi

if [ -z "${PROJECT_NAME}" ] || [ "${PROJECT_NAME}" = "null" ]; then
  PROJECT_NAME="vcluster-${NAME}"
fi

# Generate 1Password item name for kubeconfig
ONEPASSWORD_ITEM="vcluster-${NAME}-kubeconfig"

echo "Generating vcluster resources for: ${NAME}"
echo "Request namespace: ${REQUEST_NAMESPACE}"
echo "Target namespace: ${NAMESPACE}"
echo "ArgoCD project: ${PROJECT_NAME}"

# Create namespace for vcluster
cat > /kratix/output/namespace.yaml <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: ${NAMESPACE}
  labels:
    app: vcluster
    instance: ${NAME}
EOF

# Create ArgoCD Project for the vcluster
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

# Create Helm values ConfigMap
cat > /kratix/output/helm-values.yaml <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: ${NAME}-vcluster-values
  namespace: ${NAMESPACE}
data:
  values.yaml: |
    controlPlane:
      distro:
        k8s:
          enabled: true
          version: "${K8S_VERSION}"
      statefulSet:
        resources:
          requests:
            cpu: "${CPU_REQUEST}"
            memory: "${MEMORY_REQUEST}"
          limits:
            cpu: "${CPU_LIMIT}"
            memory: "${MEMORY_LIMIT}"
    
    sync:
      toHost:
        pods:
          enabled: true
EOF

# Create ArgoCD Application for vcluster
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
      valuesObject:
        controlPlane:
          distro:
            k8s:
              enabled: true
              version: "${K8S_VERSION}"
          statefulSet:
            resources:
              requests:
                cpu: "${CPU_REQUEST}"
                memory: "${MEMORY_REQUEST}"
              limits:
                cpu: "${CPU_LIMIT}"
                memory: "${MEMORY_LIMIT}"
        
        sync:
          toHost:
            pods:
              enabled: true
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

# Create Job to sync kubeconfig to 1Password after vcluster is ready
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
    resourceNames: ["vc-vcluster-${NAME}", "vcluster-${NAME}-onepassword-token"]
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
  name: vcluster-${NAME}-kubeconfig-sync
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
              until kubectl get secret vc-vcluster-${NAME} -n ${NAMESPACE} 2>/dev/null; do
                echo "Secret not found, waiting..."
                sleep 10
              done
              echo "Writing kubeconfig to shared volume..."
              kubectl get secret vc-vcluster-${NAME} -n ${NAMESPACE} -o jsonpath='{.data.config}' | base64 -d > /shared/kubeconfig
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
              apk add --no-cache ca-certificates curl jq
              KUBECONFIG_CONTENT=\$(cat /shared/kubeconfig)

              VAULT_NAME="homelab"
              OP_CONNECT_HOST_CLEAN="\$(printf '%s' "\${OP_CONNECT_HOST}" | tr -d '\r\n')"
              OP_CONNECT_TOKEN_CLEAN="\$(printf '%s' "\${OP_CONNECT_TOKEN}" | tr -d '\r\n')"
              API_BASE="\${OP_CONNECT_HOST_CLEAN%/}/v1"
              AUTH_HEADER="Authorization: Bearer \${OP_CONNECT_TOKEN_CLEAN}"

              echo "Syncing kubeconfig to 1Password item via Connect API: \${OP_ITEM_NAME}"

              VAULT_ID="\${OP_VAULT_ID:-}"
              VAULT_ID="\$(printf '%s' "\${VAULT_ID}" | tr -d '\r\n')"
              if [ -z "\${VAULT_ID}" ]; then
                VAULT_ID=\$(curl -fsS -H "\${AUTH_HEADER}" "\${API_BASE}/vaults" | jq -r --arg name "\${VAULT_NAME}" '.[] | select(.name==\$name) | .id' | head -n1)
              fi
              VAULT_ID="\$(printf '%s' "\${VAULT_ID}" | tr -d '\r\n')"
              if [ -z "\${VAULT_ID}" ]; then
                echo "Vault not found: \${VAULT_NAME}"
                exit 1
              fi

              ITEM_ID=\$(curl -fsS -H "\${AUTH_HEADER}" "\${API_BASE}/vaults/\${VAULT_ID}/items" | jq -r --arg title "\${OP_ITEM_NAME}" '.[] | select(.title==\$title) | .id' | head -n1)

              if [ -n "\${ITEM_ID}" ]; then
                echo "Item exists, replacing..."
                ITEM_PAYLOAD=\$(jq -n --arg id "\${ITEM_ID}" --arg title "\${OP_ITEM_NAME}" --arg vault "\${VAULT_ID}" --arg notes "\${KUBECONFIG_CONTENT}" '{id:\$id,title:\$title,vault:{id:\$vault},category:"SECURE_NOTE",fields:[{label:"notesPlain",type:"STRING",purpose:"NOTES",value:\$notes}]}')
                curl -fsS -X PUT -H "\${AUTH_HEADER}" -H "Content-Type: application/json" "\${API_BASE}/vaults/\${VAULT_ID}/items/\${ITEM_ID}" -d "\${ITEM_PAYLOAD}" >/dev/null
              else
                echo "Item not found, creating..."
                ITEM_PAYLOAD=\$(jq -n --arg title "\${OP_ITEM_NAME}" --arg vault "\${VAULT_ID}" --arg notes "\${KUBECONFIG_CONTENT}" '{title:\$title,vault:{id:\$vault},category:"SECURE_NOTE",fields:[{label:"notesPlain",type:"STRING",purpose:"NOTES",value:\$notes}]}')
                curl -fsS -X POST -H "\${AUTH_HEADER}" -H "Content-Type: application/json" "\${API_BASE}/vaults/\${VAULT_ID}/items" -d "\${ITEM_PAYLOAD}" >/dev/null
              fi

              echo "Kubeconfig synced successfully to 1Password"
EOF

# Create ExternalSecret to reference the kubeconfig from 1Password
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
        property: notesPlain
EOF

echo "Resources generated successfully for vcluster: ${NAME}"
