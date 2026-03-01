"""
title: Homelab Platform RAG
author: homelab
version: 0.4.1
license: MIT
description: Retrieves context from Qdrant and live observability data (Prometheus, Alertmanager, Loki) with intent-driven metric queries.
requirements: qdrant-client, requests
"""

import logging
import re
import threading
import time
from datetime import datetime, timezone
from typing import Any, Dict, List, Optional, Tuple

import requests
from pydantic import BaseModel, Field
from qdrant_client import QdrantClient
from qdrant_client.models import FieldCondition, Filter, MatchValue

log = logging.getLogger("rag-pipeline")


# ---------------------------------------------------------------------------
# Simple TTL cache — avoids hammering Prometheus/Alertmanager/Loki on every
# single message in a conversation.  Thread-safe.
# ---------------------------------------------------------------------------
class _TTLCache:
    """In-memory key→value cache with per-entry TTL (seconds)."""

    def __init__(self, default_ttl: int = 60):
        self._store: Dict[str, Tuple[float, Any]] = {}
        self._ttl = default_ttl
        self._lock = threading.Lock()

    def get(self, key: str) -> Optional[Any]:
        with self._lock:
            entry = self._store.get(key)
            if entry is None:
                return None
            ts, val = entry
            if time.monotonic() - ts > self._ttl:
                del self._store[key]
                return None
            return val

    def set(self, key: str, value: Any) -> None:
        with self._lock:
            self._store[key] = (time.monotonic(), value)

    def clear(self) -> None:
        with self._lock:
            self._store.clear()


# ---------------------------------------------------------------------------
# Well-known namespaces — used for keyword extraction from user messages
# ---------------------------------------------------------------------------
KNOWN_NAMESPACES = {
    "ai", "monitoring", "loki", "promtail", "argocd", "cert-manager",
    "nginx-gateway", "external-secrets", "kube-system", "authentik",
    "metallb-system", "cilium", "kratix-platform-system", "default",
}

# Well-known app / component names — used to build targeted Loki queries.
# Only these are treated as pod-name prefixes; arbitrary English words are ignored.
KNOWN_APPS = {
    "etcd", "coredns", "kube-apiserver", "kube-scheduler", "kube-controller-manager",
    "open-webui", "qdrant", "pipelines", "git-indexer", "ollama",
    "prometheus", "alertmanager", "grafana", "loki", "promtail",
    "argocd-server", "argocd-repo-server", "argocd-application-controller",
    "argocd-applicationset-controller", "argocd-notifications-controller",
    "nginx-gateway", "cert-manager", "external-secrets",
    "authentik", "metallb", "cilium", "hubble",
    "kratix", "vcluster", "matrix-alertmanager-receiver",
    "loki-gateway", "loki-canary",
    "hctl", "platform-status-reconciler",
}

# ---------------------------------------------------------------------------
# Intent-driven PromQL query catalog
# ---------------------------------------------------------------------------
# Each topic maps to a list of (label, promql, format) tuples.
# Formats: "pct" (0-1 → percentage), "pct100" (already 0-100), "bytes",
#          "topk" (table of metric→value), "count", "list"
# ---------------------------------------------------------------------------
QUERY_CATALOG: dict[str, list[tuple[str, str, str]]] = {
    "cpu_utilization": [
        (
            "Node CPU utilisation",
            "instance:node_cpu_utilisation:rate5m",
            "pct_per_instance",
        ),
        (
            "Top namespaces by CPU (5m rate)",
            "topk(10, sum by (namespace) (rate(container_cpu_usage_seconds_total{container!=''}[5m])))",
            "topk_cores",
        ),
    ],
    "cpu_limits": [
        (
            "Cluster CPU limits / allocatable",
            'sum(namespace_cpu:kube_pod_container_resource_limits:sum{}) / sum(kube_node_status_allocatable{resource="cpu"})',
            "pct",
        ),
        (
            "Cluster CPU requests / allocatable",
            'sum(namespace_cpu:kube_pod_container_resource_requests:sum{}) / sum(kube_node_status_allocatable{resource="cpu"})',
            "pct",
        ),
        (
            "Per-namespace CPU limits",
            "topk(10, namespace_cpu:kube_pod_container_resource_limits:sum)",
            "topk_cores",
        ),
        (
            "Per-namespace CPU requests",
            "topk(10, namespace_cpu:kube_pod_container_resource_requests:sum)",
            "topk_cores",
        ),
        (
            "Total allocatable CPU (cores)",
            'sum(kube_node_status_allocatable{resource="cpu"})',
            "scalar_cores",
        ),
    ],
    "cpu_throttling": [
        (
            "Throttled containers (>25% periods throttled)",
            "topk(10, sum by (namespace, pod, container) "
            "(rate(container_cpu_cfs_throttled_periods_total[5m])) / "
            "sum by (namespace, pod, container) "
            "(rate(container_cpu_cfs_periods_total[5m])) > 0.25)",
            "topk_pct",
        ),
    ],
    "memory_utilization": [
        (
            "Node memory utilisation",
            "instance:node_memory_utilisation:ratio",
            "pct_per_instance",
        ),
        (
            "Top namespaces by memory (working set)",
            "topk(10, sum by (namespace) (container_memory_working_set_bytes{container!=''}))",
            "topk_bytes",
        ),
    ],
    "memory_limits": [
        (
            "Cluster memory limits / allocatable",
            'sum(namespace_memory:kube_pod_container_resource_limits:sum{}) / sum(kube_node_status_allocatable{resource="memory"})',
            "pct",
        ),
        (
            "Cluster memory requests / allocatable",
            'sum(namespace_memory:kube_pod_container_resource_requests:sum{}) / sum(kube_node_status_allocatable{resource="memory"})',
            "pct",
        ),
        (
            "Per-namespace memory limits",
            "topk(10, namespace_memory:kube_pod_container_resource_limits:sum)",
            "topk_bytes",
        ),
        (
            "Per-namespace memory requests",
            "topk(10, namespace_memory:kube_pod_container_resource_requests:sum)",
            "topk_bytes",
        ),
        (
            "Total allocatable memory",
            'sum(kube_node_status_allocatable{resource="memory"})',
            "scalar_bytes",
        ),
    ],
    "memory_pressure": [
        (
            "OOMKilled containers (last terminated)",
            'kube_pod_container_status_last_terminated_reason{reason="OOMKilled"}',
            "list_oom",
        ),
        (
            "Node memory pressure",
            'kube_node_status_condition{condition="MemoryPressure",status="true"}',
            "node_condition",
        ),
        (
            "Node disk pressure",
            'kube_node_status_condition{condition="DiskPressure",status="true"}',
            "node_condition",
        ),
        (
            "Node PID pressure",
            'kube_node_status_condition{condition="PIDPressure",status="true"}',
            "node_condition",
        ),
    ],
    "pods": [
        (
            "Non-running pods",
            'kube_pod_status_phase{phase=~"Pending|Failed|Unknown"} == 1',
            "list_pods",
        ),
        (
            "Pod restarts (1h)",
            "topk(10, increase(kube_pod_container_status_restarts_total[1h]) > 0)",
            "topk_restarts",
        ),
        (
            "Unavailable deployment replicas",
            "kube_deployment_status_replicas_unavailable > 0",
            "list_deployments",
        ),
    ],
    "storage": [
        (
            "Node filesystem usage",
            '100 - (node_filesystem_avail_bytes{fstype!~"tmpfs|overlay",mountpoint="/"} / '
            'node_filesystem_size_bytes{fstype!~"tmpfs|overlay",mountpoint="/"} * 100)',
            "pct_per_instance",
        ),
    ],
    "nodes": [
        (
            "Node readiness",
            'kube_node_status_condition{condition="Ready",status="true"}',
            "node_condition",
        ),
        (
            "Node conditions (not Ready)",
            'kube_node_status_condition{condition!="Ready",status="true"}',
            "node_condition_alert",
        ),
    ],
    "platform_status": [
        (
            "VCluster readiness",
            "platform_vcluster_ready",
            "platform_ready",
        ),
        (
            "VCluster phase",
            'platform_vcluster_phase_info == 1',
            "platform_phase",
        ),
        (
            "VCluster pod health",
            "platform_vcluster_pods_ready",
            "platform_pods",
        ),
        (
            "VCluster ArgoCD sync",
            "platform_vcluster_argocd_synced",
            "platform_ready",
        ),
        (
            "VCluster ArgoCD health",
            "platform_vcluster_argocd_healthy",
            "platform_ready",
        ),
        (
            "VCluster sub-app health",
            "platform_vcluster_subapps_healthy",
            "platform_pods",
        ),
        (
            "Platform reconciler errors",
            "platform_status_reconciler_errors_total",
            "topk_restarts",
        ),
    ],
}

# ---------------------------------------------------------------------------
# Keyword → topic mapping for intent detection
# NOTE: Only include infrastructure-specific terms.  Common English words
# like "request", "status", "resource" cause false-positive matches on
# nearly every conversational message, triggering 20+ Prometheus queries
# per turn and eventually blocking the pipeline server.
# ---------------------------------------------------------------------------
TOPIC_KEYWORDS: dict[str, set[str]] = {
    "cpu":          {"cpu_utilization", "cpu_limits"},
    "processor":    {"cpu_utilization", "cpu_limits"},
    "compute":      {"cpu_utilization", "cpu_limits"},
    "throttle":     {"cpu_throttling", "cpu_utilization"},
    "throttling":   {"cpu_throttling", "cpu_utilization"},
    "memory usage": {"memory_utilization", "memory_limits"},
    "memory util":  {"memory_utilization", "memory_limits"},
    "ram":          {"memory_utilization", "memory_limits"},
    "oom":          {"memory_pressure", "memory_limits"},
    "out of memory": {"memory_pressure", "memory_limits"},
    "resource limits": {"cpu_limits", "memory_limits"},
    "resource requests": {"cpu_limits", "memory_limits"},
    "allocatable":  {"cpu_limits", "memory_limits"},
    "pressure":     {"memory_pressure"},
    "pod restart":  {"pods"},
    "pod restarts": {"pods"},
    "crashloop":    {"pods"},
    "crashloopbackoff": {"pods"},
    "pending pod":  {"pods"},
    "failed pod":   {"pods"},
    "unavailable replica": {"pods"},
    "storage":      {"storage"},
    "disk":         {"storage", "memory_pressure"},
    "pvc":          {"storage"},
    "filesystem":   {"storage"},
    "node health":  {"nodes", "cpu_utilization", "memory_utilization"},
    "node status":  {"nodes"},
    "node condition": {"nodes"},
    "cluster health": {"nodes", "pods", "cpu_utilization", "memory_utilization"},
    "cluster status": {"nodes", "pods"},
    "utilization":  {"cpu_utilization", "memory_utilization"},
    "vcluster":     {"platform_status", "pods"},
    "golden path":  {"platform_status"},
    "platform status": {"platform_status", "nodes", "pods"},
    "platform health": {"platform_status", "nodes", "pods"},
    "degraded":     {"platform_status", "pods"},
    "provisioning": {"platform_status"},
    "hctl":         {"platform_status"},
    "cli":          {"platform_status"},
}


class Pipeline:
    """Open WebUI Filter Pipeline — RAG + live observability."""

    class Valves(BaseModel):
        """Configurable parameters exposed in the Open WebUI admin UI."""

        # --- Filter wiring (required by the pipelines framework) ---
        pipelines: List[str] = ["*"]
        priority: int = 0

        # --- Qdrant / RAG ---
        QDRANT_URL: str = Field(
            default="http://qdrant.ai.svc.cluster.local:6333",
            description="Qdrant server URL",
        )
        QDRANT_COLLECTION: str = Field(
            default="homelab-platform",
            description="Qdrant collection name",
        )
        OLLAMA_URL: str = Field(
            default="http://10.0.3.4:11434",
            description="Ollama API URL",
        )
        EMBEDDING_MODEL: str = Field(
            default="nomic-embed-text",
            description="Embedding model name",
        )
        TOP_K: int = Field(default=8, description="Number of chunks to retrieve")
        SCORE_THRESHOLD: float = Field(
            default=0.3, description="Minimum similarity score"
        )

        # --- Observability ---
        ENABLE_OBSERVABILITY: bool = Field(
            default=True,
            description="Attach live metrics, alerts, and logs to every message",
        )
        PROMETHEUS_URL: str = Field(
            default="http://kube-prometheus-stack-prometheus.monitoring.svc.cluster.local:9090",
            description="Prometheus server URL",
        )
        ALERTMANAGER_URL: str = Field(
            default="http://kube-prometheus-stack-alertmanager.monitoring.svc.cluster.local:9093",
            description="Alertmanager server URL",
        )
        LOKI_URL: str = Field(
            default="http://loki-gateway.loki.svc.cluster.local:80",
            description="Loki gateway URL",
        )
        LOKI_LOG_LINES: int = Field(
            default=25, description="Max log lines to include"
        )
        LOKI_LOOKBACK: str = Field(
            default="1h", description="Loki time window (e.g. 1h, 30m, 6h)"
        )
        MAX_ALERTS: int = Field(
            default=15, description="Max alerts to include in context"
        )
        OBSERVABILITY_CACHE_TTL: int = Field(
            default=60,
            description="Seconds to cache observability data (alerts, health, metrics, logs). "
            "Prevents hammering Prometheus/Loki on every message in a conversation.",
        )
        MAX_TOPIC_QUERIES: int = Field(
            default=10,
            description="Maximum number of Prometheus queries for topic deep-dives per message. "
            "Prevents excessive query load when many topics match.",
        )

    # ---- lifecycle --------------------------------------------------------

    def __init__(self):
        self.type = "filter"
        self.name = "Homelab Platform RAG"
        self.valves = self.Valves(pipelines=["*"])
        self._qdrant: Optional[QdrantClient] = None
        self._obs_cache = _TTLCache(default_ttl=60)

    @property
    def qdrant(self) -> QdrantClient:
        if self._qdrant is None:
            self._qdrant = QdrantClient(url=self.valves.QDRANT_URL, timeout=10)
        return self._qdrant

    # ---- RAG helpers ------------------------------------------------------

    def _embed(self, text: str) -> list[float]:
        """Embed a single query string via Ollama.  Tries /api/embed first,
        then falls back to the older /api/embeddings endpoint."""
        base = self.valves.OLLAMA_URL

        # --- Newer endpoint (Ollama ≥ 0.4.0) ---
        try:
            resp = requests.post(
                f"{base}/api/embed",
                json={"model": self.valves.EMBEDDING_MODEL, "input": text},
                timeout=15,
            )
            resp.raise_for_status()
            data = resp.json()
            # Array input returns "embeddings", scalar returns "embedding"
            if "embeddings" in data and data["embeddings"]:
                return data["embeddings"][0]
            if "embedding" in data:
                return data["embedding"]
        except requests.exceptions.HTTPError:
            pass  # fall through to legacy endpoint
        except Exception as exc:
            log.warning("Embed /api/embed failed: %s", exc)

        # --- Legacy endpoint (Ollama < 0.4.0) ---
        resp = requests.post(
            f"{base}/api/embeddings",
            json={"model": self.valves.EMBEDDING_MODEL, "prompt": text},
            timeout=15,
        )
        resp.raise_for_status()
        return resp.json()["embedding"]

    @staticmethod
    def _content_to_text(content) -> str:
        """Normalise message content to a plain string.

        Open WebUI sends multimodal messages (images, files) as a list:
            [{"type": "text", "text": "..."}, {"type": "image_url", ...}]
        This helper extracts and joins the text parts so downstream code
        that expects a plain string (embedding, keyword matching, etc.)
        doesn't break.
        """
        if isinstance(content, str):
            return content
        if isinstance(content, list):
            parts = []
            for item in content:
                if isinstance(item, dict) and item.get("type") == "text":
                    parts.append(item.get("text", ""))
                elif isinstance(item, str):
                    parts.append(item)
            return " ".join(parts) if parts else ""
        return str(content) if content else ""

    def _retrieve(self, query: str) -> list[dict]:
        """Retrieve top-K relevant chunks from Qdrant."""
        try:
            query_vector = self._embed(query)
        except Exception as exc:
            log.warning("Embedding failed: %s", exc)
            return []

        try:
            results = self.qdrant.query_points(
                collection_name=self.valves.QDRANT_COLLECTION,
                query=query_vector,
                limit=self.valves.TOP_K,
                score_threshold=self.valves.SCORE_THRESHOLD,
                query_filter=Filter(
                    must_not=[
                        FieldCondition(
                            key="type", match=MatchValue(value="repo_metadata")
                        )
                    ]
                ),
            ).points
        except Exception as exc:
            log.warning("Qdrant search failed: %s", exc)
            return []

        chunks = []
        for hit in results:
            p = hit.payload or {}
            chunks.append(
                {
                    "text": p.get("text", ""),
                    "repo": p.get("repo", ""),
                    "filepath": p.get("filepath", ""),
                    "kind": p.get("kind", ""),
                    "name": p.get("name", ""),
                    "namespace": p.get("namespace", ""),
                    "chunk_type": p.get("chunk_type", ""),
                    "score": round(hit.score, 3),
                }
            )
        return chunks

    def _format_rag_context(self, chunks: list[dict]) -> str:
        if not chunks:
            return ""
        lines = ["## Retrieved Context from Platform Repos\n"]
        for i, c in enumerate(chunks, 1):
            source = c["filepath"]
            if c["repo"]:
                source = f'{c["repo"]}/{source}'
            meta = []
            if c["kind"]:
                meta.append(f'kind: {c["kind"]}')
            if c["name"]:
                meta.append(f'name: {c["name"]}')
            if c["namespace"]:
                meta.append(f'namespace: {c["namespace"]}')
            meta_str = f" ({', '.join(meta)})" if meta else ""
            lines.append(f"[{i}] **{source}**{meta_str} (score: {c['score']})")
            lines.append(f"```\n{c['text'][:2000]}\n```\n")
        return "\n".join(lines)

    # ---- Observability helpers --------------------------------------------

    def _extract_hints(self, text: str) -> dict:
        """Extract namespace / pod / app hints from the user message."""
        lower = text.lower()
        namespaces = [ns for ns in KNOWN_NAMESPACES if ns in lower]
        # Only match known app/component names — avoids treating English
        # hyphenated words ("follow-up", "etcd-related") as pod names.
        apps = [app for app in KNOWN_APPS if app in lower]
        error_focus = bool(
            re.search(r"error|crash|fail|panic|oom|restart|backoff|unhealthy", lower)
        )
        return {
            "namespaces": namespaces[:3],
            "apps": apps[:3],
            "error_focus": error_focus,
        }

    # -- Alertmanager -------------------------------------------------------

    def _get_firing_alerts(self) -> str:
        """Fetch currently firing alerts from Alertmanager."""
        try:
            resp = requests.get(
                f"{self.valves.ALERTMANAGER_URL}/api/v2/alerts",
                params={"silenced": "false", "inhibited": "false", "active": "true"},
                timeout=10,
            )
            resp.raise_for_status()
            raw_alerts = resp.json()
        except Exception as exc:
            log.warning("Alertmanager query failed: %s", exc)
            return "*Alertmanager unreachable.*\n"

        if not raw_alerts:
            return "No alerts currently firing.\n"

        # Sort by severity: critical > warning > info > none
        severity_order = {"critical": 0, "warning": 1, "info": 2, "none": 3}
        raw_alerts.sort(
            key=lambda a: severity_order.get(
                a.get("labels", {}).get("severity", "none"), 3
            )
        )
        alerts = raw_alerts[: self.valves.MAX_ALERTS]

        lines = ["## Currently Firing Alerts\n"]
        lines.append("| Severity | Alert | Namespace | Summary |")
        lines.append("|----------|-------|-----------|---------|")
        for a in alerts:
            labels = a.get("labels", {})
            annot = a.get("annotations", {})
            sev = labels.get("severity", "?")
            name = labels.get("alertname", "?")
            ns = labels.get("namespace", "-")
            summary = annot.get("summary", annot.get("description", "-"))
            if len(summary) > 120:
                summary = summary[:117] + "..."
            lines.append(f"| {sev} | {name} | {ns} | {summary} |")

        omitted = len(raw_alerts) - len(alerts)
        lines.append(f"\n*({len(alerts)} alert(s) shown"
                      + (f", {omitted} omitted" if omitted else "")
                      + ")*\n")
        return "\n".join(lines)

    # -- Prometheus ---------------------------------------------------------

    def _prom_query(self, promql: str) -> list[dict]:
        """Run an instant PromQL query and return the result vector."""
        try:
            resp = requests.get(
                f"{self.valves.PROMETHEUS_URL}/api/v1/query",
                params={"query": promql},
                timeout=10,
            )
            resp.raise_for_status()
            data = resp.json()
            if data.get("status") == "success":
                return data.get("data", {}).get("result", [])
        except Exception as exc:
            log.warning("Prometheus query failed (%s): %s", promql[:60], exc)
        return []

    def _get_cluster_health(self) -> str:
        """Build a lean cluster health baseline — nodes, restarts, non-running pods."""
        lines = ["## Cluster Health Baseline\n"]

        # 1. Node readiness
        nodes_ready = self._prom_query(
            'kube_node_status_condition{condition="Ready",status="true"}'
        )
        ready_names = [
            r["metric"].get("node", "?")
            for r in nodes_ready
            if r.get("value", [0, "0"])[1] == "1"
        ]
        lines.append(f"**Nodes**: {len(ready_names)} ready ({', '.join(ready_names)})")

        # 2. Pod restarts in the last hour
        restarts = self._prom_query(
            "topk(10, increase(kube_pod_container_status_restarts_total[1h]) > 0)"
        )
        if restarts:
            restart_lines = []
            for r in restarts[:10]:
                m = r["metric"]
                val = r.get("value", [0, "0"])[1]
                restart_lines.append(
                    f"  - {m.get('namespace', '?')}/{m.get('pod', '?')} "
                    f"({m.get('container', '?')}): {float(val):.0f} restart(s)"
                )
            lines.append(f"**Pod restarts (1h)**: {len(restarts)} container(s)")
            lines.extend(restart_lines)
        else:
            lines.append("**Pod restarts (1h)**: none")

        # 3. Non-running pods
        bad_pods = self._prom_query(
            'kube_pod_status_phase{phase=~"Pending|Failed|Unknown"} == 1'
        )
        if bad_pods:
            bp_lines = []
            for r in bad_pods[:10]:
                m = r["metric"]
                bp_lines.append(
                    f"  - {m.get('namespace', '?')}/{m.get('pod', '?')} "
                    f"→ {m.get('phase', '?')}"
                )
            lines.append(f"**Non-running pods**: {len(bad_pods)}")
            lines.extend(bp_lines)
        else:
            lines.append("**Non-running pods**: none")

        lines.append("")
        return "\n".join(lines)

    # -- Intent-driven metric deep-dives -----------------------------------

    def _match_topics(self, text: str) -> set[str]:
        """Match user message keywords to query catalog topics."""
        lower = text.lower()
        topics: set[str] = set()
        for keyword, topic_set in TOPIC_KEYWORDS.items():
            if keyword in lower:
                topics.update(topic_set)
        return topics

    @staticmethod
    def _fmt_bytes(b: float) -> str:
        """Format bytes to a human-readable string."""
        if b >= 1 << 30:
            return f"{b / (1 << 30):.1f} GiB"
        if b >= 1 << 20:
            return f"{b / (1 << 20):.1f} MiB"
        return f"{b / (1 << 10):.1f} KiB"

    def _format_query_result(
        self, label: str, result: list[dict], fmt: str
    ) -> str:
        """Format a single PromQL result vector into a readable string."""
        if not result:
            return f"**{label}**: no data\n"

        # --- scalar formats (single value) ---
        if fmt == "pct":
            val = float(result[0].get("value", [0, "0"])[1])
            return f"**{label}**: {val * 100:.1f}%\n"

        if fmt == "scalar_cores":
            val = float(result[0].get("value", [0, "0"])[1])
            return f"**{label}**: {val:.1f} cores\n"

        if fmt == "scalar_bytes":
            val = float(result[0].get("value", [0, "0"])[1])
            return f"**{label}**: {self._fmt_bytes(val)}\n"

        # --- per-instance percentage (0-1 ratio) ---
        if fmt == "pct_per_instance":
            parts = []
            for r in result:
                inst = r["metric"].get("instance", r["metric"].get("node", "?"))
                val = float(r.get("value", [0, "0"])[1])
                # Detect if already 0-100 range
                pct = val if val > 1.5 else val * 100
                parts.append(f"{inst}: {pct:.1f}%")
            return f"**{label}**: {', '.join(parts)}\n"

        # --- topk tables ---
        if fmt == "topk_cores":
            lines = [f"**{label}**:"]
            for r in result[:10]:
                ns = r["metric"].get("namespace", "?")
                val = float(r.get("value", [0, "0"])[1])
                lines.append(f"  - {ns}: {val:.3f} cores")
            return "\n".join(lines) + "\n"

        if fmt == "topk_bytes":
            lines = [f"**{label}**:"]
            for r in result[:10]:
                ns = r["metric"].get("namespace", "?")
                val = float(r.get("value", [0, "0"])[1])
                lines.append(f"  - {ns}: {self._fmt_bytes(val)}")
            return "\n".join(lines) + "\n"

        if fmt == "topk_pct":
            lines = [f"**{label}**:"]
            for r in result[:10]:
                m = r["metric"]
                ident = f"{m.get('namespace', '?')}/{m.get('pod', '?')}"
                if m.get("container"):
                    ident += f"/{m['container']}"
                val = float(r.get("value", [0, "0"])[1]) * 100
                lines.append(f"  - {ident}: {val:.1f}% throttled")
            return "\n".join(lines) + "\n"

        if fmt == "topk_restarts":
            lines = [f"**{label}**:"]
            for r in result[:10]:
                m = r["metric"]
                val = float(r.get("value", [0, "0"])[1])
                lines.append(
                    f"  - {m.get('namespace', '?')}/{m.get('pod', '?')} "
                    f"({m.get('container', '?')}): {val:.0f}"
                )
            return "\n".join(lines) + "\n"

        # --- list formats ---
        if fmt == "list_oom":
            victims = [
                r for r in result if r.get("value", [0, "0"])[1] == "1"
            ]
            if not victims:
                return f"**{label}**: none\n"
            lines = [f"**{label}**: {len(victims)} container(s)"]
            for r in victims[:10]:
                m = r["metric"]
                lines.append(
                    f"  - {m.get('namespace', '?')}/{m.get('pod', '?')} "
                    f"({m.get('container', '?')})"
                )
            return "\n".join(lines) + "\n"

        if fmt == "list_pods":
            bad = [r for r in result if r.get("value", [0, "0"])[1] == "1"]
            if not bad:
                return f"**{label}**: none\n"
            lines = [f"**{label}**: {len(bad)}"]
            for r in bad[:10]:
                m = r["metric"]
                lines.append(
                    f"  - {m.get('namespace', '?')}/{m.get('pod', '?')} → {m.get('phase', '?')}"
                )
            return "\n".join(lines) + "\n"

        if fmt == "list_deployments":
            bad = [r for r in result if float(r.get("value", [0, "0"])[1]) > 0]
            if not bad:
                return f"**{label}**: none\n"
            lines = [f"**{label}**: {len(bad)}"]
            for r in bad[:10]:
                m = r["metric"]
                val = float(r.get("value", [0, "0"])[1])
                lines.append(
                    f"  - {m.get('namespace', '?')}/{m.get('deployment', '?')}: "
                    f"{val:.0f} unavailable"
                )
            return "\n".join(lines) + "\n"

        if fmt == "node_condition":
            active = [r for r in result if r.get("value", [0, "0"])[1] == "1"]
            if not active:
                return f"**{label}**: none\n"
            nodes = [r["metric"].get("node", "?") for r in active]
            return f"**{label}**: {', '.join(nodes)}\n"

        if fmt == "node_condition_alert":
            active = [r for r in result if r.get("value", [0, "0"])[1] == "1"]
            if not active:
                return f"**{label}**: none\n"
            lines = [f"**{label}**:"]
            for r in active:
                m = r["metric"]
                lines.append(f"  - {m.get('node', '?')}: {m.get('condition', '?')}")
            return "\n".join(lines) + "\n"

        # --- platform status formats ---
        if fmt == "platform_ready":
            lines = [f"**{label}**:"]
            for r in result:
                name = r["metric"].get("name", "?")
                ns = r["metric"].get("namespace", "?")
                val = "✅ Ready" if r.get("value", [0, "0"])[1] == "1" else "❌ Not Ready"
                lines.append(f"  - {ns}/{name}: {val}")
            return "\n".join(lines) + "\n" if len(lines) > 1 else f"**{label}**: no vclusters\n"

        if fmt == "platform_phase":
            lines = [f"**{label}**:"]
            for r in result:
                name = r["metric"].get("name", "?")
                ns = r["metric"].get("namespace", "?")
                phase = r["metric"].get("phase", "?")
                lines.append(f"  - {ns}/{name}: {phase}")
            return "\n".join(lines) + "\n" if len(lines) > 1 else f"**{label}**: no vclusters\n"

        if fmt == "platform_pods":
            lines = [f"**{label}**:"]
            for r in result:
                name = r["metric"].get("name", "?")
                ns = r["metric"].get("namespace", "?")
                val = float(r.get("value", [0, "0"])[1])
                lines.append(f"  - {ns}/{name}: {val:.0f} pods")
            return "\n".join(lines) + "\n" if len(lines) > 1 else f"**{label}**: no vclusters\n"

        # Fallback
        return f"**{label}**: {len(result)} result(s)\n"

    def _run_topic_queries(self, topics: set[str]) -> str:
        """Run PromQL queries for matched topics, respecting the query budget."""
        if not topics:
            return ""

        lines = ["## Resource Deep Dive\n"]
        seen_labels: set[str] = set()
        query_count = 0
        budget = self.valves.MAX_TOPIC_QUERIES

        for topic in sorted(topics):
            queries = QUERY_CATALOG.get(topic, [])
            for label, promql, fmt in queries:
                if query_count >= budget:
                    lines.append(
                        f"\n*Query budget reached ({budget}). "
                        "Ask a more specific question for additional metrics.*\n"
                    )
                    return "\n".join(lines)
                if label in seen_labels:
                    continue  # avoid duplicate queries across overlapping topics
                seen_labels.add(label)
                result = self._prom_query(promql)
                lines.append(self._format_query_result(label, result, fmt))
                query_count += 1

        return "\n".join(lines)

    # -- Loki ---------------------------------------------------------------

    def _query_loki(self, hints: dict) -> str:
        """Query Loki for recent log lines based on extracted hints."""
        namespaces = hints.get("namespaces", [])
        apps = hints.get("apps", [])
        error_focus = hints.get("error_focus", False)

        # Build LogQL query — prefer namespace scoping, add app filter if available
        # Note: KNOWN_NAMESPACES and KNOWN_APPS contain only [a-z0-9-] so no
        # regex escaping is needed.  LogQL label matchers must NOT have a space
        # after the comma; Loki returns 400 otherwise.
        if namespaces and apps:
            ns_regex = "|".join(namespaces)
            app_regex = "|".join(apps)
            logql = f'{{namespace=~"{ns_regex}",pod=~"{app_regex}.*"}}'
        elif namespaces:
            ns_regex = "|".join(namespaces)
            logql = f'{{namespace=~"{ns_regex}"}}'
        elif apps:
            app_regex = "|".join(apps)
            logql = f'{{pod=~"{app_regex}.*"}}'
        else:
            # Default: error logs across all namespaces
            logql = '{namespace=~".+"}'
            error_focus = True

        if error_focus:
            logql += ' |~ "(?i)(error|fatal|panic|crash|oom|backoff|failed|exception)"'

        # Time range
        now_ns = int(time.time() * 1e9)
        lookback_secs = self._parse_duration(self.valves.LOKI_LOOKBACK)
        start_ns = now_ns - int(lookback_secs * 1e9)

        try:
            resp = requests.get(
                f"{self.valves.LOKI_URL}/loki/api/v1/query_range",
                params={
                    "query": logql,
                    "start": str(start_ns),
                    "end": str(now_ns),
                    "limit": str(self.valves.LOKI_LOG_LINES),
                    "direction": "backward",
                },
                timeout=15,
            )
            resp.raise_for_status()
            data = resp.json()
        except Exception as exc:
            log.warning("Loki query failed: %s", exc)
            return f"*Loki unreachable: {exc}*\n"

        streams = data.get("data", {}).get("result", [])
        if not streams:
            return (
                f"No log lines found for `{logql}` "
                f"in the last {self.valves.LOKI_LOOKBACK}.\n"
            )

        lines = [f"## Recent Logs (last {self.valves.LOKI_LOOKBACK})\n"]
        lines.append(f"Query: `{logql}`\n")
        lines.append("```")

        count = 0
        for stream in streams:
            labels = stream.get("stream", {})
            ns = labels.get("namespace", "?")
            pod = labels.get("pod", labels.get("instance", "?"))
            for ts, msg in stream.get("values", []):
                if count >= self.valves.LOKI_LOG_LINES:
                    break
                try:
                    dt = datetime.fromtimestamp(int(ts) / 1e9, tz=timezone.utc)
                    ts_str = dt.strftime("%H:%M:%S")
                except (ValueError, OSError):
                    ts_str = "??:??:??"
                msg_short = msg[:300] if len(msg) > 300 else msg
                lines.append(f"[{ts_str}] {ns}/{pod}: {msg_short}")
                count += 1
            if count >= self.valves.LOKI_LOG_LINES:
                break

        lines.append("```\n")
        return "\n".join(lines)

    @staticmethod
    def _parse_duration(s: str) -> int:
        """Parse a duration string like '1h', '30m', '6h' to seconds."""
        match = re.match(r"^(\d+)([smhd])$", s.strip())
        if not match:
            return 3600  # default 1h
        val, unit = int(match.group(1)), match.group(2)
        multipliers = {"s": 1, "m": 60, "h": 3600, "d": 86400}
        return val * multipliers.get(unit, 3600)

    # ---- System prompt ----------------------------------------------------

    SYSTEM_PREAMBLE = """You are my homelab platform assistant with access to both documentation and live cluster telemetry.

You have three sources of truth:
1. **Platform docs context** — indexed YAML/config files from the GitOps repos (below)
2. **Live observability data** — current alerts, cluster health baseline, and recent logs (below)
3. **Resource Deep Dive** — targeted Prometheus metrics matched to the user's question (below, when relevant)

RULES:
- Use ALL available sources to give the most complete answer.
- When answering "how is X configured?" — cite the doc context with file paths like `repo/path/to/file.yaml`.
- When answering "is X working?" or "what's wrong with X?" — reference the live alerts, metrics, and logs.
- When there is a **Resource Deep Dive** section, ALWAYS reference those specific numbers in your answer.
- When you see firing alerts relevant to the question, mention them proactively.
- When referencing metrics, quote the EXACT values (e.g. "CPU limits are at 67.3% of allocatable", "Node memory: 32.8%").
- When referencing logs, quote the specific log line(s).
- If neither source has the answer, say so clearly and suggest where to look.
- Prefer showing concrete data first, then generalise.
- When showing resource data, present it in a structured way (tables or bullet points with actual numbers).
- NEVER suggest running kubectl or other CLI commands when the data is already provided in the context below.
- ALWAYS cite file paths using the format: `repo/path/to/file.yaml`
"""

    # ---- Filter hooks -----------------------------------------------------

    async def inlet(self, body: dict, __user__: Optional[dict] = None) -> dict:
        """
        Filter hook — runs before every message is sent to the LLM.
        Attaches RAG context and live observability data to the system prompt.

        Observability data (alerts, health, topic metrics, logs) is cached for
        OBSERVABILITY_CACHE_TTL seconds so that follow-up messages in the same
        conversation don't re-issue 20+ Prometheus queries each time.
        """
        messages = body.get("messages", [])
        if not messages:
            return body

        # Ensure cache TTL matches the current valve setting
        self._obs_cache._ttl = self.valves.OBSERVABILITY_CACHE_TTL

        # Get the latest user message
        user_msg = None
        for msg in reversed(messages):
            if msg.get("role") == "user":
                user_msg = self._content_to_text(msg.get("content", ""))
                break
        if not user_msg:
            return body

        # --- RAG retrieval (not cached — query is message-specific) ---
        chunks = self._retrieve(user_msg)
        rag_block = self._format_rag_context(chunks)
        if not rag_block:
            rag_block = "*No relevant context found in the indexed platform repos.*\n"

        # --- Live observability (cached per data type) ---
        obs_block = ""
        if self.valves.ENABLE_OBSERVABILITY:
            hints = self._extract_hints(user_msg)

            # -- Alerts (cached) --
            alerts_block = self._obs_cache.get("alerts")
            if alerts_block is None:
                try:
                    alerts_block = self._get_firing_alerts()
                except Exception as exc:
                    log.warning("Alerts retrieval error: %s", exc)
                    alerts_block = "*Failed to fetch alerts.*\n"
                self._obs_cache.set("alerts", alerts_block)

            # -- Cluster health baseline (cached) --
            health_block = self._obs_cache.get("health")
            if health_block is None:
                try:
                    health_block = self._get_cluster_health()
                except Exception as exc:
                    log.warning("Health metrics error: %s", exc)
                    health_block = "*Failed to fetch cluster health.*\n"
                self._obs_cache.set("health", health_block)

            # -- Intent-driven metric deep-dives (cached by topic set) --
            topic_block = ""
            try:
                topics = self._match_topics(user_msg)
                if topics:
                    cache_key = f"topics:{'|'.join(sorted(topics))}"
                    topic_block = self._obs_cache.get(cache_key)
                    if topic_block is None:
                        log.info("Matched topics: %s", topics)
                        topic_block = self._run_topic_queries(topics)
                        self._obs_cache.set(cache_key, topic_block)
            except Exception as exc:
                log.warning("Topic query error: %s", exc)
                topic_block = ""

            # -- Loki logs (cached by hint fingerprint) --
            logs_cache_key = f"logs:{sorted(hints.get('namespaces', []))}:{sorted(hints.get('apps', []))}:{hints.get('error_focus')}"
            logs_block = self._obs_cache.get(logs_cache_key)
            if logs_block is None:
                try:
                    logs_block = self._query_loki(hints)
                except Exception as exc:
                    log.warning("Loki query error: %s", exc)
                    logs_block = "*Failed to query logs.*\n"
                self._obs_cache.set(logs_cache_key, logs_block)

            obs_block = f"{alerts_block}\n{health_block}\n{topic_block}\n{logs_block}"

        # --- Build augmented system message ---
        augmented_system = f"{self.SYSTEM_PREAMBLE}\n{rag_block}\n{obs_block}"

        if messages and messages[0].get("role") == "system":
            messages[0]["content"] = augmented_system
        else:
            messages.insert(0, {"role": "system", "content": augmented_system})

        body["messages"] = messages
        return body

    async def outlet(self, body: dict, __user__: Optional[dict] = None) -> dict:
        """Post-processing hook (no-op for now)."""
        return body
