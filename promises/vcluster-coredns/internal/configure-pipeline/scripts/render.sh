#!/usr/bin/env bash
set -euo pipefail

INPUT_FILE="/kratix/input/object.yaml"

NAME=$(yq eval '.spec.name' "${INPUT_FILE}")
TARGET_NAMESPACE=$(yq eval '.spec.targetNamespace' "${INPUT_FILE}")
CLUSTER_DOMAIN=$(yq eval '.spec.clusterDomain' "${INPUT_FILE}")

cat > /kratix/output/coredns-configmap.yaml <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: coredns-x-kube-system-x-${NAME}
  namespace: ${TARGET_NAMESPACE}
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
