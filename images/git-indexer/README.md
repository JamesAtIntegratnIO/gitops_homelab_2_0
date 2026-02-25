# Git Indexer

Clones configured GitHub repos, chunks files structurally (Markdown by headings,
Kubernetes YAML by document, Helm values by top-level keys), generates embeddings
via an external Ollama instance, and upserts vectors + metadata into Qdrant.

## Build & Push

```bash
docker build -t ghcr.io/jamesatintegratnio/git-indexer:latest .
docker push ghcr.io/jamesatintegratnio/git-indexer:latest
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `OLLAMA_URL` | Ollama API base URL | `http://10.0.3.4:11434` |
| `QDRANT_URL` | Qdrant HTTP endpoint | `http://qdrant.ai.svc.cluster.local:6333` |
| `QDRANT_COLLECTION` | Qdrant collection name | `homelab-platform` |
| `EMBEDDING_MODEL` | Ollama embedding model | `nomic-embed-text` |
| `REPOS` | Comma-separated `owner/repo` list | — |
| `GITHUB_TOKEN` | GitHub PAT for private repos | — |
| `INCLUDE_PATTERNS` | Comma-separated file globs | `*.yaml,*.yml,*.md,*.json,*.tf,*.sh` |
| `EXCLUDE_PATTERNS` | Comma-separated dir/file patterns to skip | `vendor/,node_modules/,*.lock,.git/` |
| `CHUNK_MAX_TOKENS` | Max tokens per chunk | `512` |

## How It Works

1. Clones each repo (or pulls if already cached in `/workspace`)
2. Walks all files matching include patterns, skipping excludes
3. **Redaction**: Strips `kind: Secret` documents, pattern-redacts tokens/passwords/keys
4. **Structural chunking**:
   - **Markdown**: Splits by H1/H2 headings; code blocks stay with their heading
   - **Kubernetes YAML**: Splits by `---` document separator; each doc is a chunk with kind/name/namespace metadata
   - **Helm values**: Splits by top-level YAML keys
   - **Other files**: Whole file if ≤ max tokens, else split by lines
5. Embeds each chunk via Ollama `/api/embed`
6. Upserts to Qdrant with metadata: `repo`, `branch`, `filepath`, `kind`, `name`, `namespace`, `chunk_type`, `commit_sha`
7. Incremental: tracks commit SHAs and only re-indexes changed files
