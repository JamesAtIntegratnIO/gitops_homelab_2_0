#!/usr/bin/env python3
"""Generate the "Homelab Overview" Grafana dashboard ConfigMap.

The dashboard is the one-screen answer to "is anything in the homelab wrong,
and where?" It is built here rather than hand-edited as JSON because it has
~60 panels and every query has to agree on the same label filters.

    python3 hack/dashboards/homelab_overview.py

writes addons/cluster-roles/control-plane/addons/platform-landing-zone/homelab-overview.yaml,
which the platform-landing-zone addon already syncs into `monitoring`, where the
Grafana sidecar picks it up by its grafana_dashboard label. Edit this file, then
regenerate; do not edit the YAML by hand.
"""

import json
import pathlib

OUT = (
    pathlib.Path(__file__).resolve().parents[2]
    / "addons/cluster-roles/control-plane/addons/platform-landing-zone/homelab-overview.yaml"
)

PROM = {"type": "prometheus", "uid": "${datasource}"}
LOKI = {"type": "loki", "uid": "${lokids}"}

# vcluster-media's Prometheus agent is designed to remote-write into the hub with
# an external label cluster="vcluster-media"; host series carry no cluster
# label. As of 2026-10-11 no series carries `cluster` at all (remote-write
# ingestion measured zero; vcluster-media is scraped directly as
# job="vcluster-media"), so H matches everything today. It stays on the
# kube-state-metrics queries so that if remote-write resumes, the vcluster's
# synthetic nodes and its view of the same pods are not counted twice.
H = 'cluster=""'

# node-exporter labels nodes by IP:9100; join this to show Talos node names.
NODE = " * on (instance) group_left (nodename) node_uname_info"

GREEN, YELLOW, ORANGE, RED, BLUE = "green", "#EAB839", "orange", "red", "blue"

_ids = iter(range(1, 1000))
_y = 0


def steps(*pairs):
    """steps(GREEN, (1, RED)) -> Grafana threshold steps."""
    base, *rest = pairs
    return {"mode": "absolute", "steps": [{"color": base, "value": None}] + [{"color": c, "value": v} for v, c in rest]}


def target(expr, legend="", instant=False, fmt=None, ref="A", ds=PROM):
    t = {"datasource": ds, "expr": expr, "legendFormat": legend, "refId": ref}
    if instant:
        t["instant"] = True
        t["range"] = False
    if fmt:
        t["format"] = fmt
    return t


def row(title):
    global _y
    p = {"type": "row", "title": title, "collapsed": False, "id": next(_ids), "panels": [],
         "gridPos": {"h": 1, "w": 24, "x": 0, "y": _y}}
    _y += 1
    return p


def place(panels, h):
    """Lay panels out left to right on one band of height h (widths preset)."""
    global _y
    x = 0
    for p in panels:
        p["gridPos"] = {"h": h, "w": p.pop("_w"), "x": x, "y": _y}
        x += p["gridPos"]["w"]
    assert x <= 24, [p["title"] for p in panels]
    _y += h
    return panels


def stat(title, expr, desc, th, unit="none", w=4, decimals=None, link=None, mappings=None):
    p = {
        "type": "stat", "id": next(_ids), "title": title, "description": desc, "datasource": PROM, "_w": w,
        "targets": [target(expr, instant=True)],
        "fieldConfig": {"defaults": {"unit": unit, "color": {"mode": "thresholds"}, "thresholds": th,
                                     "mappings": mappings or [], "noValue": "0"}, "overrides": []},
        "options": {"colorMode": "background", "graphMode": "none", "justifyMode": "center", "textMode": "value",
                    "orientation": "auto", "wideLayout": True, "showPercentChange": False,
                    "reduceOptions": {"calcs": ["lastNotNull"], "fields": "", "values": False}},
    }
    if decimals is not None:
        p["fieldConfig"]["defaults"]["decimals"] = decimals
    if link:
        p["fieldConfig"]["defaults"]["links"] = [{"title": link[0], "url": link[1], "targetBlank": link[1].startswith("http")}]
    return p


def ts(title, targets, desc, unit="none", w=8, stack=False, th=None, maxv=None, minv=None, legend_calcs=("lastNotNull", "max")):
    d = {"unit": unit, "color": {"mode": "palette-classic"},
         "custom": {"drawStyle": "line", "lineWidth": 1, "fillOpacity": 18 if stack else 6, "showPoints": "never",
                    "spanNulls": True, "stacking": {"mode": "normal" if stack else "none", "group": "A"},
                    "axisSoftMin": 0, "gradientMode": "opacity"}}
    if th:
        d["thresholds"] = th
        d["custom"]["thresholdsStyle"] = {"mode": "dashed"}
    if maxv is not None:
        d["max"] = maxv
    if minv is not None:
        d["min"] = minv
    return {
        "type": "timeseries", "id": next(_ids), "title": title, "description": desc, "datasource": targets[0]["datasource"],
        "_w": w, "targets": targets, "fieldConfig": {"defaults": d, "overrides": []},
        "options": {"legend": {"displayMode": "table", "placement": "bottom", "calcs": list(legend_calcs), "sortBy": "Last *", "sortDesc": True},
                    "tooltip": {"mode": "multi", "sort": "desc"}},
    }


def table(title, expr, desc, columns, w=12, sort=None, overrides=None, unit="none", th=None, value_name="Value"):
    """Instant-query table. columns: label names to keep, in order; the value is renamed value_name."""
    order = {c: i for i, c in enumerate(columns + ["Value"])}
    exclude = {"Time": True, "__name__": True, "endpoint": True, "service": True, "job": True, "prometheus": True,
               "alertstate": True, "container": True, "instance": True, "pod": True, "namespace": True,
               "uid": True, "cluster": True}
    for c in columns:
        exclude.pop(c, None)
    p = {
        "type": "table", "id": next(_ids), "title": title, "description": desc, "datasource": PROM, "_w": w,
        "targets": [target(expr, instant=True, fmt="table")],
        "fieldConfig": {"defaults": {"unit": unit, "custom": {"align": "auto", "cellOptions": {"type": "auto"}, "filterable": True},
                                     "thresholds": th or steps(GREEN), "noValue": "Nothing to show — all clear"},
                        "overrides": overrides or []},
        "options": {"showHeader": True, "cellHeight": "sm", "footer": {"show": False},
                    "sortBy": [{"displayName": sort[0], "desc": sort[1]}] if sort else []},
        "transformations": [
            {"id": "labelsToFields", "options": {"mode": "columns"}},
            {"id": "merge", "options": {}},
            {"id": "organize", "options": {"excludeByName": exclude, "indexByName": order,
                                           "renameByName": {"Value": value_name}}},
        ],
    }
    return p


def colored_value(name, th, unit=None, mode="color-background"):
    o = {"matcher": {"id": "byName", "options": name},
         "properties": [{"id": "custom.cellOptions", "value": {"type": mode}}, {"id": "thresholds", "value": th},
                        {"id": "color", "value": {"mode": "thresholds"}}]}
    if unit:
        o["properties"].append({"id": "unit", "value": unit})
    return o


def text(content, w=24):
    return {"type": "text", "id": next(_ids), "title": "", "_w": w, "transparent": True,
            "options": {"mode": "markdown", "content": content}}


panels = []

# ── Header ─────────────────────────────────────────────────────────────────
panels += place([text(
    "**Homelab Overview** · the-cluster (3× Talos, K8s 1.34) + vcluster-media · "
    "[ArgoCD](https://argocd.cluster.integratn.tech) · [Kargo](https://kargo.cluster.integratn.tech) · "
    "[Alertmanager](https://alertmanager.cluster.integratn.tech) · [Prometheus](https://prometheus.cluster.integratn.tech) · "
    "[Hubble](https://hubble.cluster.integratn.tech) · [Trivy](https://trivy.cluster.integratn.tech) · "
    "[Goldilocks](https://goldilocks.cluster.integratn.tech) · [Bosun](https://bosun.cluster.integratn.tech) — "
    "every tile is green when nothing needs you; red or amber tiles link to where to look."
)], 2)

# ── Is anything wrong right now? ───────────────────────────────────────────
panels.append(row("Is anything wrong right now?"))
panels += place([
    stat("Nodes Ready", f'sum(kube_node_status_condition{{{H}, condition="Ready", status="true"}}) or vector(0)',
         "Talos nodes reporting Ready. Three is everything; all three are control-plane and schedulable, so losing one also costs an etcd member.",
         steps(RED, (2, YELLOW), (3, GREEN))),
    stat("Critical alerts", 'count(ALERTS{alertstate="firing", severity="critical"}) or vector(0)',
         "Firing critical alerts. These page Matrix immediately and repeat hourly.",
         steps(GREEN, (1, RED)), link=("Alertmanager", "https://alertmanager.cluster.integratn.tech")),
    stat("Warning alert types", 'count(count by (alertname) (ALERTS{alertstate="firing", severity="warning"})) or vector(0)',
         "Distinct warning alerts firing (one Kargo problem can fire dozens of instances, so this counts alert names, not instances). These batch into the Matrix digest. See the table below for counts.",
         steps(GREEN, (1, YELLOW))),
    stat("Scrape targets down", "count(up == 0) or vector(0)",
         "Prometheus targets that failed their last scrape. A timeout here (not a refusal) is the house signature of a NetworkPolicy drop.",
         steps(GREEN, (1, ORANGE)), link=("Targets", "https://prometheus.cluster.integratn.tech/targets?health=down")),
    stat("Pods in trouble",
         f'(sum(kube_pod_status_phase{{{H}, phase=~"Pending|Failed|Unknown"}}) or vector(0)) + '
         f'(count(kube_pod_container_status_waiting_reason{{{H}, reason=~"CrashLoopBackOff|ImagePullBackOff|ErrImagePull|CreateContainerConfigError|CreateContainerError"}}) or vector(0))',
         "Pods Pending/Failed/Unknown plus containers stuck in CrashLoopBackOff, image-pull or config errors. Failed pods left behind by Jobs count too — they are worth cleaning up.",
         steps(GREEN, (1, YELLOW), (5, RED))),
    stat("Restarts (1h)", f'sum(increase(kube_pod_container_status_restarts_total{{{H}}}[1h])) or vector(0)',
         "Container restarts across the cluster in the last hour. A handful after a node reboot is normal; a steady trickle is a crash loop or lost leader elections (etcd I/O starvation).",
         steps(GREEN, (3, YELLOW), (10, RED)), decimals=0),
], 4)
panels += place([
    stat("Apps not Healthy", 'count(argocd_app_info{health_status!="Healthy"}) or vector(0)',
         "ArgoCD Applications whose health is anything but Healthy (Degraded, Progressing, Missing, Suspended, Unknown).",
         steps(GREEN, (1, YELLOW), (3, RED)), link=("ArgoCD", "https://argocd.cluster.integratn.tech/applications?health=Degraded&health=Progressing&health=Missing&health=Unknown")),
    stat("Apps OutOfSync", 'count(argocd_app_info{sync_status!="Synced"}) or vector(0)',
         "ArgoCD Applications not Synced with git. ArgoCD reconciles from origin/main, so this means main and the cluster disagree.",
         steps(GREEN, (1, YELLOW)), link=("ArgoCD", "https://argocd.cluster.integratn.tech/applications?sync=OutOfSync")),
    stat("Promotions blocked", 'sum(bosun_pipeline_findings{severity="blocking"}) or vector(0)',
         "Things in the Kargo/Bosun promotion pipeline that have stopped (failed promotions, stale pins, dead warehouses).",
         steps(GREEN, (1, ORANGE)), link=("Kargo", "https://kargo.cluster.integratn.tech")),
    stat("vClusters not Ready", "count(platform_vcluster_ready == 0) or vector(0)",
         "Virtual clusters the platform-status-reconciler reports as not Ready (today: vcluster-media).",
         steps(GREEN, (1, RED)), link=("vCluster Fleet", "/d/vcluster-fleet-health/vcluster-fleet-health")),
    stat("Last etcd backup", f'time() - max(kube_cronjob_status_last_successful_time{{{H}, cronjob=~".*-etcd-snapshot"}})',
         "Age of the newest successful etcd snapshot (host or vcluster) to NFS. Snapshots are nightly; EtcdSnapshotStale fires at 36h.",
         steps(GREEN, (26 * 3600, YELLOW), (36 * 3600, RED)), unit="s", decimals=0),
    stat("Soonest cert expiry", "min(certmanager_certificate_expiration_timestamp_seconds - time())",
         "Time until the next cert-manager certificate expires. Let's Encrypt renews at 30 days left, so anything under 14 days means renewal is failing.",
         steps(RED, (3 * 86400, ORANGE), (14 * 86400, YELLOW), (21 * 86400, GREEN)), unit="s", decimals=0),
], 4)

panels += place([
    table("Firing alerts",
          'sort_desc(count by (alertname, severity) (ALERTS{alertstate="firing", alertname!~"Watchdog|InfoInhibitor"}))',
          "Every alert firing right now, grouped by name with how many instances are firing. Critical goes to Matrix at once, warning to the daily digest, info is dashboard-only. Open Alertmanager for the individual instances.",
          ["alertname", "severity"], w=14, sort=("Firing", True), value_name="Firing",
          overrides=[{"matcher": {"id": "byName", "options": "severity"},
                      "properties": [{"id": "custom.cellOptions", "value": {"type": "color-text"}},
                                     {"id": "mappings", "value": [{"type": "value", "options": {
                                         "critical": {"color": RED, "index": 0}, "warning": {"color": YELLOW, "index": 1},
                                         "info": {"color": BLUE, "index": 2}}}]}]},
                     {"matcher": {"id": "byName", "options": "alertname"}, "properties": [{"id": "links", "value": [
                         {"title": "Open in Alertmanager", "targetBlank": True,
                          "url": "https://alertmanager.cluster.integratn.tech/#/alerts?filter=%7Balertname%3D%22${__value.raw}%22%7D"}]}]}]),
    table("Scrape targets down", "up == 0",
          "Targets Prometheus cannot scrape. Each is a blind spot: alerts built on that job cannot fire.",
          ["job", "namespace", "instance"], w=10,
          overrides=[{"matcher": {"id": "byName", "options": "Value"}, "properties": [{"id": "custom.hidden", "value": True}]}]),
], 8)

# ── Capacity ───────────────────────────────────────────────────────────────
panels.append(row("Capacity and N+1"))
REQ = (f'sum(kube_pod_container_resource_requests{{{H}, resource="%s"}} * on (namespace, pod) group_left () '
       f'(max by (namespace, pod) (kube_pod_status_phase{{{H}, phase=~"Running|Pending"}}) == 1))')
ALLOC = f'sum(kube_node_status_allocatable{{{H}, resource="%s"}})'
panels += place([
    stat("CPU used", f'1 - avg(rate(node_cpu_seconds_total{{mode="idle"}}[5m]))',
         "Actual CPU busy across all three nodes (12 cores).", steps(GREEN, (0.7, YELLOW), (0.85, RED)), unit="percentunit", decimals=0),
    stat("Memory used", "1 - sum(node_memory_MemAvailable_bytes) / sum(node_memory_MemTotal_bytes)",
         "Actual memory in use across all three nodes (96 GiB).", steps(GREEN, (0.75, YELLOW), (0.9, RED)), unit="percentunit", decimals=0),
    stat("CPU requested", f'{REQ % "cpu"} / {ALLOC % "cpu"}',
         "Sum of CPU requests over allocatable. Above 67% the cluster can no longer reschedule everything if one of three nodes dies — the N+1 check from docs/game-day.md, and what KubeCPUOvercommit watches.",
         steps(GREEN, (0.6, YELLOW), (0.67, RED)), unit="percentunit", decimals=0),
    stat("Memory requested", f'{REQ % "memory"} / {ALLOC % "memory"}',
         "Sum of memory requests over allocatable. Same N+1 line as CPU: above 67% a node loss strands pods.",
         steps(GREEN, (0.6, YELLOW), (0.67, RED)), unit="percentunit", decimals=0),
    stat("Pods / capacity", f'sum(kubelet_running_pods) / {ALLOC % "pods"}',
         "Running pods over the kubelet pod limit summed across nodes.", steps(GREEN, (0.7, YELLOW), (0.85, RED)), unit="percentunit", decimals=0),
    stat("Prometheus TSDB", '(sum(prometheus_tsdb_storage_blocks_bytes{job=~".*prometheus"}) + sum(prometheus_tsdb_wal_storage_size_bytes{job=~".*prometheus"})) / (30 * 1024 * 1024 * 1024)',
         "Prometheus blocks + WAL against its 30Gi volume (15d retention). Kubelet volume stats are not collected for the NFS-backed PVCs, so this is measured from Prometheus itself.",
         steps(GREEN, (0.75, YELLOW), (0.9, RED)), unit="percentunit", decimals=0),
], 4)
panels += place([
    ts("CPU: used vs requested vs allocatable", [
        target(f'sum(rate(container_cpu_usage_seconds_total{{{H}, container!=""}}[5m]))', "used", ref="A"),
        target(REQ % "cpu", "requested", ref="B"),
        target(ALLOC % "cpu", "allocatable", ref="C"),
        target(f'{ALLOC % "cpu"} * 2 / 3', "N+1 line (2 of 3 nodes)", ref="D"),
    ], "If 'requested' crosses the N+1 line, a node failure leaves pods Pending.", unit="short", w=12),
    ts("Memory: used vs requested vs allocatable", [
        target(f'sum(container_memory_working_set_bytes{{{H}, container!=""}})', "used", ref="A"),
        target(REQ % "memory", "requested", ref="B"),
        target(ALLOC % "memory", "allocatable", ref="C"),
        target(f'{ALLOC % "memory"} * 2 / 3', "N+1 line (2 of 3 nodes)", ref="D"),
    ], "If 'requested' crosses the N+1 line, a node failure leaves pods Pending.", unit="bytes", w=12),
], 8)

# ── Nodes ──────────────────────────────────────────────────────────────────
panels.append(row("Nodes"))
panels += place([
    ts("CPU busy by node", [target('(1 - avg by (instance) (rate(node_cpu_seconds_total{mode="idle"}[5m])))' + NODE, "{{nodename}}")],
       "Per-node CPU utilisation.", unit="percentunit", w=8, maxv=1, th=steps(GREEN, (0.85, RED))),
    ts("Memory used by node", [target("(1 - node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes)" + NODE, "{{nodename}}")],
       "Per-node memory utilisation (of 32 GiB each).", unit="percentunit", w=8, maxv=1, th=steps(GREEN, (0.9, RED))),
    ts("I/O wait by node", [target('(avg by (instance) (rate(node_cpu_seconds_total{mode="iowait"}[5m])))' + NODE, "{{nodename}}")],
       "CPU time waiting on disk. etcd, the OS and containerd share one disk per node, so iowait spikes line up with etcd fsync stalls and lost leader elections.",
       unit="percentunit", w=8),
], 7)
panels += place([
    ts("Disk busy by node", [target('(max by (instance) (rate(node_disk_io_time_seconds_total{device!~"dm-.*|loop.*|sr.*"}[5m])))' + NODE, "{{nodename}}")],
       "Fraction of time the busiest disk on each node was doing I/O. Near 100% means everything on that node, etcd included, is queueing.",
       unit="percentunit", w=8, maxv=1, th=steps(GREEN, (0.8, RED))),
    ts("Filesystem used by node", [target(
        '(1 - node_filesystem_avail_bytes{mountpoint="/var"} / node_filesystem_size_bytes{mountpoint="/var"})' + NODE,
        "{{nodename}} /var")],
       "Talos EPHEMERAL partition (/var), which holds etcd, containerd images and pod logs. Kubelet starts evicting at 85-90%.",
       unit="percentunit", w=8, maxv=1, th=steps(GREEN, (0.85, RED))),
    ts("Network by node", [
        target('(sum by (instance) (rate(node_network_receive_bytes_total{device!~"lo|veth.*|lxc.*|cilium.*"}[5m])))' + NODE, "{{nodename}} rx", ref="A"),
        target('-(sum by (instance) (rate(node_network_transmit_bytes_total{device!~"lo|veth.*|lxc.*|cilium.*"}[5m])))' + NODE, "{{nodename}} tx", ref="B"),
    ], "Physical NIC throughput; transmit drawn below zero.", unit="Bps", w=8),
], 7)
panels += place([
    ts("Temperature by node", [target("(max by (instance) (node_hwmon_temp_celsius))" + NODE, "{{nodename}}")],
       "Hottest hardware sensor per node.", unit="celsius", w=8, th=steps(GREEN, (80, RED))),
    ts("Load per core", [target('(node_load5 / on (instance) count by (instance) (node_cpu_seconds_total{mode="idle"}))' + NODE, "{{nodename}}")],
       "5-minute load average divided by core count. Above 1 means runnable work is waiting (often on I/O here).",
       unit="short", w=8, th=steps(GREEN, (1, RED))),
    table("Node uptime", "(time() - node_boot_time_seconds)" + NODE,
          "Time since each node last booted. A node you did not reboot showing minutes here crashed or was PXE-reprovisioned.",
          ["nodename"], w=8, unit="s", value_name="Uptime",
          overrides=[colored_value("Uptime", steps(RED, (3600, YELLOW), (86400, GREEN)), unit="dtdurations", mode="color-text")]),
], 7)

# ── etcd and control plane ─────────────────────────────────────────────────
panels.append(row("etcd and control plane"))
panels += place([
    ts("etcd WAL fsync p99", [target('histogram_quantile(0.99, sum by (instance, le) (rate(etcd_disk_wal_fsync_duration_seconds_bucket[5m])))', "{{instance}}")],
       "etcd wants this under 10ms. On this cluster etcd shares its disk with the OS and containerd, and sustained spikes here precede 'leaderelection lost' in unrelated controllers.",
       unit="s", w=6, th=steps(GREEN, (0.01, YELLOW), (0.1, RED))),
    ts("etcd backend commit p99", [target('histogram_quantile(0.99, sum by (instance, le) (rate(etcd_disk_backend_commit_duration_seconds_bucket[5m])))', "{{instance}}")],
       "Time to commit to bbolt; etcd wants under 25ms.", unit="s", w=6, th=steps(GREEN, (0.025, YELLOW), (0.25, RED))),
    ts("etcd leader changes", [target("max(increase(etcd_server_leader_changes_seen_total[1h]))", "changes / 1h")],
       "Leader elections in the last hour. Zero is healthy; repeated changes mean members are missing heartbeats (usually disk).",
       unit="short", w=6, legend_calcs=("lastNotNull", "max", "sum")),
    ts("etcd DB size", [
        target("max by (instance) (etcd_mvcc_db_total_size_in_bytes)", "{{instance}} on disk", ref="A"),
        target("max by (instance) (etcd_mvcc_db_total_size_in_use_in_bytes)", "{{instance}} in use", ref="B"),
        target("max(etcd_server_quota_backend_bytes)", "quota", ref="C"),
    ], "On-disk vs live keyspace. The gap is reclaimable by defrag; EtcdReclaimableSpaceHigh watches it.", unit="bytes", w=6),
], 7)
panels += place([
    ts("API server p99 latency (non-watch)", [target(
        'histogram_quantile(0.99, sum by (le, verb) (rate(apiserver_request_duration_seconds_bucket{verb!~"WATCH|CONNECT"}[5m])))', "{{verb}}")],
       "p99 request latency by verb.", unit="s", w=8, th=steps(GREEN, (1, RED))),
    ts("API server errors", [target('sum by (code) (rate(apiserver_request_total{code=~"5.."}[5m]))', "{{code}}")],
       "5xx responses from the API server.", unit="reqps", w=8),
    ts("Controller work queues", [target('topk(5, sum by (name) (workqueue_depth))', "{{name}}")],
       "Deepest controller-runtime/client-go work queues across every controller. A queue that only grows is a stuck reconciler.",
       unit="short", w=8),
], 7)

# ── Workloads ──────────────────────────────────────────────────────────────
panels.append(row("Workloads"))
panels += place([
    ts("CPU by namespace (top 10)", [target(f'topk(10, sum by (namespace) (rate(container_cpu_usage_seconds_total{{{H}, container!=""}}[5m])))', "{{namespace}}")],
       "Cores consumed per namespace. vcluster-media's apps run in the vcluster-media namespace on the host.", unit="short", w=12, stack=True),
    ts("Memory by namespace (top 10)", [target(f'topk(10, sum by (namespace) (container_memory_working_set_bytes{{{H}, container!=""}}))', "{{namespace}}")],
       "Working-set memory per namespace.", unit="bytes", w=12, stack=True),
], 8)
panels += place([
    table("Pods in trouble",
          f'sum by (namespace, pod, phase) (kube_pod_status_phase{{{H}, phase=~"Pending|Failed|Unknown"}}) > 0',
          "Pods not Running or Succeeded.", ["namespace", "pod", "phase"], w=8,
          overrides=[{"matcher": {"id": "byName", "options": "Value"}, "properties": [{"id": "custom.hidden", "value": True}]}]),
    table("Containers waiting",
          f'sum by (namespace, pod, container, reason) (kube_pod_container_status_waiting_reason{{{H}, reason!="ContainerCreating"}}) > 0',
          "Containers stuck before start: CrashLoopBackOff, ImagePullBackOff, config errors.", ["namespace", "pod", "container", "reason"], w=8,
          overrides=[{"matcher": {"id": "byName", "options": "Value"}, "properties": [{"id": "custom.hidden", "value": True}]}]),
    table("Most restarts (24h)",
          f'topk(15, sum by (namespace, pod) (increase(kube_pod_container_status_restarts_total{{{H}}}[24h])) > 0.5)',
          "Pods that restarted in the last day. Many unrelated controllers restarting together is the etcd-starvation pattern, not 15 separate bugs.",
          ["namespace", "pod"], w=8, sort=("Restarts", True), value_name="Restarts",
          overrides=[colored_value("Restarts", steps(YELLOW, (5, ORANGE), (20, RED)), mode="color-text"),
                     {"matcher": {"id": "byName", "options": "Restarts"}, "properties": [{"id": "decimals", "value": 0}]}]),
], 8)
panels += place([
    table("Deployments and StatefulSets short of replicas",
          f'(max by (namespace, deployment) (kube_deployment_spec_replicas{{{H}}}) - max by (namespace, deployment) (kube_deployment_status_replicas_available{{{H}}})) > 0 '
          f'or label_replace((max by (namespace, statefulset) (kube_statefulset_replicas{{{H}}}) - max by (namespace, statefulset) (kube_statefulset_status_replicas_ready{{{H}}})) > 0, "deployment", "$1", "statefulset", "(.*)")',
          "Controllers with fewer ready replicas than desired. StatefulSets are listed under the same name column.",
          ["namespace", "deployment"], w=12, value_name="Missing",
          overrides=[colored_value("Missing", steps(YELLOW, (2, RED)), mode="color-text")]),
    table("DaemonSets not fully scheduled",
          f'(max by (namespace, daemonset) (kube_daemonset_status_desired_number_scheduled{{{H}}}) - max by (namespace, daemonset) (kube_daemonset_status_number_ready{{{H}}})) > 0',
          "DaemonSets (Cilium, node-exporter, promtail…) missing a ready pod on some node.",
          ["namespace", "daemonset"], w=12, value_name="Missing",
          overrides=[colored_value("Missing", steps(YELLOW, (2, RED)), mode="color-text")]),
], 6)

# ── GitOps and delivery ────────────────────────────────────────────────────
panels.append(row("GitOps and delivery"))
panels += place([
    table("ArgoCD apps needing attention",
          'argocd_app_info{health_status!="Healthy"} or argocd_app_info{sync_status!="Synced"}',
          "Applications that are not both Synced and Healthy.", ["name", "project", "health_status", "sync_status", "dest_server"], w=12,
          overrides=[{"matcher": {"id": "byName", "options": "Value"}, "properties": [{"id": "custom.hidden", "value": True}]},
                     {"matcher": {"id": "byName", "options": "name"}, "properties": [{"id": "links", "value": [
                         {"title": "Open in ArgoCD", "url": "https://argocd.cluster.integratn.tech/applications/argocd/${__value.raw}", "targetBlank": True}]}]},
                     {"matcher": {"id": "byName", "options": "health_status"}, "properties": [
                         {"id": "custom.cellOptions", "value": {"type": "color-text"}},
                         {"id": "mappings", "value": [{"type": "value", "options": {
                             "Healthy": {"color": GREEN, "index": 0}, "Progressing": {"color": BLUE, "index": 1},
                             "Degraded": {"color": RED, "index": 2}, "Missing": {"color": ORANGE, "index": 3},
                             "Suspended": {"color": YELLOW, "index": 4}, "Unknown": {"color": ORANGE, "index": 5}}}]}]},
                     {"matcher": {"id": "byName", "options": "sync_status"}, "properties": [
                         {"id": "custom.cellOptions", "value": {"type": "color-text"}},
                         {"id": "mappings", "value": [{"type": "value", "options": {
                             "Synced": {"color": GREEN, "index": 0}, "OutOfSync": {"color": YELLOW, "index": 1},
                             "Unknown": {"color": ORANGE, "index": 2}}}]}]}]),
    ts("ArgoCD syncs", [target('sum by (phase) (increase(argocd_app_sync_total[1h]))', "{{phase}}")],
       "Sync operations per hour by outcome. Error/Failed should stay at zero.", unit="short", w=6, stack=True,
       legend_calcs=("sum",)),
    ts("Promotion pipeline findings", [target('sum by (severity) (bosun_pipeline_findings)', "{{severity}}")],
       "Bosun's view of the Kargo promotion pipeline. 'blocking' means a pin has stopped moving.", unit="short", w=6),
], 8)
panels += place([
    table("Kargo stages whose last promotion did not succeed",
          'kargo_stage_last_promotion_phase{phase=~"Errored|Failed|Aborted"} == 1',
          "Stages whose most recent promotion ended Errored/Failed/Aborted. Exported by kube-state-metrics from the kargo-observability CustomResourceState config (Kargo itself exports nothing about promotions).",
          ["namespace", "stage", "phase"], w=12,
          overrides=[{"matcher": {"id": "byName", "options": "Value"}, "properties": [{"id": "custom.hidden", "value": True}]}]),
    table("vClusters",
          "platform_vcluster_ready",
          "Each vcluster the platform-status-reconciler tracks and whether it is Ready.", ["name", "namespace"], w=12, value_name="Ready",
          overrides=[{"matcher": {"id": "byName", "options": "Ready"}, "properties": [
              {"id": "custom.cellOptions", "value": {"type": "color-background"}},
              {"id": "mappings", "value": [{"type": "value", "options": {"0": {"text": "Not ready", "color": RED, "index": 0},
                                                                           "1": {"text": "Ready", "color": GREEN, "index": 1}}}]}]}]),
], 6)

# ── Storage and backups ────────────────────────────────────────────────────
panels.append(row("Storage (Tower NFS) and backups"))
panels += place([
    ts("NFS requests by node", [target('sum by (instance) (rate(node_nfs_requests_total[5m]))', "{{instance}}")],
       "NFS client operations against Tower (10.0.0.12). A node dropping to zero while its pods still mount NFS is the stale-mount pattern after a Tower restart.",
       unit="reqps", w=8),
    ts("NFS retransmissions", [target('sum by (instance) (rate(node_nfs_rpc_retransmissions_total[5m]))', "{{instance}}")],
       "RPC retransmits — Tower slow or unreachable. NFSMountErrors alerts on this.", unit="reqps", w=8, th=steps(GREEN, (0.01, RED))),
    table("PVC usage",
          'max by (namespace, persistentvolumeclaim) (kubelet_volume_stats_used_bytes / kubelet_volume_stats_capacity_bytes)',
          "Per-PVC usage from kubelet. nfs-subdir volumes report the whole Tower share they live on, so every PVC on the same class shows the same figure — read it as share fullness.",
          ["namespace", "persistentvolumeclaim"], w=8, sort=("Used", True), value_name="Used",
          overrides=[colored_value("Used", steps(GREEN, (0.8, YELLOW), (0.9, RED)), unit="percentunit", mode="gradient-gauge")]),
], 8)
panels += place([
    table("Backup and scheduled jobs",
          f'time() - max by (namespace, cronjob) (kube_cronjob_status_last_successful_time{{{H}}})',
          "Every CronJob and how long since it last succeeded. etcd snapshots should be under a day old.",
          ["namespace", "cronjob"], w=12, sort=("Since last success", True), value_name="Since last success",
          overrides=[colored_value("Since last success", steps(GREEN, (26 * 3600, YELLOW), (36 * 3600, RED)), unit="dtdurations", mode="color-text")]),
    table("PVCs not Bound",
          f'max by (namespace, persistentvolumeclaim, phase) (kube_persistentvolumeclaim_status_phase{{{H}, phase!="Bound"}}) > 0',
          "PersistentVolumeClaims stuck Pending or Lost — usually the NFS provisioner or Tower being down.",
          ["namespace", "persistentvolumeclaim", "phase"], w=12,
          overrides=[{"matcher": {"id": "byName", "options": "Value"}, "properties": [{"id": "custom.hidden", "value": True}]}]),
], 7)

# ── Edge and certificates ──────────────────────────────────────────────────
panels.append(row("Edge, certificates and security"))
panels += place([
    table("Certificates",
          "certmanager_certificate_expiration_timestamp_seconds - time()",
          "cert-manager certificates (Let's Encrypt via Cloudflare DNS-01) and time left. Renewal happens at 30 days.",
          ["exported_namespace", "name"], w=8, sort=("Expires in", False), value_name="Expires in",
          overrides=[colored_value("Expires in", steps(RED, (3 * 86400, ORANGE), (14 * 86400, YELLOW), (21 * 86400, GREEN)), unit="dtdurations", mode="color-text")]),
    ts("Cilium drops (Hubble)", [target('topk(8, sum by (reason) (rate(hubble_drop_total{reason!~"VLAN_FILTERED|UNSUPPORTED_L3_PROTOCOL"}[5m])))', "{{reason}}")],
       "Packets Cilium dropped, by reason, from Hubble. LAN background noise (VLAN_FILTERED, ~33/s, and UNSUPPORTED_L3_PROTOCOL) is filtered out. A jump in POLICY_DENIED right after a deploy is a missing NetworkPolicy half — the zero-byte-hang pattern.",
       unit="pps", w=8),
    ts("Critical/high vulnerabilities", [target('sum by (severity) (trivy_image_vulnerabilities{severity=~"Critical|High"})', "{{severity}}")],
       "Fixable Critical and High CVEs across scanned images (Trivy, ignoreUnfixed). A drop to zero can also mean the scanner wedged — check the Trivy dashboard.",
       unit="short", w=8),
], 8)

# ── Logs ───────────────────────────────────────────────────────────────────
panels.append(row("Logs"))
# Case-sensitive on purpose: (?i) made \bERROR\b match the word "error" inside
# info/warning lines. Matches logfmt/JSON level fields, klog E-lines, Go panics
# and upper-case ERROR/FATAL.
ERR = '|~ "(level=(error|fatal)|\\"level\\":\\"(error|fatal)\\"|^E[0-9]{4} |panic:|\\\\bERROR\\\\b|\\\\bFATAL\\\\b|OOMKilled)"'
panels += place([
    ts("Error log lines by namespace", [target(f'topk(10, sum by (namespace) (count_over_time({{namespace=~".+"}} {ERR} [5m])))', "{{namespace}}", ds=LOKI)],
       "Error/fatal/panic lines per 5 minutes from Promtail (host and vcluster pods alike). Use it to spot which namespace started shouting.",
       unit="short", w=24, stack=True),
], 7)
panels += place([{
    "type": "logs", "id": next(_ids), "title": "Latest errors", "datasource": LOKI, "_w": 24,
    "description": "Most recent error-level lines across the cluster, health-check noise filtered out.",
    "targets": [target(f'{{namespace=~".+"}} {ERR} !~ "(?i)(healthz|readyz|livez)"', ds=LOKI)],
    "options": {"showTime": True, "showLabels": False, "showCommonLabels": False, "wrapLogMessage": True,
                "prettifyLogMessage": False, "enableLogDetails": True, "sortOrder": "Descending", "dedupStrategy": "signature"},
}], 10)

# ── Monitoring stack itself ────────────────────────────────────────────────
panels.append(row("Monitoring stack"))
panels += place([
    ts("Prometheus active series", [target('sum(prometheus_tsdb_head_series)', "series")],
       "Head series in the hub Prometheus, vcluster remote-write included. Sudden growth is a cardinality leak.", unit="short", w=8),
    ts("Samples ingested", [
        target('sum(rate(prometheus_tsdb_head_samples_appended_total[5m]))', "samples/s", ref="A"),
    ], "Samples appended per second.", unit="short", w=8),
    ts("Alert notifications", [
        target('sum(increase(alertmanager_notifications_total{integration="webhook"}[1h]))', "sent / 1h", ref="A"),
        target('sum(increase(alertmanager_notifications_failed_total{integration="webhook"}[1h]))', "failed / 1h", ref="B"),
    ], "Alertmanager webhook deliveries to the Matrix receiver (the only integration in use). Failures mean alerts are firing into nothing.", unit="short", w=8),
], 7)

dashboard = {
    "uid": "homelab-overview",
    "title": "Homelab Overview",
    "description": "One screen for the whole homelab: what is broken right now, capacity and N+1, nodes, etcd, workloads, GitOps, storage, certificates and logs.",
    "tags": ["platform", "overview", "homelab"],
    "editable": True,
    "graphTooltip": 1,
    "refresh": "1m",
    "time": {"from": "now-6h", "to": "now"},
    "timepicker": {},
    "timezone": "browser",
    "schemaVersion": 39,
    "version": 1,
    "fiscalYearStartMonth": 0,
    "liveNow": False,
    "id": None,
    "annotations": {"list": [{"builtIn": 1, "datasource": {"type": "grafana", "uid": "-- Grafana --"}, "enable": True, "hide": True,
                              "iconColor": "rgba(0, 211, 255, 1)", "name": "Annotations & Alerts", "type": "dashboard"}]},
    "links": [
        {"title": "Platform Landing Zone", "url": "/d/platform-landing-zone/platform-landing-zone", "type": "link", "icon": "dashboard"},
        {"title": "ArgoCD", "url": "/d/argocd-overview/argocd-overview", "type": "link", "icon": "dashboard"},
        {"title": "Kratix", "url": "/d/kratix-platform/kratix-platform", "type": "link", "icon": "dashboard"},
        {"title": "vCluster Fleet", "url": "/d/vcluster-fleet-health/vcluster-fleet-health", "type": "link", "icon": "dashboard"},
        {"title": "NFS", "url": "/d/nfs-performance/nfs-performance", "type": "link", "icon": "dashboard"},
        {"title": "Trivy", "url": "/d/trivy-operator/trivy-operator-dashboard", "type": "link", "icon": "dashboard"},
        {"title": "Kargo", "url": "/d/kargo-delivery/kargo-delivery", "type": "link", "icon": "dashboard"},
        {"title": "etcd", "url": "/d/c2f4e12cdf69feb95caa41a5a1b423d9/etcd", "type": "link", "icon": "dashboard"},
        {"title": "Logs", "url": "/d/loki-kubernetes-logs/loki-kubernetes-logs", "type": "link", "icon": "dashboard"},
    ],
    "templating": {"list": [
        {"name": "datasource", "label": "Prometheus", "type": "datasource", "query": "prometheus", "current": {"text": "Prometheus", "value": "prometheus"},
         "hide": 0, "includeAll": False, "multi": False, "options": [], "refresh": 1, "regex": "", "skipUrlSync": False},
        {"name": "lokids", "label": "Loki", "type": "datasource", "query": "loki", "current": {"text": "Loki", "value": "loki"},
         "hide": 0, "includeAll": False, "multi": False, "options": [], "refresh": 1, "regex": "", "skipUrlSync": False},
    ]},
    "panels": panels,
}

body = json.dumps(dashboard, indent=2)
OUT.write_text(
    "# Generated by hack/dashboards/homelab_overview.py -- edit that, then regenerate.\n"
    "apiVersion: v1\n"
    "kind: ConfigMap\n"
    "metadata:\n"
    "  name: homelab-overview-dashboard\n"
    "  namespace: monitoring\n"
    "  labels:\n"
    '    grafana_dashboard: "1"\n'
    "  annotations:\n"
    '    grafana_folder: "Platform"\n'
    "data:\n"
    "  homelab-overview.json: |\n" + "\n".join("    " + l for l in body.splitlines()) + "\n"
)


def all_exprs():
    for p in panels:
        for t in p.get("targets", []):
            yield p["title"], t["datasource"]["type"], t["expr"]


if __name__ == "__main__":
    import sys
    if "--exprs" in sys.argv:
        for title, kind, e in all_exprs():
            print(f"{kind}\t{title}\t{e}")
    print(f"wrote {OUT.relative_to(OUT.parents[5])} ({len(panels)} panels)", file=sys.stderr)
