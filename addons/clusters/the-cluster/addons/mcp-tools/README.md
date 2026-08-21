# MCP Tools — RETIRED

> **This addon generation is retired.** The `mcp-kubernetes`, `mcp-github`,
> `mcp-sequential-thinking` and `mcp-grafana` addons in
> [`addons/clusters/the-cluster/addons.yaml`](../../addons.yaml) are all
> `enabled: false`. The values files in the sibling `mcp-*/` directories are
> kept only so the history reads.
>
> **The live MCP servers are the `mcp-system` addon**, at
> [`addons/cluster-roles/control-plane/addons/mcp-system/`](../../../../cluster-roles/control-plane/addons/mcp-system/).
>
> **See [docs/mcp.md](../../../../../docs/mcp.md) for current documentation.**

## What changed

| | This generation (retired) | `mcp-system` (current) |
|---|---|---|
| Deployment | Stakater `application` Helm chart, one addon per server | Raw manifests, one `type: manifest` addon |
| Namespace | `mcp-system` | `mcp-system` |
| Transport | SSE, `/{server}/sse` | Streamable HTTP, `/{server}/mcp` |
| Images | `acuvity/mcp-server-*` | upstream first-party images, digest-pinned |
| Servers | kubernetes, github, sequential-thinking, grafana | + argocd, + mcpo (MCP→OpenAPI for Open WebUI) |
| Consumer | Hermes Agent, LM Studio | Open WebUI, any MCP client |

The hostname is unchanged: `mcp.cluster.integratn.tech`.
