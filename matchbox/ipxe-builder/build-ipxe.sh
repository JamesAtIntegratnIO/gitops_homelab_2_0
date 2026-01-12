#!/usr/bin/env bash
set -euo pipefail

# Build custom iPXE artifacts that embed a boot script for a Matchbox host.
# Usage: ./build-ipxe.sh <matchbox_host_or_url> [--port 8080] [--arch x86_64|arm64|both] [--build-dir path]
# Example: ./build-ipxe.sh 10.0.112.2 --port 9090 --arch both

if [[ $# -lt 1 ]]; then
    echo "Usage: $0 <matchbox_host_or_url> [--port 8080] [--arch x86_64|arm64|both] [--build-dir path]" >&2
    exit 1
fi

MATCHBOX_TARGET="$1"
shift

PORT=8080
ARCH_CHOICE="x86_64"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BUILD_DIR="${SCRIPT_DIR}/build"

while [[ $# -gt 0 ]]; do
    case "$1" in
        --port)
            if [[ $# -lt 2 ]]; then
                echo "--port requires a value" >&2
                exit 1
            fi
            PORT="$2"
            shift 2
            ;;
        --arch)
            if [[ $# -lt 2 ]]; then
                echo "--arch requires a value" >&2
                exit 1
            fi
            ARCH_CHOICE="$2"
            shift 2
            ;;
        --build-dir)
            if [[ $# -lt 2 ]]; then
                echo "--build-dir requires a value" >&2
                exit 1
            fi
            BUILD_DIR="$2"
            shift 2
            ;;
        *)
            echo "Unknown option: $1" >&2
            exit 1
            ;;
    esac
done

case "${ARCH_CHOICE}" in
    x86_64)
        ARCHES=("x86_64")
        ;;
    arm64)
        ARCHES=("arm64")
        ;;
    both)
        ARCHES=("x86_64" "arm64")
        ;;
    *)
        echo "Unsupported arch selection: ${ARCH_CHOICE}" >&2
        exit 1
        ;;
esac

if [[ " ${ARCHES[*]} " =~ " arm64 " ]]; then
    if ! command -v aarch64-linux-gnu-gcc >/dev/null 2>&1; then
        echo "aarch64-linux-gnu-gcc not found. Install the cross toolchain (e.g. sudo apt install gcc-aarch64-linux-gnu)." >&2
        exit 1
    fi
    if ! command -v sfdisk >/dev/null 2>&1; then
        echo "sfdisk not found. Install util-linux (e.g. sudo apt install util-linux)." >&2
        exit 1
    fi
    ARM64_CROSS="CROSS_COMPILE=aarch64-linux-gnu-"
else
    ARM64_CROSS=""
fi

BUILD_DIR="$(mkdir -p "${BUILD_DIR}" && cd "${BUILD_DIR}" && pwd)"
IPXE_REPO="https://github.com/ipxe/ipxe.git"
IPXE_SRC="${BUILD_DIR}/ipxe"
BOOT_SCRIPT="${BUILD_DIR}/boot.ipxe"

if [[ ${MATCHBOX_TARGET} == http* ]]; then
    MATCHBOX_URL="${MATCHBOX_TARGET}"
else
    MATCHBOX_URL="http://${MATCHBOX_TARGET}:${PORT}/boot.ipxe"
fi

if [[ ! -d "${IPXE_SRC}/.git" ]]; then
    git clone "${IPXE_REPO}" "${IPXE_SRC}"
else
    git -C "${IPXE_SRC}" pull --ff-only
fi

cat >"${BOOT_SCRIPT}" <<EOS
#!ipxe
echo Configure DHCP ...
dhcp
chain ${MATCHBOX_URL}
EOS

pushd "${IPXE_SRC}/src" >/dev/null

for ARCH in "${ARCHES[@]}"; do
    make veryclean
    case "${ARCH}" in
        x86_64)
            make -j "$(nproc)" bin/ipxe.lkrn bin-x86_64-efi/ipxe.efi EMBED="${BOOT_SCRIPT}"
            ./util/genfsimg -o "${BUILD_DIR}/ipxe-x86_64.usb" bin/ipxe.lkrn bin-x86_64-efi/ipxe.efi
            cp bin/ipxe.lkrn "${BUILD_DIR}/ipxe-x86_64.lkrn"
            cp bin-x86_64-efi/ipxe.efi "${BUILD_DIR}/ipxe-x86_64.efi"
            ;;
        arm64)
            make -j "$(nproc)" bin-arm64-efi/ipxe.efi EMBED="${BOOT_SCRIPT}" ARCH=arm64 ${ARM64_CROSS}
            cp bin-arm64-efi/ipxe.efi "${BUILD_DIR}/ipxe-arm64.efi"
            ./util/genfsimg -o "${BUILD_DIR}/ipxe-arm64.usb" bin-arm64-efi/ipxe.efi

            ARM_USB="${BUILD_DIR}/ipxe-arm64.usb"
            ARM_IMG="${BUILD_DIR}/ipxe-arm64.img"
            SECTOR_SIZE=512
            START_SECTOR=2048
            USB_SIZE=$(stat -c%s "${ARM_USB}")
            USB_SECTORS=$(( (USB_SIZE + SECTOR_SIZE - 1) / SECTOR_SIZE ))
            TOTAL_SECTORS=$(( START_SECTOR + USB_SECTORS ))
            IMG_SIZE=$(( TOTAL_SECTORS * SECTOR_SIZE ))

            truncate -s "${IMG_SIZE}" "${ARM_IMG}"

            {
                echo "label: dos"
                echo "unit: sectors"
                echo
                echo "1 : start=${START_SECTOR}, size=${USB_SECTORS}, type=c, bootable"
            } | sfdisk "${ARM_IMG}" >/dev/null

            dd if="${ARM_USB}" of="${ARM_IMG}" bs=${SECTOR_SIZE} seek=${START_SECTOR} conv=notrunc status=none
            ;;
    esac
done

popd >/dev/null

echo "Custom iPXE artifacts created under ${BUILD_DIR}"
