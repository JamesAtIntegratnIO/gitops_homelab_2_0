#!/usr/bin/env python3
"""
Git Indexer — clones repos, chunks files structurally, embeds via Ollama,
and upserts vectors + metadata into Qdrant.
"""

import hashlib
import json
import logging
import os
import re
import sys
import time
from fnmatch import fnmatch
from pathlib import Path
from typing import Optional

import git
import requests
import tiktoken
import yaml
from qdrant_client import QdrantClient
from qdrant_client.models import (
    Distance,
    PointStruct,
    VectorParams,
)

# ── Configuration ───────────────────────────────────────────────────
OLLAMA_URL = os.environ.get("OLLAMA_URL", "http://10.0.3.4:11434")
QDRANT_URL = os.environ.get("QDRANT_URL", "http://qdrant.ai.svc.cluster.local:6333")
QDRANT_COLLECTION = os.environ.get("QDRANT_COLLECTION", "homelab-platform")
EMBEDDING_MODEL = os.environ.get("EMBEDDING_MODEL", "nomic-embed-text")
REPOS = [r.strip() for r in os.environ.get("REPOS", "").split(",") if r.strip()]
GITHUB_TOKEN = os.environ.get("GITHUB_TOKEN", "")
INCLUDE_PATTERNS = [
    p.strip()
    for p in os.environ.get(
        "INCLUDE_PATTERNS", "*.yaml,*.yml,*.md,*.json,*.tf,*.sh,*.mmd"
    ).split(",")
]
EXCLUDE_PATTERNS = [
    p.strip()
    for p in os.environ.get(
        "EXCLUDE_PATTERNS", "vendor/,node_modules/,*.lock,.git/,bin/,result/"
    ).split(",")
]
CHUNK_MAX_TOKENS = int(os.environ.get("CHUNK_MAX_TOKENS", "512"))
WORKSPACE = Path(os.environ.get("WORKSPACE", "/workspace"))
EMBEDDING_DIM = 768  # nomic-embed-text dimension
RETRY_MAX = 3
RETRY_DELAY = 5

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(message)s",
    datefmt="%Y-%m-%d %H:%M:%S",
)
log = logging.getLogger("git-indexer")

# tiktoken encoder for token counting (cl100k_base works well as approximation)
_enc = tiktoken.get_encoding("cl100k_base")


# ── Redaction ───────────────────────────────────────────────────────
SECRET_PATTERNS = [
    re.compile(r"(token|password|secret|apikey|api_key|client_secret|authorization)\s*[:=]\s*\S+", re.IGNORECASE),
    re.compile(r"Bearer\s+\S+", re.IGNORECASE),
    re.compile(r"(-----BEGIN[A-Z ]+-----[\s\S]*?-----END[A-Z ]+-----)"),
]


def redact_secrets(text: str) -> str:
    """Pattern-redact common secret values from text."""
    for pat in SECRET_PATTERNS:
        text = pat.sub(lambda m: m.group(0).split(":")[0] + ": <REDACTED>" if ":" in m.group(0) else "<REDACTED>", text)
    return text


def is_k8s_secret(doc_text: str) -> bool:
    """Check if a YAML document is a Kubernetes Secret."""
    try:
        obj = yaml.safe_load(doc_text)
        if isinstance(obj, dict):
            return obj.get("kind") == "Secret"
    except yaml.YAMLError:
        pass
    return False


# ── Token counting ──────────────────────────────────────────────────
def count_tokens(text: str) -> int:
    return len(_enc.encode(text))


# ── Chunking ────────────────────────────────────────────────────────
def chunk_markdown(text: str, filepath: str) -> list[dict]:
    """Split markdown by H1/H2 headings, keeping code blocks with their heading."""
    chunks = []
    current_heading = filepath
    current_lines: list[str] = []

    for line in text.splitlines(keepends=True):
        if re.match(r"^#{1,2}\s+", line):
            # Flush previous section
            if current_lines:
                body = "".join(current_lines).strip()
                if body:
                    chunks.append({
                        "text": f"# {current_heading}\n\n{body}",
                        "heading": current_heading,
                    })
            current_heading = line.strip().lstrip("#").strip()
            current_lines = []
        else:
            current_lines.append(line)

    # Flush last section
    if current_lines:
        body = "".join(current_lines).strip()
        if body:
            chunks.append({
                "text": f"# {current_heading}\n\n{body}",
                "heading": current_heading,
            })

    return chunks if chunks else [{"text": text, "heading": filepath}]


def chunk_k8s_yaml(text: str, filepath: str) -> list[dict]:
    """Split multi-document YAML by --- separator. Extract kind/name/namespace metadata."""
    docs = re.split(r"\n---\s*\n", text)
    chunks = []

    for doc in docs:
        doc = doc.strip()
        if not doc:
            continue

        # Skip Secret manifests entirely
        if is_k8s_secret(doc):
            log.info("Skipping Secret document in %s", filepath)
            continue

        metadata = {"kind": "", "name": "", "namespace": ""}
        try:
            obj = yaml.safe_load(doc)
            if isinstance(obj, dict):
                metadata["kind"] = obj.get("kind", "")
                meta = obj.get("metadata", {}) or {}
                metadata["name"] = meta.get("name", "")
                metadata["namespace"] = meta.get("namespace", "")
        except yaml.YAMLError:
            pass

        doc = redact_secrets(doc)
        chunks.append({"text": doc, **metadata})

    return chunks


def chunk_helm_values(text: str, filepath: str) -> list[dict]:
    """Split Helm values YAML by top-level keys."""
    try:
        obj = yaml.safe_load(text)
    except yaml.YAMLError:
        return [{"text": redact_secrets(text), "section": "root"}]

    if not isinstance(obj, dict):
        return [{"text": redact_secrets(text), "section": "root"}]

    chunks = []
    for key, value in obj.items():
        section_text = yaml.dump({key: value}, default_flow_style=False)
        section_text = redact_secrets(section_text)
        if count_tokens(section_text) > 0:
            chunks.append({"text": section_text, "section": key})

    return chunks if chunks else [{"text": redact_secrets(text), "section": "root"}]


def chunk_generic(text: str, filepath: str) -> list[dict]:
    """For files that don't match a structural pattern, chunk by token limit."""
    text = redact_secrets(text)
    if count_tokens(text) <= CHUNK_MAX_TOKENS:
        return [{"text": text}]

    # Split by lines, accumulate until token limit
    chunks = []
    current: list[str] = []
    current_tokens = 0

    for line in text.splitlines(keepends=True):
        line_tokens = count_tokens(line)
        if current_tokens + line_tokens > CHUNK_MAX_TOKENS and current:
            chunks.append({"text": "".join(current)})
            current = []
            current_tokens = 0
        current.append(line)
        current_tokens += line_tokens

    if current:
        chunks.append({"text": "".join(current)})

    return chunks


def classify_and_chunk(text: str, filepath: str) -> list[dict]:
    """Determine file type and dispatch to appropriate chunker."""
    ext = Path(filepath).suffix.lower()
    name = Path(filepath).name.lower()

    if ext in (".md", ".mmd"):
        raw_chunks = chunk_markdown(text, filepath)
        return [{"chunk_type": "markdown", **c} for c in raw_chunks]

    if ext in (".yaml", ".yml"):
        # Is this a values file (Helm)?
        if "values" in name or name == "chart.yaml":
            raw_chunks = chunk_helm_values(text, filepath)
            return [{"chunk_type": "helm-values", **c} for c in raw_chunks]
        # Otherwise treat as Kubernetes YAML
        raw_chunks = chunk_k8s_yaml(text, filepath)
        return [{"chunk_type": "k8s-yaml", **c} for c in raw_chunks]

    # Everything else (tf, sh, json, etc.)
    raw_chunks = chunk_generic(text, filepath)
    return [{"chunk_type": "generic", **c} for c in raw_chunks]


# ── Embedding ───────────────────────────────────────────────────────
EMBED_MAX_TOKENS = 8000  # nomic-embed-text context is 8192, leave headroom


def _truncate_to_tokens(text: str, max_tokens: int = EMBED_MAX_TOKENS) -> str:
    """Truncate text to fit within a token budget."""
    tokens = _enc.encode(text)
    if len(tokens) <= max_tokens:
        return text
    return _enc.decode(tokens[:max_tokens])


def _embed_single(url: str, text: str) -> list[float]:
    """Embed a single text string, retrying on failure."""
    text = _truncate_to_tokens(text)
    payload = {"model": EMBEDDING_MODEL, "input": text}
    for attempt in range(1, RETRY_MAX + 1):
        try:
            resp = requests.post(url, json=payload, timeout=120)
            if resp.status_code != 200:
                log.warning("Single-embed attempt %d/%d status=%d body=%s",
                            attempt, RETRY_MAX, resp.status_code, resp.text[:300])
                if attempt < RETRY_MAX:
                    time.sleep(RETRY_DELAY * attempt)
                    continue
                resp.raise_for_status()
            data = resp.json()
            return data["embeddings"][0]
        except (requests.RequestException, KeyError) as exc:
            log.warning("Single-embed attempt %d/%d failed: %s", attempt, RETRY_MAX, exc)
            if attempt < RETRY_MAX:
                time.sleep(RETRY_DELAY * attempt)
            else:
                raise
    return []


def embed_texts(texts: list[str]) -> list[list[float]]:
    """Embed a batch of texts via Ollama /api/embed endpoint.
    
    Falls back to one-at-a-time embedding if the batch call fails.
    """
    url = f"{OLLAMA_URL}/api/embed"

    # Sanitise: replace empty/whitespace texts with a placeholder
    clean_texts = [t if t.strip() else "<empty>" for t in texts]
    # Truncate to token budget so Ollama doesn't reject for context length
    clean_texts = [_truncate_to_tokens(t) for t in clean_texts]

    payload = {"model": EMBEDDING_MODEL, "input": clean_texts}

    # Try batch first
    try:
        resp = requests.post(url, json=payload, timeout=120)
        if resp.status_code != 200:
            log.warning("Batch embed status=%d body=%s", resp.status_code, resp.text[:300])
        resp.raise_for_status()
        data = resp.json()
        return data["embeddings"]
    except (requests.RequestException, KeyError) as exc:
        log.warning("Batch embed failed (%s), falling back to single-text mode", exc)

    # Fallback: embed one at a time
    embeddings = []
    for i, text in enumerate(clean_texts):
        try:
            emb = _embed_single(url, text)
            embeddings.append(emb)
        except Exception as exc:
            log.error("Failed to embed text %d (len=%d): %s — using zero vector", i, len(text), exc)
            embeddings.append([0.0] * EMBEDDING_DIM)
    return embeddings


# ── Git operations ──────────────────────────────────────────────────
def clone_or_pull(repo_slug: str) -> tuple[git.Repo, str]:
    """Clone or pull a repo. Returns (Repo, head_sha)."""
    repo_dir = WORKSPACE / repo_slug.replace("/", "_")
    clone_url = f"https://x-access-token:{GITHUB_TOKEN}@github.com/{repo_slug}.git" if GITHUB_TOKEN else f"https://github.com/{repo_slug}.git"

    if repo_dir.exists():
        log.info("Pulling %s", repo_slug)
        repo = git.Repo(repo_dir)
        origin = repo.remotes.origin
        # Update URL in case token changed
        origin.set_url(clone_url)
        origin.pull()
    else:
        log.info("Cloning %s", repo_slug)
        repo = git.Repo.clone_from(clone_url, repo_dir)

    head_sha = repo.head.commit.hexsha
    log.info("Repo %s at %s", repo_slug, head_sha[:12])
    return repo, head_sha


def should_include(filepath: str) -> bool:
    """Check if file matches include patterns and doesn't match exclude patterns."""
    # Check excludes first
    for pattern in EXCLUDE_PATTERNS:
        if pattern.endswith("/"):
            if f"/{pattern}" in f"/{filepath}/" or filepath.startswith(pattern):
                return False
        elif fnmatch(Path(filepath).name, pattern):
            return False

    # Check includes
    for pattern in INCLUDE_PATTERNS:
        if fnmatch(Path(filepath).name, pattern):
            return True
    return False


def get_changed_files(repo: git.Repo, last_sha: Optional[str]) -> set[str]:
    """Get files changed since last_sha. If None, return all tracked files."""
    if last_sha is None:
        return {item.path for item in repo.head.commit.tree.traverse() if item.type == "blob"}

    try:
        diff = repo.commit(last_sha).diff(repo.head.commit)
        changed = set()
        for d in diff:
            if d.a_path:
                changed.add(d.a_path)
            if d.b_path:
                changed.add(d.b_path)
        return changed
    except (git.GitCommandError, ValueError):
        log.warning("Could not diff from %s, re-indexing all files", last_sha)
        return {item.path for item in repo.head.commit.tree.traverse() if item.type == "blob"}


# ── Qdrant operations ──────────────────────────────────────────────
def init_qdrant(client: QdrantClient) -> None:
    """Ensure collection exists with correct config."""
    collections = [c.name for c in client.get_collections().collections]
    if QDRANT_COLLECTION not in collections:
        log.info("Creating Qdrant collection: %s", QDRANT_COLLECTION)
        client.create_collection(
            collection_name=QDRANT_COLLECTION,
            vectors_config=VectorParams(size=EMBEDDING_DIM, distance=Distance.COSINE),
        )
    else:
        log.info("Collection %s already exists", QDRANT_COLLECTION)


def get_last_sha(client: QdrantClient, repo_slug: str) -> Optional[str]:
    """Retrieve the last indexed commit SHA for a repo from Qdrant metadata."""
    # We store a special metadata point with id derived from repo name
    meta_id = _repo_meta_id(repo_slug)
    try:
        points = client.retrieve(
            collection_name=QDRANT_COLLECTION,
            ids=[meta_id],
            with_payload=True,
            with_vectors=False,
        )
        if points:
            return points[0].payload.get("commit_sha")
    except Exception:
        pass
    return None


def save_last_sha(client: QdrantClient, repo_slug: str, sha: str) -> None:
    """Store the indexed commit SHA as a metadata point."""
    meta_id = _repo_meta_id(repo_slug)
    # Use a zero vector as placeholder — this point is metadata only
    client.upsert(
        collection_name=QDRANT_COLLECTION,
        points=[
            PointStruct(
                id=meta_id,
                vector=[0.0] * EMBEDDING_DIM,
                payload={
                    "type": "repo_metadata",
                    "repo": repo_slug,
                    "commit_sha": sha,
                },
            )
        ],
    )


def delete_file_points(client: QdrantClient, repo_slug: str, filepath: str) -> None:
    """Delete all points for a specific file before re-indexing."""
    from qdrant_client.models import Filter, FieldCondition, MatchValue

    client.delete(
        collection_name=QDRANT_COLLECTION,
        points_selector=Filter(
            must=[
                FieldCondition(key="repo", match=MatchValue(value=repo_slug)),
                FieldCondition(key="filepath", match=MatchValue(value=filepath)),
            ]
        ),
    )


def _repo_meta_id(repo_slug: str) -> str:
    """Deterministic UUID-like ID for repo metadata points."""
    return hashlib.md5(f"meta:{repo_slug}".encode()).hexdigest()


def _chunk_id(repo_slug: str, filepath: str, idx: int) -> str:
    """Deterministic ID for a chunk."""
    return hashlib.md5(f"{repo_slug}:{filepath}:{idx}".encode()).hexdigest()


# ── Main pipeline ───────────────────────────────────────────────────
def index_repo(client: QdrantClient, repo_slug: str) -> None:
    """Full indexing pipeline for one repo."""
    repo, head_sha = clone_or_pull(repo_slug)
    last_sha = get_last_sha(client, repo_slug)

    if last_sha == head_sha:
        log.info("Repo %s unchanged at %s, skipping", repo_slug, head_sha[:12])
        return

    changed_files = get_changed_files(repo, last_sha)
    eligible_files = {f for f in changed_files if should_include(f)}

    log.info(
        "Repo %s: %d changed files, %d eligible for indexing",
        repo_slug,
        len(changed_files),
        len(eligible_files),
    )

    if not eligible_files:
        save_last_sha(client, repo_slug, head_sha)
        return

    repo_dir = WORKSPACE / repo_slug.replace("/", "_")
    total_chunks = 0
    batch_size = 20  # Embed N chunks at a time

    all_points: list[PointStruct] = []
    all_texts: list[str] = []
    all_metadata: list[dict] = []

    for filepath in sorted(eligible_files):
        full_path = repo_dir / filepath
        if not full_path.exists() or not full_path.is_file():
            continue

        try:
            text = full_path.read_text(encoding="utf-8", errors="replace")
        except Exception as exc:
            log.warning("Could not read %s: %s", filepath, exc)
            continue

        if not text.strip():
            continue

        # Delete old points for this file
        delete_file_points(client, repo_slug, filepath)

        # Chunk the file
        chunks = classify_and_chunk(text, filepath)

        for idx, chunk in enumerate(chunks):
            chunk_text = chunk.pop("text", "")
            if not chunk_text.strip():
                continue

            # Build metadata
            metadata = {
                "repo": repo_slug,
                "branch": "main",
                "filepath": filepath,
                "commit_sha": head_sha,
                "chunk_index": idx,
                **{k: v for k, v in chunk.items() if v},
            }

            # Prefix text with metadata for better retrieval
            prefix = f"repo: {repo_slug} | file: {filepath}"
            if chunk.get("kind"):
                prefix += f" | kind: {chunk['kind']}"
            if chunk.get("name"):
                prefix += f" | name: {chunk['name']}"
            if chunk.get("namespace"):
                prefix += f" | namespace: {chunk['namespace']}"
            search_text = f"{prefix}\n\n{chunk_text}"

            all_texts.append(search_text)
            all_metadata.append(metadata)
            total_chunks += 1

    # Embed and upsert in batches
    log.info("Embedding %d chunks for %s", total_chunks, repo_slug)

    for i in range(0, len(all_texts), batch_size):
        batch_texts = all_texts[i : i + batch_size]
        batch_meta = all_metadata[i : i + batch_size]

        embeddings = embed_texts(batch_texts)

        points = []
        for j, (emb, meta) in enumerate(zip(embeddings, batch_meta)):
            point_id = _chunk_id(repo_slug, meta["filepath"], meta["chunk_index"])
            points.append(
                PointStruct(id=point_id, vector=emb, payload={**meta, "text": batch_texts[j]})
            )

        client.upsert(collection_name=QDRANT_COLLECTION, points=points)
        log.info("Upserted batch %d-%d / %d", i, i + len(batch_texts), total_chunks)

    save_last_sha(client, repo_slug, head_sha)
    log.info("Finished indexing %s: %d chunks indexed", repo_slug, total_chunks)


def main() -> None:
    if not REPOS:
        log.error("No repos configured (REPOS env var is empty)")
        sys.exit(1)

    log.info("Starting git-indexer")
    log.info("Ollama: %s | Qdrant: %s | Model: %s", OLLAMA_URL, QDRANT_URL, EMBEDDING_MODEL)
    log.info("Repos: %s", ", ".join(REPOS))

    # Verify Ollama connectivity
    try:
        resp = requests.get(f"{OLLAMA_URL}/api/tags", timeout=10)
        resp.raise_for_status()
        models = [m["name"] for m in resp.json().get("models", [])]
        log.info("Ollama models available: %s", ", ".join(models))
        if EMBEDDING_MODEL not in models and f"{EMBEDDING_MODEL}:latest" not in models:
            log.warning("Embedding model %s not found in Ollama, will attempt to pull on first embed", EMBEDDING_MODEL)
    except requests.RequestException as exc:
        log.error("Cannot reach Ollama at %s: %s", OLLAMA_URL, exc)
        sys.exit(1)

    # Connect to Qdrant
    client = QdrantClient(url=QDRANT_URL, timeout=30)
    init_qdrant(client)

    # Index each repo
    WORKSPACE.mkdir(parents=True, exist_ok=True)
    errors = []

    for repo_slug in REPOS:
        try:
            index_repo(client, repo_slug)
        except Exception as exc:
            log.error("Failed to index %s: %s", repo_slug, exc, exc_info=True)
            errors.append(repo_slug)

    if errors:
        log.error("Indexing failed for: %s", ", ".join(errors))
        sys.exit(1)

    log.info("All repos indexed successfully")


if __name__ == "__main__":
    main()
