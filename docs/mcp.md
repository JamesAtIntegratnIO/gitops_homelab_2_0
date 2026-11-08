# MCP Servers

Model Context Protocol servers let LLM agents call real tools — query the
cluster, read Grafana, sync an ArgoCD app — instead of guessing. This platform
runs a set of them in-cluster and exposes them on one hostname.

Two unrelated things in this repo share the name "MCP". Know which one you are
looking at:

| | Where | Who consumes it |
|---|---|---|
| **In-cluster MCP servers** | `mcp-system` namespace, `addons/cluster-roles/control-plane/addons/mcp-system/` | External MCP clients, agents |
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
| `/kubernetes` | `kubernetes-mcp` | `quay.io/containers/kubernetes_mcp_server` | 8080 | Read-only `kubernetes-mcp-viewer` ClusterRole; settings in ConfigMap `kubernetes-mcp-config`, because the server takes no runtime flags since v0.0.68 |
| `/argocd` | `argocd-mcp` | `ghcr.io/argoproj-labs/mcp-for-argocd` | 3000 | Token from 1Password |
| `/github` | `github-mcp` | `ghcr.io/github/github-mcp-server` | 8082 | PAT from 1Password |
| `/grafana` | `grafana-mcp` | `docker.io/grafana/mcp-grafana` | 8000 | Token minted by a PostSync Job |
| `/sequential-thinking` | `sequential-thinking` | `ghcr.io/supercorp-ai/supergateway` | 8000 | Wraps the stdio server; npm pre-install initContainer |
| (none: Hermes only) | `github-mcp-hermes` | `ghcr.io/supercorp-ai/supergateway` + `github-mcp-server` image volume | 8083 | Writable (branches, commits, PRs; no merge/delete) for Hermes Agent's GitOps fixes. Token server-side from 1Password `hermes-agent`/`GITHUB_TOKEN`. No HTTPRoute; only namespace `hermes` may connect. See [Hermes Agent](okf/addons/hermes.md) |
| (none: Hermes only) | `argocd-mcp-ops` | `ghcr.io/argoproj-labs/mcp-for-argocd` (same pin as argocd-mcp) | 3000 | Hermes Agent's ArgoCD syncs, each approved by James: writes on, but ArgoCD account `hermes-ops` may only get and sync (token 1Password `argocd-hermes-ops`/`token`); Hermes includes only `sync_application`. No HTTPRoute; only namespace `hermes` may connect |
| (none: Hermes only) | `kubernetes-mcp` in vcluster-media (ns `media`) | `quay.io/containers/kubernetes_mcp_server` (same pin as kubernetes-mcp) | 8080 | Hermes Agent's read-only view of the media vcluster's own API (Deployments, PVCs, events, its ArgoCD Applications), with a vcluster ServiceAccount bound to `view` plus ArgoCD CRD reads; Secrets denied. Deployed by the vcluster's ArgoCD from `workloads/vcluster-media/addons/kubernetes-mcp`. Reached on the synced Service `kubernetes-mcp-x-media-x-vcluster-media.vcluster-media`; only Hermes may connect |
| (none: Hermes only) | `arr-mcp` | `ghcr.io/bardesss/arr-mcp` | 6060 | Hermes Agent inside the media apps: Sonarr, Radarr, Bazarr, SABnzbd (safe writes, previewed and confirmed) and Jellyfin (read-only); nothing can delete. Config rendered by ExternalSecret `arr-mcp-config` from 1Password `homepage-secret` + `arr-mcp`. Runs on the Pi 5 (local-nvme PVC). No HTTPRoute; only namespace `hermes` may connect. See [Hermes Agent](okf/addons/hermes.md) |
| (none: Hermes only) | `kubernetes-mcp-ops` | `quay.io/containers/kubernetes_mcp_server` (same pin as kubernetes-mcp) | 8080 | Hermes Agent's operational actions, each approved by James: exposes only `pods_delete` and `resources_create_or_update`; ClusterRole `kubernetes-mcp-ops` allows pod delete, node patch (cordon/uncordon) and Deployment/StatefulSet/DaemonSet patch (rollout restart), plus the viewer role; Secrets denied. No HTTPRoute; only namespace `hermes` may connect |
| `/umami` | `umami` (namespace `umami`) | `ghcr.io/umami-software/umami` | 3000 | Read-only web analytics; Umami's own `/mcp` (>= 3.4.0, `MCP_ENABLED=1`). Needs an Umami API key. Not part of this addon: see below |

**Umami is the one server outside `mcp-system`.** It is the analytics app
itself ([umami/](../addons/cluster-roles/control-plane/addons/umami/)), which
serves MCP on its own `/mcp`. Its route, `mcp-umami`, lives in the `umami`
namespace beside its Service (an HTTPRoute in the backend's namespace needs no
ReferenceGrant), and unlike the others it rewrites to `/mcp` and nothing else:
stripping `/umami` to `/` would publish Umami's dashboard and API on this host
without authentik in front. Umami authenticates every call with an API key
(`Authorization: Bearer umami_...`) and refuses a browser session.

Every image is **digest-pinned**. It was not always: an unpinned `:latest`
silently carried grafana-mcp from 0.14.0 to 1.1.0 on an unrelated rollout. Pin
new servers the same way.

Transport is **streamable HTTP** (`/mcp`), not SSE. Older docs and the disabled
`mcp-*` addons under `addons/clusters/the-cluster/` describe an SSE layout with
`acuvity/*` images — that generation is retired.

### mcpo — removed

`mcpo`, an MCP→OpenAPI bridge built for Open WebUI's "External Tools", ran here
until November 2026. Nothing used it: Open WebUI had no tool server configured
and no network path to it. It also dropped every backend's session on one
failing request (upstream open-webui/mcpo#302), which kept it broken for three
weeks unnoticed, so keeping it meant carrying a workaround for a bridge nobody
called. The history is in [known issues](okf/cluster/known-issues.md). If Open
WebUI ever needs these tools, its native `type: mcp` tool servers speak
streamable HTTP to the servers above directly, with no bridge.

The Open WebUI system prompt — the tool-first mandate that stops the model asking
questions it could look up — is checked in at
[system-prompt.md](../addons/cluster-roles/control-plane/addons/mcp-system/system-prompt.md).
It was applied by hand in the Open WebUI admin panel and is not reconciled.

### Secrets

Nothing here holds a literal secret. All of them come from 1Password via
ExternalSecrets against the `onepassword-store` ClusterSecretStore:

| Kubernetes secret | 1Password item | Used by |
|---|---|---|
| `mcp-github-token` | `mcp-github-token` / `GITHUB_TOKEN` | github-mcp |
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
- monitoring namespace (9090/80/443/3000) for grafana-mcp
- argocd namespace, `app: argocd-mcp` only; the other half is the
  `allow-argocd-server` ingress in
  [network-policies/argocd.yaml](../addons/cluster-roles/control-plane/addons/network-policies/argocd.yaml)
- public internet on 443, RFC1918 excluded, for `github-mcp` (GitHub API) and
  `sequential-thinking` (npm)
- ingress from `nginx-gateway` on 8080/8082/8000/3000
- ingress from Hermes Agent (namespace `hermes`) to kubernetes-mcp,
  kubernetes-mcp-ops, argocd-mcp, argocd-mcp-ops, grafana-mcp, github-mcp-hermes
  and arr-mcp only, on 8080/3000/8000/8083/6060 (`allow-hermes-ingress`); Hermes calls them by
  Service DNS, see [Hermes Agent](okf/addons/hermes.md)
- Hermes to the read-only kubernetes-mcp inside vcluster-media on 8080
  (`allow-vcluster-media-mcp-egress` in hermes.yaml; ingress half
  `allow-hermes-kubernetes-mcp-ingress` in vcluster-media.yaml)
- arr-mcp to the Sonarr/Radarr/Bazarr/SABnzbd pods in `vcluster-media`
  (8989/7878/6767/8080) and Jellyfin on Tower 10.0.0.12:8096
  (`allow-arr-mcp-egress`; ingress half `allow-arr-mcp-ingress` in
  [network-policies/vcluster-media.yaml](../addons/cluster-roles/control-plane/addons/network-policies/vcluster-media.yaml))
- intra-namespace both ways
- the gateway's admission for `/umami` is in
  [network-policies/umami.yaml](../addons/cluster-roles/control-plane/addons/network-policies/umami.yaml),
  beside Umami's other policies

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
5. Commit, push, merge to `main` — ArgoCD does the rest.

### Connecting a client

Any MCP client that speaks streamable HTTP:

```
https://mcp.cluster.integratn.tech/kubernetes/mcp
https://mcp.cluster.integratn.tech/argocd/mcp
https://mcp.cluster.integratn.tech/github/mcp
https://mcp.cluster.integratn.tech/grafana/mcp
https://mcp.cluster.integratn.tech/sequential-thinking/mcp
https://mcp.cluster.integratn.tech/umami/mcp        # header: Authorization: Bearer umami_...
```

For Claude Code, from an interactive terminal:

```bash
claude mcp add --transport http homelab-k8s https://mcp.cluster.integratn.tech/kubernetes/mcp
```

The endpoints are gateway-exposed with TLS but have **no authentication in front
of them**. They are reachable by anything that can resolve the hostname. The
kubernetes server is read-only by RBAC and argocd by `MCP_READ_ONLY` and its
`mcp-readonly` account; github is not. Umami is the
exception that checks a credential itself: no API key, no tools. None of these
are public -- the cloudflared tunnel admits only Umami's tracker paths, never
`/mcp`.

### Troubleshooting

```bash
kubectl -n mcp-system get pods,svc,httproute
kubectl -n mcp-system logs deploy/grafana-mcp
kubectl -n argocd get application mcp-system-the-cluster
```

| Symptom | Look at |
|---|---|
| Client connects, tools list empty | Wrong transport — it is streamable HTTP on `/mcp`, not SSE on `/sse` |
| Request hangs, no error | Missing egress NetworkPolicy |
| 404 through the gateway | HTTPRoute prefix vs the `ReplacePrefixMatch` rewrite |
| Pod restart-looping | A probe pointing at a dependency that is down |
| `initialize` answers 200 but every tool call fails | The server cannot reach its own upstream; `initialize` never touches it. Make a real tool call. argocd-mcp shipped this way for months |

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
