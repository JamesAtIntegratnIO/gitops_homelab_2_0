---
type: Observability Stack
title: Observability — metrics, logs, dashboards, alerting
description: Hub-and-spoke Prometheus with vcluster agents, Loki + Promtail logging, Grafana dashboards, and Matrix alert delivery with Loki deep links.
tags: [observability, prometheus, grafana, loki, alertmanager, matrix]
status: stable
generated: { by: claude-code/claude-opus-5, at: 2026-08-22T13:40:53Z }
stale_after: 2027-02-20
sources:
  - id: obs-doc
    resource: ../../observability.md
    title: docs/observability.md
  - id: ops-doc
    resource: ../../operations.md
    title: docs/operations.md (Matrix alerting, Loki correlation)
  - id: live-obs
    resource: kubectl get prometheusrules,servicemonitors,cm -l grafana_dashboard on the-cluster, 2026-08-20
    title: Live monitoring resources
---

# Hub and spoke

- **Hub (the-cluster, `monitoring` ns)**: kube-prometheus-stack (chart 88.5.3;
  Prometheus v3.9.1, 15d retention, 30Gi; `enableRemoteWriteReceiver: true`),
  Alertmanager v0.31.1, Grafana (admin creds via ExternalSecret, OIDC via
  Authentik), kube-state-metrics, node-exporter DaemonSet.
- **Spokes (vclusters)**: kube-prometheus-stack-agent — Prometheus in agent
  mode, external labels `cluster=<name>`, remote-writing to
  `https://prom-remote.cluster.integratn.tech/api/v1/write` through the main
  gateway. Grafana/Alertmanager/node-exporter disabled in spokes.

# Logs

Loki 3.7.6 (`loki` ns; 50Gi NFS; gateway + memcached caches + canary) fed by
**host-level Promtail 3.5.1** tailing `/var/log/pods` — this covers
vcluster-synced pods too, since they run on the host nodes. A per-vcluster
promtail addon exists but is disabled by design. External push endpoint:
`loki.cluster.integratn.tech`.

Monolithic deployment mode, filesystem storage, boltdb-shipper on schema v12.
The chart is **`grafana-community/loki` 18.11.0**, not `grafana/loki` — upstream
made its chart Grafana Enterprise Logs only at 7.0.0 and pointed OSS users at
the community fork, which continues the same lineage from 6.55.0. Both the
addon and Kargo's `loki` target moved to
`https://grafana-community.github.io/helm-charts` on 2026-08-22; see
[known issues](../cluster/known-issues.md) for what the migration changed in the
values file.

**Grafana is already on the community chart** and needed no change: since
kube-prometheus-stack 88.5.3 the `grafana` dependency resolves to
`grafana-community/grafana` 12.11.1, which is why the cluster runs Grafana
13.2.0 while the deprecated `grafana/grafana` chart tops out at app 12.3.1.
**Promtail cannot move** — see [known issues](../cluster/known-issues.md).

# Dashboards & rules (live counts, 2026-08-20)

40 PrometheusRules, 34 ServiceMonitors, ~30 Grafana dashboard ConfigMaps —
including custom ones for the Kratix platform (controller-runtime/workqueue
metrics — Kratix exposes no `kratix_*` metrics of its own), ArgoCD overview,
vcluster fleet, Trivy, Authentik, NFS performance, Loki logs, and a
"platform landing zone" dashboard.[^live-obs]

The custom **platform-status-reconciler** exports `platform_vcluster_*`
metrics (phase/ready/pods) with alerts like `VClusterNotReady` — see
[Kratix](/promises/kratix.md).

# Alerting path

```
Prometheus rules → Alertmanager → matrix-alertmanager-receiver (monitoring ns)
                → Matrix room on matrix.integratn.tech
```

All 25 custom alert rules carry a `logs_url` annotation deep-linking to
Grafana Explore with a pre-filtered Loki query for the alert's namespace —
alert messages arrive in Matrix with a clickable "Logs" link.[^ops-doc]
Single delivery channel today; redundancy is an open roadmap item.

# Endpoints

grafana / prometheus / alertmanager / loki / prom-remote, all under
`*.cluster.integratn.tech` via the [gateway](/platform/networking.md).
Grafana is auth'd (1Password + OIDC); Prometheus/Alertmanager/Loki are not.

[^obs-doc]: docs/observability.md
[^ops-doc]: docs/operations.md (Matrix alerting, Loki correlation)
[^live-obs]: Live monitoring resources
