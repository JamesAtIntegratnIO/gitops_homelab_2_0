---
type: Workload
title: Hermes Agent
description: Nous Research's Hermes Agent running as a service on the-cluster's Raspberry Pi 5 worker, on the Anthropic API, with its state on local NVMe.
tags: [ai, agent, hermes, raspberry-pi, arm64]
status: draft
generated: { by: claude-code, at: 2026-11-07T20:20:00Z }
stale_after: 2027-02-07
sources:
  - id: manifests
    resource: ../../../addons/cluster-roles/control-plane/addons/hermes/hermes-agent.yaml
    title: Hermes addon manifests
  - id: policies
    resource: ../../../addons/cluster-roles/control-plane/addons/network-policies/hermes.yaml
    title: Hermes namespace and network policies
  - id: optimization-guide
    resource: https://github.com/OnlyTerp/hermes-optimization-guide/tree/f865f8a
    title: OnlyTerp hermes-optimization-guide (pinned to Hermes v0.21.4)
  - id: upstream-docker
    resource: https://github.com/NousResearch/hermes-agent/blob/v0.23.0/website/docs/user-guide/docker.md
    title: Hermes Agent Docker guide (v0.23.0)
---

# What runs

One pod, `hermes-agent` in namespace `hermes`, pinned to `talos-pi5-1` (it
tolerates the node's arm64 taint). The image is the official multi-arch
`nousresearch/hermes-agent:v0.23.0`, digest-pinned. It runs `gateway run`; with
no chat platform configured, the service it offers is the OpenAI-compatible API
server on port 8642 (ClusterIP `hermes-agent`, no ingress from other namespaces
yet). Open WebUI (namespace `ai`) may reach it, with `API_SERVER_KEY`.[^manifests]

# Browser UIs

Both are internal only, behind nginx-gateway (the gateway VIP is on the LAN;
reach it from home or over Tailscale):

| UI | URL | Port |
|---|---|---|
| Webapp (chat, Hermes Desktop in a browser) | `https://hermes.cluster.integratn.tech` | 9120 |
| Dashboard (sessions, config, logs, skills) | `https://hermes-admin.cluster.integratn.tech` | 9119 |

Hermes signs users in itself with its self-hosted OIDC plugin against authentik:
a public PKCE client `hermes` (blueprint `10-hermes-provider.yaml`, no client
secret, which is all the plugin supports), bound to the `authentik Admins`
group because every account that can sign in acts as the machine owner. There is
no authentik forward-auth in front. `dashboard.trusted_proxies` is the pod CIDR
so the gateway's `X-Forwarded-Proto` is honoured.[^manifests]

Open WebUI keeps its OpenAI connections in its own database, so adding Hermes
there is a one-time step in its admin settings (URL
`http://hermes-agent.hermes.svc.cluster.local:8642/v1`, key `API_SERVER_KEY`);
only the network path is in git.

The model is `claude-sonnet-5-5` through the native `anthropic` provider, billed
per token against the key in 1Password item `hermes-agent` (`homelab` vault,
fields `ANTHROPIC_API_KEY` and `API_SERVER_KEY`). The terminal backend is
`local`: shell commands run inside the pod.[^manifests]

# Constraints from the image

- s6-overlay `/init` starts as root, chowns `/opt/data`, then drops to uid
  10000. The pod must not set `runAsNonRoot` or a read-only root filesystem,
  and must not override the entrypoint.[^upstream-docker]
- `/opt/data` is all of Hermes's state. `state.db` is SQLite in WAL mode, which
  needs a local filesystem and one gateway per data directory: a 20 Gi
  `local-nvme` claim, one replica, `Recreate`.[^upstream-docker]
- `config.yaml` is copied from the `hermes-config` ConfigMap at every start, so
  settings changed inside the pod do not survive a restart.[^manifests]

# Network

Egress is DNS plus TCP 443/80 to anywhere outside RFC1918: Hermes reaches the
Anthropic API and the public web, but no LAN host and no in-cluster service,
except authentik through the gateway pods on 443 for OIDC discovery and JWKS.
Ingress: nginx-gateway to 9119/9120, and Open WebUI to 8642.[^policies]

[^manifests]: Hermes addon manifests
[^policies]: Hermes namespace and network policies
[^upstream-docker]: Hermes Agent Docker guide (v0.23.0)

# Tuning

Applied 2026-11-07 from the third-party optimization guide, which targets
v0.21.4, with every key checked against v0.23.0's built-in defaults first.[^optimization-guide]

- **Cost**: the side tasks (compression, titles, background review, approval
  classifier, curator, goal judge) run on `claude-haiku-5-5` instead of the main
  Sonnet model. `tts` is disabled, at most 4 concurrent subagents, and
  exact-repeat tool loops hard-stop.
- **Security**: smart approvals with a deny list (`sudo`, piping `curl`/`wget`
  to a shell, `dd` to devices, force-push, `authorized_keys`, `kubectl`), no
  private URLs, secret redaction, no runtime pip installs, internal hosts on the
  website blocklist, agent-written skills scanned, no borrowed external logins,
  no vault CLIs. The guide's `terminal.backend: docker` is impossible in a pod;
  the pod and its network policy are the sandbox.
- **Ops**: sessions are pruned after 90 days, `/dev/shm` is a 1 Gi memory
  volume for Chromium, and a liveness probe on `/health` restarts a wedged
  gateway.
- **Monitoring**: Hermes has no Prometheus endpoint; its gateway-health export
  is OTLP push only and is not wired up. Alert `HermesGatewayDown` (platform
  alerts, group `platform.hermes`) uses kube-state-metrics instead. There is no
  cron-health signal without OTLP.
- **Not used**: `security.tirith_fail_open` does not exist in v0.23.0.
  Delegation and cron stay on the main model.

[^optimization-guide]: OnlyTerp hermes-optimization-guide (pinned to Hermes v0.21.4)

