"""
Homelab Platform RAG Pipeline — Open WebUI Filter

This pipeline intercepts user messages, retrieves relevant context from the
Qdrant vector store (populated by the git-indexer), and augments the system
prompt so the LLM answers with file-path citations.

Upload this file via Open WebUI Admin → Settings → Pipelines → Add Filter,
or mount it into the Pipelines container at /app/pipelines/.
"""

import os
import logging
from typing import Optional

import requests
from qdrant_client import QdrantClient
from qdrant_client.models import Filter, FieldCondition, MatchValue

log = logging.getLogger("rag-pipeline")


class Pipeline:
    """Open WebUI Filter Pipeline for homelab RAG."""

    class Valves:
        """Configurable parameters exposed in the Open WebUI UI."""
        QDRANT_URL: str = os.environ.get("QDRANT_URL", "http://qdrant.ai.svc.cluster.local:6333")
        QDRANT_COLLECTION: str = os.environ.get("QDRANT_COLLECTION", "homelab-platform")
        OLLAMA_URL: str = os.environ.get("OLLAMA_URL", "http://10.0.3.4:11434")
        EMBEDDING_MODEL: str = os.environ.get("EMBEDDING_MODEL", "nomic-embed-text")
        TOP_K: int = 8
        SCORE_THRESHOLD: float = 0.3
        MAX_CONTEXT_TOKENS: int = 4000

    def __init__(self):
        self.name = "Homelab Platform RAG"
        self.valves = self.Valves()
        self._qdrant: Optional[QdrantClient] = None

    @property
    def qdrant(self) -> QdrantClient:
        if self._qdrant is None:
            self._qdrant = QdrantClient(url=self.valves.QDRANT_URL, timeout=10)
        return self._qdrant

    def _embed(self, text: str) -> list[float]:
        """Embed a single query via Ollama."""
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
            results = self.qdrant.search(
                collection_name=self.valves.QDRANT_COLLECTION,
                query_vector=query_vector,
                limit=self.valves.TOP_K,
                score_threshold=self.valves.SCORE_THRESHOLD,
                query_filter=Filter(
                    must_not=[
                        FieldCondition(key="type", match=MatchValue(value="repo_metadata"))
                    ]
                ),
            )
        except Exception as exc:
            log.warning("Qdrant search failed: %s", exc)
            return []

        chunks = []
        for hit in results:
            payload = hit.payload or {}
            chunks.append({
                "text": payload.get("text", ""),
                "repo": payload.get("repo", ""),
                "filepath": payload.get("filepath", ""),
                "kind": payload.get("kind", ""),
                "name": payload.get("name", ""),
                "namespace": payload.get("namespace", ""),
                "chunk_type": payload.get("chunk_type", ""),
                "score": round(hit.score, 3),
            })
        return chunks

    def _format_context(self, chunks: list[dict]) -> str:
        """Format retrieved chunks into a context block for the system prompt."""
        if not chunks:
            return ""

        lines = ["## Retrieved Context from Platform Repos\n"]
        for i, chunk in enumerate(chunks, 1):
            source = chunk["filepath"]
            if chunk["repo"]:
                source = f'{chunk["repo"]}/{source}'
            meta_parts = []
            if chunk["kind"]:
                meta_parts.append(f'kind: {chunk["kind"]}')
            if chunk["name"]:
                meta_parts.append(f'name: {chunk["name"]}')
            if chunk["namespace"]:
                meta_parts.append(f'namespace: {chunk["namespace"]}')
            meta_str = f" ({', '.join(meta_parts)})" if meta_parts else ""

            lines.append(f"[{i}] **{source}**{meta_str} (score: {chunk['score']})")
            lines.append(f"```\n{chunk['text'][:2000]}\n```\n")

        return "\n".join(lines)

    SYSTEM_PREAMBLE = """You are my homelab platform documentation assistant.

RULES:
- Answer ONLY from the retrieved context below. Do not use prior knowledge.
- If the context doesn't contain the answer, say "I couldn't find that in the indexed platform docs" and list the sources you searched.
- ALWAYS cite file paths using the format: `repo/path/to/file.yaml`
- Prefer showing the one most relevant snippet, plus 1-3 supporting references.
- When describing patterns, show the concrete example from the codebase first, then generalize.
- If code or YAML is referenced, quote the relevant section.
"""

    def inlet(self, body: dict, __user__: Optional[dict] = None) -> dict:
        """
        Filter hook: runs before the message is sent to the LLM.
        Retrieves context and prepends it to the system message.
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

        # Retrieve relevant chunks
        chunks = self._retrieve(user_msg)
        context_block = self._format_context(chunks)

        if not context_block:
            context_block = "*No relevant context was found in the indexed platform repos.*"

        # Build augmented system message
        augmented_system = f"{self.SYSTEM_PREAMBLE}\n{context_block}"

        # Prepend or replace system message
        if messages and messages[0].get("role") == "system":
            messages[0]["content"] = augmented_system
        else:
            messages.insert(0, {"role": "system", "content": augmented_system})

        body["messages"] = messages
        return body

    def outlet(self, body: dict, __user__: Optional[dict] = None) -> dict:
        """Post-processing hook (no-op for now, can add citation formatting later)."""
        return body
