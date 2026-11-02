#!/bin/sh
# whisper_translate.sh <clip.wav> <out prefix> <model.bin>
# Local Japanese->English translation with whisper.cpp (nixpkgs#whisper-cpp 1.9.5, ggml-medium from
# https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-medium.bin). Writes <prefix>.srt (clip-relative times).
set -eu
B=$(nix build nixpkgs#whisper-cpp --no-link --print-out-paths | tail -1)
"$B/bin/whisper-cli" -m "$3" -f "$1" -l ja -tr -osrt -of "$2" -np -bs 5 >/dev/null 2>&1
