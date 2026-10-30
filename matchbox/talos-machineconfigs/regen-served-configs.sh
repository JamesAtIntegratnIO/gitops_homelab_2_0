#!/usr/bin/env bash
# Rebuild the machine configs Matchbox serves from what each node is actually running.
#
#   ./matchbox/talos-machineconfigs/regen-served-configs.sh --dry-run 1.11.6
#   ./matchbox/talos-machineconfigs/regen-served-configs.sh 1.11.6
#
# <version> is the Talos installer a reinstalled node should get: what the other
# nodes run at that moment. Run it before any reinstall, and when a ladder finishes.
#
# `talosctl patch mc` changes a node and nothing else, so the rendered files drift
# until the next reinstall reads them. A reinstall from February's file would have
# dropped `cni: none` and the kube-proxy and CoreDNS switches, the etcd timeouts and
# the watchdog. So each file is the node's live config with exactly two changes:
#
#   machine.install.image     the live value can be far behind and lack the schematic
#   machine.network.hostname  otherwise a new disk means a new generated name
#
# Prints no config content: the files hold the cluster's secrets. Each one is held
# in a private temp file only until it is on the Matchbox host.
#
# Exit: 0 all served files current, 1 failed, 2 usage.

set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=/dev/null
source "$ROOT/.claude/skills/homelab-toolchain/scripts/tools.sh" 2>/dev/null || true

DRY=0
[ "${1:-}" = "--dry-run" ] && { DRY=1; shift; }
VERSION="${1:-}"
VERSION="${VERSION#v}"
[[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "usage: $0 [--dry-run] <talos-version x.y.z>" >&2; exit 2; }

TALOS_CTX="${TALOS_CTX:-admin@the-cluster}"
MATCHBOX_HOST="${MATCHBOX_HOST:-root@10.0.0.12}"
MATCHBOX_DIR=/mnt/user/appdata/matchbox
SCHEMATIC=c9078f9419961640c712a8bf2bb9174933dfcf1da383fd8ea2b7dc21493f8bac
IMAGE="factory.talos.dev/metal-installer/$SCHEMATIC:v$VERSION"

kill_tree() { local c; for c in $(pgrep -P "$1" 2>/dev/null); do kill_tree "$c"; done; kill -TERM "$1" 2>/dev/null; }
deadline() { # Go ignores SIGALRM, and talosctl hangs forever on a node whose etcd is down
  local end=$(( SECONDS + $1 )); shift
  "$@" &
  local pid=$!
  while kill -0 "$pid" 2>/dev/null; do
    if [ "$SECONDS" -ge "$end" ]; then
      kill_tree "$pid"; sleep 2; kill -KILL "$pid" 2>/dev/null
      wait "$pid" 2>/dev/null; return 124
    fi
    sleep 0.2
  done
  wait "$pid"
}
tc() { deadline 60 "$TALOSCTL" --context "$TALOS_CTX" "$@"; }
remote() { deadline 60 ssh -o BatchMode=yes -o ConnectTimeout=10 "$MATCHBOX_HOST" "$@"; }

WORK="$(umask 077; mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
umask 077

code=$(curl -s -o /dev/null -m 15 -w '%{http_code}' -H 'Accept: application/vnd.oci.image.index.v1+json' \
  "https://factory.talos.dev/v2/metal-installer/$SCHEMATIC/manifests/v$VERSION")
[ "$code" = 200 ] || { echo "STOP: the Image Factory has no $IMAGE (HTTP $code)" >&2; exit 1; }

# json_without <file>: the config as JSON, minus the two fields this script owns
json_without() { "$YQ" ea -o=json '[.] | del(.[].machine.install.image, .[].machine.network.hostname)' "$1"; }

regen() { # regen <slot> <ip>
  local slot=$1 ip=$2 hn config rel live="$WORK/live" new="$WORK/new" have want
  local profile=("$ROOT"/matchbox/profiles/cp-*-"$slot".json)
  [ "${#profile[@]}" = 1 ] && [ -f "${profile[0]}" ] || { echo "  STOP: expected one profile for slot $slot" ; return 1; }
  # The profile says which file this node is served, so the two cannot disagree.
  config="$("$JQ" -r '.boot.args[] | select(startswith("talos.config=")) | sub("^talos.config="; "")' "${profile[0]}")"
  rel="${config#http://*/}"
  [ -n "$config" ] && [ "$rel" != "$config" ] || { echo "  STOP: no talos.config in ${profile[0]}"; return 1; }
  local file="$MATCHBOX_DIR/$rel"

  hn="$(tc -n "$ip" get hostname -o json 2>/dev/null | "$JQ" -r '.spec.hostname // empty')"
  [ -n "$hn" ] || { echo "  STOP: could not read $ip's hostname"; return 1; }
  tc -n "$ip" get machineconfig v1alpha1 -o json 2>/dev/null | "$JQ" -r '.spec // empty' > "$live"
  grep -q '^machine:' "$live" || { echo "  STOP: could not read $ip's machine config"; return 1; }

  "$YQ" "with(select(.machine != null); .machine.install.image = \"$IMAGE\" | .machine.network.hostname = \"$hn\")" "$live" > "$new" \
    || { echo "  STOP: could not patch"; return 1; }
  grep -q "^ *image: $IMAGE" "$new" && grep -q "^ *hostname: $hn" "$new" || { echo "  STOP: the patch did not take"; return 1; }
  [ "$(json_without "$live")" = "$(json_without "$new")" ] || { echo "  STOP: the patch changed more than two fields"; return 1; }
  "$TALOSCTL" validate --config "$new" --mode metal >/dev/null 2>&1 || { echo "  STOP: talosctl validate rejected it"; return 1; }

  want="$(shasum -a 256 "$new" 2>/dev/null | cut -d' ' -f1)"
  have="$(remote "sha256sum '$file' 2>/dev/null | cut -d' ' -f1")"
  echo "  $ip $hn -> $rel"
  if [ "$want" = "$have" ]; then echo "  unchanged"; return 0; fi
  if [ "$DRY" = 1 ]; then echo "  would replace it"; return 0; fi

  # Backups go outside assets/ so Matchbox does not serve them.
  remote "set -e
    mkdir -p '$MATCHBOX_DIR/backup'; chmod 700 '$MATCHBOX_DIR/backup'
    if [ -e '$file' ]; then cp -p '$file' '$MATCHBOX_DIR/backup/${rel##*/}.$(date -u +%Y%m%dT%H%M%SZ)'; fi
    cat > '$file.new'
    if [ -e '$file' ]; then chown --reference='$file' '$file.new'; chmod --reference='$file' '$file.new'; else chmod 644 '$file.new'; fi
    mv '$file.new' '$file'" < "$new" || { echo "  STOP: could not install it"; return 1; }
  have="$(remote "sha256sum '$file' | cut -d' ' -f1")"
  [ "$want" = "$have" ] || { echo "  STOP: what landed does not match what was built"; return 1; }
  echo "  replaced"
}

echo "installer: $IMAGE$( [ "$DRY" = 1 ] && echo ', DRY RUN' )"
rc=0
for n in "1 10.0.4.101" "2 10.0.4.102" "3 10.0.4.103"; do
  # shellcheck disable=SC2086
  regen $n || rc=1
done
if [ "$rc" = 0 ] && [ "$DRY" = 0 ]; then
  echo "Do not run sync-to-matchbox.sh now: it pushes the gitignored local copies over these."
fi
exit "$rc"
