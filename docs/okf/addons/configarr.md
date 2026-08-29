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

**Two profiles per app**, plus TRaSH's anime profile:

| App | Library default (1080p) | Opt-in 4K | Anime |
|---|---|---|---|
| Sonarr | `WEB-1080p` | `WEB-2160p` | `Remux-1080p - Anime` |
| Radarr | `HD Bluray + WEB` | `UHD Bluray + WEB` | `Remux-1080p - Anime` |

The **default** profile has no 2160p tier at all. That is deliberate and both
directions were tried: ranking 2160p *below* 1080p makes Sonarr replace
existing 4K files with 1080p, and ranking it *above* makes every new grab
chase 4K. Excluding it leaves neither failure mode, and the titles that should
be 4K — including everything already at 2160p, so it is not downgraded — go on
the opt-in profile instead.

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
