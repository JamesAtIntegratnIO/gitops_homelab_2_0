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
  if [ -n "${PERSISTENCE_STORAGE_CLASS}" ] && [ "${PERSISTENCE_STORAGE_CLASS}" != "null" ]; then
    PERSISTENCE_STORAGE_CLASS_CM_LINE="            storageClass: \"${PERSISTENCE_STORAGE_CLASS}\""
  fi
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

  yq eval '.spec.vcluster.helmOverrides // {}' "${INPUT_FILE}" > "${VALUES_OVERRIDES_FILE}"
  yq eval-all 'select(fileIndex==0) * select(fileIndex==1)' "${VALUES_BASE_FILE}" "${VALUES_OVERRIDES_FILE}" > "${VALUES_MERGED_FILE}"

  VALUES_CONFIGMAP=$(sed 's/^/    /' "${VALUES_MERGED_FILE}")
  VALUES_OBJECT=$(sed 's/^/        /' "${VALUES_MERGED_FILE}")
}

build_sync_policy() {
  SYNC_POLICY_DEFAULT="/tmp/argocd-sync-policy-default.yaml"
  SYNC_POLICY_OVERRIDE="/tmp/argocd-sync-policy-override.yaml"
  SYNC_POLICY_MERGED="/tmp/argocd-sync-policy.yaml"

  cat > "${SYNC_POLICY_DEFAULT}" <<EOF
automated:
  selfHeal: true
  prune: true
syncOptions:
  - CreateNamespace=true
EOF

  yq eval '.spec.argocdApplication.syncPolicy // {}' "${INPUT_FILE}" > "${SYNC_POLICY_OVERRIDE}"
  yq eval-all 'select(fileIndex==0) * select(fileIndex==1)' "${SYNC_POLICY_DEFAULT}" "${SYNC_POLICY_OVERRIDE}" > "${SYNC_POLICY_MERGED}"

  SYNC_POLICY_OBJECT=$(sed 's/^/    /' "${SYNC_POLICY_MERGED}")
}

write_vcluster_core_request() {
  cat > /kratix/output/vcluster-core-request.yaml <<EOF
apiVersion: platform.integratn.tech/v1alpha1
kind: VClusterCore
metadata:
  name: ${NAME}
  namespace: ${REQUEST_NAMESPACE}
spec:
  name: ${NAME}
  targetNamespace: ${TARGET_NAMESPACE}
  valuesYaml: |
${VALUES_CONFIGMAP}
EOF
}

write_vcluster_coredns_request() {
  cat > /kratix/output/vcluster-coredns-request.yaml <<EOF
apiVersion: platform.integratn.tech/v1alpha1
kind: VClusterCoredns
metadata:
  name: ${NAME}
  namespace: ${REQUEST_NAMESPACE}
spec:
  name: ${NAME}
  targetNamespace: ${TARGET_NAMESPACE}
  clusterDomain: ${CLUSTER_DOMAIN}
EOF
}

write_argocd_project_request() {
  cat > /kratix/output/argocd-project-request.yaml <<EOF
apiVersion: platform.integratn.tech/v1alpha1
kind: ArgoCDProject
metadata:
  name: ${PROJECT_NAME}
  namespace: ${REQUEST_NAMESPACE}
spec:
  name: ${PROJECT_NAME}
  namespace: argocd
  description: VCluster project for ${NAME}
  annotations:
    argocd.argoproj.io/sync-wave: "-1"
  labels:
    app.kubernetes.io/managed-by: kratix
    kratix.io/promise-name: vcluster-orchestrator
    kratix.io/resource-request: ${NAME}
    argocd.argoproj.io/project-group: appteam
  sourceRepos:
    - https://charts.loft.sh
  destinations:
    - server: https://kubernetes.default.svc
      namespace: ${TARGET_NAMESPACE}
  clusterResourceWhitelist:
    - group: '*'
      kind: '*'
  namespaceResourceWhitelist:
    - group: '*'
      kind: '*'
EOF
}

write_argocd_application_request() {
  cat > /kratix/output/argocd-application-request.yaml <<EOF
apiVersion: platform.integratn.tech/v1alpha1
kind: ArgoCDApplication
metadata:
  name: vcluster-${NAME}
  namespace: ${REQUEST_NAMESPACE}
spec:
  name: vcluster-${NAME}
  namespace: argocd
  annotations:
    argocd.argoproj.io/sync-wave: "0"
  finalizers:
    - resources-finalizer.argocd.argoproj.io
  project: ${PROJECT_NAME}
  source:
    repoURL: ${ARGOCD_REPO_URL}
    chart: ${ARGOCD_CHART}
    targetRevision: ${ARGOCD_TARGET_REVISION}
    helm:
      releaseName: ${NAME}
      valuesObject:
${VALUES_OBJECT}
  destination:
    server: ${ARGOCD_DEST_SERVER}
    namespace: ${TARGET_NAMESPACE}
  syncPolicy:
${SYNC_POLICY_OBJECT}
EOF
}

write_kubeconfig_sync_request() {
  ONEPASSWORD_ITEM="vcluster-${NAME}-kubeconfig"

  cat > /kratix/output/vcluster-kubeconfig-sync-request.yaml <<EOF
apiVersion: platform.integratn.tech/v1alpha1
kind: VClusterKubeconfigSync
metadata:
  name: ${NAME}
  namespace: ${REQUEST_NAMESPACE}
spec:
  name: ${NAME}
  targetNamespace: ${TARGET_NAMESPACE}
  kubeconfigSyncJobName: ${KUBECONFIG_SYNC_JOB_NAME}
  onepasswordItem: ${ONEPASSWORD_ITEM}
  hostname: ${HOSTNAME}
  apiPort: ${API_PORT}
  serverUrl: ${EXTERNAL_SERVER_URL}
EOF
}

write_kubeconfig_external_secret_request() {
  ONEPASSWORD_ITEM="vcluster-${NAME}-kubeconfig"

  cat > /kratix/output/vcluster-kubeconfig-external-secret-request.yaml <<EOF
apiVersion: platform.integratn.tech/v1alpha1
kind: VClusterKubeconfigExternalSecret
metadata:
  name: ${NAME}
  namespace: ${REQUEST_NAMESPACE}
spec:
  name: ${NAME}
  targetNamespace: ${TARGET_NAMESPACE}
  onepasswordItem: ${ONEPASSWORD_ITEM}
EOF
}

write_argocd_cluster_request() {
  ONEPASSWORD_ITEM="vcluster-${NAME}-kubeconfig"

  cat > /kratix/output/vcluster-argocd-cluster-request.yaml <<EOF
apiVersion: platform.integratn.tech/v1alpha1
kind: VClusterArgoCDClusterRegistration
metadata:
  name: ${NAME}
  namespace: ${REQUEST_NAMESPACE}
spec:
  name: ${NAME}
  argocdNamespace: argocd
  onepasswordItem: ${ONEPASSWORD_ITEM}
  labels:
${ARGOCD_CLUSTER_LABELS_INDENTED}
  annotations:
${ARGOCD_CLUSTER_ANNOTATIONS_INDENTED}
EOF
}

render_all_resources() {
  build_service_values
  build_proxy_extra_sans
  build_persistence_storage_class_lines
  build_values_files
  build_sync_policy

  write_vcluster_core_request
  write_vcluster_coredns_request
  write_argocd_project_request
  write_argocd_application_request
  write_kubeconfig_sync_request
  write_kubeconfig_external_secret_request
  write_argocd_cluster_request

  echo "Orchestrator outputs rendered for vcluster: ${NAME}"
}
