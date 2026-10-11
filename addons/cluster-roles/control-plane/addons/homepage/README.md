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

## Adding an API widget

Most service widgets (ArgoCD, Sonarr, Radarr, SABnzbd, Plex, Proxmox, UniFi,
Home Assistant) need an API key. Never put one in `configmap.yaml`. Instead:

1. Put the key in 1Password (vault `homelab`).
2. Add an `ExternalSecret` in this directory against the `onepassword-store`
   ClusterSecretStore, producing a Secret `homepage-env` with keys named
   `HOMEPAGE_VAR_<NAME>`, and add the ExternalSecret `ignoreDifferences` block
   from the umami entry in `addons.yaml`.
3. Load it in `deployment.yaml` with `envFrom: [{secretRef: {name: homepage-env}}]`.
4. Reference it in `services.yaml` as `key: "{{HOMEPAGE_VAR_<NAME>}}"`.
5. Allow the egress: an in-cluster service needs a NetworkPolicy pair (this
   namespace's egress and the target's ingress); a LAN host needs a line in
   `allow-lan-monitors`. The vcluster-media apps are reached through the
   vcluster's gateway at 10.0.4.201 on 443.

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
