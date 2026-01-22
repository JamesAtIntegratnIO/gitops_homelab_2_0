#!/usr/bin/env bash

write_coredns_configmap() {
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
EOF
}

render_all_resources() {
  write_coredns_configmap
}
