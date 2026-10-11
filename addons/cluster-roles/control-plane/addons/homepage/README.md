# homepage

[Homepage](https://gethomepage.dev) is the cluster's landing page, at
`https://home.cluster.integratn.tech`, behind authentik. One screen that answers
"is anything wrong, and where do I go": Prometheus health tiles, cluster and
node usage, a status dot for every service's pods, and a link to every UI in
the cluster, the media vcluster and the house.

Grafana's Homelab Overview is the deep view; this page is the front door and
links into it.

It is plain manifests from the official image,
`ghcr.io/gethomepage/homepage`, pinned by tag and digest and tracked by Kargo
(target `homepage`, `autoMerge: minor`).

| File | What it is |
|---|---|
| `configmap.yaml` | the whole configuration: `settings`, `services`, `widgets`, `bookmarks`, `kubernetes`, plus the empty skeleton files |
| `deployment.yaml` | Homepage: one replica, read-only root filesystem, the Service `homepage.homepage.svc.cluster.local:3000` |
| `externalsecret.yaml` | `homepage-env`, the widget keys as `HOMEPAGE_VAR_*`, from 1Password |
| `rbac.yaml` | ServiceAccount and a read-only ClusterRole: namespaces, pods, nodes and `metrics.k8s.io` |
| `httproute.yaml`, `snippetsfilter.yaml` | `home.cluster.integratn.tech` behind authentik forward-auth, and its http redirect |

Elsewhere:

- The namespace and its NetworkPolicies are in
  [`network-policies/homepage.yaml`](../network-policies/homepage.yaml). The
  other halves are the homepage entries in `allow-nginx-dataplane`
  (`nginx-gateway.yaml`) and `allow-prometheus` (`monitoring.yaml`).
- The authentik provider is `08c-homepage-proxy-provider.yaml` in the blueprint
  ConfigMap (`clusters/the-cluster/addons/authentik/`), and it is on the
  embedded outpost's list in `09-embedded-outpost.yaml`.

## Editing the page

Edit `configmap.yaml` and merge. The ConfigMap is mounted as a directory (not
with `subPath`), so the kubelet refreshes it in the running pod within a
minute or so and Homepage re-reads it; no restart is needed.

- **A new service card** goes in `services.yaml`. Give it `namespace` and
  `app` (matched against `app.kubernetes.io/name`) for the pod status dot, or
  `podSelector` when the name is shared by several components.
- **A host outside the cluster** (a `siteMonitor`, or a widget that calls an
  API on the LAN) also needs its host and port added to `allow-lan-monitors`
  in `network-policies/homepage.yaml`. Without it the tile reads "Error" after
  a timeout. There is no internet egress at all.
- **A new health tile** is a PromQL query on one of the `prometheusmetric`
  cards. Write it `... or vector(0)` when an empty result means "none". The
  `percent` format divides by 100, so a 0-1 ratio needs `scale: 100`;
  `duration` takes seconds.

## API widgets

ArgoCD, Sonarr, Radarr, SABnzbd, Plex, Home Assistant, Unraid, Proxmox and
UniFi show live numbers from their own APIs. Their keys never touch
`configmap.yaml`: `externalsecret.yaml` builds the Secret `homepage-env` from
1Password (item `homepage-secret`, whose fields are already named
`HOMEPAGE_VAR_*`, plus `unifi-mcp-login` and `argocd-mcp-api-token`),
`deployment.yaml` loads it with `envFrom`, and `services.yaml` refers to each
as `"{{HOMEPAGE_VAR_<NAME>}}"`.

To add one:

1. Add the field to the 1Password item `homepage-secret` named
   `HOMEPAGE_VAR_<NAME>` (it is picked up whole), or a `data` entry in
   `externalsecret.yaml` for a field in another item.
2. Reference it in `services.yaml`.
3. Allow the egress. An in-cluster service needs both halves: this
   namespace's `allow-widget-apis` and the target namespace's ingress (see
   `argocd.yaml` and `vcluster-media.yaml` in network-policies). A LAN host
   needs a line in `allow-lan-monitors`.
4. Restart the pod (`envFrom` is read at start); a ConfigMap-only change
   needs no restart.

The media apps are called on the Services vcluster syncs into the host
namespace, `<app>-x-media-x-vcluster-media.vcluster-media.svc`, not through
`*.vcluster-media.integratn.tech`: the vcluster gateway's LoadBalancer only
admits traffic from outside the cluster.

## Gotchas

- **`HOMEPAGE_ALLOWED_HOSTS` must list every Host header the page is served
  under.** Anything else gets a 400 "Host validation failed". The pod IP is
  listed for the kubelet's probes.
- **The config mount is read-only, so every skeleton file must exist.**
  Homepage copies missing files in from its skeleton on start, and on a
  read-only mount that copy fails. Keep the empty `docker.yaml`,
  `proxmox.yaml`, `custom.css` and `custom.js`.
- **`LOG_TARGETS=stdout`** stops Homepage writing a `logs/` directory into the
  config mount.
