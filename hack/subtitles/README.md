# Subtitles: fitting fansubs to a BluRay, and Whisper for the gaps

My copy of Fate/stay night: Unlimited Blade Works is the ADE BluRay release, and the only English subs out there are from TV and fansub releases (Commie, UTW, HorribleSubs on OpenSubtitles). They're close but not the same cut. Most episodes are just ~0.9 s off, a few have scenes moved around, and some have whole scenes the TV broadcast never had, so the English file has nothing to say there. Bazarr can't fix any of that: its sync is one global shift, which is useless the moment the BD has an extra two minutes in the middle.

This is how I fixed it, mostly so I can do it again for the next show that does this. It's a flow, not a polished tool. The scripts take `EXX` file names and assume the files sit in the current directory.

## The Trick: The BD Already Has a Timed Subtitle

The ADE files carry Chinese ASS tracks (`chs[SumiSora]`, style `CN`) that were timed for this exact BD. I don't need to read them. I just use their timestamps as the reference, and every check here compares English cue times against those Chinese lines.

```bash
docker exec Jellyfin /usr/lib/jellyfin-ffmpeg/ffmpeg -nostdin -v error -y \
  -i "/data/tvshows/<show>/Season 1/<file>.mkv" -map 0:s:0 -f ass /tmp/E01.ass
```

That runs on Tower (10.0.0.12). The Jellyfin container has ffmpeg and sees the library under `/data/tvshows`.

## Aligning the English

1. Grab a candidate. `os_download_candidates.py` runs inside the Bazarr pod (`kubectl --context admin@media-cluster -n media exec -i deploy/bazarr -c bazarr -- python3 - < os_download_candidates.py`) and uses Bazarr's stored OpenSubtitles login. It prefers Commie, then anything BD.
2. Find the shape. `rolling_offset.py` gives the best offset per minute, and a jump in that list is a cut. `viterbi_align.py` does a first-pass piecewise alignment: one offset per segment, a 3 s penalty to switch, and offsets that only go up, because the BD adds material and never removes it.
3. Place each cut by hand with `inspect_cut.py EP off1 off2 lo hi`, which prints each English line next to the nearest Chinese line under both offsets. The automatic cut is often off by a few lines (on the Prologue it was 7 lines early), so read both tracks and put the cut where the meaning flips.
4. Apply it. `fixed_offset.py EP` for a single shift, `piecewise_offset.py EP 0:0.9 153:161.5` for cuts. Each offset gets refined ±0.3 s, `{...}` typesetting notes get dropped, and frame-by-frame sign spam gets collapsed.
5. Check it. A file only goes on disk if every 4-minute region's median is within **0.3 s** of the Chinese track. Mine all landed at 0.27 s or better.

## Filling BD-Only Scenes With Whisper

`find_gaps.py aligned.srt EXX.ass` lists runs of Chinese lines (2 or more, 8 s or longer) with no English over them. Those are the scenes that need new lines.

### Cut the Audio

```bash
./extract_audio.sh "/data/tvshows/<show>/Season 1/<file>.mkv" 505 150 E02b.wav
```

- `505 150` is the start and length in seconds. Give it a few seconds of margin on each side of the gap.
- It runs Jellyfin's ffmpeg with `-map 0:a:0 -ac 1 -ar 16000 -c:a pcm_s16le`, the first audio track as 16 kHz mono PCM, which is what whisper.cpp wants.
- The clip lands on the Mac. Audio never goes to any outside service.

### Translate Locally

```bash
./whisper_translate.sh E02b.wav E02b.wh ggml-medium.bin
```

- Uses [whisper.cpp](https://github.com/ggerganov/whisper.cpp) 1.9.5 from `nix build nixpkgs#whisper-cpp`, no install.
- Uses the `ggml-medium.bin` model from https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-medium.bin (1.5 GB). Delete it when you're done.
- The real call is `whisper-cli -m ggml-medium.bin -f clip.wav -l ja -tr -osrt -np -bs 5`. `-l ja` pins the source language, `-tr` makes Whisper translate to English itself, and `-bs 5` is beam search.
- Run it without `-tr` too if you want the Japanese transcript to sanity-check against.

### Drop the Lines In

```bash
python3 fill_gaps.py E02.final.srt E02.ass.out E02b.wh.srt 505 E02.filled.srt
```

`505` is the clip start, so Whisper's clip-relative times get shifted back onto the episode. The rules:

- A Whisper segment is only kept if it overlaps a Chinese line that has no English yet.
- Segments more than 50% covered by existing English get dropped, since those are bits of the neighboring scene caught in the margin.
- Starts and ends snap to the Chinese line boundaries when they're within **1.5 s**, so the new lines are timed like the BD.
- Nothing overlaps existing cues, and anything under 0.3 s gets dropped.
- Obvious Whisper junk ("thank you for watching", punctuation-only lines) gets filtered out.

It prints what it added and how many uncovered Chinese lines are now covered. Run `find_gaps.py` again afterwards, and it should come back empty.

### Be Honest About Quality

This English is machine translation and nobody fixed it up by hand. Names and tone drift from the fansub around it (stuff like "You, who are Archer, know it differently."). It's way better than nothing in the middle of a scene, but if you want it nice, edit those lines or go find the BD fansub.

What got filled:

| Episode | Gap | Cues |
|---|---|---|
| S00E01 | 26:42–29:42 (park scene) | 56 |
| S01E02 | 0:28–1:11 (longer cold open) | 10 |
| S01E02 | 9:07–10:19 | 16 |
| S01E12 | 27:06–27:18 | 3 |

## Putting It on Disk

Copy the file next to the mkv as `<name>.en.srt`, then have Bazarr rescan and check it didn't touch anything:

```bash
./bazarr_api.sh PATCH /api/series --data-urlencode seriesid=251 --data-urlencode action=scan-disk
cmp written.srt "/mnt/user/data/media/tv/<show>/Season 1/<name>.en.srt"
```

Two things bit me here, so don't repeat them:

- **Don't use Bazarr's upload endpoint for these.** `use_subsync: true` runs ffsubsync on every upload, which applies one global shift and quietly undoes the piecewise work. On the Prologue it added +0.11 s.
- **Don't pipe the file through `kubectl exec -i`.** My first upload got cut off at exactly 32,768 bytes, and the file looked fine until the last 10 minutes had no subs. Use `scp` to Tower instead.

Backups of anything replaced go in `/mnt/user/kube_storage/bazarr-config-pvc/backup/`.

## Gotchas I Hit on the Way

- **Bazarr can't search specials on OpenSubtitles.** The provider does `if self.video.season:`, so season 0 gets dropped and S00E01 searches for "episode 1 of anything". OpenSubtitles files the Prologue as S01E00 anyway.
- **Numbering doesn't match.** Sonarr has one season of 25 episodes, while OpenSubtitles has S01 (1–13) and S02 (1–12), so E14+ won't match by number.
- **AniDB.** Bazarr needs `api.anidb.net:9001` open (the `bazarr-allow-anidb-egress` NetworkPolicy) and a valid AniDB client. Without the client, any AniDB failure kills the whole search before a provider is even asked. Dropping `animetosho` from `enabled_providers` turns the AniDB lookup off.
- **Scores.** These fansubs score 86 on Bazarr, under `minimum_score: 88`, so automatic search won't grab them. I left the threshold alone on purpose, because these files need the alignment above anyway.

`COMMANDS.md` has the raw command log and thresholds if you want the short version.
