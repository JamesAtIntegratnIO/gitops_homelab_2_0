# homepage

The cluster's landing page, at `https://home.cluster.integratn.tech`, behind
authentik. It runs [Glance](https://github.com/glanceapp/glance); the addon,
namespace, Deployment and Kargo target keep the name `homepage` from the
Homepage (gethomepage.dev) it replaced, so the authentik provider and the
NetworkPolicy halves in other namespaces did not move.

One screen that answers "is anything wrong, and what is happening": a status
line with a verdict and chips (critical alerts, warnings, unhealthy apps,
failed promotions, stuck pods), CPU and memory sparklines, per-node cards and
the N+1 request line, firing alerts by name, ArgoCD and Kargo problems, the
week's TV and downloads, Umami for integratn.io and bosun.integratn.io,
upcoming films, Plex, Proxmox and Tower, upstream releases, and a launcher for
every UI. A second page, News, has Hacker News, Lobsters and RSS.

Grafana's Homelab Overview is the deep view; this page links into it.

## Brand

It wears the IntegratnIO design system, the same one integratn.io,
Specmarshal and Bosun use (specmarshal `DESIGN.md`, "The Night Shift Ledger"):
near-black ground, flat panels with a 1px hairline and a 6px radius, system
sans for prose, mono with tabular figures for every number, 12px uppercase
labels. Colour is spent only on a verdict and always sits next to a word
(`ALL CLEAR`, `WARNING`, `DEGRADED`, `HOT`); bars and charts are neutral until
they cross a threshold. Signal orange is the Integratn mark in the header
(served from `https://integratn.io/favicon.svg`) and nothing else, so Glance's
`primary-color` is the neutral text colour. A figure that cannot be read says
"Not recorded" and why, in dim text, never an error box. The tokens are the
`:root` block at the top of `document.head` in `configmap.yaml`.

| File | What it is |
|---|---|
| `configmap.yaml` | `glance-config` (the whole `glance.yml`) and `glance-env` (API URLs plus an empty default for every key) |
| `deployment.yaml` | Glance: one replica, non-root, read-only root filesystem, no service-account token; the Service `homepage.homepage.svc.cluster.local:8080` |
| `externalsecret.yaml` | the API keys as `HOMEPAGE_VAR_*`, from 1Password item `homepage-secret` |
| `httproute.yaml`, `snippetsfilter.yaml` | `home.cluster.integratn.tech` behind authentik forward-auth, and its http redirect |

Elsewhere:

- The namespace and its NetworkPolicies are in
  [`network-policies/homepage.yaml`](../network-policies/homepage.yaml). The
  other halves are the homepage entries in `allow-nginx-dataplane`
  (`nginx-gateway.yaml`), `allow-prometheus` (`monitoring.yaml`) and
  `allow-homepage-ingress` (`vcluster-media.yaml`, `umami.yaml`).
- The authentik provider is `08c-homepage-proxy-provider.yaml` in the blueprint
  ConfigMap (`clusters/the-cluster/addons/authentik/`), and it is on the
  embedded outpost's list in `09-embedded-outpost.yaml`.

## Editing the page

Edit `glance.yml` inside `configmap.yaml`, merge, then restart:
`kubectl -n homepage rollout restart deploy/homepage`. Glance reads its config
and environment once, at start.

- **Test before merging.** Run the same version locally with
  `glance --config glance.yml config:validate`, then serve it against a mock
  of the APIs and look at it; a template error renders as an error box on that
  widget only.
- **Everything about the cluster comes from Prometheus** through `custom-api`
  widgets, so a new tile is a PromQL query, not RBAC. Write counts
  `... or vector(0)`.
- **`${NAME}` in `glance.yml` must be set**, or Glance refuses to start. Add a
  default for every new name to `glance-env`.
- **A host outside the cluster** needs its host and port in
  `allow-lan-monitors` (`network-policies/homepage.yaml`). A new
  `*.cluster.integratn.tech` monitor needs nothing: the gateway is already
  allowed. The internet is open on 443 only, for the releases and news widgets.
- **Stay on brand.** Use the `hl-*` classes in `document.head`: `hl-tag ok|warn|bad`
  for a verdict word, `hl-kv` for a label/figure row, `hl-bar` with
  `warm`/`hot`, `hl-none` for an unreadable figure. Do not introduce new
  colours or put orange on anything but the mark.

## API keys

Sonarr, Radarr, SABnzbd, Umami, Plex, Proxmox and Unraid are read from their
own APIs. Their keys never touch `configmap.yaml`: `externalsecret.yaml` reads
1Password item `homepage-secret` whole (its fields are already named
`HOMEPAGE_VAR_*`), `deployment.yaml` loads it with an optional `envFrom` after
`glance-env`, and `glance.yml` refers to each as `${HOMEPAGE_VAR_<NAME>}`.

To add one:

1. Add the field to `homepage-secret` named `HOMEPAGE_VAR_<NAME>`, and an
   empty default to `glance-env`. For a field in another item, add a new
   ExternalSecret and `envFrom` entry rather than a line in the existing one:
   a Secret is written only when every field it reads resolves, so one bad
   field blanks every key in it.
2. Reference it in `glance.yml`.
3. Allow the egress. An in-cluster service needs both halves: this
   namespace's `allow-widget-apis` and the target namespace's ingress. A LAN
   host needs a line in `allow-lan-monitors`.
4. Restart the pod.

The media apps are called on the Services vcluster syncs into the host
namespace, `<app>-x-media-x-vcluster-media.vcluster-media.svc`, not through
`*.vcluster-media.integratn.tech`: the vcluster gateway's LoadBalancer only
admits traffic from outside the cluster.

The Umami tiles show visitors online now, and today's visitors and pageviews
against the same span yesterday. The website IDs are not secret (they ship in
each site's page source). The key is `HOMEPAGE_VAR_UMAMI_KEY`: a Umami API key
(Settings -> API keys, the whole `umami_...` string).
