---
type: Workload
title: JellyPlex-Watched
description: The hourly CronJob in vcluster-media that keeps watched and in-progress state in step between Plex (owner boboysdadda) and Jellyfin (root), both on Tower.
tags: [media, vcluster-media, plex, jellyfin, cronjob]
status: stable
generated: { by: claude-code/claude-opus-5-5, at: 2026-11-05T15:00:00Z }
stale_after: 2027-02-05
sources:
  - id: values
    resource: ../../../workloads/vcluster-media/addons/jellyplex-watched/values.yaml
    title: jellyplex-watched values.yaml
  - id: addons
    resource: ../../../workloads/vcluster-media/addons.yaml
    title: workloads/vcluster-media addons.yaml
  - id: kargo
    resource: ../../../addons/cluster-roles/control-plane/addons/kargo-projects/values.yaml
    title: kargo-projects target list (jellyplex-watched)
  - id: upstream
    resource: https://github.com/luigi311/JellyPlex-Watched (v9.0.0, README and sample.config.yaml)
    title: Upstream docs
  - id: dryrun
    resource: JellyPlex-Watched 9.0.0 dry run against live Plex and Jellyfin from holocron (non-root, read-only rootfs), 2026-11-05
    title: Pre-merge dry run
---

# What it is

[JellyPlex-Watched](https://github.com/luigi311/JellyPlex-Watched) compares
the watch history on Plex and Jellyfin and writes the difference to each side:
watched flags and resume positions. Here it syncs **one person**: Plex owner
account `boboysdadda` and Jellyfin user `root`, in **both directions**, for the
**Movies** and **TV** libraries only.[^values]

It holds no state. Every pass fetches both full histories, so it runs as a
CronJob, like [configarr](configarr.md).

# Shape

| Aspect | Value |
|---|---|
| CronJob | `jellyplex-watched-sync`, namespace `media`, `15 * * * *` America/Denver, `run_only_once: true` |
| Image | `ghcr.io/luigi311/jellyplex-watched`, plain `N.N.N` tags (alpine build), no `v` prefix; Kargo target `jellyplex-watched` |
| Runs as | 1000:1000, read-only rootfs; log + mark files on a `/tmp` emptyDir. The entrypoint skips its gosu step when started non-root |
| Config | whole `config.yaml` rendered by ExternalSecret `eso-jellyplex-watched-config` into Secret `jellyplex-watched-config`, mounted at `/app/config` |
| Secrets | `HOMEPAGE_VAR_PLEX_TOKEN` and `HOMEPAGE_VAR_JELLYFIN_KEY` from 1Password item `homepage-secret` |
| Egress | `jellyplex-watched-allow-egress`: 10.0.0.12 on 32400 and 8096, plus :443 to non-RFC1918 for plex.tv |

**Mappings.** Plex library `TV Shows` maps to Jellyfin `TV`, and `Movies` to
`Movies`. Matching uses provider GUIDs and file **names**, not full paths, so
Plex's `/data/tv` and Jellyfin's `/data/tvshows` (the same Tower folder) need
no path mapping.[^values]

**plex.tv is required.** v9 resolves the Plex account and its users through
`myPlexAccount()`, which calls plex.tv over HTTPS. Without the :443 rule, the
user lookup fails.[^dryrun]

**Fixed Plex device ID.** python-plexapi defaults `X-Plex-Client-Identifier` to
the host's MAC address. Every CronJob pod has a new MAC, so each hourly run
showed up as a new device and Plex sent a "new device" alert to the owner.
`PLEXAPI_HEADER_IDENTIFIER` is pinned in the values file (with
`PLEXAPI_HEADER_DEVICE_NAME`), so Plex sees one device. Don't change the
identifier, or the alerts come back once.

**Settings load once at start.** Version 9 reads `config.yaml` at startup. A
CronJob starts fresh every run, so a config change applies at the next run.

# Dry run before go-live

On 2026-11-05 the 9.0.0 image ran with this config (`dryrun: true`) against the
live servers from holocron, as 1000:1000 with a read-only rootfs. It exited 0
with no errors or warnings. Changes it would make:[^dryrun]

| Direction | Movies | TV |
|---|---|---|
| Plex → Jellyfin | 627 watched, 25 progress | 5,739 watched, 47 progress (183 shows) |
| Jellyfin → Plex | 22 watched, 4 progress | 30 watched, 1 progress |

The large Plex → Jellyfin number is years of Plex history landing on a
Jellyfin account that was nearly empty. Spot checks were all real titles in
the right library. It **never marks anything unwatched**. About 50 items appear
twice (multiple files or versions), which is harmless. Small Plex resume points,
such as 2 minutes in, become Jellyfin resume points.

To pause writes without removing the job, set `dryrun: true` in the values file.
A dry run then logs every change it would make and writes nothing.

[^values]: [values.yaml](../../../workloads/vcluster-media/addons/jellyplex-watched/values.yaml)
[^dryrun]: Pre-merge dry run, 2026-11-05.
