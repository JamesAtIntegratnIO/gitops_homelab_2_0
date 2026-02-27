"""
title: Homelab Platform RAG
author: homelab
version: 0.2.0
license: MIT
description: Search indexed GitOps platform docs and configuration from Qdrant. Pair with MCP tools for live cluster data.
requirements: qdrant-client, requests
"""

import logging
from typing import Optional

import requests
from pydantic import BaseModel, Field
from qdrant_client import QdrantClient
from qdrant_client.models import FieldCondition, Filter, MatchValue

log = logging.getLogger("rag-tool")


class Tools:
    """Open WebUI Tools — RAG search over indexed GitOps platform repositories."""

    class Valves(BaseModel):
        """Configurable parameters exposed in the Open WebUI admin UI."""

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
            default=0.3, description="Minimum similarity score",
        )

    def __init__(self):
        self.valves = self.Valves()
        self._qdrant: Optional[QdrantClient] = None

    @property
    def qdrant(self) -> QdrantClient:
        if self._qdrant is None:
            self._qdrant = QdrantClient(url=self.valves.QDRANT_URL, timeout=10)
        return self._qdrant

    def _embed(self, text: str) -> list[float]:
        resp = requests.post(
            f"{self.valves.OLLAMA_URL}/api/embed",
            json={"model": self.valves.EMBEDDING_MODEL, "input": [text]},
            timeout=30,
        )
        resp.raise_for_status()
        return resp.json()["embeddings"][0]

    def _format_results(self, results, prefix: str = "") -> str:
        if not results:
            return "No relevant documents found."
        lines = [f"{prefix}Found {len(results)} relevant document(s):\n"]
        for i, hit in enumerate(results, 1):
            p = hit.payload or {}
            source = p.get("filepath", "?")
            if p.get("repo"):
                source = f'{p["repo"]}/{source}'
            meta = []
            if p.get("kind"):
                meta.append(f'kind: {p["kind"]}')
            if p.get("name"):
                meta.append(f'name: {p["name"]}')
            if p.get("namespace"):
                meta.append(f'namespace: {p["namespace"]}')
            meta_str = f" ({', '.join(meta)})" if meta else ""
            lines.append(f"[{i}] **{source}**{meta_str} (score: {hit.score:.3f})")
            text = p.get("text", "")
            lines.append(f"```\n{text[:2000]}\n```\n")
        return "\n".join(lines)

    # ====================================================================
    # Public Tools
    # ====================================================================

    def search_platform_docs(
        self,
        query: str = Field(
            ..., description="Natural language search query about the homelab platform configuration, architecture, or setup."
        ),
    ) -> str:
        """
        Search the indexed homelab platform documentation and configuration files.
        Use this to answer questions about how things are configured, what Helm values are set,
        what Kubernetes manifests exist, ArgoCD application definitions, promise pipelines,
        Terraform modules, and general platform architecture.
        Returns relevant YAML/config snippets from the GitOps repositories with file paths and similarity scores.
        This searches the git repos — for live cluster state, use the MCP Kubernetes and Prometheus tools instead.
        """
        try:
            query_vector = self._embed(query)
        except Exception as exc:
            return f"Embedding failed: {exc}"

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
            return f"Qdrant search failed: {exc}"

        return self._format_results(results)

    def search_platform_docs_by_kind(
        self,
        query: str = Field(
            ..., description="Natural language search query."
        ),
        kind: str = Field(
            ..., description="Kubernetes resource kind to filter by (e.g. 'Deployment', 'HelmRelease', 'Application', 'Promise', 'ConfigMap')."
        ),
    ) -> str:
        """
        Search indexed platform docs filtered to a specific Kubernetes resource kind.
        Use this when you know you're looking for a specific type of resource — e.g. all Applications,
        all Promises, all ConfigMaps matching a query. More precise than the general search.
        """
        try:
            query_vector = self._embed(query)
        except Exception as exc:
            return f"Embedding failed: {exc}"

        try:
            results = self.qdrant.query_points(
                collection_name=self.valves.QDRANT_COLLECTION,
                query=query_vector,
                limit=self.valves.TOP_K,
                score_threshold=self.valves.SCORE_THRESHOLD,
                query_filter=Filter(
                    must=[
                        FieldCondition(key="kind", match=MatchValue(value=kind)),
                    ],
                    must_not=[
                        FieldCondition(
                            key="type", match=MatchValue(value="repo_metadata")
                        )
                    ],
                ),
            ).points
        except Exception as exc:
            return f"Qdrant search failed: {exc}"

        if not results:
            return f"No documents found for kind='{kind}' matching '{query}'."
        return self._format_results(results)

    def search_platform_docs_by_namespace(
        self,
        query: str = Field(
            ..., description="Natural language search query."
        ),
        namespace: str = Field(
            ..., description="Kubernetes namespace to filter by (e.g. 'argocd', 'monitoring', 'ai')."
        ),
    ) -> str:
        """
        Search indexed platform docs filtered to resources in a specific namespace.
        Use this when investigating a namespace — finds all indexed configs, manifests,
        and Helm values targeting that namespace.
        """
        try:
            query_vector = self._embed(query)
        except Exception as exc:
            return f"Embedding failed: {exc}"

        try:
            results = self.qdrant.query_points(
                collection_name=self.valves.QDRANT_COLLECTION,
                query=query_vector,
                limit=self.valves.TOP_K,
                score_threshold=self.valves.SCORE_THRESHOLD,
                query_filter=Filter(
                    must=[
                        FieldCondition(key="namespace", match=MatchValue(value=namespace)),
                    ],
                    must_not=[
                        FieldCondition(
                            key="type", match=MatchValue(value="repo_metadata")
                        )
                    ],
                ),
            ).points
        except Exception as exc:
            return f"Qdrant search failed: {exc}"

        if not results:
            return f"No documents found in namespace '{namespace}' matching '{query}'."
        return self._format_results(results, prefix=f"Namespace '{namespace}': ")
