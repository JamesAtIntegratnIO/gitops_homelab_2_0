#!/usr/bin/env bash
# Build the two USB stick images used to rebuild a control-plane node.
#
#   ./matchbox/usb-sticks/build.sh ipxe [--url http://10.0.112.2/boot.ipxe]
#   ./matchbox/usb-sticks/build.sh relabel --mac 00:23:24:e7:25:10 --model PNY
#
# Writes build/{ipxe,relabel}.img and a .env beside each for test.sh. Runs on
# x86_64 Linux with Nix; every tool comes from the nixpkgs pinned in flake.lock,
# and the one download, the Alpine ISO, is checked against its sha256 below.
#
# Exit: 0 built, 1 failed, 2 usage.

set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$HERE/../.." && pwd)"
BUILD="$HERE/build"

ALPINE_VERSION=3.24.3
ALPINE_SHA256=1629ffe7308d4f3d3eeeed3a97d09ad62293f6d37ef3e483a7306077fdac12be

usage() {
  sed -n 2,6p "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//' >&2
  exit 2
}

NIXFLAGS=(--extra-experimental-features 'nix-command flakes')
REV="$(sed -n '/"nixpkgs": {/,/"type"/p' "$ROOT/flake.lock" | sed -n 's/.*"rev": "\([0-9a-f]\{40\}\)".*/\1/p' | head -1)"
[ -n "$REV" ] || { echo "no nixpkgs rev in $ROOT/flake.lock" >&2; exit 1; }
NP="github:NixOS/nixpkgs/$REV"

# Re-enter with the pinned tools on PATH.
if [ -z "${USB_STICKS_TOOLS:-}" ]; then
  USB_STICKS_TOOLS=1 exec nix "${NIXFLAGS[@]}" shell \
    "$NP#curl" "$NP#dosfstools" "$NP#gnutar" "$NP#gzip" "$NP#libarchive" "$NP#mtools" "$NP#util-linux" \
    -c bash "${BASH_SOURCE[0]}" "$@"
fi

mkfat() { # mkfat <out.img> <MiB> <VOLUME-NAME> <srcdir>
  # One FAT32 ESP on MBR, so nothing here needs root. MBR because a GPT backup
  # header belongs at the end of the disk and this image is far smaller than a stick.
  local out=$1 mib=$2 vol=$3 src=$4 part="$BUILD/part.img" e
  rm -f "$out" "$part"
  mkfs.vfat -F 32 -n "$vol" -C "$part" $(( (mib - 1) * 1024 )) >/dev/null
  while IFS= read -r e; do
    mcopy -s -Q -i "$part" "$src/$e" ::/
  done < <(ls -A "$src")
  truncate -s "${mib}M" "$out"
  printf 'label: dos\nunit: sectors\n\nstart=2048, type=ef, bootable\n' | sfdisk -q "$out"
  dd if="$part" of="$out" bs=1M seek=1 conv=notrunc status=none
  rm -f "$part"
}

build_ipxe() {
  local url=http://10.0.112.2/boot.ipxe script out src
  while [ $# -gt 0 ]; do
    case "$1" in
      --url) url="${2:?}"; shift 2 ;;
      *) usage ;;
    esac
  done
  script="$BUILD/boot.ipxe"
  sed "s|@MATCHBOX_URL@|$url|" "$HERE/boot.ipxe.in" > "$script"
  out="$(IPXE_SCRIPT="$script" nix "${NIXFLAGS[@]}" build --impure --no-link --print-out-paths --expr "
    let p = import (builtins.getFlake \"$NP\") { system = \"x86_64-linux\"; };
    in p.ipxe.override { embedScript = builtins.toFile \"boot.ipxe\" (builtins.readFile (builtins.getEnv \"IPXE_SCRIPT\")); }")"
  src="$BUILD/ipxe-src"
  rm -rf "$src"; mkdir -p "$src/EFI/BOOT"
  install -m 644 "$out/ipxe.efi" "$src/EFI/BOOT/BOOTX64.EFI"
  mkfat "$BUILD/ipxe.img" 64 IPXE "$src"
  rm -rf "$src"
  printf 'MATCHBOX_URL=%s\n' "$url" > "$BUILD/ipxe.env"
  echo "built $BUILD/ipxe.img, chains to $url"
}

build_relabel() {
  local mac="" model="" iso src ovl
  while [ $# -gt 0 ]; do
    case "$1" in
      --mac) mac="${2:?}"; shift 2 ;;
      --model) model="${2:?}"; shift 2 ;;
      *) usage ;;
    esac
  done
  [[ "$mac" =~ ^([0-9a-f]{2}:){5}[0-9a-f]{2}$ ]] || { echo "--mac must be lowercase aa:bb:cc:dd:ee:ff, got '$mac'" >&2; exit 2; }
  # The kernel shows the first 16 characters of the model; match inside those.
  [[ "$model" =~ ^[A-Za-z0-9._-]+$ ]] || { echo "--model must be one word from the drive's model string, got '$model'" >&2; exit 2; }

  iso="$BUILD/alpine-standard-$ALPINE_VERSION-x86_64.iso"
  [ -f "$iso" ] || curl -fsSL -m 600 -o "$iso" "https://dl-cdn.alpinelinux.org/alpine/v${ALPINE_VERSION%.*}/releases/x86_64/${iso##*/}"
  echo "$ALPINE_SHA256  $iso" | sha256sum -c --quiet

  src="$BUILD/relabel-src"; ovl="$BUILD/relabel-ovl"
  rm -rf "$src" "$ovl"; mkdir -p "$src" "$ovl/etc/local.d" "$ovl/etc/apk" "$ovl/etc/runlevels/default"
  bsdtar -xf "$iso" -C "$src"; chmod -R u+w "$src"

  # Alpine applies *.apkovl.tar.gz from the boot media. Providing one turns off its
  # default services unless .default_boot_services says otherwise, and without those
  # nothing loads the drivers.
  : > "$ovl/etc/.default_boot_services"
  printf 'alpine-base\ne2fsprogs\ne2fsprogs-extra\n' > "$ovl/etc/apk/world"   # -extra has e2label
  echo relabel > "$ovl/etc/hostname"
  ln -s /etc/init.d/local "$ovl/etc/runlevels/default/local"
  sed -e "s/@WANT_MAC@/$mac/" -e "s/@WANT_MODEL@/$model/" "$HERE/relabel.start.in" > "$ovl/etc/local.d/relabel.start"
  ! grep -q '@WANT_' "$ovl/etc/local.d/relabel.start" || { echo "template not fully rendered" >&2; exit 1; }
  chmod 755 "$ovl/etc/local.d/relabel.start"
  tar --owner=0 --group=0 -czf "$src/relabel.apkovl.tar.gz" -C "$ovl" etc

  cat > "$src/boot/grub/grub.cfg" <<GRUB
set timeout=1

menuentry "Relabel disk as TALOS_INSTALL" {
linux	/boot/vmlinuz-lts modules=loop,squashfs,sd-mod,usb-storage,vfat console=ttyS0,115200 console=tty0
initrd	/boot/initramfs-lts
}
GRUB
  mkfat "$BUILD/relabel.img" 512 RELABEL "$src"
  rm -rf "$src" "$ovl"
  printf 'WANT_MAC=%s\nWANT_MODEL=%s\n' "$mac" "$model" > "$BUILD/relabel.env"
  echo "built $BUILD/relabel.img for MAC $mac, model *$model*"
}

mkdir -p "$BUILD"
case "${1:-}" in
  ipxe) shift; build_ipxe "$@" ;;
  relabel) shift; build_relabel "$@" ;;
  *) usage ;;
esac
