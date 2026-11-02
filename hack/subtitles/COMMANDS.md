# Commands and thresholds used (Fate/stay night UBW, Nov 2026)

Raw notes for the subtitle work on Tower (10.0.0.12). A proper write-up comes separately.

## Reference track
- BD (ADE) files carry Chinese ASS tracks (`chs[SumiSora]`, style `CN`) timed for the BD. Extracted with
  `docker exec Jellyfin /usr/lib/jellyfin-ffmpeg/ffmpeg -nostdin -v error -y -i <mkv> -map 0:s:0 -f ass /tmp/EXX.ass`
- Region check: median nearest-cue delta per 2- or 4-minute window; keep a file only if every region is within 0.3 s.

## Alignment
- Clean episodes: `fixed_offset.py` (single offset, typically +0.82..+0.95 s).
- Cuts/inserts: `rolling_offset.py` and `viterbi_align.py` to find them, `inspect_cut.py` to place each cut
  by reading both tracks, then `piecewise_offset.py EP 0:o1 idx:o2 ...`.
- Writing: copy the .srt next to the mkv as `<name>.en.srt` on Tower, then `bazarr_api.sh PATCH /api/series
  --data-urlencode seriesid=251 --data-urlencode action=scan-disk`, then `cmp` to confirm byte-identical.
  (Bazarr's upload endpoint auto-syncs with ffsubsync (`use_subsync: true`) and applies one global shift,
  which undoes piecewise alignment. Piping the file through `kubectl exec -i` also truncated it at 32,768 bytes.)

## BD-only scenes (local Whisper)
- Audio: `extract_audio.sh <mkv> <start> <dur> clip.wav` (ffmpeg `-map 0:a:0 -ac 1 -ar 16000 -c:a pcm_s16le`).
- Translate: `whisper_translate.sh clip.wav prefix model.bin`, i.e. whisper.cpp 1.9.5 from `nix build nixpkgs#whisper-cpp`,
  `whisper-cli -m ggml-medium.bin -f clip.wav -l ja -tr -osrt -np -bs 5`.
  Model: ggml-medium.bin from https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-medium.bin (1,533,763,059 bytes).
- Insert: `fill_gaps.py aligned.srt chinese.ass clip.wh.srt <clip start> out.srt`. Snaps to CN line boundaries
  within 1.5 s, drops segments >50% covered by existing English, drops cues <0.3 s, never overlaps.
- The English in these gaps is Whisper's machine translation, not hand-written.
- The S00E01 park scene used the same steps (window 1598 s + 194 s); its insertion logic is the one in fill_gaps.py.
