#!/usr/bin/env bash
# Container runtime. idpbuilder builds a kind cluster, and kind needs a docker
# socket -- on macOS that means a VM.
#
# colima rather than Docker Desktop: it installs and starts from the command
# line with no admin password and no GUI, which is the difference between a
# setup you can automate and one you have to click through.
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

: "${RUNTIME_CPU:=4}"
: "${RUNTIME_MEMORY:=10}"   # CNOE's floor is 6; the kit adds Prometheus
: "${RUNTIME_DISK:=60}"

say "container runtime"
if ! command -v colima >/dev/null 2>&1; then
  step "installing colima and the docker CLI"
  brew install colima docker
fi
command -v kind >/dev/null 2>&1 || { step "installing kind"; brew install kind; }
command -v idpbuilder >/dev/null 2>&1 || { step "installing idpbuilder"; brew install cnoe-io/tap/idpbuilder; }

if colima status >/dev/null 2>&1; then
  ok "colima already running"
else
  step "starting colima (${RUNTIME_CPU} cpu, ${RUNTIME_MEMORY}g, ${RUNTIME_DISK}g disk)"
  colima start --cpu "$RUNTIME_CPU" --memory "$RUNTIME_MEMORY" --disk "$RUNTIME_DISK"
fi
docker info >/dev/null 2>&1 && ok "docker reachable" || bad "docker is not reachable"
