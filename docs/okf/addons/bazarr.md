---
type: Workload
title: Bazarr
description: Subtitle manager in vcluster-media. It reads Sonarr's and Radarr's libraries and writes external subtitles (mostly English, from OpenSubtitles.com) next to the media files that Jellyfin on Tower serves.
tags: [media, vcluster-media, subtitles, opensubtitles, sonarr, radarr]
status: stable
generated: { by: claude-code/claude-opus-5-5, at: 2026-11-02T01:30:00Z }
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
  - providers `opensubtitlescom` + `animetosho`
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
  x of every season. OpenSubtitles often files anime specials as S01E00. For
  Fate/stay night UBW, the Prologue (Sonarr S00E01, IMDb tt3782680) was fetched
  by hand from OpenSubtitles (HorribleSubs "- 00") and uploaded through
  `POST /api/episodes/subtitles`. Subsync shifted it +0.88 s to the ADE BluRay,
  and the median offset against the embedded Chinese track is 0.02 s.
- **HI naming:** uploads and some provider downloads are saved as `.en.hi.srt`
  even with HI off. Cause not found.
- **Old DB:** during the first sync after reuse there were `FOREIGN KEY
  constraint failed` errors on episode insert (e.g. The Conners S07). Not
  investigated.
