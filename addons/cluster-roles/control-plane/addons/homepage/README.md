# homepage

The cluster's landing page, at `https://home.cluster.integratn.tech`, behind
authentik. It runs [Glance](https://github.com/glanceapp/glance); the addon,
namespace, Deployment and Kargo target keep the name `homepage` from the
Homepage (gethomepage.dev) it replaced, so the authentik provider and the
NetworkPolicy halves in other namespaces did not move.

Five tabs. **Home** answers "is anything wrong": a status line with a verdict
and chips (critical alerts, warnings, unhealthy apps, failed promotions, stuck
pods), CPU and memory sparklines, per-node cards and the N+1 request line,
firing alerts by name, ArgoCD and Kargo problems, Umami for integratn.io and
bosun.integratn.io, one line per host outside the cluster, upstream releases,
and a launcher for every UI. **Needs you** has the detail behind every chip
and row (below). **Hosts** has the machines outside the cluster (below).
**Media** has TV and films (only what is coming, plus anything aired or
released and not downloaded, flagged Missing and linked to its Sonarr/Radarr
page), Plex, and SABnzbd (flags a paused queue and this week's failed
downloads). **News** has Hacker News, Lobsters and RSS.

Grafana's Homelab Overview is the deep view; this page links into it.

## Needs you

Every chip in the status line, and every row of Home's Firing alerts, opens
the **Needs you** tab at that section or alert (`/needs-you#needs-pods`,
`/needs-you#alert-<name>`; a small script in `document.head` scrolls there
once the widget has loaded). Delivery's rows open the ArgoCD app filtered to
its bad resources and the Kargo promotion itself. Each item there is one specific thing, says what
is wrong in a sentence, and links straight to where it is fixed, never to a
whole dashboard or an unfiltered list:

| Section | One item per | Says | Links |
|---|---|---|---|
| Alerts | active alert instance that is not silenced or inhibited, critical then warning | the rule's `summary` and `description`, the alert name and its namespace/object, how long it has fired | Kargo promotion and stage (`promotion` label, `kargo_url`), the ArgoCD app filtered to its unhealthy resources (`ArgoCD*` alerts), Loki logs for the pod (`namespace`/`pod` or `exported_*`), `runbook_url`, the rule's query, and a prefilled Alertmanager silence |
| Argo apps | app not Healthy or not Synced | what that health or sync state means here | the app's tree filtered to the Degraded/Missing/... or OutOfSync resources, plus `argocd app get` |
| Kargo promotions | stage whose last promotion Errored, Failed or Aborted | what that phase means and what to do next | the promotion itself (its failing step and error) and the stage |
| Stuck pods | pod older than 5 minutes that is crash-looping, cannot pull, cannot start, unschedulable, stuck creating, Pending, Failed or Unknown | the reason in plain words, age and restarts | Loki logs for the pod, plus `kubectl describe` |

Alerts are read from Alertmanager (`AM_URL`, `/api/v2/alerts?silenced=false&inhibited=false`),
not Prometheus, so silencing an alert is how to take it off the page; the
Critical and Warnings chips and Home's Firing alerts read the same set. If Alertmanager does not
answer, the list says so and the chips fall back to Prometheus `ALERTS`. The
stuck-pod PromQL appears twice, in the Status chip and in Needs you; keep the
two in step.

## Hosts

Everything the cluster does not run on, all read from Prometheus:

| Card | Source |
|---|---|
| Machines: holocron, pitdroid, tower | node_exporter, `job="host-node"`, one `host` label each. holocron and pitdroid push it themselves (nix_dots_2 `host-metrics.nix`, a Prometheus agent remote-writing to `prom-remote`); Tower is scraped by the `host-exporters` addon's ScrapeConfig. CPU, memory (on macOS Activity Monitor's "Memory Used"), every real filesystem, uptime. A host that never reported says what to install or deploy. |
| CI runners | `github_runner_up` / `github_runner_busy` per instance, written each minute by nix_dots_2's runner-metrics job into the textfile collector. |
| Proxmox | `pve_*` from pve-exporter (`host-exporters`): node CPU/memory, storage pools, every guest with Running / Stopped (set to start at boot, so wrong) / Off. |
| Tower array | Unraid's own GraphQL API (`HOMEPAGE_VAR_UNRAID_KEY`): array state and capacity. |
| UniFi | `unpoller_*` from unpoller (`host-exporters`): health per subsystem, internet latency and drops, clients, devices offline, firmware updates. |

Each card has an id (`#host-holocron`, `#host-proxmox`, ...), which Home's
Hosts lines and the host alerts' `action_url` open. Problems are alerts
(`platform-alerts/host-rules.yaml`), so they reach Needs you like any other;
an alert's `action_url` / `action_title` annotations become a button there.

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
  (`nginx-gateway.yaml`), `allow-prometheus` and `allow-alertmanager` (`monitoring.yaml`) and
  `allow-homepage-ingress` (`vcluster-media.yaml`, `umami.yaml`).
- The authentik provider is `08c-homepage-proxy-provider.yaml` in the blueprint
  ConfigMap (`clusters/the-cluster/addons/authentik/`), and it is on the
  embedded outpost's list in `09-embedded-outpost.yaml`.

## Editing the page

Edit `glance.yml` inside `configmap.yaml`, bump the
`homepage.integratn.io/config-revision` annotation in `deployment.yaml` in the
same commit, and merge; the pod rolls with the new config.

- **On a phone** Glance shows one column at a time and switches with a bottom
  bar. Its stock dots are relabelled per tab in `document.head` (Home:
  LAUNCH, CLUSTER, HOSTS; Media: COMING, NOW; News: LINKS, FEEDS), by page
  slug and column index; keep them in step if columns move.
- **Icons** load in the browser from jsDelivr. `di:` names must exist in
  homarr-labs/dashboard-icons (some only as png: `di:trivy.png`); Kargo has
  none, so it uses an `mdi:` icon.

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

## Gotchas

- **Removing a field from the Deployment does not clear it live.** The pod
  spec names `serviceAccountName: default` explicitly because the Homepage
  version's `homepage` account survived in the live object after its
  ServiceAccount was pruned, and every new pod was refused.
- **`*.cluster.integratn.tech` from inside the pod is the gateway VIP,
  10.0.4.205.** `allow-gateway-egress` names both the VIP and the data-plane
  pods; with only the pods, the monitors time out and read as down.
