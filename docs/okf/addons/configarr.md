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
| Sonarr | `WEB-1080p` | `Remux-1080p - Anime` |
| Radarr | `HD Bluray + WEB` | `Remux-1080p - Anime` |

The main profile is extended beyond the upstream template so that **2160p
tiers sit below the 1080p tiers**. With `quality_sort: top` and the upgrade
cutoff at the 1080p tier, that makes 4K a fallback that gets *upgraded away*
when a 1080p release appears — deliberate, not an oversight.

Dolby Vision and the DTS family are rejected outright: 8 custom formats
(`DV Boost`, `DV (Disk)`, `DV (w/o HDR fallback)`, `DTS`, `DTS-ES`,
`DTS-HD HRA`, `DTS-HD MA`, `DTS X`) are scored `-10000` against the main
profile, whose `min_format_score` is `0`. Neither upstream template scores
these, so the config is their sole source — there is no override conflict.

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
