#!/usr/bin/env bash
set -euo pipefail

# Sync local matchbox assets to an Unraid host running matchbox persistently.
# Usage: ./sync-to-matchbox.sh [user@host]  (defaults to root@10.0.0.12)
# Expects matchbox data on the remote under /mnt/user/appdata/matchbox.

REMOTE_HOST="${1:-root@10.0.0.12}"
REMOTE_BASE="/mnt/user/appdata/matchbox"

# Ensure the Unraid share paths exist before we push updates.
ssh "${REMOTE_HOST}" "mkdir -p ${REMOTE_BASE}/assets ${REMOTE_BASE}/profiles ${REMOTE_BASE}/groups"

# Mirror local directories to the remote matchbox instance so PXE assets stay current.
# Exclude any example files from the sync to avoid cluttering the Unraid instance.
rsync -av --delete --exclude='*example' assets/ "${REMOTE_HOST}:${REMOTE_BASE}/assets/"
rsync -av --delete --exclude='*example' profiles/ "${REMOTE_HOST}:${REMOTE_BASE}/profiles/"
rsync -av --delete --exclude='*example' groups/ "${REMOTE_HOST}:${REMOTE_BASE}/groups/"
