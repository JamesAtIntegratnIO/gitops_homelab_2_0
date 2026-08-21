# MCP Servers

Model Context Protocol servers let LLM agents call real tools — query the
cluster, read Grafana, sync an ArgoCD app — instead of guessing. This platform
runs a set of them in-cluster and exposes them on one hostname.

Two unrelated things in this repo share the name "MCP". Know which one you are
looking at:

| | Where | Who consumes it |
|---|---|---|
| **In-cluster MCP servers** | `mcp-system` namespace, `addons/cluster-roles/control-plane/addons/mcp-system/` | Open WebUI (via mcpo), external MCP clients, agents |
| **Editor-side MCP config** | [.vscode/mcp.json](../.vscode/mcp.json), [.continue/mcpServers/](../.continue/mcpServers/) | VS Code / Copilot, Continue |

Claude Code working in this repo needs neither — use `kubectl` and `argocd` from
the Nix flake directly.

## In-cluster: the `mcp-system` namespace

### Deployment

Managed as the `mcp-system` addon (`type: manifest`) declared in
[addons/cluster-roles/control-plane/addons/addons.yaml](../addons/cluster-roles/control-plane/addons/addons.yaml),
rendering the raw manifests in
[addons/cluster-roles/control-plane/addons/mcp-system/](../addons/cluster-roles/control-plane/addons/mcp-system/).

It was originally applied out-of-band as a standalone `mcp-system-app`
Application. That is gone — the addon owns it now, so a cluster rebuild
recreates it from git.

### Servers

All routed under `https://mcp.cluster.integratn.tech`, path prefix stripped by
an `URLRewrite` filter so each backend sees its native `/mcp` path.

| Path | Deployment | Image | Port | Notes |
|---|---|---|---|---|
| `/kubernetes` | `kubernetes-mcp` | `quay.io/containers/kubernetes_mcp_server` | 8080 | Read-only `kubernetes-mcp-viewer` ClusterRole |
| `/argocd` | `argocd-mcp` | `ghcr.io/argoproj-labs/mcp-for-argocd:v0.5.0` | 3000 | Token from 1Password |
| `/github` | `github-mcp` | `ghcr.io/github/github-mcp-server` | 8082 | PAT from 1Password |
| `/grafana` | `grafana-mcp` | `docker.io/grafana/mcp-grafana:1.1.0` | 8000 | Token minted by a PostSync Job |
| `/sequential-thinking` | `sequential-thinking` | `ghcr.io/supercorp-ai/supergateway` | 8000 | Wraps the stdio server; npm pre-install initContainer |
| — | `mcpo` | `ghcr.io/open-webui/mcpo:main` | 8000 | MCP→OpenAPI bridge, cluster-internal only |

Every image is **digest-pinned**. It was not always: an unpinned `:latest`
silently carried grafana-mcp from 0.14.0 to 1.1.0 on an unrelated rollout. Pin
new servers the same way.

Transport is **streamable HTTP** (`/mcp`), not SSE. Older docs and the disabled
`mcp-*` addons under `addons/clusters/the-cluster/` describe an SSE layout with
`acuvity/*` images — that generation is retired.

### mcpo — the Open WebUI bridge

Open WebUI speaks OpenAPI "External Tools", not MCP. `mcpo` translates. Its
server list lives in
[mcpo-configmap.yaml](../addons/cluster-roles/control-plane/addons/mcp-system/mcpo-configmap.yaml)
and covers the in-cluster servers above plus two stdio subprocesses it runs
itself via `uvx`:

- `prometheus` — `prometheus-mcp-server` against
  `kube-prometheus-stack-prometheus.monitoring:9090`
- `fetch` — `mcp-server-fetch`, pages as markdown

An initContainer renders `${GITHUB_TOKEN}` into the config from the
ExternalSecret before mcpo starts.

The matching Open WebUI system prompt — the tool-first mandate that stops the
model asking questions it could look up — is checked in at
[system-prompt.md](../addons/cluster-roles/control-plane/addons/mcp-system/system-prompt.md).
It is applied by hand in the Open WebUI admin panel; it is not reconciled.

### Secrets

Nothing here holds a literal secret. All three come from 1Password via
ExternalSecrets against the `onepassword-store` ClusterSecretStore:

| Kubernetes secret | 1Password item | Used by |
|---|---|---|
| `mcp-github-token` | `mcp-github-token` / `GITHUB_TOKEN` | github-mcp, mcpo |
| `argocd-mcp-secrets` | see [externalsecret-argocd-mcp.yaml](../addons/cluster-roles/control-plane/addons/mcp-system/externalsecret-argocd-mcp.yaml) | argocd-mcp |
| `grafana-mcp-secrets` | minted in-cluster by `create-grafana-mcp-sa-token` (PostSync Job) | grafana-mcp |

The Grafana token is the odd one: rather than a hand-maintained 1Password item,
a PostSync Job creates a Grafana service account and writes its token into the
secret, so it survives a Grafana rebuild.

### Networking

`mcp-system` is **excluded from the Kyverno `generate-default-deny-netpol`
policy**, so its baseline `default-deny-all` is managed explicitly in
[network-policies/mcp-system.yaml](../addons/cluster-roles/control-plane/addons/network-policies/mcp-system.yaml).
Everything is deny-by-default plus narrow allows:

- DNS to kube-dns, kube-apiserver via a CiliumNetworkPolicy
- monitoring namespace (9090/80/443/3000) for grafana-mcp and prometheus
- argocd namespace, `app: argocd-mcp` only
- public internet on 443, RFC1918 excluded, for `github-mcp` (GitHub API) and
  `sequential-thinking` (npm)
- ingress from `nginx-gateway` on 8080/8082/8000/3000
- intra-namespace both ways, so mcpo can reach the backends

Adding a server that talks to something new means adding an egress rule. It will
fail silently — as a hang, not an error — if you forget.

### Hardening

Non-root, read-only root filesystem, all capabilities dropped,
`seccompProfile: RuntimeDefault`, resource requests/limits set.
`priorityClassName: platform-batch` — this stack is developer tooling and is
deliberately evictable ahead of platform components.

Readiness probes *should* track dependencies; liveness probes must **not**.
grafana-mcp's liveness pointed at `/healthz`, which reports non-200 when the
Grafana connection is unauthorized — so a stale service-account token put the
container into CrashLoopBackOff with 11 restarts when the correct behaviour was
to sit there not-ready until the token was repaired. Liveness is now a
`tcpSocket` check: "is this process wedged", nothing more.

### Adding a server

1. Add a Deployment + Service to
   `addons/cluster-roles/control-plane/addons/mcp-system/`, digest-pinned,
   with the standard securityContext block (copy an existing one).
2. Add an HTTPRoute stanza to `httproutes.yaml` with a `ReplacePrefixMatch: "/"`
   URLRewrite.
3. Add any egress the server needs to `network-policies/mcp-system.yaml`, and
   its port to `allow-gateway-ingress`.
4. Credentials → an ExternalSecret, never a literal.
5. To surface it in Open WebUI, add it to `mcpo-configmap.yaml`.
6. Commit, push, merge to `main` — ArgoCD does the rest.

### Connecting a client

Any MCP client that speaks streamable HTTP:

```
https://mcp.cluster.integratn.tech/kubernetes/mcp
https://mcp.cluster.integratn.tech/argocd/mcp
https://mcp.cluster.integratn.tech/github/mcp
https://mcp.cluster.integratn.tech/grafana/mcp
https://mcp.cluster.integratn.tech/sequential-thinking/mcp
```

For Claude Code, from an interactive terminal:

```bash
claude mcp add --transport http homelab-k8s https://mcp.cluster.integratn.tech/kubernetes/mcp
```

The endpoints are gateway-exposed with TLS but have **no authentication in front
of them**. They are reachable by anything that can resolve the hostname. The
kubernetes server is read-only by RBAC; github and argocd are not.

### Troubleshooting

```bash
kubectl -n mcp-system get pods,svc,httproute
kubectl -n mcp-system logs deploy/mcpo -c mcpo
kubectl -n mcp-system logs deploy/grafana-mcp
kubectl -n argocd get application mcp-system
```

| Symptom | Look at |
|---|---|
| Client connects, tools list empty | Wrong transport — it is streamable HTTP on `/mcp`, not SSE on `/sse` |
| Request hangs, no error | Missing egress NetworkPolicy |
| 404 through the gateway | HTTPRoute prefix vs the `ReplacePrefixMatch` rewrite |
| Pod restart-looping | A probe pointing at a dependency that is down |
| mcpo missing one server | `mcpo-configmap.yaml`, then restart the Deployment — the config is rendered at init |

## Editor-side MCP config

[.vscode/mcp.json](../.vscode/mcp.json) wires kubernetes, github,
memory, and sequential-thinking servers into VS Code / Copilot Chat, launched
through `nix develop` with Node packages from `node_modules/`.

**It is stale.** The paths are hard-coded to
`/home/boboysdadda/projects/gitops_homelab_2_0` from the original Linux
workstation and do not resolve on macOS. `package.json` and `node_modules/` are
gitignored, so a fresh clone has no Node packages either. Treat it as a
reference for the auto-approve policy in [.vscode/settings.json](../.vscode/settings.json)
— which does encode a useful read-only-by-default stance — rather than something
that works out of the box.

[.continue/mcpServers/new-mcp-server.yaml](../.continue/mcpServers/new-mcp-server.yaml)
is an empty scaffold.

## See also

- [docs/okf/platform/ai-stack.md](okf/platform/ai-stack.md) — the AI layer as a
  whole: Open WebUI, Qdrant, MCP, and the retired llmkube/git-indexer
- [docs/okf/cluster/known-issues.md](okf/cluster/known-issues.md) — current drift
- [CLAUDE.md](../CLAUDE.md) — agent operating manual for this repo
