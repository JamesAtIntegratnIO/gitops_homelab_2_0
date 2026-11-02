#!/bin/sh
# extract_audio.sh <mkv path as Jellyfin sees it> <start s> <dur s> <out.wav>
# Cuts a 16 kHz mono PCM clip of the first audio track on Tower (Jellyfin's ffmpeg) and copies it here.
set -eu
ssh root@10.0.0.12 "docker exec Jellyfin /usr/lib/jellyfin-ffmpeg/ffmpeg -nostdin -v error -y -ss $2 -t $3 -i '$1' -map 0:a:0 -ac 1 -ar 16000 -c:a pcm_s16le /tmp/clip.wav && docker cp Jellyfin:/tmp/clip.wav /tmp/clip.wav && docker exec Jellyfin rm -f /tmp/clip.wav"
scp -q root@10.0.0.12:/tmp/clip.wav "$4"; ssh root@10.0.0.12 rm -f /tmp/clip.wav
