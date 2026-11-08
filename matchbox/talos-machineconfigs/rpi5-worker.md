# Raspberry Pi 5 worker (talos-pi5-1)

Joined the-cluster on 2026-11-07 as its first dedicated worker and its first
arm64 node. It is not PXE-booted: it boots a community Talos build from its own
NVMe, so Matchbox has no group or profile for it.

| | |
|---|---|
| Board | Raspberry Pi 5 Model B Rev 1.1, 16 GB (15,928 MiB) |
| MAC / IP | `2c:cf:67:ed:44:0e`, fixed DHCP lease `10.0.4.104` in UniFi (client name `talos-pi5-1`) |
| Switch | SwitchyMcSwitchFace-POE port 8 (corrected 2026-11-08), Homelab VLAN, same L2 as the control plane |
| Disk | Samsung SSD 990 PRO 1 TB on the M.2 HAT: boot, STATE, EPHEMERAL (100 GiB), user volume `local-nvme` (rest, `/var/mnt/local-nvme`) |
| Talos | v1.13.12, kernel 6.18.57-talos (raspberrypi/linux fork), U-Boot 2026.10 (talos-rpi5/sbc-raspberrypi5) |
| Patch | [rpi5-worker.yaml](rpi5-worker.yaml) |

## Why not the official image

The Image Factory `rpi_5` overlay (siderolabs/sbc-raspberrypi v0.2.0, still the
one served for v1.13.12) ships the Raspberry Pi 4 U-Boot. On a Pi 5 the USB ports
and the NIC sit behind the RP1 south bridge and the NVMe behind BCM2712 PCIe,
and that U-Boot drives neither. The firmware loads `u-boot.bin` from USB or NVMe,
then U-Boot cannot read the disk it came from and stops: a switch port with link
and no DHCP request. Only SD boot can work, and
[sbc-raspberrypi#96](https://github.com/siderolabs/sbc-raspberrypi/issues/96)
reports that failing on D0-stepping boards too. The fixes
([#97](https://github.com/siderolabs/sbc-raspberrypi/pull/97),
[#88](https://github.com/siderolabs/sbc-raspberrypi/pull/88)) are unmerged.

## The image

Built on holocron (colima, native arm64, about 30 min) from
[yama6a/talos-raspberry-pi5](https://github.com/yama6a/talos-raspberry-pi5) at
`19d8e06`, with two local edits:

1. `versions.env`: `TALOS_VERSION="v1.13.12"`, to match the control plane. The
   repo only releases 1.14.x.
2. `kernel/patch-skip.txt`: removed the four `net-cadence-macb-*` EEE entries.
   They name pkgs patches that exist only from Talos 1.14 on, and preflight
   fails on a skip entry that matches nothing. The list then equals what the
   repo shipped for 1.13.9.

```bash
make preflight && make build && make validate     # under nix shell: gnumake go zstd xz crane
```

Keep the checkout under `$HOME`. The build serves the kernel tarball from an
nginx container, and colima does not share `/private/tmp` into its VM: from
there the container serves nothing, and the build fails with a "digest
mismatch".

| Artifact | |
|---|---|
| `metal-arm64-rpi5.raw.xz` | sha256 `df16594d9b1a88c9af5476e8b2bca445d9c3dff1d220d86591ae3cccf86e0a2d` (raw `07a9fa06...db5a`) |
| installer | `localhost:5010/talos-rpi5/installer:v1.13.12-arm64@sha256:582a77a4a720ce3fa8bb4c6ea39450b459aa8f3c0c427ccd7c243b645952c477`, holocron's build registry only |

The raw image is written straight to the NVMe (`dd` to the whole device). It is
the installed system: applying a config ran `install sequence: 0 phase(s)`. The
default Pi 5 `BOOT_ORDER` (0xf461) reaches NVMe with no SD card in.

## Render and apply

The worker joins with the-cluster's CA and bootstrap token, taken from a live
control-plane config. Nothing below is printed or committed.

```bash
source .claude/skills/homelab-toolchain/scripts/tools.sh
W=$(umask 077; mktemp -d)
$TALOSCTL --context admin@the-cluster -n 10.0.4.103 get mc v1alpha1 -o json \
  | $JQ -r .spec > "$W/cp.yaml"
$TALOSCTL gen secrets --from-controlplane-config "$W/cp.yaml" -o "$W/secrets.yaml"
rm "$W/cp.yaml"
$TALOSCTL gen config the-cluster https://10.0.4.100:6443 \
  --with-secrets "$W/secrets.yaml" --kubernetes-version 1.36.6 \
  --output-types worker \
  --config-patch @matchbox/talos-machineconfigs/rpi5-worker.yaml \
  -o "$W/worker.yaml"
$TALOSCTL validate --config "$W/worker.yaml" --mode metal

# First time, in maintenance mode:
$TALOSCTL -n <ip> get disks --insecure               # nvme0n1 = 1 TB
$TALOSCTL -n <ip> apply-config --insecure -f "$W/worker.yaml"
# Later changes (the node trusts the-cluster's talosconfig):
$TALOSCTL --context admin@the-cluster -e 10.0.4.104 -n 10.0.4.104 apply-config -f "$W/worker.yaml" --dry-run
rm -rf "$W"
```

`apply-config --dry-run` prints unchanged lines around the diff, and those
include `cluster.id` and `cluster.secret`. Do not paste its output anywhere.

## The arm64 taint

Six running images are amd64-only: the four Kratix pipeline images in
`platform-requests`, `bosun` and `delivery-agent`. The scheduler does not check
image architecture, so the node carries `node.integratn.tech/arch=arm64:NoSchedule`.
Workloads that should run here tolerate it: promtail and loki-canary. cilium,
cilium-envoy and node-exporter tolerate everything already.

The taint is set through the kubelet's `registerWithTaints`, not
`machine.nodeTaints`. On a worker the NodeRestriction admission plugin forbids
the node from changing its own taints after it registers, and Talos's
NodeApplyController then failed the labels too ("node talos-pi5-1 is not allowed
to modify taints"). `registerWithTaints` only acts when the node registers, so
the first time it was added by hand:

```bash
kubectl --context admin@the-cluster taint node talos-pi5-1 node.integratn.tech/arch=arm64:NoSchedule
```

## Cluster settings that name a NIC

The Lenovo nodes' NIC is `eno1` and the Pi's is `end0`.

- Cilium `devices` lists both. With `eno1` alone the agent on the Pi exits with
  "unable to determine direct routing device".
- The MetalLB `L2Advertisement` deliberately stays `eno1` only, so the Pi never
  announces a LoadBalancer VIP. The Pi 5 NIC has an open TX-stall bug
  ([sbc-raspberrypi#91](https://github.com/siderolabs/sbc-raspberrypi/issues/91);
  the build carries a 1 s TX-watchdog mitigation), and the public gateway VIP
  should not depend on it.

## Not done yet

- **Upgrades.** `install.image` names `ghcr.io/jamesatintegratnio/talos-rpi5-installer:v1.13.12`,
  which does not exist yet. Push the installer there (or to a registry the node
  can reach) before any `talosctl upgrade`, or upgrade by reflashing the NVMe.
  Rebuild it for each Talos version the control plane moves to.
- **local-nvme StorageClass.** The volume is mounted but nothing provisions on it.
- **Watchdog.** `watchdog.yaml` is not applied here; check `/dev/watchdog0` first.
