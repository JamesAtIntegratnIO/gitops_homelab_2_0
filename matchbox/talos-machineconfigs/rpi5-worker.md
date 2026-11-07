# Raspberry Pi 5 worker (talos-pi5-1)

DRAFT, 2026-11-07. Nothing here has been applied.

A Pi 5 (16 GB, arm64) joins the-cluster as its first dedicated worker and its
first arm64 node. It is not PXE-booted: it boots Talos from a USB stick written
with the Image Factory raw image, so Matchbox has no group or profile for it.

| | |
|---|---|
| MAC | `2c:cf:67:ed:44:0e` (UniFi: Homelab VLAN, SwitchyMcSwitchFace-POE port 6) |
| Boot | USB stick, Samsung FIT 256 GB |
| Data | 1 TB NVMe on the M.2 HAT: EPHEMERAL (100 GiB) + user volume `local-nvme` |
| Image | schematic `a636242df247ad4aad2e36d1026d8d4727b716a3061749bd7b19651e548f65e4` (overlay `rpi_5`, no extensions) |
| Raw image | `https://factory.talos.dev/image/<schematic>/v1.13.12/metal-arm64.raw.xz` |
| Installer | `factory.talos.dev/metal-installer/<schematic>:v1.13.12` |
| Patch | [rpi5-worker.yaml](rpi5-worker.yaml) |

## Why a separate patch

`all.yaml` installs to `/dev/disk/by-label/TALOS_INSTALL` with `wipe: true`
and the x86 schematic, and `work.yaml` is only DHCP. An SBC runs the raw image
it was written with; `install.disk` is what an upgrade rewrites, so here it is
the USB stick with `wipe: false`. The patch carries everything a worker needs
from the live control-plane config: CNI none, kube-proxy off, the same
resolvers, hostDNS forwarding.

## Before it joins

1. **Taint.** The patch taints the node `node.integratn.tech/arch=arm64:NoSchedule`.
   Six running images are amd64-only (checked 2026-11-07 with `crane manifest`):
   the four Kratix pipeline images in `platform-requests`, `bosun`, and
   `delivery-agent`. Kubernetes does not look at image architecture, so an
   untainted arm64 node would get those pods and they would fail with
   `exec format error`.
   All six DaemonSet images (cilium, cilium-envoy, metallb-speaker,
   node-exporter, promtail, loki-canary) are multi-arch, but only cilium,
   cilium-envoy and node-exporter tolerate an arbitrary NoSchedule taint.
   With the taint in place, promtail, metallb-speaker and loki-canary do not run
   on the Pi: no logs from it, and it does not announce LoadBalancer IPs.
2. **etcd snapshot job.** `talos-etcd-backup/talos-etcd-snapshot.yaml` calls the
   Talos API on its own node (`--endpoints=$(NODE_IP)`) and assumes every node is
   a control-plane member. It needs a control-plane nodeSelector before any
   untainted worker exists.
3. **Fixed IP.** DHCP gave it `10.0.5.101`. Reserve an address in UniFi so the
   node IP is stable.

## Render (secrets stay on this machine)

The worker joins with the-cluster's existing CA and bootstrap token, taken from a
live control-plane config. Nothing below is printed or committed.

```bash
source .claude/skills/homelab-toolchain/scripts/tools.sh
W=$(umask 077; mktemp -d)
$TALOSCTL --context admin@the-cluster -n 10.0.4.103 get mc v1alpha1 -o json \
  | $JQ -r .spec > "$W/cp.yaml"
$TALOSCTL gen secrets --from-controlplane-config "$W/cp.yaml" -o "$W/secrets.yaml"
$TALOSCTL gen config the-cluster https://10.0.4.100:6443 \
  --with-secrets "$W/secrets.yaml" \
  --kubernetes-version 1.36.6 \
  --output-types worker \
  --config-patch @matchbox/talos-machineconfigs/rpi5-worker.yaml \
  -o "$W/worker.yaml"
$TALOSCTL validate --config "$W/worker.yaml" --mode metal
```

## Apply

```bash
# The Pi is in maintenance mode, booted from USB.
$TALOSCTL -n <pi-ip> --insecure get disks        # sda = 256 GB usb, nvme0n1 = 1 TB
$TALOSCTL -n <pi-ip> --insecure apply-config -f "$W/worker.yaml"
rm -rf "$W"

# Then:
kubectl --context admin@the-cluster get node talos-pi5-1 -o wide
$TALOSCTL --context admin@the-cluster -n <pi-ip> get volumestatus   # EPHEMERAL + u-local-nvme on nvme0n1
```

If `get disks` shows the stick as something other than `sda`, change
`machine.install.disk` before rendering.
