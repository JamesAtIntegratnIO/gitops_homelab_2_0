#!/usr/bin/env bash
# Write a built image to a USB stick, named by serial so it cannot land on the wrong disk.
#
#   ./matchbox/usb-sticks/write.sh                                  # list the sticks
#   ./matchbox/usb-sticks/write.sh build/ipxe.img <serial-prefix>   # erase one and write
#
# Refuses anything that is not removable, on USB, and unmounted. Reads the image
# back afterwards. Needs write access to the device: the disk group, or sudo.
#
# Exit: 0 written and verified, 1 failed or refused, 2 usage.

set -euo pipefail

sticks() {
  local s dev
  for s in /sys/block/sd*; do
    [ -e "$s" ] || continue
    dev="/dev/${s##*/}"
    [ "$(cat "$s/size")" -gt 0 ] || continue   # an empty card-reader slot
    [ "$(lsblk -dno TRAN "$dev")" = usb ] && [ "$(lsblk -dno RM "$dev" | tr -d ' ')" = 1 ] || continue
    printf '%s  %s  %s  serial=%s  label=%s\n' "$dev" "$(lsblk -dno SIZE "$dev" | tr -d ' ')" \
      "$(lsblk -dno MODEL "$dev")" "$(lsblk -dno SERIAL "$dev")" "$(lsblk -no LABEL "$dev" | grep . | head -1)"
  done
}

if [ $# -eq 0 ]; then sticks; exit 0; fi
[ $# -eq 2 ] || { sed -n 2,5p "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//' >&2; exit 2; }
IMG=$1 SERIAL=$2
[ -f "$IMG" ] || { echo "no image at $IMG" >&2; exit 1; }
[ "${#SERIAL}" -ge 6 ] || { echo "serial prefix too short to be sure of: '$SERIAL'" >&2; exit 1; }

MATCH="$(sticks | grep -E "serial=$SERIAL" || true)"
[ -n "$MATCH" ] || { echo "no removable USB stick has a serial starting $SERIAL" >&2; sticks >&2; exit 1; }
[ "$(printf '%s\n' "$MATCH" | wc -l)" -eq 1 ] || { echo "more than one stick matches $SERIAL" >&2; exit 1; }
DEV="${MATCH%% *}"
[ -z "$(lsblk -nro MOUNTPOINTS "$DEV" | tr -d ' \n')" ] || { echo "$DEV has something mounted" >&2; exit 1; }

echo "erasing $MATCH"
for p in "$DEV"?*; do [ -b "$p" ] && wipefs -a "$p" >/dev/null; done
wipefs -a "$DEV" >/dev/null   # both GPT headers, including the one at the far end
dd if="$IMG" of="$DEV" bs=4M conv=fsync status=none
sync
cmp -n "$(stat -c %s "$IMG")" "$IMG" "$DEV"
echo "wrote and verified $IMG on $DEV"
