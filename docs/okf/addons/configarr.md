---
type: Workload
title: configarr
description: The daily CronJob in vcluster-media that reconciles Sonarr's and Radarr's custom formats and quality profiles from the TRaSH-Guides, so that config lives in git rather than only in each app's database.
tags: [media, vcluster-media, custom-formats, trash-guides, cronjob]
status: stable
generated: { by: claude-code/claude-opus-5, at: 2026-11-08T18:20:00Z }
stale_after: 2027-02-08
sources:
  - id: values
    resource: ../../../workloads/vcluster-media/addons/configarr/values.yaml
    title: configarr values.yaml
  - id: addons
    resource: ../../../workloads/vcluster-media/addons.yaml
    title: workloads/vcluster-media addons.yaml
  - id: kargo
    resource: ../../../addons/cluster-roles/control-plane/addons/kargo-projects/values.yaml
    title: kargo-projects target list
  - id: dryrun
    resource: configarr 1.30.2 DRY_RUN against live sonarr/radarr via port-forward, 2026-08-28
    title: Pre-merge dry run
  - id: cutoff-audit
    resource: live sonarr/radarr API (qualityprofile, wanted/cutoff, release) via port-forward, 2026-08-29
    title: cutoff-unmet and mediaInfo measurement behind the Bluray-1080p cutoff
  - id: tv-1080p
    resource: live sonarr API (qualityprofile item order, release?episodeId for Tracker S04E05), 2026-11-02
    title: TV made 1080p-only; 2160p tiers rank below 1080p in WEB-1080p
  - id: sonarr-profiles-1108
    resource: live sonarr API (qualityprofile, series qualityProfileId, config/mediamanagement), 2026-11-08
    title: Sonarr profiles after the 1080p move
---

# What it is

[configarr](https://github.com/raydak-labs/configarr) reads a single
declarative `config.yml` and writes **custom formats**, **quality profiles**
and **quality definitions** into Sonarr and Radarr over their REST APIs. It is
the answer to "the arr config only exists in each app's database": the desired
state is now a ConfigMap in git, and a daily job makes reality match it.[^values]

It is a config *reconciler*, not a service — no UI, no port, no state. It
runs, calls the APIs, and exits.

# Shape

Deployed as the fifth entry in
[workloads/vcluster-media/addons.yaml](../../../workloads/vcluster-media/addons.yaml),
on the same `stakater/application` chart as its siblings, but it is the only
one that renders a **CronJob** rather than a Deployment.[^addons]

| Aspect | Value |
|---|---|
| CronJob | `configarr-sync` (name = `applicationName` + job key), namespace `media` |
| Schedule | `0 4 * * *`, `timeZone: America/Denver` |
| Image | `ghcr.io/raydak-labs/configarr` — tags carry **no `v` prefix** |
| Storage | none — `emptyDir` for `/app/repos` (git cache) and `/tmp` |
| Secrets | `SONARR_API_KEY` / `RADARR_API_KEY` from the existing `homepage-secret` |
| Egress | own NetworkPolicy, :443 to non-RFC1918 — it clones two git repos per run |

Because the chart defaults `deployment.enabled` and `service.enabled` to
`true`, both are explicitly set to `false`. Leaving either at its default
would render an empty Deployment and a Service pointing at nothing.

# What it manages

Radarr has **two profiles** plus TRaSH's anime profile; **Sonarr is 1080p only**
since 2026-11-02:

| App | Library default (1080p) | Opt-in 4K | Anime |
|---|---|---|---|
| Sonarr | `WEB-1080p` | — (removed 2026-11-02) | `Remux-1080p - Anime` |
| Radarr | `HD Bluray + WEB` | `UHD Bluray + WEB` | `Remux-1080p - Anime` |

The **default** profile has no allowed 2160p tier. Disallowed qualities still
sit in the profile, and Sonarr ranks them **below** 1080p: on `WEB-1080p` a
2160p file reads as cutoff-unmet and a 1080p release is an approved upgrade
(checked live on Tracker S04E05). That is what Radarr's opt-in profile
exists to prevent for 4K movies. For TV it is now the point: James wants every
series at 1080p after 4K HEVC played out of sync on the Living Room TV, so
the series on Sonarr's old `WEB-2160p` (56) and `Any - exclude looniversity`
(3) move to `WEB-1080p`, and their 4K episodes are replaced by searching.
On 2026-11-02 that was 1,718 episode files in 75 series, about 10.5 TB. Sonarr
only swaps a file after the new one imports, and the Media Management recycle
bin (`/data/media/.sonarr-recycle`, 7-day cleanup) catches what it replaces,
which was verified on Tracker S04E05.[^tv-1080p] configarr does not delete
profiles it no longer manages, so the old `WEB-2160p` profile was deleted from
Sonarr by hand. As of 2026-11-08 TV is 1080p-only: 267 of 277 series are on
`WEB-1080p` and the other 10 are on `Any`, which allows nothing above 1080p.
Sonarr's built-in `Ultra-HD` profile (id 5) still exists and still allows
2160p, but no series uses it.[^sonarr-profiles-1108]

**`until_score` is effectively disabled (`-100000`).** No file falls below it,
so nothing is ever flagged for replacement on score alone; quality upgrades
still work, because a file below the *quality* cutoff is still searched.

It was briefly `-1000`, to purge DV and DTS out of existing files. **That does
not work, and the reason is worth keeping**: every TRaSH DV and DTS custom
format is a `ReleaseTitleSpecification` — a regex over the release *name*. It
can refuse a release that advertises DTS; it can never see the audio track of a
file already on disk. Measured over one night on this library: of 131 Radarr
imports, **0** had DTS in the title and **14 (10.7%) had actual DTS audio**.
Library DTS went **839 → 844** while ~1,500 grabs churned through the queue,
and the 236 DV files that never declared themselves were never even flagged.
DV did fall 236 → 213, because DV is advertised in titles more consistently.

The lesson generalises: **custom formats are a download filter, not a library
auditor.** For the library side see
[scripts/arr-audio-audit.py](../../../scripts/arr-audio-audit.py), which reads
`mediaInfo` instead — the only place the real codec is recorded.

**The cutoff names the lowest acceptable 1080p tier, not the template's
default.** Both apps needed the same correction and it points in opposite
directions, because the two profiles rank the tiers differently under
`quality_sort: top`:

| Profile | Tier order, highest first | TRaSH cutoff | Ours |
|---|---|---|---|
| Sonarr `WEB-1080p` | WEB 1080p, Bluray-1080p | `WEB 1080p` | `Bluray-1080p` |
| Radarr `HD Bluray + WEB` | Bluray-1080p, WEB 1080p, Bluray-720p | `Bluray-1080p` | `WEB 1080p` |

Keeping the template value makes every file in the *other* 1080p tier read as
cutoff-unmet, and the app spends the library chasing a sideways move. Radarr
had **499** WEBDL/WEBRip files being pulled toward Bluray. Sonarr had **4,997**
Bluray-1080p episodes being pulled toward WEB, which trades a lossless source
for a lossy one. Naming the bottom tier keeps both 1080p qualities at cutoff,
while everything outside the profile — DVD, SDTV, the 480p and 720p tail —
still ranks below it and is still swept up to 1080p. Grab *preference* is
unchanged either way: the tier order is untouched, only the stopping point
moves.

Measured on Sonarr immediately before the change: **8,956 episodes
cutoff-unmet, 5,035 of them Bluray-1080p**, leaving a genuine tail of **3,959**.
Sonarr stores the profile with `Bluray-1080p` at item index 19 and `WEB 1080p`
at index 20, and the cutoff pointed at index 20 — which is the whole bug, read
straight off the API. This deliberately **stops DTS falling on Sonarr**: 1,750
of those 5,035 carried DTS-HD MA or DV, and the only reason that number moved
was the sweep replacing lossless Bluray audio with WEB.

**Dolby Vision and the DTS family are blocked outright** — all three DV
formats (`DV Boost`, `DV (Disk)`, `DV (w/o HDR fallback)`) and all five DTS
formats scored `-10000` against a profile whose `min_format_score` is `0`. So
they are neither downloadable nor, once scored, retainable. Note `DV Boost`
matches any DV release by *title*, which is what makes the blanket block work.

An earlier revision followed TRaSH's own DV defaults instead, which *prefer*
DV carrying an HDR10 fallback (+1000) and reject only the no-fallback kind.
That is the better setting for hardware that handles DV well; it was replaced
because this one does not.

No upstream template scores DV or DTS, so this config is their sole source and
there is no override conflict. The **anime profile is deliberately left
alone** — it uses `score_set: anime-sonarr` with `min_format_score: 100`, and
adding a −10000 there risks starving anime grabs, where dual-audio DTS is
common.

# Two traps worth knowing

- **TRaSH `trash_id`s are per-app.** Every one of those 8 custom formats has a
  *different* id in Sonarr than in Radarr. Reusing one app's ids for the other
  resolves to nothing. The ids came from
  `docs/json/{sonarr,radarr}/cf/` in the TRaSH-Guides repo and were confirmed
  distinct pair by pair.
- **The image has no `v` prefix on its tags.** The GitHub *release* is
  `v1.30.2`; the container tag is `1.30.2`, and zero `v`-prefixed tags exist
  in the registry. The Kargo `allowTags` regex is therefore `^\d+\.\d+\.\d+$`.
  A `^v…` regex would have matched nothing and left the Warehouse at
  `NoImageReferencesDiscovered` with no error — the silent failure described
  in [kargo](/platform/kargo.md).

# Version updates

Tracked by Kargo as the 7th target in the `workloads` project, on the default
`minor` merge policy (patch and minor merge themselves; a major waits for a
human).[^kargo]

# Verified

Before first merge, configarr 1.30.2 was run with `DRY_RUN=true` against the
live Sonarr and Radarr through a port-forward: both instances reported
success, all 10 recyclarr templates resolved, all 8 custom formats resolved on
both sides, and there were zero warnings.[^dryrun] Note the run also reports a
**quality-definition change on 14 qualities per app** — applying TRaSH's sizes
raises `maxSize`/`preferredSize` substantially over the current values. That
is intended, but it is the change most likely to be felt.

[^values]: configarr values.yaml
[^addons]: workloads/vcluster-media addons.yaml
[^kargo]: kargo-projects target list
[^dryrun]: Pre-merge dry run
[^tv-1080p]: TV made 1080p-only; 2160p tiers rank below 1080p in WEB-1080p
[^sonarr-profiles-1108]: Sonarr profiles after the 1080p move
