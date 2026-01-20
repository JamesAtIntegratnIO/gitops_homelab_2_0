#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)

source "${SCRIPT_DIR}/lib/ip-utils.sh"
source "${SCRIPT_DIR}/lib/read-inputs.sh"
source "${SCRIPT_DIR}/lib/defaults.sh"
source "${SCRIPT_DIR}/lib/render-resources.sh"

render_all_resources
