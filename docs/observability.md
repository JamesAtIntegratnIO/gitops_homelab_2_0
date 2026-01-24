# Observability

## Architecture
The host cluster runs the full observability stack. Each vcluster runs a Prometheus **agent** that scrapes in‑cluster metrics and remote‑writes to the host Prometheus.

**Host cluster**
- `kube-prometheus-stack` (Prometheus + Alertmanager + Grafana)
- `loki` (single‑binary) + `promtail`
- Storage: `config-nfs-client`

**vcluster**
- `kube-prometheus-stack` **agent only** (no Grafana/Alertmanager)
- `kube-state-metrics` enabled
- No `node_exporter` inside vcluster

## Data Flow
1. vcluster agent scrapes ServiceMonitors/PodMonitors and kubelet metrics.
2. Remote write pushes to host Prometheus receiver.
3. Grafana reads from host Prometheus + Loki.

## Endpoints
- Grafana: `https://grafana.cluster.integratn.tech`
- Prometheus remote write: `https://prom-remote.cluster.integratn.tech/api/v1/write`
- Loki push: `https://loki.cluster.integratn.tech/loki/api/v1/push`

## Grafana
- Credentials are stored in ExternalSecret `grafana-admin`.
- Default kube‑prometheus dashboards are enabled.
- Imported dashboards: Kubernetes cluster (gnet 315) and Node Exporter (gnet 1860).

## Host vs vcluster Metrics
- **Host node metrics**: `node_exporter` in host cluster.
- **vcluster workloads**: agent + kube‑state‑metrics + kubelet scrape.
- **Control plane metrics**: kube‑prometheus defaults (apiserver, scheduler, etc.).

## Operational Checks
Use Grafana to validate:
- vcluster metrics are labeled with `cluster=<vcluster-name>`.
- Node dashboards show host nodes (not vcluster).
- Kubernetes dashboards show vcluster namespaces and workloads.

## Key Files
- Host stack values: `addons/cluster-roles/control-plane/addons/kube-prometheus-stack/values.yaml`
- vcluster agent values: `addons/cluster-roles/vcluster/addons/kube-prometheus-stack-agent/values.yaml`
- Loki values: `addons/cluster-roles/control-plane/addons/loki/values.yaml`
- HTTPRoutes: `addons/cluster-roles/control-plane/addons/observability-secrets/observability-httproutes.yaml`
