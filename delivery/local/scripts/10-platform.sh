#!/usr/bin/env bash
# Installs what the delivery kit needs underneath it: cert-manager, Kargo,
# Argo Rollouts, and a trimmed monitoring stack.
#
# These go in with helm rather than through ArgoCD on purpose. They are the
# platform the thing under test runs on -- the equivalent of what idpbuilder
# itself installed -- and putting them behind a reconcile loop only adds a
# minute to every run without making the demo more honest. The sample repo and
# the kit's own charts DO go through ArgoCD, because that loop is the thing
# being demonstrated.
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

say "helm repositories"
helm repo add jetstack https://charts.jetstack.io >/dev/null 2>&1 || true
helm repo add argo https://argoproj.github.io/argo-helm >/dev/null 2>&1 || true
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts >/dev/null 2>&1 || true
helm repo update >/dev/null
ok "repositories updated"

say "cert-manager (Kargo's webhooks need it)"
helm upgrade --install cert-manager jetstack/cert-manager \
  --kube-context "$CLUSTER_CONTEXT" \
  --namespace cert-manager --create-namespace \
  --set crds.enabled=true \
  --set resources.requests.cpu=10m \
  --wait --timeout 6m >/dev/null
ok "cert-manager ready"

say "Argo Rollouts (Kargo creates AnalysisRuns; it does not run them)"
# Learned in production: Kargo only *creates* AnalysisRuns. Without a Rollouts
# controller they sit Pending forever and every Stage reports unverified with
# no error anywhere.
helm upgrade --install argo-rollouts argo/argo-rollouts \
  --kube-context "$CLUSTER_CONTEXT" \
  --namespace argo-rollouts --create-namespace \
  --set controller.resources.requests.cpu=10m \
  --wait --timeout 6m >/dev/null
ok "argo-rollouts ready"

say "monitoring (Prometheus + kube-state-metrics + Grafana)"
# Trimmed hard for kind: no Alertmanager, no node exporter, short retention.
# kube-state-metrics stays because the kit's whole metric surface is a
# CustomResourceState config it reads.
helm upgrade --install monitoring prometheus-community/kube-prometheus-stack \
  --kube-context "$CLUSTER_CONTEXT" \
  --namespace monitoring --create-namespace \
  --set alertmanager.enabled=false \
  --set nodeExporter.enabled=false \
  --set prometheus.prometheusSpec.retention=2h \
  --set prometheus.prometheusSpec.resources.requests.cpu=50m \
  --set prometheus.prometheusSpec.ruleSelectorNilUsesHelmValues=false \
  --set prometheus.prometheusSpec.serviceMonitorSelectorNilUsesHelmValues=false \
  --set grafana.sidecar.dashboards.searchNamespace=ALL \
  --wait --timeout 12m >/dev/null
ok "monitoring ready"

say "scraping ArgoCD"
# kargo-pipelines' verification asks Prometheus
#   sum(max by (name) (argocd_app_info{health_status="Healthy", sync_status="Synced"}))
# so if nothing scrapes ArgoCD the query returns no rows, every AnalysisRun
# fails, and no error anywhere names the reason. idpbuilder ships ArgoCD's
# metrics Services but no ServiceMonitor.
cat <<'YAML' | kc apply -f - >/dev/null
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: argocd-metrics
  namespace: argocd
  labels:
    release: monitoring
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: argocd-metrics
  endpoints:
    - port: metrics
      interval: 15s
YAML
ok "argocd ServiceMonitor"

say "Kargo"
helm upgrade --install kargo oci://ghcr.io/akuity/kargo-charts/kargo \
  --kube-context "$CLUSTER_CONTEXT" \
  --namespace kargo --create-namespace \
  --set api.service.type=ClusterIP \
  --set api.tls.enabled=false \
  --set api.adminAccount.passwordHash='$2a$10$Zrhhie4vLz5ygtVSaif6o.qN36jgs6vjtMBdM6yrU1FOeiAAMMxOm' \
  --set api.adminAccount.tokenSigningKey=local-proving-ground-not-a-secret \
  --set controller.resources.requests.cpu=25m \
  --wait --timeout 10m >/dev/null
ok "kargo ready"

say "done"
kc get pods -A --no-headers | awk '{print $1}' | sort | uniq -c | sort -rn | sed 's/^/  /'
