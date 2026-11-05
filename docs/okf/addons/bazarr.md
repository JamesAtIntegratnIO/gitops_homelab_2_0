---
type: Workload
title: Bazarr
description: Subtitle manager in vcluster-media. It reads Sonarr's and Radarr's libraries and writes external subtitles (mostly English, from OpenSubtitles.com) next to the media files that Jellyfin on Tower serves.
tags: [media, vcluster-media, subtitles, opensubtitles, sonarr, radarr]
status: stable
generated: { by: claude-code/claude-opus-5-5, at: 2026-11-05T17:45:00Z }
stale_after: 2027-02-01
sources:
  - id: values
    resource: ../../../workloads/vcluster-media/addons/bazarr/values.yaml
    title: bazarr values.yaml
  - id: addons
    resource: ../../../workloads/vcluster-media/addons.yaml
    title: workloads/vcluster-media addons.yaml
  - id: kargo
    resource: ../../../addons/cluster-roles/control-plane/addons/kargo-projects/values.yaml
    title: kargo-projects target list (bazarr, bazarr-yq)
  - id: live
    resource: live Bazarr API + pod logs via kubectl --context admin@media-cluster, 2026-11-01/02
    title: Post-deploy verification
  - id: incident
    resource: live Bazarr API, pod logs and /config/log/bazarr.log via kubectl --context admin@media-cluster, 2026-11-05
    title: Corrupt-DB diagnosis
---

# What it is

[Bazarr](https://www.bazarr.media/) `ghcr.io/linuxserver/bazarr` (v1.6.2 at
deploy) in namespace `media`, at `https://bazarr.vcluster-media.integratn.tech`.
It has no login of its own (`auth.type: null`), the same exposure as
Sonarr and Radarr on the LAN-only vcluster gateway. It was added in #580, with
fixes in #585 (init `/tmp`) and #595 (AniDB egress).

# How it is wired

- **Config** `/config` is a static Retain PV on
  `10.0.0.12:/mnt/user/kube_storage/bazarr-config-pvc`. That folder came from an
  earlier Bazarr install (January 2026) and was reused, including its SQLite DB
  (`postgresql.enabled: false`) and its stored OpenSubtitles.com login. The
  folder is owned by 99:100 on Tower, so the init container's `chown` is a no-op.
- **Media** `/data` uses the same `/mnt/user/data` share and the same path as
  Sonarr and Radarr, so no path mappings are needed. Jellyfin sees the same
  files at `/data/tvshows`. Bazarr's `.srt` files land next to the media, where
  Jellyfin picks them up.
- **Init container `bazarr-config`** (`mikefarah/yq`) rewrites the git-owned
  keys in `config/config.yaml` on every start:
  - `use_sonarr`/`use_radarr`
  - `sonarr:8989` and `radarr:7878` (the old config pointed at
    `*.media.svc.cluster.local:80`)
  - the API keys from `homepage-secret` (`HOMEPAGE_VAR_SONARR_KEY`,
    `HOMEPAGE_VAR_RADARR_KEY`)
  - `opensubtitlescom` in `enabled_providers`
  
  An optional Secret `bazarr-opensubtitles` (username/password) would override
  the stored login. No ExternalSecret creates it, because no OpenSubtitles item
  exists in the 1Password `homelab` vault. The only opensubtitles.com login in
  1Password is in James's Private vault.
- **UI-owned:**
  - language profile `English` (id 1: en, not forced, HI off)
  - `serie_default_enabled` and `movie_default_enabled` set to **false**, so
    new items get no profile until one is assigned
  - providers: `opensubtitlescom` only. `animetosho` was removed because the
    stored AniDB client name is invalid ("Client is disabled or does not
    exists"), and that error aborts every anime search before any provider runs.
    Re-adding it needs a registered AniDB client in `anidb.api_client`.
- **Egress:** OpenSubtitles is internet :443, which the generated
  `allow-vcluster-external` allows. AniDB's API is `api.anidb.net:9001`, which
  needs `bazarr-allow-anidb-egress`. Without AniDB, `get_video()`'s refiners
  fail, and anime searches return nothing from *any* provider.
- **Kargo:** target `bazarr` (linuxserver NewestBuild, autoMerge always) and
  `bazarr-yq` (Docker Hub, 6h).

# Gotchas

- **OpenSubtitles account:** VIP, 1000 downloads/day (resets 23:59:59 UTC).
  Jellyfin's Open Subtitles plugin on Tower uses the same account. On first
  sync, Bazarr hit `DownloadLimitExceeded` after 21 downloads anyway (cause
  unknown). Saving settings resets Bazarr's provider throttles.
- **Season 0 specials:** the opensubtitlescom provider drops `season_number`
  when it is 0 (`if self.video.season:`), so an S00Exx search asks for episode
  x of every season. OpenSubtitles often files anime specials as S01E00.
- **Season splits:** OpenSubtitles splits Fate/stay night UBW into S01 (1–13)
  and S02 (1–12), while Sonarr has a single 25-episode season. Bazarr can't
  map between them, so episodes 13–25 never match automatically (OS
  S02E01–13 = Sonarr S01E13–25). They were fetched by hand: Crunchyroll
  English on the FRNcR CR WEB-DL files, checked against the embedded
  Portuguese track.
- **Sonarr numbering:** UBW is a `standard` series, so absolute-numbered BD
  releases (e.g. Moozzi2 "- 15") are rejected as "Wrong season", and UTW titles
  parse as Unknown quality. E15 and E20 were grabbed manually: UTW 720p mp4
  with embedded English, which Bazarr counts through `use_embedded_subs`.
- **`minimum_score: 88`:** fansub results for the UBW BluRay files score 86,
  so automatic search rejects them. The threshold was left alone on purpose,
  because TV/fansub timings often don't fit BD files.
- **BD vs TV cuts:** BD releases add scenes and the OP, so subtitles timed to
  TV or a different release need piecewise offsets, not one global shift.
  Bazarr's upload endpoint runs ffsubsync, which applies one global shift and
  undoes a piecewise alignment. Write the `.srt` next to the media and rescan
  instead. The 2026-11-02 UBW work, with scripts and the local whisper.cpp
  gap-fill for BD-only scenes, is in `hack/subtitles/README.md`.
- **HI naming:** uploads and some provider downloads are saved as `.en.hi.srt`
  even with HI off. Cause not found.
- **Old DB:** during the first sync after reuse there were `FOREIGN KEY
  constraint failed` errors on episode insert (e.g. The Conners S07). Not
  investigated.
- **SQLite on NFS, one writer only.** `bazarr.db` and its `-wal`/`-shm` live
  on Tower's NFS export, where SQLite's locks do not hold. `ReadWriteOnce` does
  not stop a second pod mounting an NFS volume either. The Deployment therefore
  uses `strategy: Recreate`: with one replica the chart's default
  RollingUpdate (25%/25%) rounds to surge 1, unavailable 0, so every rollout
  ran the old and new Bazarr on the same database for a while.

# Incident: corrupted database (2026-11-04/05)

Kargo merged the `v1.6.2-ls371` bump (#626) at 04:57 MST on 2026-11-04 and the
new pod started at 05:01 beside the old one. From 19:30 that day the log shows
`sqlite3.DatabaseError: database disk image is malformed` (first seen on
`table_settings_notifier`). The double writer is the probable cause, inferred
from the timeline rather than proven.

What it looked like: the pod stayed `1/1 Running` with no restarts, and
OpenSubtitles was `Good` and unthrottled. But from 04:16 the next morning
every 15-minute "Sync with Sonarr/Radarr" logged `skipped: maximum number of
running instances reached (1)`. `/api/system/tasks` showed 7 of 12 jobs with
`job_running: true`, `/api/badges` and `/api/system/health` hung past 60 s, and
`/api/system/settings` returned 500. The web server answers probes while the
scheduler's workers are wedged, so nothing restarts it. A restart alone only
clears the hung jobs until they reach the damaged pages again.

There was no Bazarr DB backup: the weekly backup job runs Sundays 03:00 and
the folder had been in use only since 2026-11-01.

Recovery (run on Tower against the config folder, with the Deployment at 0
replicas and no Bazarr pod running). This is what was done on 2026-11-05:

1. Copy `db/bazarr.db`, `bazarr.db-wal` and `bazarr.db-shm` to a dated folder
   such as `db-corrupt-YYYYMMDD/pristine/` with `cp -p`, check the md5s, and
   **never open that copy**. Opening a SQLite file that has a `-wal` beside it
   checkpoints the WAL into it and deletes the WAL, even for a read-only
   `PRAGMA integrity_check`. The first attempt's backup was changed that way.
2. Work on a second copy in Tower's `/tmp` (RAM). Through the Unraid user
   share (`/mnt/user`, shfs/FUSE) `integrity_check` and `.recover` crawled
   for minutes; from `/tmp` `.recover` took about 2 s. There is no tmux on
   Tower, so run it detached:
   `nohup sh -c 'sqlite3 /tmp/bz/bazarr.db ".recover" | sqlite3 /tmp/bz/recovered.db' &`.
3. `PRAGMA integrity_check` on the result (must print `ok`) and compare the
   row count of every table in `sqlite_master` against the original. The
   language profiles live in `table_languages_profiles`.
4. Delete what `.recover` salvaged from damaged pages if it is not real data.
5. Swap it in as `db/bazarr.db`, delete the old `-wal`/`-shm`, `chown 99:100`,
   `chmod 644`.
6. Scale back to 1 and check `/api/system/settings` returns 200, no job is
   stuck in `/api/system/tasks`, `/api/badges` and `/api/system/health`
   answer, the English profile (id 1) and `opensubtitlescom` are still there,
   and a Sonarr sync completes.

Outcome on 2026-11-05: every table came back with identical row counts (275
shows, 18,191 episodes, 68,593 episode subtitles, 2,794 movies, 15,063 movie
subtitles, 216 history rows, the English profile) except
`table_settings_notifier`, the damaged one. Its 451 rows were unreadable and
`.recover` produced two garbage rows (names `2` and `3`), which were deleted.
Bazarr re-seeds that table from Apprise on startup (129 rows), so only any
configured notification URLs were lost. After the restart every endpoint
answered 200, the startup Sonarr sync finished in about a minute and Radarr's
in seconds, and no errors were logged. The originals are kept in
`db-corrupt-20261105/pristine/` and the recovered file as
`db-corrupt-20261105/recovered-20261105.db`.

The switch to `Recreate` itself needed a one-off patch. The live Deployment
still carried the `strategy.rollingUpdate` block the API server had defaulted.
No field manager owns it, so ArgoCD's server-side apply left it in place and
every sync failed with `spec.strategy.rollingUpdate: Forbidden: may not be
specified when strategy type is 'Recreate'`. Sonarr had hit the same thing.
The fix is
`kubectl patch deploy <app> --type json -p '[{"op":"replace","path":"/spec/strategy","value":{"type":"Recreate"}}]'`,
which does not restart the pod.

If recovery fails, starting from an empty DB is possible. Bazarr rebuilds its
series and movie tables from Sonarr and Radarr and rescans the `.srt` files on
disk. What is lost is the history, the UI-owned language profile, provider
settings and the stored OpenSubtitles login.

Bazarr can also use PostgreSQL (`postgresql.enabled`), as Sonarr and Radarr do
on 10.0.3.1. That would take SQLite off NFS entirely. It has not been done.
