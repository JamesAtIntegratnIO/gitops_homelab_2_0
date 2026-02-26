"""
title: Homelab Platform RAG
author: homelab
version: 0.2.2
license: MIT
description: Retrieves context from Qdrant and live observability data (Prometheus, Alertmanager, Loki) to augment every chat message.
requirements: qdrant-client, requests
"""

import logging
import re
import time
from datetime import datetime, timezone
from typing import List, Optional

import requests
from pydantic import BaseModel, Field
from qdrant_client import QdrantClient
from qdrant_client.models import FieldCondition, Filter, MatchValue

log = logging.getLogger("rag-pipeline")

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

    # ---- lifecycle --------------------------------------------------------

    def __init__(self):
        self.type = "filter"
        self.name = "Homelab Platform RAG"
        self.valves = self.Valves(pipelines=["*"])
        self._qdrant: Optional[QdrantClient] = None

    @property
    def qdrant(self) -> QdrantClient:
        if self._qdrant is None:
            self._qdrant = QdrantClient(url=self.valves.QDRANT_URL, timeout=10)
        return self._qdrant

    # ---- RAG helpers ------------------------------------------------------

    def _embed(self, text: str) -> list[float]:
        """Embed a single query string via Ollama."""
        resp = requests.post(
            f"{self.valves.OLLAMA_URL}/api/embed",
            json={"model": self.valves.EMBEDDING_MODEL, "input": [text]},
            timeout=30,
        )
        resp.raise_for_status()
        return resp.json()["embeddings"][0]

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
        """Build a cluster health snapshot from a curated set of PromQL queries."""
        lines = ["## Cluster Health Snapshot\n"]

        # 1. Node readiness
        nodes_ready = self._prom_query(
            'kube_node_status_condition{condition="Ready",status="true"}'
        )
        nodes_not_ready = self._prom_query(
            'kube_node_status_condition{condition="Ready",status="false"}'
        )
        ready_names = [
            r["metric"].get("node", "?")
            for r in nodes_ready
            if r.get("value", [0, "0"])[1] == "1"
        ]
        not_ready_names = [
            r["metric"].get("node", "?")
            for r in nodes_not_ready
            if r.get("value", [0, "0"])[1] == "1"
        ]
        lines.append(
            f"**Nodes**: {len(ready_names)} ready"
            + (f", {len(not_ready_names)} NOT ready ({', '.join(not_ready_names)})"
               if not_ready_names else "")
            + f"  ({', '.join(ready_names)})"
        )

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

        # 4. Node CPU utilisation
        cpu = self._prom_query("instance:node_cpu_utilisation:rate5m")
        if cpu:
            cpu_parts = []
            for r in cpu:
                node = r["metric"].get("instance", "?")
                val = float(r.get("value", [0, "0"])[1]) * 100
                cpu_parts.append(f"{node}: {val:.1f}%")
            lines.append(f"**Node CPU**: {', '.join(cpu_parts)}")

        # 5. Node memory utilisation
        mem = self._prom_query("instance:node_memory_utilisation:ratio")
        if mem:
            mem_parts = []
            for r in mem:
                node = r["metric"].get("instance", "?")
                val = float(r.get("value", [0, "0"])[1]) * 100
                mem_parts.append(f"{node}: {val:.1f}%")
            lines.append(f"**Node Memory**: {', '.join(mem_parts)}")

        lines.append("")
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

You have two sources of truth:
1. **Platform docs context** — indexed YAML/config files from the GitOps repos (below)
2. **Live observability data** — current alerts, cluster health metrics, and recent logs (below)

RULES:
- Use BOTH sources to give the most complete answer.
- When answering "how is X configured?" — cite the doc context with file paths like `repo/path/to/file.yaml`.
- When answering "is X working?" or "what's wrong with X?" — reference the live alerts, metrics, and logs.
- When you see firing alerts relevant to the question, mention them proactively.
- When referencing metrics, quote the actual values (e.g. "Node CPU is at 34.2%").
- When referencing logs, quote the specific log line(s).
- If neither source has the answer, say so clearly and suggest where to look.
- Prefer showing concrete data first, then generalise.
- If code or YAML is referenced, quote the relevant section.
- ALWAYS cite file paths using the format: `repo/path/to/file.yaml`
"""

    # ---- Filter hooks -----------------------------------------------------

    async def inlet(self, body: dict, __user__: Optional[dict] = None) -> dict:
        """
        Filter hook — runs before every message is sent to the LLM.
        Attaches RAG context and live observability data to the system prompt.
        """
        messages = body.get("messages", [])
        if not messages:
            return body

        # Get the latest user message
        user_msg = None
        for msg in reversed(messages):
            if msg.get("role") == "user":
                user_msg = msg.get("content", "")
                break
        if not user_msg:
            return body

        # --- RAG retrieval ---
        chunks = self._retrieve(user_msg)
        rag_block = self._format_rag_context(chunks)
        if not rag_block:
            rag_block = "*No relevant context found in the indexed platform repos.*\n"

        # --- Live observability ---
        obs_block = ""
        if self.valves.ENABLE_OBSERVABILITY:
            hints = self._extract_hints(user_msg)

            alerts_block = ""
            health_block = ""
            logs_block = ""

            try:
                alerts_block = self._get_firing_alerts()
            except Exception as exc:
                log.warning("Alerts retrieval error: %s", exc)
                alerts_block = "*Failed to fetch alerts.*\n"

            try:
                health_block = self._get_cluster_health()
            except Exception as exc:
                log.warning("Health metrics error: %s", exc)
                health_block = "*Failed to fetch cluster health.*\n"

            try:
                logs_block = self._query_loki(hints)
            except Exception as exc:
                log.warning("Loki query error: %s", exc)
                logs_block = "*Failed to query logs.*\n"

            obs_block = f"{alerts_block}\n{health_block}\n{logs_block}"

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
