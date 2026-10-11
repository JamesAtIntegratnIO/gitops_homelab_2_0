# umami

[Umami](https://umami.is) is cookieless web analytics. This addon runs it for
two static sites, integratn.io and bosun.integratn.io, from the official image,
`ghcr.io/umami-software/umami`, pinned by tag and digest and tracked by Kargo.
It is plain manifests: there is no first-party chart, and one Deployment and
one Service are less to read than a third-party chart's values.

It is the first thing in this cluster a stranger's browser reaches. Every
visitor to the two sites loads the tracker and posts pageviews, so two of
Umami's paths are public, through the cloudflared tunnel
([`../cloudflared/`](../cloudflared/)), and nothing else of it is.

| File | What it is |
|---|---|
| `deployment.yaml` | Umami: one replica, read-only root filesystem, the environment below, and the Service `umami.umami.svc.cluster.local:3000` |
| `redis.yaml` | a Redis for Umami's cache and dashboard sessions, no persistence |
| `externalsecret.yaml` | `umami-env` (`DATABASE_URL`, `APP_SECRET`, `REDIS_URL`) and `umami-redis` (the Redis password) |
| `httproute.yaml`, `snippetsfilter.yaml` | `stats.cluster.integratn.tech`, the dashboard, behind authentik; and `mcp.cluster.integratn.tech/umami/mcp`, Umami's MCP endpoint |

Elsewhere:

- The namespace and its NetworkPolicies are in
  [`network-policies/umami.yaml`](../network-policies/umami.yaml). Umami is
  admitted on 3000 from three places, and each has its egress half on its own
  side: the gateway (`nginx-gateway.yaml`), the tunnel (`cloudflared.yaml`) and
  mcpo (`mcp-system.yaml`).
- The authentik provider is `08b-umami-proxy-provider.yaml` in the blueprint
  ConfigMap (`clusters/the-cluster/addons/authentik/`), and it is on the
  embedded outpost's list in `09-embedded-outpost.yaml`.
- mcpo's entry is in `mcp-system/mcpo-configmap.yaml`, with its key from
  `mcp-system/externalsecret-umami-mcp.yaml`.
- The tunnel's public hostname and path rule are OpenTofu in
  IntegratnIO/infrastructure.

## How a pageview gets here

```
visitor's browser
  -> the site, on Cloudflare Pages: Pages Functions answer /c and /api/c
  -> https://edge.integratn.io/c.js, /api/c
  -> Cloudflare tunnel, path rule ^/(c\.js|api/c)$, everything else 404
  -> cloudflared pod -> umami.umami.svc.cluster.local:3000
```

The tunnel paths are reached by the sites' Functions. The Function sends the
visitor's address in `x-umami-client-ip`, and Umami reads it
(`CLIENT_IP_HEADER`) before anything else. Nothing stops a direct caller of
edge.integratn.io from setting that header too; at worst that forges where one
pageview came from. For the same reason the location comes from the GeoLite
database in the image (`SKIP_LOCATION_HEADERS=1`): Cloudflare's own location
headers on a Function's request describe the Function, not the visitor.

**Why the names are bland:** ad blockers match on the script's name, the
collect endpoint and the hostname (`script.js`, `/api/send`, `stats.`,
`analytics.`), so a visitor's browser sees `<site>/c` posting to
`/api/c`. The site path is `/c`, not `/c.js`: Pages serves `functions/c.js` at
`/c`, and `/c.js` on a site returns its HTML. `TRACKER_SCRIPT_NAME=c.js` and `COLLECT_API_ENDPOINT=/api/c` make
Umami answer on those paths, and at start it rewrites its tracker to post to
`/api/c` -- the one file it writes, which is why `public/` is an emptyDir.

**The tunnel does not expose `/mcp`, the dashboard, the login or the API.** Its
path rule admits exactly `/c.js` and `/api/c`; `/mcp` and everything else
gets Cloudflare's 404 and never reaches the cluster.

## Before merging

The Deployment migrates the database on start, so the merge is the deploy, and
these have to exist first, or the pod crashloops in `check-db.js` until they
do. Nothing in the cluster is broken by that meanwhile; the sites' tracker
calls just fail.

### 1. The database

Postgres on `10.0.3.1`, the server specmarshal and authentik use. Umami creates
and migrates its own tables (it needs Postgres 9.4 or newer); it needs a role
that owns its database:

```sql
CREATE ROLE umami WITH LOGIN PASSWORD '<password>';
CREATE DATABASE umami OWNER umami ENCODING 'UTF8' TEMPLATE template0;
REVOKE ALL ON DATABASE umami FROM PUBLIC;
\c umami
ALTER SCHEMA public OWNER TO umami;
```

`pg_hba.conf` on `10.0.3.1` has to admit the cluster for that role and
database. Copy the line that admits specmarshal, with the same address and
method, and change the database and user:

```
host  umami  umami  <same address as specmarshal's line>  scram-sha-256
```

then reload Postgres (`SELECT pg_reload_conf();`). The address is whatever the
cluster's traffic arrives from at 10.0.3.1 -- pod addresses or node addresses,
depending on Cilium's masquerading -- and the specmarshal line already has it
right, so copy it rather than guess.

### 2. 1Password items

Every item is read by `onepassword-store`, and every field by its **label**.
Renaming a field breaks the ExternalSecret that reads it.

| Item | Fields | Notes |
|---|---|---|
| `umami-db-connection` | `host` (`10.0.3.1`), `port` (`5432`), `text` (`umami`), `password` | the role from step 1. The password goes into `DATABASE_URL` URL-encoded, because Prisma's migration reads the URL alone; it must not contain spaces |
| `umami-app-secret` | `password` | any long random string. Signs dashboard logins; changing it logs everyone out |
| `umami-redis` | `password` | a generated password, no spaces. The Redis server and `REDIS_URL` both read it |
| `umami-mcp` | `token` | created after Umami is running -- see below. mcpo starts without it |

The tunnel's item, `cloudflared-tunnel`, is in [`../cloudflared/README.md`](../cloudflared/README.md).

### 3. The tunnel

`edge.integratn.io` and its path rule are IntegratnIO/infrastructure's. That
stack must be applied and its token in 1Password (cloudflared's README) for
the public paths to answer; the dashboard does not need it.

### 4. authentik

Nothing by hand: the provider and application are in the blueprint ConfigMap,
and authentik applies it on its own. Check the **Umami** tile appears under
Platform Services.

## Once it is running

1. **Change the default login at once.** Umami creates `admin` / `umami` on its
   first migration. Open `https://stats.cluster.integratn.tech` (authentik
   first, then Umami's own login), sign in as `admin` / `umami`, and change the
   password under Settings -> Profile. authentik in front means nobody off the
   LAN can reach the login, but it is still the first thing to do.

2. **Add the two websites.** Settings -> Websites -> Add website, once for
   `integratn.io` and once for `bosun.integratn.io`. Copy each one's Website ID.

3. **Put them in the sites.** integratn-io-astro `src/analytics.ts` and bosun
   `site/analytics.mjs`, each with its own ID:

   ```js
   { tool: 'umami', src: '/c', websiteId: '<id>' }
   ```

   Then check a pageview lands in Realtime.

4. **The MCP key.** Settings -> API keys -> create one (it is the whole
   `umami_...` string, shown once). Put it in 1Password item `umami-mcp`, field
   `token`, wait for the ExternalSecret (up to an hour, or annotate it with
   `force-sync`), then `kubectl -n mcp-system rollout restart deploy/mcpo`:
   mcpo renders its config only when it starts. Clients outside Open WebUI use
   `https://mcp.cluster.integratn.tech/umami/mcp` with
   `Authorization: Bearer umami_...`. The endpoint is read-only.

## Running it

- `kubectl -n umami logs deploy/umami` -- the start prints the database checks,
  the migration, and `Updated tracker endpoint: /api/c.` A pod that never gets
  that far is the database (`pg_hba`, the role, the item).
- `curl -sI https://edge.integratn.io/c.js` should be 200 and
  `https://edge.integratn.io/` 404 -- the second is the tunnel's catch-all and
  proves the path rule, not Umami.
- Redis down costs nothing but speed and every dashboard login; Umami falls
  back to the database. Its readiness never looks at Redis or Postgres.
- A new version is a Kargo PR (target `umami`), held for a human above a patch:
  every start runs the new version's migrations, and they are one-way. Read
  the release notes before merging.
