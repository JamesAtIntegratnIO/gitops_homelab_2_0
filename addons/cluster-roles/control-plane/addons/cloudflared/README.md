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
| `sonarr.integratn.tech` | everything but the blocked paths below; Cloudflare Access (authentik) first | `http://sonarr-x-media-x-vcluster-media.vcluster-media.svc.cluster.local:8989` |
| `radarr.integratn.tech` | same | `http://radarr-x-media-x-vcluster-media.vcluster-media.svc.cluster.local:7878` |
| (anything else) | | `http_status:404` |

(The table predates `stats.integratn.io` and `auth.integratn.tech`; the
authoritative list is `cloudflare_zero_trust_tunnel_cloudflared_config` in
IntegratnIO/infrastructure `sites/main.tf`.)

### Sonarr and Radarr away from home

Cloudflare Access stands in front of both hosts and signs people in with
authentik (OIDC client `cloudflare-access`, blueprint
`authentik-blueprints-cloudflare-access`, group `media-management`). A request
without an Access session never reaches the homelab, and cloudflared checks the
Access token again before it passes a request on. The tunnel goes straight to
the apps' synced Services, not the vcluster gateway, so the LAN rule that
leaves `/api` open to API-key holders does not apply: from outside, `/api`
works only inside a signed-in browser session, which is how the UI calls it.

Paths that answer 404 at Cloudflare's edge (case-insensitive, any number of
slashes, plus any `.`/`..` segment): the app's own settings that hold its API
key (`/api/v*/config/host`, `/api/v*/settings/general`), backups
(`/api/v*/system/backup`, `/backup/`), restart/shutdown, updates, log files
(`/api/v*/log/file`, `/logfile/`, `/updatelogfile/`), and the connections that
hold other systems' credentials (indexers, download clients, notifications,
metadata, and import-list edits). Library work -- browsing, adding, searching,
grabbing, the queue, history, calendar, wanted, manual import, renames -- is
untouched. Phone apps that use the API key do not work from outside, on
purpose.

Bazarr and SABnzbd stay LAN-only: Bazarr's UI loads `/api/system/settings` on
every page, which returns the Sonarr/Radarr keys and the subtitle providers'
passwords in clear, and SABnzbd's UI is mostly its config (Usenet logins, API
and NZB keys).

`/mcp`, the dashboard, the login and Umami's API are not on that list, so they
are not public. The paths are reached by the sites' Cloudflare Pages Functions,
which proxy `/c.js` and `/api/c` on each site to edge.integratn.io.

| File | What it is |
|---|---|
| `deployment.yaml` | two replicas, the token from the Secret below, `/ready` on the metrics port 2000 |
| `externalsecret.yaml` | `cloudflared-tunnel`, the tunnel token |

The namespace and its NetworkPolicies are in
[`network-policies/cloudflared.yaml`](../network-policies/cloudflared.yaml):
DNS, Cloudflare's edge on 7844 (UDP and TCP) and 443, `umami:3000`,
`authentik-server:9000`, and Sonarr `:8989` / Radarr `:7878` in vcluster-media.
No ingress at all.

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
