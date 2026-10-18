#!/usr/bin/env bash
# Rebuild the authentik brand manifests (assets ConfigMap + brand blueprint)
# from the sources here. See README.md.
set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"
repo="$(cd "$here/../.." && pwd)"
out="$repo/addons/clusters/the-cluster/addons/authentik"
node "$here/gen.js" "$out"
wc -c "$out/authentik-brand-assets.yaml" "$out/authentik-blueprints-brand.yaml"
