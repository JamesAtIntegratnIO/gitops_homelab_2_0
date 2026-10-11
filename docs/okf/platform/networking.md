---
type: Network Architecture
title: Networking — Cilium, MetalLB, Gateway API, DNS, TLS
description: The full traffic path from Cloudflare DNS through the MetalLB VIP and nginx-gateway-fabric into services, plus the Cilium CNI specifics.
tags: [networking, cilium, metallb, gateway-api, dns, tls, cloudflare]
status: stable
generated: { by: claude-code/claude-opus-5.5, at: 2026-10-11T18:00:00Z }
stale_after: 2027-02-20
sources:
  - id: live-net
    resource: kubectl get svc,gateways,httproutes,ipaddresspools,certificates -A on the-cluster, 2026-08-20
    title: Live networking state
  - id: cp-patch
    resource: ../../../matchbox/talos-machineconfigs/cp.yaml
    title: Talos control-plane patch (CNI none, proxy disabled)
  - id: addons-net
    resource: ../../../addons/environments/production/addons/addons.yaml
    title: cilium / metallb / nginx-gateway-fabric addon definitions
  - id: tunnel
    resource: ../../../addons/cluster-roles/control-plane/addons/cloudflared/README.md
    title: cloudflared addon -- the tunnel and what it exposes
  - id: tunnel-netpol
    resource: ../../../addons/cluster-roles/control-plane/addons/network-policies/cloudflared.yaml
    title: cloudflared NetworkPolicies
---

# Address plan

| Thing | Address |
|---|---|
| Cluster subnet | `10.0.4.0/24` (inside supernet `10.0.0.0/9`) |
| Kubernetes API VIP (Talos-managed) | `10.0.4.100` |
| Nodes | `10.0.4.101–103` |
| MetalLB L2 pool (`default`, iface `eno1`) | `10.0.4.200–253` |
| Main gateway LB (`nginx-gateway/nginx-gateway-nginx`) | `10.0.4.205` |
| vcluster-media API LB | `10.0.4.200` |
| vcluster-media gateway LB | `10.0.4.201` |
| NFS server / Matchbox host (Unraid) | `10.0.0.12` |
| 1Password Connect | `10.0.1.139` (connect.integratn.tech) |
| External Postgres (used by media apps, authentik, specmarshal, umami) | `10.0.3.1:5432` |

# CNI: Cilium 1.17.3

Talos is configured with `cni: none` and `proxy.disabled: true` — **Cilium
replaces both CNI and kube-proxy**, with Hubble relay + UI
(`hubble.cluster.integratn.tech`).[^cp-patch] Cilium network policy is in
**enforce mode** (flipped from audit in Feb 2026). CiliumNetworkPolicies
complement vanilla NetworkPolicies where entity-based selection is needed
(`kube-apiserver` entity, host/remote-node SNAT cases). Note: neither the
README nor AGENTS.md mentions Cilium — cluster reality is ahead of the docs.

# North-south traffic path

**LAN.** Every `*.cluster.integratn.tech` name is a Cloudflare DNS record to the
gateway VIP, a private address, so all of this is reachable only from the LAN:

```
client → Cloudflare DNS (A record → 10.0.4.205)
       → MetalLB L2 ARP for the VIP
       → nginx-gateway-fabric 2.6.7 (Gateway `nginx-gateway`, TLS terminate)
       → HTTPRoute (hostname match) → ClusterIP service → pod
```

- **Gateway**: one `Gateway` (`nginx-gateway/nginx-gateway`, class `nginx`)
  with `http` (redirect) and `https` listeners for `*.cluster.integratn.tech`.
- **TLS**: wildcard Certificate `wildcard-cluster-integratn-tech` from
  ClusterIssuer `letsencrypt-prod` (DNS-01 via Cloudflare API key from
  1Password). cert-manager v1.16.2.
- **Routes**: ~28 HTTPRoutes; the convention is an https route plus an
  `-http-redirect` (301) twin. Notable hostnames: argocd, grafana, prometheus,
  alertmanager, loki, prom-remote (vcluster remote-write ingest), auth
  (authentik), chat (open-webui), qdrant, hubble, goldilocks, trivy, llm,
  bosun (the delivery agent's status page, behind authentik forward-auth like
  qdrant -- the bosun chart can render this route from `web.httpRoute` but its
  HTTPRoute takes no `filters`, so it is hand-written instead),
  and `mcp.cluster.integratn.tech` with path-prefix routing to
  five MCP servers (`/kubernetes`, `/argocd`, `/github`, `/grafana`,
  `/sequential-thinking`).[^live-net]
- **DNS**: Terraform manages a few static records + wildcard; external-dns
  v0.19.0 (sources: gateway-httproute + service, `txtOwnerId: the-cluster`)
  manages per-service records; each vcluster runs its own external-dns with
  its own owner id.

**Public: one path, through a tunnel (since 2026-10-11).** The gateway is still
not on the internet. The only public way in is a remotely-managed Cloudflare
Tunnel whose connectors run in-cluster (`cloudflared` addon, 2 replicas) and
dial *out* to Cloudflare's edge on 7844 (QUIC, or HTTP/2 over TCP); no port is
forwarded and nothing listens on the LAN for it.[^tunnel]

```
browser -> site on Cloudflare Pages -> Pages Functions proxy /c, /api/c
        -> edge.integratn.io (tunnel ingress: ^/(c\.js|api/c)$, else 404)
        -> cloudflared pod -> umami.umami.svc.cluster.local:3000
```

- **What is public is decided outside this repo**: the tunnel, its ingress
  rules and the `edge.integratn.io` record are OpenTofu in
  IntegratnIO/infrastructure. **What the connectors can reach is decided
  here**: `network-policies/cloudflared.yaml` allows DNS, the edge
  (0.0.0.0/0 minus RFC1918 on 7844 UDP+TCP and 443) and `umami:3000`, and
  nothing else.[^tunnel-netpol] A new public hostname needs both.
- Two paths, both Umami's tracker (`/c.js`) and collect endpoint (`/api/c`),
  bland names on purpose because ad blockers match `script.js`, `/api/send`,
  `stats.` and `analytics.`. The dashboard (`stats.cluster.integratn.tech`,
  behind authentik) and Umami's `/mcp` stay LAN-only.
- Umami takes the visitor address from `x-umami-client-ip`, set by the
  sites' Functions; a direct caller can forge it, which at worst forges one
  pageview's location.

# vcluster networking

Each vcluster gets: a LoadBalancer VIP for its API (external-dns hostname,
e.g. `media.integratn.tech` → 10.0.4.200), its own nginx-gateway (e.g.
`*.vcluster-media...` LB 10.0.4.201), CoreDNS forwarding to Talos node-local
DNS (`169.254.116.108`), and Cilium-aware policies generated by the
[vcluster-orchestrator-v2 promise](/promises/vcluster-orchestrator-v2.md)
(the 8443 ingress and LB-SNAT rules work around Cilium kube-proxy-replacement
DNAT ordering).

East-west traffic is governed by per-namespace default-deny — see
[security posture](/platform/security-posture.md).

[^live-net]: Live networking state
[^cp-patch]: Talos control-plane patch (CNI none, proxy disabled)
[^addons-net]: cilium / metallb / nginx-gateway-fabric addon definitions
[^tunnel]: cloudflared addon -- the tunnel and what it exposes
[^tunnel-netpol]: cloudflared NetworkPolicies
