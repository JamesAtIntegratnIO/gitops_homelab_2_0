---
type: Workload
title: configarr
description: The daily CronJob in vcluster-media that reconciles Sonarr's and Radarr's custom formats and quality profiles from the TRaSH-Guides, so that config lives in git rather than only in each app's database.
tags: [media, vcluster-media, custom-formats, trash-guides, cronjob]
status: stable
generated: { by: claude-code/claude-opus-5, at: 2026-08-28T20:35:00Z }
stale_after: 2026-11-28
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

One combined profile per app plus TRaSH's anime profile:

| App | Main profile | Anime profile |
|---|---|---|
| Sonarr | `WEB-2160p` | `Remux-1080p - Anime` |
| Radarr | `UHD Bluray + WEB` | `Remux-1080p - Anime` |

The main profile builds on the **2160p** template, not the 1080p one — the 4K
templates carry the HDR scoring that only matters at that resolution — and is
then extended *downwards* with the 1080p tiers. The upgrade cutoff sits at
`WEB 2160p`, so media can arrive at 1080p and keeps upgrading until it reaches
4K, at which point the search stops. On Radarr the cutoff is deliberately
`WEB 2160p` rather than the template's `Bluray-2160p`, so a 4K WEB release
ends the search instead of leaving it hunting for a Bluray that may never
appear.

**Dolby Vision follows TRaSH's own defaults** — the config assigns the three
DV formats with no explicit score. That means `DV Boost` (+1000) and
`DV (Disk)` (+101) *prefer* Dolby Vision carrying an HDR10 fallback, while
`DV (w/o HDR fallback)` (−10000) rejects the kind that renders wrong on
non-DV hardware. Note `DV Boost` matches any DV release by title; the
three scores compose so that DV-with-fallback nets positive and
DV-without-fallback nets about −9000.

**The DTS family is rejected outright**: `DTS`, `DTS-ES`, `DTS-HD HRA`,
`DTS-HD MA` and `DTS X` are scored `-10000` against a profile whose
`min_format_score` is `0`. No upstream template scores DV or DTS, so this
config is their sole source and there is no override conflict.

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
