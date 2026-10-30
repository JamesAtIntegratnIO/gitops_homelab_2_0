#!/usr/bin/env bash
# Boot the built images in QEMU and check what they do. Run before writing a stick.
#
#   ./matchbox/usb-sticks/test.sh relabel   # the refusal cases and the real one
#   ./matchbox/usb-sticks/test.sh ipxe      # reaches Matchbox with a MAC that is no node
#
# The relabel stick wipes without asking, so its refusals are what need proving.
# A check that stops matching looks exactly like a disk that is safe to wipe: the
# first version of the Talos-partition check passed here and wiped its disk.
#
# Exit: 0 every case behaved, 1 a case did not, 2 usage.

set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$HERE/../.." && pwd)"
BUILD="$HERE/build"
T="$BUILD/test"

NIXFLAGS=(--extra-experimental-features 'nix-command flakes')
REV="$(sed -n '/"nixpkgs": {/,/"type"/p' "$ROOT/flake.lock" | sed -n 's/.*"rev": "\([0-9a-f]\{40\}\)".*/\1/p' | head -1)"
NP="github:NixOS/nixpkgs/$REV"

if [ -z "${USB_STICKS_TOOLS:-}" ]; then
  USB_STICKS_TOOLS=1 exec nix "${NIXFLAGS[@]}" shell \
    "$NP#qemu_kvm" "$NP#e2fsprogs" "$NP#util-linux" -c bash "${BASH_SOURCE[0]}" "$@"
fi
FD="$(nix "${NIXFLAGS[@]}" build --no-link --print-out-paths "$NP#OVMF.fd")/FV/OVMF.fd"
QEMU="$(command -v qemu-kvm)"

[ -r /dev/kvm ] || { echo "needs /dev/kvm" >&2; exit 1; }
mkdir -p "$T"
FAILED=0
pass() { echo "  ok    $*"; }
bad() { echo "  FAIL  $*"; FAILED=1; }
sum() { sha256sum "$1" | cut -d' ' -f1; }
blank() { rm -f "$1"; truncate -s 20G "$1"; }

boot() { # boot <name> <mac> <model> <disk> [second-disk]
  local n=$1 mac=$2 model=$3 disk=$4 second=${5:-} log="$T/$1.log" pid i=0
  local extra=()
  [ -n "$second" ] && extra=(-drive "if=none,id=d2,file=$second,format=raw" -device "ide-hd,drive=d2,bus=a.1,model=$model")
  install -m 644 "$FD" "$T/$n.fd"
  rm -f "$log"
  "$QEMU" -machine q35,accel=kvm -cpu host -m 1536 -display none -serial "file:$log" -no-reboot \
    -drive "if=pflash,format=raw,file=$T/$n.fd" \
    -device qemu-xhci -drive "if=none,id=u,file=$BUILD/relabel.img,format=raw,snapshot=on" -device usb-storage,drive=u,bootindex=1 \
    -device ahci,id=a -drive "if=none,id=d,file=$disk,format=raw" -device "ide-hd,drive=d,bus=a.0,model=$model" "${extra[@]}" \
    -nic "user,model=e1000,mac=$mac,restrict=on" >/dev/null 2>&1 &
  pid=$!
  # A refusal stays at a login prompt and a success powers off, so stop on either message.
  while [ $i -lt 180 ] && kill -0 "$pid" 2>/dev/null; do
    grep -qaE 'REFUSED|FAILED|Powering off in' "$log" 2>/dev/null && break
    sleep 2; i=$((i + 2))
  done
  if grep -qa 'Powering off in' "$log" 2>/dev/null; then
    i=0; while [ $i -lt 45 ] && kill -0 "$pid" 2>/dev/null; do sleep 1; i=$((i + 1)); done
    if kill -0 "$pid" 2>/dev/null; then bad "$n: did not power itself off"; else pass "$n: powered itself off"; fi
  fi
  kill "$pid" 2>/dev/null; wait "$pid" 2>/dev/null
  return 0
}
said() { grep -qa "$2" "$T/$1.log"; }

test_relabel() {
  [ -f "$BUILD/relabel.img" ] && [ -f "$BUILD/relabel.env" ] || { echo "build it first: build.sh relabel ..." >&2; exit 1; }
  # shellcheck source=/dev/null
  . "$BUILD/relabel.env"
  local good="PNY $WANT_MODEL 250GB SSD" other=52:54:00:aa:bb:cc d="$T/disk" d2="$T/disk2" before
  [ "$WANT_MAC" != "$other" ] || { echo "the wrong-machine MAC is the target" >&2; exit 1; }

  echo "right machine, blank disk: labels it"
  blank "$d"; boot good "$WANT_MAC" "$good" "$d"
  if said good RELABEL-DONE && [ "$(e2label "$d")" = TALOS_INSTALL ]; then pass "labeled"; else bad "disk label is '$(e2label "$d" 2>&1)'"; fi

  echo "stick left in, boots again: touches nothing"
  before="$(sum "$d")"; boot again "$WANT_MAC" "$good" "$d"
  if said again ALREADY-LABELED && [ "$before" = "$(sum "$d")" ]; then pass "unchanged"; else bad "disk changed or no ALREADY-LABELED"; fi

  echo "factory MBR partition on the new disk: labels it"
  blank "$d"; printf 'label: dos\ntype=7\n' | sfdisk -q "$d"; boot mbr "$WANT_MAC" "$good" "$d"
  if said mbr RELABEL-DONE && [ "$(e2label "$d")" = TALOS_INSTALL ]; then pass "labeled"; else bad "not labeled"; fi

  echo "any other machine: refuses"
  blank "$d"; before="$(sum "$d")"; boot wrongmac "$other" "$good" "$d"
  if said wrongmac REFUSED && [ "$before" = "$(sum "$d")" ]; then pass "refused, unchanged"; else bad "wrote to another machine's disk"; fi

  echo "the old drive still fitted: refuses"
  blank "$d"; before="$(sum "$d")"; boot wrongmodel "$WANT_MAC" "SAMSUNG MZ7TY256HDHP-000L7" "$d"
  if said wrongmodel REFUSED && [ "$before" = "$(sum "$d")" ]; then pass "refused, unchanged"; else bad "wrote to the wrong model"; fi

  echo "two matching disks: refuses"
  blank "$d"; blank "$d2"; before="$(sum "$d")$(sum "$d2")"; boot twodisks "$WANT_MAC" "$good" "$d" "$d2"
  if said twodisks REFUSED && [ "$before" = "$(sum "$d")$(sum "$d2")" ]; then pass "refused, unchanged"; else bad "picked a disk out of two"; fi

  echo "the right disk with Talos on it: refuses"
  blank "$d"; printf 'label: gpt\nsize=64M, name=EFI\nsize=1M, name=META\nsize=100M, name=STATE\nname=EPHEMERAL\n' | sfdisk -q "$d"
  before="$(sum "$d")"; boot talos "$WANT_MAC" "$good" "$d"
  if said talos REFUSED && [ "$before" = "$(sum "$d")" ]; then pass "refused, unchanged"; else bad "wiped a disk with Talos on it"; fi
  rm -f "$d" "$d2"
}

test_ipxe() {
  [ -f "$BUILD/ipxe.img" ] || { echo "build it first: build.sh ipxe" >&2; exit 1; }
  # Never a node's MAC here. Matchbox would hand this VM that node's kernel and real
  # machine config, secrets and VIP included, and it would try to join the cluster.
  local mac=52:54:00:12:34:99 log="$T/ipxe.log" pid i=0
  echo "reaches Matchbox ($mac matches no group, so it stops at 'not found' and retries)"
  install -m 644 "$FD" "$T/ipxe.fd"; rm -f "$log"
  "$QEMU" -machine q35,accel=kvm -cpu host -m 512 -display none -serial "file:$log" -no-reboot \
    -drive "if=pflash,format=raw,file=$T/ipxe.fd" \
    -device qemu-xhci -drive "if=none,id=u,file=$BUILD/ipxe.img,format=raw,snapshot=on" -device usb-storage,drive=u,bootindex=1 \
    -nic "user,model=e1000,mac=$mac" >/dev/null 2>&1 &
  pid=$!
  while [ $i -lt 90 ] && kill -0 "$pid" 2>/dev/null; do
    grep -qa 'Chain to matchbox failed' "$log" 2>/dev/null && break
    sleep 2; i=$((i + 2))
  done
  kill "$pid" 2>/dev/null; wait "$pid" 2>/dev/null
  if grep -qaE 'boot\.ipxe[. ]*ok' "$log"; then pass "fetched boot.ipxe"; else bad "never fetched boot.ipxe (is Matchbox running? see $log)"; fi
}

case "${1:-}" in
  relabel) test_relabel ;;
  ipxe) test_ipxe ;;
  *) sed -n 2,5p "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//' >&2; exit 2 ;;
esac
[ "$FAILED" = 0 ] && echo "PASS" || echo "FAIL"
exit "$FAILED"
