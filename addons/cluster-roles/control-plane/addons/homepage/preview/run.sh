#!/usr/bin/env bash
# Preview the landing page locally, against made-up data, before it ships.
#
#   preview/run.sh            # http://localhost:8088, state "busy"
#   preview/run.sh quiet      # all clear; also: fresh, amdown (see mock.py)
#   preview/run.sh --print    # the full config with every $include resolved
#
# Runs the same Glance version values.yaml pins (downloaded once into
# ~/.cache/glance-preview), on a copy of glance/, with every API URL pointed
# at mock.py. Glance reloads when a file in the copy changes, not the
# original, so re-run after an edit. Needs python3 and curl, nothing else.
# PORT and MOCK_PORT override 8088 and 9999.
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
addon=$(dirname "$here")
port=${PORT:-8088}
mock_port=${MOCK_PORT:-9999}
state=${1:-busy}

ver=$(sed -n 's|.*glanceapp/glance:\(v[0-9.]*\)@.*|\1|p' "$addon/values.yaml")
os=$(uname -s | tr '[:upper:]' '[:lower:]')
case $(uname -m) in x86_64|amd64) arch=amd64 ;; arm64|aarch64) arch=arm64 ;; *) arch=$(uname -m) ;; esac
cache=${XDG_CACHE_HOME:-$HOME/.cache}/glance-preview
bin=$cache/glance-$ver-$os-$arch
if [ ! -x "$bin" ]; then
  mkdir -p "$cache"
  echo "Downloading Glance $ver ($os-$arch)..." >&2
  curl -fsSL "https://github.com/glanceapp/glance/releases/download/$ver/glance-$os-$arch.tar.gz" | tar -xz -C "$cache" glance
  mv "$cache/glance" "$bin"
fi

work=$(mktemp -d)
mock_pid=
cleanup() { [ -n "$mock_pid" ] && kill "$mock_pid" 2>/dev/null; rm -rf "$work"; }
trap cleanup EXIT
cp "$addon"/glance/* "$work/"
sed -i.bak "s/^  port: .*/  port: $port/; s/^  proxied: .*/  proxied: false/" "$work/glance.yml"

# templates/env.yaml's keys (every ${NAME} the config uses), then the URLs swapped for
# the mock. API keys stay empty.
while IFS= read -r kv; do export "$kv"; done < <(sed -n 's/^  \([A-Z_]*\): *"\{0,1\}\([^"]*\)"\{0,1\}$/\1=\2/p' "$addon/templates/env.yaml")
m=http://127.0.0.1:$mock_port
export PROM_URL=$m AM_URL=$m SONARR_URL=$m/sonarr RADARR_URL=$m/radarr SABNZBD_URL=$m/sab \
       UMAMI_URL=$m/umami PLEX_URL=$m/plex PVE_URL=$m/pve UNRAID_URL=$m/unraid \
       HA_URL=$m/ha JELLYFIN_URL=$m/jellyfin GITHUB_API_URL=$m/gh \
       HOMEPAGE_VAR_HA_KEY=mock HOMEPAGE_VAR_JELLYFIN_KEY=mock

if [ "$state" = "--print" ]; then
  "$bin" --config "$work/glance.yml" config:print
  exit
fi

python3 "$here/mock.py" "$state" "$mock_port" &
mock_pid=$!
echo "Landing page preview ($state): http://localhost:$port" >&2
"$bin" --config "$work/glance.yml"
