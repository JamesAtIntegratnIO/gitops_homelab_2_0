---
type: Workload
title: Hermes Agent
description: Nous Research's Hermes Agent running as a service on the-cluster's Raspberry Pi 5 worker, on the Anthropic API, with its state on local NVMe.
tags: [ai, agent, hermes, raspberry-pi, arm64]
status: draft
generated: { by: claude-code, at: 2026-11-07T19:10:00Z }
stale_after: 2027-02-07
sources:
  - id: manifests
    resource: ../../../addons/cluster-roles/control-plane/addons/hermes/hermes-agent.yaml
    title: Hermes addon manifests
  - id: policies
    resource: ../../../addons/cluster-roles/control-plane/addons/network-policies/hermes.yaml
    title: Hermes namespace and network policies
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
yet). The dashboard is off.[^manifests]

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
Anthropic API and the public web, but no LAN host and no in-cluster
service.[^policies]

[^manifests]: Hermes addon manifests
[^policies]: Hermes namespace and network policies
[^upstream-docker]: Hermes Agent Docker guide (v0.23.0)
