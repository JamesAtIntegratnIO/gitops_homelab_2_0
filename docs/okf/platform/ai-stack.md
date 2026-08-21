---
type: Platform Capability
title: AI stack — Open WebUI, Qdrant, MCP servers, (retired) llmkube & git-indexer
description: The self-hosted AI assistant layer over the platform — chat UI, vector store, MCP tool servers behind one gateway host, and the parts that have been retired.
tags: [ai, open-webui, qdrant, mcp, rag, llm]
status: stable
generated: { by: claude-code/claude-fable-5, at: 2026-08-20T23:40:00Z }
stale_after: 2026-11-20
sources:
  - id: mcp-dir
    resource: ../../../addons/cluster-roles/control-plane/addons/mcp-system/
    title: mcp-system manifests
  - id: live-ai
    resource: kubectl get all -n ai,mcp-system,llmkube-system on the-cluster, 2026-08-20
    title: Live AI namespaces
  - id: git-log
    resource: git log (mcp/llmkube commits, ce6ce24, 604c506)
    title: Recent git history
---

# Active components

**`ai` namespace**
- **Open WebUI v0.8.6** (chart 12.5.0) — chat UI at
  `chat.cluster.integratn.tech`, OIDC via Authentik, configured for an
  external Ollama at `10.0.3.4:11434`.
- **Qdrant v1.17.0** — vector DB at `qdrant.cluster.integratn.tech`
  (HTTPRoutes with SnippetsFilter auth).

**`mcp-system` namespace** — six Deployments exposing MCP tool servers,
path-routed on `mcp.cluster.integratn.tech`:[^mcp-dir]

| Path | Server | Image |
|---|---|---|
| `/kubernetes` | kubernetes-mcp (read-only RBAC) | quay.io/containers/kubernetes_mcp_server |
| `/argocd` | mcp-for-argocd v0.5.0 | ghcr.io/argoproj-labs |
| `/github` | github-mcp-server | ghcr.io/github |
| `/grafana` | mcp-grafana (token minted by a PostSync Job) | grafana/mcp-grafana |
| `/sequential-thinking` | supergateway wrapping the stdio server (npm pre-install initContainer) | ghcr.io/supercorp-ai/supergateway |
| — | mcpo (MCP→OpenAPI bridge for Open WebUI; also wires prometheus + fetch) | ghcr.io/open-webui/mcpo |

Recent git history is dominated by hardening this stack (routing rewrites,
egress policies, securityContexts, supergateway transport fixes).[^git-log]

⚠️ Deployment mechanism: `mcp-system-app.yaml` is a plain ArgoCD Application
manifest that **no addon or kustomization applies** — it was applied
out-of-band and self-manages from there. See
[known issues](/cluster/known-issues.md).

# Retired components (with live remnants)

- **git-indexer** — a Python RAG indexer (still in `images/git-indexer/`;
  chunks repo content structurally, redacts secrets, embeds via Ollama into
  Qdrant; `rag_pipeline.py`/`rag_tool.py` are Open WebUI plugins). Removed
  from the deployment in commit `ce6ce24` ("github-mcp provides real-time repo
  access via API") — but its hourly CronJob **still exists in the `ai`
  namespace and fails every run** (BackoffLimitExceeded). `hctl ai reindex`
  targets this CronJob.
- **llmkube** — local LLM inference operator (llama.cpp-based,
  `llm.cluster.integratn.tech`). The addon was disabled in commit `604c506`,
  but the operator, InferenceService/Model CRs (llama-3.1-8b), a 0/1
  Deployment, and two 50Gi model-cache PVCs remain running/allocated with no
  ArgoCD app managing them.[^live-ai]

Both remnants are catalogued in [known issues](/cluster/known-issues.md).

[^mcp-dir]: mcp-system manifests
[^live-ai]: Live AI namespaces
[^git-log]: Recent git history
