# MCP Tools

Model Context Protocol (MCP) servers deployed to the cluster, providing external capabilities for LLM agents via SSE (Server-Sent Events) transport.

## Overview

This addon deploys 5 MCP servers in the `mcp-system` namespace, each exposed via the NGINX Gateway under a unified hostname with path-based routing.

## Deployed Servers

| Server | Image | Purpose | SSE Endpoint |
|--------|-------|---------|--------------|
| Kubernetes | acuvity/mcp-server-kubernetes | Cluster operations, pod/service management | `https://mcp.cluster.integratn.tech/kubernetes/sse` |
| GitHub | acuvity/mcp-server-github | Repository operations, PRs, issues | `https://mcp.cluster.integratn.tech/github/sse` |
| Sequential Thinking | acuvity/mcp-server-sequential-thinking | Reasoning and problem solving | `https://mcp.cluster.integratn.tech/sequential-thinking/sse` |
| Brave Search | acuvity/mcp-server-brave-search | Web search | `https://mcp.cluster.integratn.tech/brave-search/sse` |
| Grafana | grafana/mcp-grafana | Monitoring and observability | `https://mcp.cluster.integratn.tech/grafana/sse` |

## Prerequisites - 1Password Secrets

Before deploying, create these secrets in 1Password:

1. `mcp-github-token` with property `GITHUB_TOKEN` - GitHub Personal Access Token with repo/org scope
2. `mcp-brave-search-api-key` with property `BRAVE_API_KEY` - Brave Search API key
3. `mcp-grafana-token` with property `GRAFANA_TOKEN` - Grafana service account token

## Configuration

Each MCP server is deployed as an individual addon using the Stakater `application` Helm chart (v8.1.2). Configuration is in:

- `addons/clusters/the-cluster/addons.yaml` - Addon entries
- `addons/clusters/the-cluster/addons/mcp-*/values.yaml` - Per-server values

### Adding a New MCP Server

1. Add an entry to `addons/clusters/the-cluster/addons.yaml`:
```yaml
mcp-new-tool:
  enabled: true
  namespace: mcp-system
  chartName: application
  chartRepository: "https://stakater.github.io/stakater-charts"
  defaultVersion: "8.1.2"
  project: platform-services
  selectorMatchLabels:
    cluster_role: control-plane
  syncPolicy:
    automated:
      selfHeal: true
      prune: true
    syncOptions:
      - CreateNamespace=true
      - ServerSideApply=true
```

2. Create `addons/clusters/the-cluster/addons/mcp-new-tool/values.yaml` with Stakater chart values including `httpRoute` configuration.

3. If the server requires secrets, add an `ExternalSecret` in `extraObjects` referencing 1Password via `ClusterSecretStore`.

## Configuring Hermes Agent to Use MCP Servers

Hermes Agent can connect to these MCP servers as native MCP tools. Add the following to your `config.yaml`:

```yaml
mcp:
  servers:
    kubernetes:
      transport: sse
      url: https://mcp.cluster.integratn.tech/kubernetes/sse
    github:
      transport: sse
      url: https://mcp.cluster.integratn.tech/github/sse
    sequential-thinking:
      transport: sse
      url: https://mcp.cluster.integratn.tech/sequential-thinking/sse
    brave-search:
      transport: sse
      url: https://mcp.cluster.integratn.tech/brave-search/sse
    grafana:
      transport: sse
      url: https://mcp.cluster.integratn.tech/grafana/sse
```

Or configure via CLI:

```bash
hermes mcp add kubernetes --transport sse --url https://mcp.cluster.integratn.tech/kubernetes/sse
hermes mcp add github --transport sse --url https://mcp.cluster.integratn.tech/github/sse
hermes mcp add sequential-thinking --transport sse --url https://mcp.cluster.integratn.tech/sequential-thinking/sse
hermes mcp add brave-search --transport sse --url https://mcp.cluster.integratn.tech/brave-search/sse
hermes mcp add grafana --transport sse --url https://mcp.cluster.integratn.tech/grafana/sse
```

Verify with:

```bash
hermes mcp list
```

### LM Studio Configuration

For LM Studio's built-in MCP client, configure each server with the SSE endpoint URL:

```json
{
  "mcpServers": {
    "kubernetes": {
      "url": "https://mcp.cluster.integratn.tech/kubernetes/sse"
    },
    "github": {
      "url": "https://mcp.cluster.integratn.tech/github/sse"
    },
    "sequential-thinking": {
      "url": "https://mcp.cluster.integratn.tech/sequential-thinking/sse"
    },
    "brave-search": {
      "url": "https://mcp.cluster.integratn.tech/brave-search/sse"
    },
    "grafana": {
      "url": "https://mcp.cluster.integratn.tech/grafana/sse"
    }
  }
}
```

## Architecture

```
LLM Client (Hermes / LM Studio)
    |
    | SSE over HTTPS
    v
NGINX Gateway (mcp.cluster.integratn.tech)
    |
    +-- /kubernetes/sse     --> mcp-kubernetes:3000
    +-- /github/sse         --> mcp-github:3000
    +-- /sequential-thinking/sse --> mcp-sequential-thinking:3000
    +-- /brave-search/sse   --> mcp-brave-search:3000
    +-- /grafana/sse        --> mcp-grafana:3000
    |
    v
mcp-system namespace
    |
    +-- ExternalSecrets --> 1Password Connect
    +-- Kubernetes MCP --> ClusterRole (read-only)
```

## Security

- All servers run as non-root (UID 65534) with read-only root filesystem
- Capabilities dropped (ALL)
- Seccomp profile: RuntimeDefault
- Resource limits: 50m CPU, 128Mi-256Mi memory per server
- Kubernetes MCP server has read-only ClusterRole (pods, deployments, services, etc.)
- All secrets managed via External Secrets Operator + 1Password
