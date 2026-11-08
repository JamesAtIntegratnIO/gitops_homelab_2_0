# Raspberry Pi 4 workers (talos-pi4-1, talos-pi4-2)

Joined the-cluster on 2026-11-08 as arm64 workers, beside the Pi 5
([rpi5-worker.md](rpi5-worker.md)). Unlike the Pi 5 they run the **official**
Talos image from the Image Factory, so upgrades are ordinary `talosctl upgrade`.
They are not PXE-booted: each boots the raw image written to a USB stick.

| Node | MAC | IP (UniFi fixed lease) | Boot stick | RAM |
|---|---|---|---|---|
| `talos-pi4-1` | `dc:a6:32:c3:f6:84` | `10.0.4.105` | Samsung FIT 256 GB | 8 GB (7.8 GiB) |
| `talos-pi4-2` | `dc:a6:32:c3:f6:ad` | `10.0.4.106` | Samsung FIT 64 GB | 8 GB (7.8 GiB) |

Both are Pi 4 Model B Rev 1.4 on SwitchyMcSwitchFace-POE (pi4-1 likely port 4,
pi4-2 port 6, USB-C powered), Homelab VLAN, same L2 as the control plane. The
stick is `sda` and holds everything: boot, STATE and EPHEMERAL. NIC is `end0`
(already in Cilium's `devices`; MetalLB's L2Advertisement stays `eno1`, so
these never announce a VIP). No user volume and no `local-nvme` here.

Patch: [rpi4-worker.yaml](rpi4-worker.yaml), shared by every Pi 4. The hostname
is a second, per-node patch.

## The image

Image Factory schematic `ee21ef4a5ef808a9b7484cc0dda0f25075021691c8c09a276591eedb638ea1f9`:

```yaml
overlay:
  image: siderolabs/sbc-raspberrypi
  name: rpi_generic
```

No extensions, no extra kernel args. factory.talos.dev is blocked from cloud
containers, so fetch from holocron.

| Artifact | |
|---|---|
| `https://factory.talos.dev/image/ee21ef4a.../v1.13.12/metal-arm64.raw.xz` | sha256 `4f13ee8589a286478326791bfd1b1b84c4928205a79e190c35b48f5595bc86bd` (raw `547cbd0e...26d6`, 2,355,101,696 bytes) |
| installer | `factory.talos.dev/metal-installer/ee21ef4a...:v1.13.12` |

Flashed on carbonite (`ssh root@carbonite`): check the target is
`lsblk -dno TRAN` = `usb`, then `dd if=metal-arm64.raw of=/dev/sdX bs=4M
conv=fsync`, `blockdev --rereadpt`, and `cmp -n <raw size>` against the image.
The Pi 4 EEPROM on these boards already boots USB with no SD card.

## Render and apply

Same as [rpi5-worker.md](rpi5-worker.md#render-and-apply), with this patch and a
hostname file per node:

```bash
cat > "$W/host.yaml" <<'YAML'
apiVersion: v1alpha1
kind: HostnameConfig
auto: off
hostname: talos-pi4-1
YAML
$TALOSCTL gen config the-cluster https://10.0.4.100:6443 \
  --with-secrets "$W/secrets.yaml" --kubernetes-version 1.36.6 \
  --output-types worker \
  --config-patch @matchbox/talos-machineconfigs/rpi4-worker.yaml \
  --config-patch @"$W/host.yaml" \
  -o "$W/worker.yaml"
$TALOSCTL validate --config "$W/worker.yaml" --mode metal
$TALOSCTL -n <ip> get disks --insecure        # sda = the stick
$TALOSCTL -n <ip> apply-config --insecure -f "$W/worker.yaml"
```

Check the MAC of the maintenance-mode IP before applying, so each hostname goes
to the right board. The taint comes from `registerWithTaints`; no manual taint
was needed.

## Adding more Pi 4s

Three more spare Pi 4s exist. Flash a stick with the same image, boot it on the
Homelab VLAN, add a UniFi fixed lease (next: `10.0.4.107`), render with
`hostname: talos-pi4-3`, apply. Nothing in the cluster needs changing: Cilium
`end0`, the Prometheus node-scrape policy and the tolerations already cover new
workers.

## Caveats

- USB sticks are the only disk, so EPHEMERAL (containerd, logs, emptyDir) wears
  them. Fine for light arm64 work; move to USB SSDs if they carry real load.
- Like the Pi 5, they carry `node.integratn.tech/arch=arm64:NoSchedule`; only
  DaemonSets and workloads that tolerate it run there. Hermes is pinned to
  `node.integratn.tech/board: rpi5` and does not move.
