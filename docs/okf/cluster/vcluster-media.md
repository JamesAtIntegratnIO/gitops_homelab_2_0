---
type: Virtual Cluster
title: vcluster-media
description: The one live tenant vcluster — prod-preset HA control plane with external etcd, its own ArgoCD/gateway/DNS, running the media stack.
tags: [vcluster, media, tenant]
resource: https://media.integratn.tech:443
status: stable
generated: { by: claude-code/claude-opus-5-5, at: 2026-11-25T20:30:00Z }
stale_after: 2026-12-08
sources:
  - id: request
    resource: ../../../platform/vclusters/vcluster-media.yaml
    title: VClusterOrchestratorV2 request
  - id: live-vc
    resource: kubectl get vco,pods,svc -n vcluster-media / platform-requests, 2026-08-20
    title: Live vcluster state
  - id: workloads
    resource: ../../../workloads/vcluster-media/
    title: workloads/vcluster-media (addons.yaml + app values)
---

# Spec (from the ResourceRequest)

Requested via [vcluster-orchestrator-v2](/promises/vcluster-orchestrator-v2.md):
preset **prod** — 3 control-plane replicas (vcluster-oss 0.37.6, K8s v1.36.6 inside, matching the host; see the [upgrade
ladder](../../../platform/vclusters/upgrades.md), supported to 2026-12-08), dedicated **3-replica etcd** (registry.k8s.io/etcd:3.6.16-0,
cert-manager-issued certs, client-cert-auth disabled), 2Gi memory, exposed at
`media.integratn.tech:443`, NFS egress enabled plus a postgres egress rule to
`10.0.3.1:5432`.[^request]

# Live footprint on the host (2026-08-20)

- 41 pods in namespace `vcluster-media`: 3 control-plane + 3 etcd
  StatefulSet pods, plus **synced** pods (suffix `-x-<ns>-x-vcluster-media`)
  for everything running *inside* the vcluster.
- LoadBalancer IPs: API `10.0.4.200`, its own nginx-gateway `10.0.4.201`.
- Storage: 3×10Gi control-plane + 3×5Gi etcd PVCs, plus synced app PVCs.

# Inside the vcluster

Its own ArgoCD (with dex), cert-manager, external-dns, external-secrets,
kyverno, nginx-gateway-fabric, prometheus agent + kube-state-metrics
(remote-writing to the hub with `cluster=vcluster-media` labels), CoreDNS ×2 —
all delivered by the [vcluster-role addons](/addons/addon-inventory.md)
rendered on the host. Registered in host ArgoCD as cluster
`vcluster-media` (labels `cluster_role: vcluster`, `environment: production`).

**Workloads** (namespace `media`, defined in
[workloads/vcluster-media/](../../../workloads/vcluster-media/), deployed by
the vcluster's own ArgoCD): **sonarr, radarr, sabnzbd, otterwiki** — each a
Stakater application with config/media PVCs; the *arr apps use the external
Postgres at 10.0.3.1 (datacron, Postgres 18 since 2026-11-05).[^workloads]
Sonarr runs the **v5 beta** (linuxserver `develop-*` tags) since 2026-10-31; its
schema migration is one-way, so a downgrade needs a database restore.

Sonarr and Radarr each have an **Emby / Jellyfin connection** ("Jellyfin
(Tower)", Settings → Connect) pointing at `10.0.0.12:8096`, with *Update
Library* on import, upgrade, rename and delete, so Jellyfin refreshes the
affected path right away instead of at its scheduled library scan. The
connection lives in each app's Postgres database, not in git; git holds only
the `<app>-allow-jellyfin-egress` NetworkPolicy it needs. Its API key is the
`HOMEPAGE_VAR_JELLYFIN_KEY` field of 1Password item `homepage-secret` (the
same key the Glance Jellyfin widget reads). To recreate it, `POST
/api/v3/notification` with `implementation: MediaBrowser`, `host: 10.0.0.12`,
`port: 8096`, `updateLibrary: true`.

**Bazarr** (subtitles, since 2026-11-01) is a Stakater Deployment too, but it
keeps its state in a **SQLite** database on its NFS config volume rather than
in Postgres. NFS does not give SQLite working locks, so it runs with
`strategy: Recreate` to keep rollouts to one writer; a rolling update during
a Kargo bump corrupted the DB on 2026-11-04. See [Bazarr](/addons/bazarr.md).

**configarr** is the fifth entry and the odd one out: the same Stakater chart,
but rendered as a **CronJob** (`configarr-sync`, 04:00 America/Denver) rather
than a Deployment, with `deployment.enabled` and `service.enabled` both forced
off. It holds no state — no PVC, just `emptyDir` for its git clone cache — and
its whole job is to reconcile Sonarr's and Radarr's **custom formats and
quality profiles** from the TRaSH-Guides via their REST APIs, so that config
lives in git instead of only in each app's database. Media arrives at whatever
resolution is available and upgrades until it reaches 2160p. It reads the two
API keys
from the existing `homepage-secret` and needs an egress NetworkPolicy for
:443, because it clones the TRaSH-Guides and recyclarr template repos on every
run. See [configarr](/addons/configarr.md).

**jellyplex-watched** is a second CronJob on the same chart
(`jellyplex-watched-sync`, hourly at :15). It syncs watched and in-progress
state both ways between Plex owner `boboysdadda` and Jellyfin `root` for Movies
and TV. Its credentials are the Plex token and Jellyfin key in
`homepage-secret`. Its egress policy allows Tower on 32400 and 8096, and :443
for plex.tv. See [JellyPlex-Watched](/addons/jellyplex-watched.md).

# Sign-in on the web UIs

Sonarr, Radarr, Bazarr and SABnzbd sit behind **authentik forward-auth**,
group `media-management`; each app's own login is off (Sonarr/Radarr
`AuthenticationMethod: External`, Bazarr none, SABnzbd no username), so there
is one sign-in. `/api` on each host is left outside forward-auth for API-key
clients (phone apps, scripts) and is still guarded by the app's API key.
In-cluster callers (Homepage widgets, arr-mcp, exportarr) use the synced
Services and never touch the gateway. The pieces:

- `<app>-forward-auth` SnippetsFilter (namespace `media`) in each app's
  values, referenced by the `/` rule of its HTTPRoute. The vcluster's
  nginx-gateway-fabric has `nginxGateway.snippetsFilters.enable: true`
  (`nginx-gateway-fabric-vcluster` in the vcluster cluster-role addons).
- The outpost is authentik's embedded one on the HOST. The vcluster reaches it
  as `authentik-server.nginx-gateway.svc.cluster.local`, a Service replicated
  from host `authentik/authentik-server` by
  `networking.replicateServices.fromHost` in the ResourceRequest's
  `helmOverrides` (lists replace in the promise's merge, so `default/kubernetes`
  is repeated there).
- Host NetworkPolicies: `allow-gateway-authentik-egress` in
  `vcluster-media` and the vcluster-media rule of `authentik-server-ingress`.
- authentik blueprint `08d-media-proxy-providers.yaml` (one `forward_single`
  provider + application per host), the embedded outpost list (`09`), and the
  `media-management` bindings (`11`).

Left out on purpose: otterwiki, tracearr and the vcluster's ArgoCD (own
logins; forward-auth would be a second one), and Jellyfin/Plex (on Tower;
their TV and phone clients cannot do a browser sign-in).

# Access

- kubeconfig: ExternalSecret `vcluster-media-kubeconfig` (1Password item of
  the same name), or `hctl vcluster kubeconfig vcluster-media`.
- Status: `kubectl get vco vcluster-media -n platform-requests` — the
  [status contract](/promises/kratix.md) reports Ready/ArgoSynced/PodsReady/
  KubeconfigAvailable. It read `Ready` on 2026-11-01.[^live-vc]

[^request]: VClusterOrchestratorV2 request
[^live-vc]: Live vcluster state
[^workloads]: workloads/vcluster-media (addons.yaml + app values)
