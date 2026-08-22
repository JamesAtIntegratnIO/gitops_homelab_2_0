#!/usr/bin/env bash
# Deletes the cluster. colima keeps running, because starting it again is the
# slowest part of coming back up and it costs nothing idle.
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

: "${KIND_CLUSTER:=localdev}"
say "deleting kind/${KIND_CLUSTER}"
kind delete cluster --name "$KIND_CLUSTER"
ok "gone -- `colima stop` if you want the VM back too"
