#!/usr/bin/env python3
"""Audit (and optionally re-search) Sonarr/Radarr files by their ACTUAL media.

Why this exists
---------------
TRaSH's DV and DTS custom formats are every one of them a
`ReleaseTitleSpecification` -- a regex over the release NAME. That makes them a
perfectly good filter on what gets grabbed, and useless for finding what is
already on disk: a release that carries DTS audio without advertising it in its
title scores 0 and imports clean.

Measured on this library over one night: of 131 Radarr imports, **0** had DTS in
the title and **14** (10.7%) had DTS audio. Library DTS went 839 -> 844 while
~1500 grabs churned through the queue.

This script reads `mediaInfo` off each file instead -- the audio codec and video
dynamic range that Sonarr/Radarr probed after import -- so it sees what the
custom formats cannot.

Usage
-----
    # report only (default -- touches nothing)
    ./scripts/arr-audio-audit.py --radarr-url http://localhost:7878 --radarr-key "$K"

    # same, for Sonarr
    ./scripts/arr-audio-audit.py --sonarr-url http://localhost:8989 --sonarr-key "$K"

    # write the matching ids out for inspection
    ./scripts/arr-audio-audit.py ... --json /tmp/dts.json

    # actually trigger searches for the matches (asks first unless --yes)
    ./scripts/arr-audio-audit.py ... --search --limit 50

Keys live in the `homepage-secret` Secret in the `media` namespace:

    kubectl -n media get secret homepage-secret \
      -o jsonpath='{.data.HOMEPAGE_VAR_RADARR_KEY}' | base64 -d

A caveat this script cannot fix
-------------------------------
A replacement is chosen by title too, so ~1 in 10 replacements will itself carry
undeclared DTS. Re-run the audit after a batch to see whether you actually came
out ahead; `--search` on everything at once is how you burn a night of bandwidth
to move a number by five.
"""

from __future__ import annotations

import argparse
import json
import sys
import urllib.error
import urllib.request

DTS_CODECS = ("DTS",)
DV_RANGES = ("DV", "DOLBY VISION")


def api(base: str, key: str, path: str, method: str = "GET", body: dict | None = None):
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(
        base.rstrip("/") + path,
        data=data,
        method=method,
        headers={"X-Api-Key": key, "Content-Type": "application/json"},
    )
    with urllib.request.urlopen(req, timeout=300) as r:
        raw = r.read()
    return json.loads(raw) if raw else None


def classify(mi: dict, want_dts: bool, want_dv: bool) -> bool:
    audio = (mi.get("audioCodec") or "").upper()
    hdr = (mi.get("videoDynamicRangeType") or "").upper()
    if want_dts and any(c in audio for c in DTS_CODECS):
        return True
    if want_dv and any(c in hdr for c in DV_RANGES):
        return True
    return False


def audit_radarr(base: str, key: str, want_dts: bool, want_dv: bool):
    hits = []
    for m in api(base, key, "/api/v3/movie"):
        f = m.get("movieFile")
        if not f:
            continue
        if classify(f.get("mediaInfo") or {}, want_dts, want_dv):
            mi = f.get("mediaInfo") or {}
            hits.append(
                {
                    "id": m["id"],
                    "title": m.get("title"),
                    "audio": mi.get("audioCodec"),
                    "hdr": mi.get("videoDynamicRangeType"),
                    "release": f.get("sceneName") or f.get("relativePath"),
                }
            )
    return hits


def audit_sonarr(base: str, key: str, want_dts: bool, want_dv: bool):
    hits = []
    for s in api(base, key, "/api/v3/series"):
        try:
            files = api(base, key, f"/api/v3/episodefile?seriesId={s['id']}")
        except urllib.error.HTTPError:
            continue
        for f in files or []:
            if classify(f.get("mediaInfo") or {}, want_dts, want_dv):
                mi = f.get("mediaInfo") or {}
                hits.append(
                    {
                        "id": f["id"],
                        "seriesId": s["id"],
                        "title": s.get("title"),
                        "audio": mi.get("audioCodec"),
                        "hdr": mi.get("videoDynamicRangeType"),
                        "release": f.get("sceneName") or f.get("relativePath"),
                    }
                )
    return hits


def report(app: str, hits: list, total_note: str = "") -> None:
    print(f"\n=== {app}: {len(hits)} file(s) match {total_note} ===")
    from collections import Counter

    by_audio = Counter(h["audio"] or "?" for h in hits)
    for k, v in by_audio.most_common(10):
        print(f"  {k:<22} {v}")
    print("  sample:")
    for h in hits[:5]:
        print(f"    - {str(h['title'])[:48]:<48} {h['audio']}  {(h['release'] or '')[:44]}")


def main() -> int:
    p = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    p.add_argument("--radarr-url")
    p.add_argument("--radarr-key")
    p.add_argument("--sonarr-url")
    p.add_argument("--sonarr-key")
    p.add_argument("--dts", action="store_true", help="match files whose audio is DTS (default if neither given)")
    p.add_argument("--dv", action="store_true", help="match files whose video is Dolby Vision")
    p.add_argument("--json", dest="json_out", help="write matching records to this path")
    p.add_argument("--search", action="store_true", help="trigger a search for each match (default: report only)")
    p.add_argument("--limit", type=int, default=0, help="cap how many are searched (0 = no cap)")
    p.add_argument("--yes", action="store_true", help="skip the confirmation prompt for --search")
    a = p.parse_args()

    want_dts, want_dv = a.dts, a.dv
    if not want_dts and not want_dv:
        want_dts = True
    what = " + ".join([x for x, on in (("DTS audio", want_dts), ("Dolby Vision", want_dv)) if on])

    out = {}
    if a.radarr_url and a.radarr_key:
        hits = audit_radarr(a.radarr_url, a.radarr_key, want_dts, want_dv)
        report("RADARR", hits, f"({what})")
        out["radarr"] = hits
    if a.sonarr_url and a.sonarr_key:
        hits = audit_sonarr(a.sonarr_url, a.sonarr_key, want_dts, want_dv)
        report("SONARR", hits, f"({what})")
        out["sonarr"] = hits
    if not out:
        p.error("give --radarr-url/--radarr-key and/or --sonarr-url/--sonarr-key")

    if a.json_out:
        with open(a.json_out, "w") as fh:
            json.dump(out, fh, indent=1)
        print(f"\nwrote {a.json_out}")

    if not a.search:
        print("\nreport only -- pass --search to act on these (see the caveat in the docstring)")
        return 0

    total = sum(len(v) for v in out.values())
    n = min(total, a.limit) if a.limit else total
    if not a.yes:
        print(f"\nAbout to trigger searches for {n} of {total} matched file(s).")
        print("Each search may grab a replacement, delete the current file on import,")
        print("and roughly 1 in 10 replacements will itself carry undeclared DTS.")
        if input("Type 'yes' to continue: ").strip().lower() != "yes":
            print("aborted")
            return 1

    if out.get("radarr"):
        ids = [h["id"] for h in out["radarr"]][: a.limit or None]
        for i in range(0, len(ids), 100):
            api(a.radarr_url, a.radarr_key, "/api/v3/command", "POST",
                {"name": "MoviesSearch", "movieIds": ids[i:i + 100]})
            print(f"  radarr: queued {min(i + 100, len(ids))}/{len(ids)}")
    if out.get("sonarr"):
        sids = sorted({h["seriesId"] for h in out["sonarr"]})[: a.limit or None]
        for sid in sids:
            api(a.sonarr_url, a.sonarr_key, "/api/v3/command", "POST",
                {"name": "SeriesSearch", "seriesId": sid})
        print(f"  sonarr: queued searches for {len(sids)} series")
    return 0


if __name__ == "__main__":
    sys.exit(main())
