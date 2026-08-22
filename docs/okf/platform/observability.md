---
type: Observability Stack
title: Observability — metrics, logs, dashboards, alerting
description: Hub-and-spoke Prometheus with vcluster agents, Loki + Promtail logging, Grafana dashboards, and Matrix alert delivery with Loki deep links.
tags: [observability, prometheus, grafana, loki, alertmanager, matrix]
status: stable
generated: { by: claude-code/claude-fable-5, at: 2026-08-20T23:40:00Z }
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

Loki 3.6.3 (`loki` ns; 50Gi NFS; gateway + memcached caches + canary) fed by
**host-level Promtail 2.7.3** tailing `/var/log/pods` — this covers
vcluster-synced pods too, since they run on the host nodes. A per-vcluster
promtail addon exists but is disabled by design. External push endpoint:
`loki.cluster.integratn.tech`.

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
