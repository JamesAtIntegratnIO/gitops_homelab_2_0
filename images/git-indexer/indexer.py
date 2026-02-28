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
        "INCLUDE_PATTERNS", "*.yaml,*.yml,*.md,*.json,*.tf,*.sh,*.mmd,*.go,*.py,Makefile,Dockerfile"
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
# nomic-embed-text uses a BERT tokenizer with context_length=2048.
# Ollama's `truncate` param has a known bug (uses num_ctx=8192 as threshold
# instead of context_length=2048) so inputs between 2048-8192 tokens slip
# through and error.  We enforce a conservative character limit client-side.
# Dense content (Grafana JSON, YAML with UUIDs) can be ~3 chars/token,
# so 2048 * 2.5 ≈ 5120.  We use 5000 for safety.
EMBED_MAX_CHARS = int(os.environ.get("EMBED_MAX_CHARS", "5000"))
RETRY_MAX = 3
RETRY_DELAY = 5

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(message)s",
    datefmt="%Y-%m-%d %H:%M:%S",
)
log = logging.getLogger("git-indexer")


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


# ── Token counting (approximate — ~4 chars per token for English text) ──
def count_tokens(text: str) -> int:
    """Approximate token count. Used only for chunking, not embedding."""
    return len(text) // 4


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


# ── Go-aware chunking ──────────────────────────────────────────────
# Regex matching top-level Go declarations: func, type, var, const.
# Handles doc comments attached above declarations.
_GO_DECL_RE = re.compile(
    r"^(//[^\n]*\n)*"           # optional doc comment lines
    r"(func |type |var |const )",  # declaration keyword at line start
    re.MULTILINE,
)


def _extract_go_package(text: str) -> str:
    """Extract the Go package name from source."""
    m = re.search(r"^package\s+(\w+)", text, re.MULTILINE)
    return m.group(1) if m else ""


def _go_symbol_name(decl_text: str) -> tuple[str, str]:
    """Extract symbol name and type (function/method/type/var/const) from a Go declaration."""
    # Strip leading doc comments
    stripped = re.sub(r"^(//[^\n]*\n)+", "", decl_text).lstrip()

    # func (receiver) Name(...)
    m = re.match(r"func\s+\([^)]+\)\s+(\w+)", stripped)
    if m:
        return m.group(1), "method"

    # func Name(...)
    m = re.match(r"func\s+(\w+)", stripped)
    if m:
        return m.group(1), "function"

    # type Name struct/interface/...
    m = re.match(r"type\s+(\w+)", stripped)
    if m:
        return m.group(1), "type"

    # var Name or var (
    m = re.match(r"var\s+(\w+)", stripped)
    if m:
        return m.group(1), "var"

    # const Name or const (
    m = re.match(r"const\s+(\w+)", stripped)
    if m:
        return m.group(1), "const"

    return "", "unknown"


def chunk_go(text: str, filepath: str) -> list[dict]:
    """Split a Go file into structural chunks: package/imports + each declaration.

    Each chunk gets metadata: language, package, symbol, symbol_type.
    This makes Go functions/types individually searchable in the vector DB.
    Only splits at brace-depth 0 to avoid matching declarations inside function bodies.
    """
    package_name = _extract_go_package(text)
    chunks: list[dict] = []

    # Find top-level declaration boundaries by tracking brace depth.
    # Only declarations at depth 0 are real top-level declarations.
    boundaries: list[int] = []
    brace_depth = 0
    in_string = False
    in_raw_string = False
    in_line_comment = False
    in_block_comment = False
    i = 0
    line_start = 0

    while i < len(text):
        ch = text[i]

        # Track newlines for line-comment reset
        if ch == '\n':
            in_line_comment = False
            line_start = i + 1
            i += 1
            continue

        # Skip comments
        if in_line_comment:
            i += 1
            continue
        if in_block_comment:
            if ch == '*' and i + 1 < len(text) and text[i + 1] == '/':
                in_block_comment = False
                i += 2
            else:
                i += 1
            continue

        # Detect comment starts
        if ch == '/' and i + 1 < len(text):
            if text[i + 1] == '/':
                in_line_comment = True
                i += 2
                continue
            if text[i + 1] == '*':
                in_block_comment = True
                i += 2
                continue

        # Skip strings
        if in_string:
            if ch == '\\' and i + 1 < len(text):
                i += 2  # skip escaped char
                continue
            if ch == '"':
                in_string = False
            i += 1
            continue
        if in_raw_string:
            if ch == '`':
                in_raw_string = False
            i += 1
            continue
        if ch == '"':
            in_string = True
            i += 1
            continue
        if ch == '`':
            in_raw_string = True
            i += 1
            continue

        # Track braces
        if ch == '{':
            brace_depth += 1
        elif ch == '}':
            brace_depth = max(0, brace_depth - 1)

        # At brace depth 0 and start of line, check for declaration keywords
        if brace_depth == 0 and i == line_start:
            rest = text[i:]
            # Walk back to include doc comments
            decl_start = i
            j = i - 1
            while j >= 0:
                # Find start of previous line
                prev_newline = text.rfind('\n', 0, j)
                prev_line = text[prev_newline + 1: j + 1].strip() if prev_newline >= 0 else text[: j + 1].strip()
                if prev_line.startswith('//'):
                    decl_start = prev_newline + 1 if prev_newline >= 0 else 0
                    j = prev_newline - 1 if prev_newline >= 0 else -1
                else:
                    break

            if re.match(r"(func |type |var |const )", rest):
                boundaries.append(decl_start)

        i += 1

    if not boundaries:
        # No declarations found (unlikely for valid Go) — return whole file
        return [{
            "text": text,
            "language": "go",
            "package": package_name,
            "symbol": filepath,
            "symbol_type": "file",
        }]

    # First chunk: everything before the first declaration (package + imports)
    preamble = text[: boundaries[0]].strip()
    if preamble:
        # Extract imported packages as summary
        imports = re.findall(r'"([^"]+)"', preamble)
        import_summary = ", ".join(imports[:20])
        if len(imports) > 20:
            import_summary += f" (+{len(imports) - 20} more)"

        # Build a file overview listing all exported symbols
        symbols = []
        for i, b in enumerate(boundaries):
            end = boundaries[i + 1] if i + 1 < len(boundaries) else len(text)
            decl_text = text[b:end]
            sym_name, sym_type = _go_symbol_name(decl_text)
            if sym_name and sym_name[0].isupper():  # exported
                symbols.append(f"{sym_type} {sym_name}")

        overview_lines = [preamble, ""]
        if symbols:
            overview_lines.append(f"// Exported symbols: {', '.join(symbols)}")

        chunks.append({
            "text": "\n".join(overview_lines),
            "language": "go",
            "package": package_name,
            "symbol": filepath,
            "symbol_type": "file_overview",
            "imports": import_summary,
        })

    # Each declaration becomes its own chunk
    for i, b in enumerate(boundaries):
        end = boundaries[i + 1] if i + 1 < len(boundaries) else len(text)
        decl_text = text[b:end].rstrip()
        sym_name, sym_type = _go_symbol_name(decl_text)

        chunks.append({
            "text": decl_text,
            "language": "go",
            "package": package_name,
            "symbol": sym_name or f"decl_{i}",
            "symbol_type": sym_type,
        })

    return chunks


# ── Python-aware chunking ──────────────────────────────────────────
_PY_DECL_RE = re.compile(
    r"^((?:#[^\n]*\n|\"\"\"[\s\S]*?\"\"\"\n)*)"  # optional comments/docstrings
    r"((?:class |def |async def ))",               # declaration keyword
    re.MULTILINE,
)


def _extract_py_symbol(decl_text: str) -> tuple[str, str]:
    """Extract symbol name and type from a Python declaration."""
    stripped = re.sub(r"^(#[^\n]*\n)+", "", decl_text).lstrip()
    stripped = re.sub(r'^"""[\s\S]*?"""\n', "", stripped).lstrip()

    m = re.match(r"class\s+(\w+)", stripped)
    if m:
        return m.group(1), "class"

    m = re.match(r"(?:async\s+)?def\s+(\w+)", stripped)
    if m:
        return m.group(1), "function"

    return "", "unknown"


def chunk_python(text: str, filepath: str) -> list[dict]:
    """Split a Python file by top-level class/function definitions."""
    chunks: list[dict] = []

    # Find top-level declarations (no leading whitespace)
    boundaries: list[int] = []
    for m in re.finditer(r"^((?:#[^\n]*\n)*)(?:class |def |async def )", text, re.MULTILINE):
        # Only top-level (check that the match position is at column 0 or preceded by comments at column 0)
        line_start = text.rfind("\n", 0, m.start()) + 1
        leading = text[line_start:m.start()]
        if leading.strip() == "" or leading.startswith("#"):
            boundaries.append(m.start())

    if not boundaries:
        return [{
            "text": text,
            "language": "python",
            "symbol": filepath,
            "symbol_type": "file",
        }]

    # Preamble
    preamble = text[: boundaries[0]].strip()
    if preamble:
        chunks.append({
            "text": preamble,
            "language": "python",
            "symbol": filepath,
            "symbol_type": "file_overview",
        })

    for i, b in enumerate(boundaries):
        end = boundaries[i + 1] if i + 1 < len(boundaries) else len(text)
        decl_text = text[b:end].rstrip()
        sym_name, sym_type = _extract_py_symbol(decl_text)
        chunks.append({
            "text": decl_text,
            "language": "python",
            "symbol": sym_name or f"decl_{i}",
            "symbol_type": sym_type,
        })

    return chunks


def classify_and_chunk(text: str, filepath: str) -> list[dict]:
    """Determine file type and dispatch to appropriate chunker."""
    ext = Path(filepath).suffix.lower()
    name = Path(filepath).name.lower()

    if ext in (".md", ".mmd"):
        raw_chunks = chunk_markdown(text, filepath)
        tagged = [{"chunk_type": "markdown", **c} for c in raw_chunks]
    elif ext == ".go":
        raw_chunks = chunk_go(text, filepath)
        tagged = [{"chunk_type": "go-code", **c} for c in raw_chunks]
    elif ext == ".py":
        raw_chunks = chunk_python(text, filepath)
        tagged = [{"chunk_type": "python-code", **c} for c in raw_chunks]
    elif ext in (".yaml", ".yml"):
        # Is this a values file (Helm)?
        if "values" in name or name == "chart.yaml":
            raw_chunks = chunk_helm_values(text, filepath)
            tagged = [{"chunk_type": "helm-values", **c} for c in raw_chunks]
        else:
            # Otherwise treat as Kubernetes YAML
            raw_chunks = chunk_k8s_yaml(text, filepath)
            tagged = [{"chunk_type": "k8s-yaml", **c} for c in raw_chunks]
    else:
        # Everything else (tf, sh, json, etc.)
        raw_chunks = chunk_generic(text, filepath)
        tagged = [{"chunk_type": "generic", **c} for c in raw_chunks]

    # ── Enforce EMBED_MAX_CHARS: sub-chunk any oversized chunks ──
    final: list[dict] = []
    for chunk in tagged:
        txt = chunk.get("text", "")
        if len(txt) <= EMBED_MAX_CHARS:
            final.append(chunk)
            continue
        # Sub-chunk by lines, staying under EMBED_MAX_CHARS
        sub_parts: list[str] = []
        current_lines: list[str] = []
        current_len = 0
        for line in txt.splitlines(keepends=True):
            if current_len + len(line) > EMBED_MAX_CHARS and current_lines:
                sub_parts.append("".join(current_lines))
                current_lines = []
                current_len = 0
            current_lines.append(line)
            current_len += len(line)
        if current_lines:
            sub_parts.append("".join(current_lines))
        for idx, part in enumerate(sub_parts):
            sub = dict(chunk)  # shallow copy all metadata
            sub["text"] = part
            if len(sub_parts) > 1:
                sub["sub_chunk"] = f"{idx + 1}/{len(sub_parts)}"
            final.append(sub)
        if len(sub_parts) > 1:
            log.info("Sub-chunked oversized chunk (%d chars) into %d parts in %s",
                     len(txt), len(sub_parts), filepath)
    return final


# ── Embedding ───────────────────────────────────────────────────────
# nomic-embed-text BERT context_length is 2048 tokens but Ollama sets
# num_ctx=8192.  The `truncate` param uses num_ctx as its threshold, so
# inputs between 2048 and 8192 tokens slip through and error out.
# We enforce EMBED_MAX_CHARS in the chunker to prevent this, and still
# pass truncate=True as a last-resort safety net.


def _embed_single(url: str, text: str) -> list[float]:
    """Embed a single text string, retrying on failure."""
    payload = {"model": EMBEDDING_MODEL, "input": text, "truncate": True}
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

    payload = {"model": EMBEDDING_MODEL, "input": clean_texts, "truncate": True}

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
