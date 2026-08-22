---
type: Platform Capability
title: AI stack — Open WebUI, Qdrant, MCP servers, (retired) llmkube & git-indexer
description: The self-hosted AI assistant layer over the platform — chat UI, vector store, MCP tool servers behind one gateway host, and the parts that have been retired.
tags: [ai, open-webui, qdrant, mcp, rag, llm]
status: stable
generated: { by: claude-code/claude-opus-5, at: 2026-08-21T18:00:00Z }
stale_after: 2026-11-21
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
- **Open WebUI v0.11.0** (chart 16.0.0) — chat UI at
  `chat.cluster.integratn.tech`, OIDC via Authentik, configured for an
  external Ollama at `10.0.3.4:11434`.
- **Qdrant v1.17.0** — vector DB at `qdrant.cluster.integratn.tech`
  (HTTPRoutes with SnippetsFilter auth).

**`mcp-system` namespace** — six Deployments exposing MCP tool servers,
path-routed on `mcp.cluster.integratn.tech`:[^mcp-dir]

| Path | Server | Image |
|---|---|---|
| `/kubernetes` | kubernetes-mcp (read-only RBAC) | quay.io/containers/kubernetes_mcp_server |
| `/argocd` | mcp-for-argocd v0.9.0 | ghcr.io/argoproj-labs |
| `/github` | github-mcp-server | ghcr.io/github |
| `/grafana` | mcp-grafana (token minted by a PostSync Job) | grafana/mcp-grafana |
| `/sequential-thinking` | supergateway wrapping the stdio server (npm pre-install initContainer) | ghcr.io/supercorp-ai/supergateway |
| — | mcpo (MCP→OpenAPI bridge for Open WebUI; also wires prometheus + fetch) | ghcr.io/open-webui/mcpo |

Recent git history is dominated by hardening this stack (routing rewrites,
egress policies, securityContexts, supergateway transport fixes).[^git-log]

Deployment mechanism: the stack is now the `mcp-system` addon
(`type: manifest`, control-plane only) declared in
`addons/cluster-roles/control-plane/addons/addons.yaml`, so a cluster rebuild
recreates it from git. The out-of-band `mcp-system-app` Application it replaced
has been retired. Every image is digest-pinned after an unpinned `:latest`
carried grafana-mcp 0.14.0 → 1.1.0 on an unrelated rollout, and grafana-mcp's
liveness probe is a `tcpSocket` check — pointing it at `/healthz` made a stale
Grafana token restart-loop the pod. Full reference:
[docs/mcp.md](../../mcp.md).

That single unpinned-tag jump has now caused **three** distinct failures, the
last of which was misdiagnosed twice:

1. `/healthz` in 1.1.0 rejects an unauthenticated Grafana connection where
   0.14.0 tolerated it — a stale token became a CrashLoop (liveness fix).
2. The token Job could not repair the token because it validated only the
   presence of a key, not its acceptance (PreSync + `Force=true,Replace=true`).
3. **1.1.0 added Host-header validation**, and `--allowed-hosts` defaults to
   "loopback variants of `--address`" — for `0.0.0.0:8000` that is exactly
   `localhost:8000` and `127.0.0.1:8000`. Every real caller got
   `403 forbidden: host not allowed`: the kubelet probe (Host is the pod IP),
   mcpo (the Service FQDN), and anything arriving through the Gateway. This was
   predicted to be a Grafana *authorization* problem — Viewer vs Admin on
   `GET /api/datasources` — and it was not; Grafana was never in the path, and
   escalating the service account would have changed nothing. Fixed by
   allowlisting the real callers and giving the readiness probe an explicit
   `Host` header, because pod IPs are per-pod and cannot be allowlisted.

The general lesson is narrower than "pin images": a pinned *major* jump still
arrives with new default-deny security behaviour, and a 403 is worth reading the
body of before assuming which system produced it.

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
