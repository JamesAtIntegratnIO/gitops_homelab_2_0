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

# Get namespaces from ResourceRequest
REQUEST_NAMESPACE=$(yq eval '.metadata.namespace' /kratix/input/object.yaml)
NAMESPACE=$(yq eval '.spec.targetNamespace // ""' /kratix/input/object.yaml)

if [ -z "${NAMESPACE}" ] || [ "${NAMESPACE}" = "null" ]; then
  NAMESPACE="${REQUEST_NAMESPACE}"
fi

# Generate 1Password item name for kubeconfig
ONEPASSWORD_ITEM="vcluster-${NAME}-kubeconfig"

echo "Generating vcluster resources for: ${NAME}"
echo "Request namespace: ${REQUEST_NAMESPACE}"
echo "Target namespace: ${NAMESPACE}"

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
  finalizers:
    - resources-finalizer.argocd.argoproj.io
spec:
  project: default
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
        key: onepassword-connect
        property: token
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
    resourceNames: ["vc-vcluster-${NAME}"]
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
      initContainers:
        # Wait for vcluster kubeconfig secret to be created
        - name: wait-for-kubeconfig
          image: bitnami/kubectl:latest
          command:
            - /bin/bash
            - -c
            - |
              echo "Waiting for vcluster kubeconfig secret vc-${NAME}..."
              until kubectl get secret vc-vcluster-${NAME} -n ${NAMESPACE} 2>/dev/null; do
                echo "Secret not found, waiting..."
                sleep 10
              done
              echo "Secret found!"
        # Wait for 1Password token secret to be synced
        - name: wait-for-token
          image: bitnami/kubectl:latest
          command:
            - /bin/bash
            - -c
            - |
              echo "Waiting for 1Password Connect token secret..."
              until kubectl get secret vcluster-${NAME}-onepassword-token -n ${NAMESPACE} 2>/dev/null; do
                echo "Token secret not found, waiting..."
                sleep 10
              done
              echo "Token secret ready!"
      containers:
        - name: sync-to-onepassword
          image: 1password/op:2
          env:
            - name: OP_CONNECT_HOST
              value: "https://connect.integratn.tech"
            - name: OP_CONNECT_TOKEN
              valueFrom:
                secretKeyRef:
                  name: vcluster-${NAME}-onepassword-token
                  key: token
            - name: NAMESPACE
              value: "${NAMESPACE}"
            - name: VCLUSTER_NAME
              value: "${NAME}"
            - name: OP_ITEM_NAME
              value: "${ONEPASSWORD_ITEM}"
          command:
            - /bin/sh
            - -c
            - |
              set -e
              
              # Get kubeconfig from vcluster secret
              KUBECONFIG_B64=\$(kubectl get secret vc-vcluster-\${VCLUSTER_NAME} -n \${NAMESPACE} -o jsonpath='{.data.config}')
              KUBECONFIG_CONTENT=\$(echo "\$KUBECONFIG_B64" | base64 -d)
              
              # Create or update 1Password item
              echo "Syncing kubeconfig to 1Password item: \${OP_ITEM_NAME}"
              
              # Check if item exists
              if op item get "\${OP_ITEM_NAME}" --vault "homelab" 2>/dev/null; then
                echo "Item exists, updating..."
                op item edit "\${OP_ITEM_NAME}" --vault "homelab" kubeconfig="\${KUBECONFIG_CONTENT}"
              else
                echo "Item does not exist, creating..."
                op item create --category=SecureNote --title="\${OP_ITEM_NAME}" --vault="homelab" kubeconfig="\${KUBECONFIG_CONTENT}"
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
        property: kubeconfig
EOF

echo "Resources generated successfully for vcluster: ${NAME}"
