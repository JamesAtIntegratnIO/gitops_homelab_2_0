# cloudflared

The cluster's one public way in: two connectors of a Cloudflare Tunnel, running
`cloudflared tunnel run`. Each pod dials out to Cloudflare's edge and keeps the
tunnel open; Cloudflare sends the tunnel's public hostnames down it. Nothing
listens on the LAN, no router port is forwarded, and the gateway VIP
(`10.0.4.205`) stays LAN-only.

Today the tunnel carries one hostname and two paths, for Umami
([`../umami/`](../umami/)):

| Public hostname | Path | Origin |
|---|---|---|
| `edge.integratn.io` | `^/(c\.js|api/c)$` | `http://umami.umami.svc.cluster.local:3000` |
| (anything else) | | `http_status:404` |

`/mcp`, the dashboard, the login and Umami's API are not on that list, so they
are not public. The paths are reached by the sites' Cloudflare Pages Functions,
which proxy `/c.js` and `/api/c` on each site to edge.integratn.io.

| File | What it is |
|---|---|
| `deployment.yaml` | two replicas, the token from the Secret below, `/ready` on the metrics port 2000 |
| `externalsecret.yaml` | `cloudflared-tunnel`, the tunnel token |

The namespace and its NetworkPolicies are in
[`network-policies/cloudflared.yaml`](../network-policies/cloudflared.yaml):
DNS, Cloudflare's edge on 7844 (UDP and TCP) and 443, and `umami:3000`. No
ingress at all.

## Why remotely managed

The tunnel, its ingress rules and the `edge.integratn.io` DNS record are
OpenTofu in **IntegratnIO/infrastructure**, and this addon runs a connector
with the token that stack outputs. The alternative, a locally-managed tunnel,
keeps the rules in a ConfigMap here but needs a credentials file made by hand
with `cloudflared tunnel login` / `create` and a DNS record made by hand with
`tunnel route dns` -- two pieces of Cloudflare state outside any code. The
OpenTofu stack holds all three in code, reviewed and applied like the rest of
the Cloudflare account, so git is still the source of truth; it is just a
different repository's.

The consequence to remember: **what is public is decided in
IntegratnIO/infrastructure, and what the connectors can reach is decided
here.** A new hostname there does nothing until `cloudflared.yaml` lets the
pods reach its origin, and the origin's namespace admits them (the two halves,
as at the gateway). Neither half alone exposes anything.

## Before merging

1. **Apply the tunnel stack** in IntegratnIO/infrastructure: the tunnel, the
   ingress rules in the table above, and the `edge.integratn.io` CNAME to
   `<tunnel-id>.cfargotunnel.com` (proxied).
2. **1Password item `cloudflared-tunnel`**, field `token`: the tunnel token from
   that stack's output (`tofu output -raw <name>`), one line. Read by its
   label.

Without them the pods start, fail to authenticate, and stay not-ready; nothing
else is affected.

## Running it

- `kubectl -n cloudflared logs deploy/cloudflared` -- a healthy connector logs
  `Registered tunnel connection` four times (two edge locations, two each).
- Both pods Ready means both have a live connection to the edge. The tunnel's
  page in the Cloudflare dashboard shows two connectors.
- `curl -sI https://edge.integratn.io/` must be 404 (the catch-all) and
  `https://edge.integratn.io/c.js` 200 (Umami). A 502 on `/c.js` is the
  connector unable to reach Umami: the NetworkPolicy halves, or Umami down.
- A rotated or recreated tunnel means a new token: update the item, then
  `kubectl -n cloudflared rollout restart deploy/cloudflared` once the
  ExternalSecret has refreshed.
- A new version is a Kargo PR (target `cloudflared`).
