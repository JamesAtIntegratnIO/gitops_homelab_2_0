#!/usr/bin/env bash

build_blocks() {
  ANNOTATIONS_FILE="/tmp/argocd-app-annotations.yaml"
  LABELS_FILE="/tmp/argocd-app-labels.yaml"
  FINALIZERS_FILE="/tmp/argocd-app-finalizers.yaml"
  VALUES_OBJECT_FILE="/tmp/argocd-app-values-object.yaml"
  SYNC_POLICY_FILE="/tmp/argocd-app-sync-policy.yaml"

  mkdir -p /tmp

  yq eval '.spec.annotations // {}' "${INPUT_FILE}" > "${ANNOTATIONS_FILE}"
  yq eval '.spec.labels // {}' "${INPUT_FILE}" > "${LABELS_FILE}"
  yq eval '.spec.finalizers // []' "${INPUT_FILE}" > "${FINALIZERS_FILE}"
  yq eval '.spec.source.helm.valuesObject // {}' "${INPUT_FILE}" > "${VALUES_OBJECT_FILE}"
  yq eval '.spec.syncPolicy // {}' "${INPUT_FILE}" > "${SYNC_POLICY_FILE}"

  ANNOTATIONS_LENGTH=$(yq eval 'length' "${ANNOTATIONS_FILE}")
  LABELS_LENGTH=$(yq eval 'length' "${LABELS_FILE}")
  FINALIZERS_LENGTH=$(yq eval 'length' "${FINALIZERS_FILE}")
  VALUES_OBJECT_LENGTH=$(yq eval 'length' "${VALUES_OBJECT_FILE}")
  SYNC_POLICY_LENGTH=$(yq eval 'length' "${SYNC_POLICY_FILE}")

  if [ "${ANNOTATIONS_LENGTH}" -eq 0 ]; then
    ANNOTATIONS="    {}"
  else
    ANNOTATIONS=$(sed 's/^/    /' "${ANNOTATIONS_FILE}")
  fi

  if [ "${LABELS_LENGTH}" -eq 0 ]; then
    LABELS="    {}"
  else
    LABELS=$(sed 's/^/    /' "${LABELS_FILE}")
  fi

  if [ "${FINALIZERS_LENGTH}" -eq 0 ]; then
    FINALIZERS_BLOCK=""
  else
    FINALIZERS_BLOCK=$(printf "  finalizers:\n%s\n" "$(sed 's/^/  - /' "${FINALIZERS_FILE}")")
  fi

  if [ "${VALUES_OBJECT_LENGTH}" -eq 0 ]; then
    VALUES_OBJECT="        {}"
  else
    VALUES_OBJECT=$(sed 's/^/        /' "${VALUES_OBJECT_FILE}")
  fi

  if [ "${SYNC_POLICY_LENGTH}" -eq 0 ]; then
    SYNC_POLICY="  {}"
  else
    SYNC_POLICY=$(sed 's/^/  /' "${SYNC_POLICY_FILE}")
  fi
}

write_application() {
  cat > /kratix/output/argocd-application.yaml <<EOF
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: ${APP_NAME}
  namespace: ${NAMESPACE}
  annotations:
${ANNOTATIONS}
  labels:
${LABELS}
${FINALIZERS_BLOCK}spec:
  project: ${PROJECT}
  source:
    repoURL: ${REPO_URL}
    chart: ${CHART}
    targetRevision: ${TARGET_REVISION}
    helm:
      releaseName: ${RELEASE_NAME}
      valuesObject:
${VALUES_OBJECT}
  destination:
    server: ${DEST_SERVER}
    namespace: ${DEST_NAMESPACE}
  syncPolicy:
${SYNC_POLICY}
EOF
}

render_all_resources() {
  build_blocks
  write_application
}
