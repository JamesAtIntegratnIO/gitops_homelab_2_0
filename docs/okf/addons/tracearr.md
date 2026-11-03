---
type: Workload
title: Tracearr
description: Plex and Jellyfin watch history and stream monitoring in vcluster-media, with its own TimescaleDB and Redis. Replaces Tautulli, which ran as a Docker container on Tower.
tags: [media, vcluster-media, plex, jellyfin, tautulli, timescaledb, redis]
status: stable
generated: { by: claude-code/claude-opus-5-5, at: 2026-11-02T23:30:00Z }
stale_after: 2027-02-01
sources:
  - id: values
    resource: ../../../workloads/vcluster-media/addons/tracearr/values.yaml
    title: tracearr values.yaml
  - id: addons
    resource: ../../../workloads/vcluster-media/addons.yaml
    title: workloads/vcluster-media addons.yaml
  - id: kargo
    resource: ../../../addons/cluster-roles/control-plane/addons/kargo-projects/values.yaml
    title: kargo-projects target list (tracearr, tracearr-redis)
  - id: upstream
    resource: https://github.com/connorgallopo/Tracearr (docker/helm/tracearr chart 3.1.3, docs.tracearr.com)
    title: Upstream chart and docs
  - id: recon
    resource: read-only checks on Tower and both clusters from holocron, 2026-11-02
    title: Pre-deploy reconnaissance
  - id: live
    resource: kubectl --context admin@media-cluster -n media (pods, logs, ExternalSecret) and curl from holocron, 2026-11-03
    title: Post-deploy verification
---

# What it is

[Tracearr](https://github.com/connorgallopo/Tracearr)
`ghcr.io/connorgallopo/tracearr` (3.1.3 at deploy) in namespace `media`, at
`https://tracearr.vcluster-media.integratn.tech`. It watches Plex
(Tower `10.0.0.12:32400`) and Jellyfin (Tower `:8096`) and keeps watch history,
stream and user statistics. It has its own login: the first account created on
a fresh install becomes the owner.

It replaces **Tautulli**, a Docker container on Tower (`tautulli/tautulli`,
port 8181, config `/mnt/user/appdata/tautulli`). On 2026-11-02 Tautulli held
3,991 plays from 2025-04-20 on; Tracearr imports them through Tautulli's API
(Settings → Import → Tautulli).

# How it is wired

Shaped after upstream's own Helm chart (`docker/helm/tracearr`), written out as
`extraObjects` on the stakater chart, because the addon factory cannot take a
chart from another repository's git path.

- **App:** a Deployment, `Recreate` (Tracearr is a singleton poller), UID 1001,
  port 3000, probes on `/health`. `TRUST_PROXY=true` and `CORS_ORIGIN` set to its
  own URL for the gateway hop.
- **TimescaleDB** (`tracearr-db`, StatefulSet + headless Service):
  `timescale/timescaledb-ha` pg18 with the `timescaledb` and
  `timescaledb_toolkit` extensions. Tracearr does not run on plain Postgres, so
  the shared Postgres on `10.0.3.1` (13.23, no Timescale) is not an option.
  Memory is sized down from upstream's 4 GiB to a 1.5 GiB limit.
- **Redis** (`tracearr-redis`): BullMQ job queues and caches. `noeviction` is
  required: eviction silently corrupts the queues. AOF only.
- **Storage:** one static Retain PV on
  `10.0.0.12:/mnt/user/kube_storage/tracearr`, split by `subPath`: `db`,
  `redis`, `app` (Tracearr's `/data/backup` and `/data/image-cache`).
- **Secrets:** ExternalSecret `eso-tracearr-secret` from 1Password item
  `tracearr` (vault `homelab`), fields `jwt-secret`, `cookie-secret`,
  `db-password`. The template builds `DATABASE_URL` with the password
  URL-encoded.
- **Egress:** `tracearr-allow-media-servers-egress` allows `10.0.0.12` on 32400
  (Plex), 8096 (Jellyfin) and 8181 (Tautulli, for the import). Without it the
  generated `allow-vcluster-external` lets nothing reach RFC1918 except NFS.
  Plex sign-in goes to plex.tv on 443, which is already allowed.
- **Kargo:** `tracearr` (ghcr semver, `minor`, because the app migrates its
  database on start) and `tracearr-redis` (`8.x-alpine`, `patch`). TimescaleDB
  is not tracked: its tags are not semver and a Postgres major needs a
  dump and restore.

# Gotchas

- **Postgres runs as 99:100.** Tower's exports squash every uid to `99:100`
  (`all_squash`, see [storage](../platform/storage.md)), and Postgres refuses a
  data directory owned by anyone but itself. Running as 99 makes the owner
  match; the timescaledb-ha entrypoint fakes the passwd entry initdb needs
  with nss_wrapper. Postgres on NFS is the accepted trade-off here; the write
  load is a few rows per poll.
- **The DB password is read once.** `POSTGRES_PASSWORD` only matters on the
  first start (initdb). Changing the 1Password field later also needs
  `ALTER ROLE tracearr PASSWORD ...` inside the database.
- **JWT_SECRET and COOKIE_SECRET**: losing them signs everyone out, and
  rotating `JWT_SECRET` also rotates the key derived for stored notification
  secrets.
- **Plex is added by plex.tv sign-in**, not URL + token: Settings → Servers →
  Link Plex Account, then add the server. Pick the local `10.0.0.12:32400`
  connection; the egress policy allows nothing else on 32400.
- **Jellyfin** needs an administrator API key (Jellyfin Dashboard → API Keys)
  and the URL `http://10.0.0.12:8096`.
- **Verified live 2026-11-03:** all three pods Running with 0 restarts; Postgres
  as 99:100 initialised its data directory and Tracearr's migrations ran clean;
  `/health` 200 through the gateway; from the pod Plex `/identity` 200, Jellyfin
  200, Tautulli 303 (login redirect).
- **A new folder on Tower is not served at once.** For about 3 minutes after
  `mkdir /mnt/user/kube_storage/tracearr`, mounts failed with `reason given by
  server: No such file or directory`; kubelet's retry then succeeded with no change.
- **Kyverno `restrict-image-registries`** (audit) reports `tracearr-db-0`,
  because `timescale/` is not an approved Docker Hub organisation. If that policy
  moves to Enforce, add `timescale` first or the database will not start.
- **`BETTER_AUTH_SECRET`** is unset, so Tracearr derives it from `JWT_SECRET`.
  A dedicated field in the 1Password item would decouple them.
- **Tautulli import (2026-11-03):** 2,886 plays imported in 37 s, 77 skipped,
  all from user `Brammage`, who is no longer a Plex user Tracearr knows.
  Tautulli's 3,991 `session_history` rows are 2,963 plays once resumes are
  grouped. Tautulli was then stopped and removed from Unraid autostart
  (backup `/var/lib/docker/unraid-autostart.bak-20261102173021`); its
  container and `/mnt/user/appdata/tautulli` are kept, so the 77 can still be
  imported if Brammage reappears (re-running the import skips duplicates).
  Tauticord, the Discord bot that read from Tautulli, was stopped and removed
  from autostart the same way (backup `.bak-20261102175610`), container kept.
- **Jellyfin is polled**, not pushed: without the Tracearr SSE plugin in
  Jellyfin, `/api/sse/events` 404s every 3 minutes ("Plugin diagnosis ...
  missing") and Tracearr falls back to polling. Harmless.
- **Before first sync:** `/mnt/user/kube_storage/tracearr` must exist on Tower
  (NFS cannot mount a missing path) and the 1Password item must exist, or the
  pods sit in `ContainerCreating`.
