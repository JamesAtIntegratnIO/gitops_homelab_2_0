---
type: Provisioning System
title: Matchbox PXE provisioning
description: How bare-metal nodes network-boot into Talos via iPXE and Matchbox, and which MACs map to which nodes.
tags: [matchbox, pxe, ipxe, talos, bare-metal]
status: stable
generated: { by: claude-code/claude-opus-5-5, at: 2026-11-01T21:30:00Z }
stale_after: 2027-04-29
sources:
  - id: matchbox-dir
    resource: ../../../matchbox/README.md
    title: matchbox/ directory (groups, profiles, sync script, ipxe-builder)
  - id: bootstrap-doc
    resource: ../../bootstrap.md
    title: Bootstrap & Bare-Metal Talos guide
  - id: factory-schematic
    resource: https://factory.talos.dev/schematics/c9078f9419961640c712a8bf2bb9174933dfcf1da383fd8ea2b7dc21493f8bac
    title: Image Factory definition of the running schematic, read 2026-10-29
  - id: live-extensions
    resource: talosctl get extensions on 10.0.4.101-103, 2026-10-29
    title: Extensions loaded on the running nodes
  - id: matchbox-host-1030
    resource: docker inspect and docker logs Matchbox on 10.0.0.12, 2026-10-30
    title: The Matchbox container's state, port and request history
  - id: served-vs-live-1030
    resource: redacted diff of served controlplane{1,2,3}.yaml against talosctl get machineconfig, 2026-10-30
    title: How far the served configs had drifted from the nodes
  - id: usb-sticks
    resource: ../../../matchbox/usb-sticks/README.md
    title: Build, test and write the iPXE and relabel sticks
  - id: rebuild-1101
    resource: talosctl get disks/metakeys/routes on 10.0.4.101-103, TFTP logs and the node's screen, 2026-11-01
    title: What the .102 rebuild showed on real hardware
  - id: explicit-route-1101
    resource: talosctl get routespecs/resolverspecs --namespace network-config on 10.0.4.101-103 and dig against 10.0.0.1, 1.1.1.1 and 8.8.8.8 for every HTTPRoute hostname, 2026-11-01
    title: The explicit default route on .101 and .103, and what each resolver answers
---

# Boot flow

1. Node boots from a custom iPXE image (USB/EFI) whose embedded script runs
   `dhcp` then `chain http://10.0.112.2/boot.ipxe` — this avoids having to
   reconfigure the DHCP server for network boot. No stick is left in the nodes,
   so it is plugged in for a rebuild.
2. **Matchbox** (HTTP on `10.0.112.2:80`, not the 8080 older docs give[^matchbox-host-1030])
   matches the node's MAC against
   `matchbox/groups/*.json` selectors.
3. The matched group names a profile in `matchbox/profiles/*.json` supplying
   kernel, initramfs, and kernel args.
4. Kernel arg `talos.config=http://10.0.112.2/assets/talos/<ver>/<file>.yaml`
   tells Talos where to fetch its machine config.
5. Talos installs to `/dev/disk/by-label/TALOS_INSTALL`, reboots from disk;
   etcd is then bootstrapped once, manually, with `talosctl bootstrap`.

# MAC → node mapping

| Group file | MAC | Profile | Node |
|---|---|---|---|
| `groups/cp-1.11.5.1.json` | `00:23:24:e7:29:40` | cp-1.11.5-1 | controlplane1 (10.0.4.101) |
| `groups/cp-1.11.5.2.json` | `00:23:24:e7:25:10` | cp-1.11.5-2 | controlplane2 (10.0.4.102) |
| `groups/cp-1.11.5.3.json` | `00:23:24:b5:54:f9` | cp-1.11.5-3 | controlplane3 (10.0.4.103) |
| `groups/worker-1.11.5-amd64.json` | `dc:a6:32:c3:f6:af` | worker-1.11.5-amd64 | (worker, amd64) |
| `groups/worker-1.11.5-arm64.json` | `dc:a6:32:c3:f6:xx` (placeholder!) | worker-1.11.5-arm64 | (never matches) |

Known gaps: the arm64 worker group still contains the literal placeholder MAC
`…:xx` so it can never match, and `groups/default.json` references a `default`
profile that has no file in `profiles/` (acknowledged placeholder). See
[known issues](/cluster/known-issues.md).[^matchbox-dir]

# Matchbox is not always running

The `Matchbox` container on the Unraid host is only needed while a node reinstalls.
It sat stopped from March to 2026-10-30, and nothing alerts on that, because nothing
needs it until a node has an empty disk.[^matchbox-host-1030] Start it for a rebuild and
stop it afterwards: it serves the machine configs, secrets included, over plain HTTP.

# Served configs drift from the nodes

`talosctl patch mc` changes a node and leaves the rendered files alone. On 2026-10-30
the files in `assets/talos/1.11.5/` dated from February and had never been served to a
node. Compared with the live configs they lacked `cni: none`, the kube-proxy and CoreDNS
switches, etcd's timeouts, the apiserver tolerations, `kubernetesTalosAPIAccess` and the
watchdog, though every secret matched.[^served-vs-live-1030] A reinstall from them should
be expected to break the cluster, so run
`matchbox/talos-machineconfigs/regen-served-configs.sh <version>` first. It rebuilds
each file from its node's live config, and pins the installer image and hostname.

# Rebuilding a node with no second machine

On 2026-11-01 .102's new disk was installed from a plain Talos ISO on a stick, in
maintenance mode, with `install.diskSelector` and no Matchbox.[^rebuild-1101] That is now the
documented route: [replace-disk.md](../../../matchbox/talos-machineconfigs/replace-disk.md).

[matchbox/usb-sticks/](../../../matchbox/usb-sticks/README.md) also builds a relabel stick and
an iPXE loader, which pass in QEMU and failed on the M900.[^usb-sticks] The iPXE stick (2.0.0) hung
after `autoexec.ipxe`, before its script ran, and the relabel stick never finished; it was
built for a model string the drive does not report.[^rebuild-1101]

A blank disk does not reach Matchbox by netbooting either. The firmware netboots in UEFI
mode and DHCP hands it `netboot.xyz.kpxe`, a BIOS file, so it loops every 4.6 minutes with
clean TFTP logs.[^rebuild-1101] That cause is inferred from the screen and logs; a UEFI loader
was built and never tried.

# A rebuilt disk gets its default route from the config

All three nodes carry an explicit default route (`0.0.0.0/0` via `10.0.0.1`) and nameservers
(`10.0.0.1, 1.1.1.1, 8.8.8.8`) in their machine configs, and so do the served files.[^explicit-route-1101]
.102 got them during its rebuild. .101 and .103 got them on 2026-11-01, as a manual change
outside GitOps.

Before that, .101 and .103 took their route from `META` key `0x0a` on their disks (a `dhcp4`
operator and resolvers), while their configs held only the static address.[^rebuild-1101] A new
disk has no `META`, so it had no route and every image pull failed, and a config diff could not
show why. `META` remains on both disks, and the `configuration` layer now overrides it.

The list adds the router as a resolver, a change `dns.yaml` withdrew in August. `META` named
`10.0.0.1` too, but at the `platform` layer, where DHCP's `1.1.1.1, 8.8.8.8` overrode it, so the
router was not in .101's or .103's path before this. All 23 HTTPRoute hostnames answered the same
from all three resolvers on 2026-11-01. They would stop agreeing if the router's LAN records
drift from Cloudflare's.[^explicit-route-1101]

.101 and .103 now select their install disk by SATA port (`busPath`), so a replacement drive
needs no edit; no install has exercised that selector yet. .102 still has `model: 'PNY*'`.

# Assets and sync

- Talos kernel/initramfs come from the **Talos Image Factory** with a pinned
  schematic hash, `c9078f94…`, whose only customisation is
  `siderolabs/iscsi-tools` — confirmed against the factory's definition and the
  extensions loaded on all three nodes on 2026-10-29.[^factory-schematic][^live-extensions]
  It does **not** carry AMD GPU firmware or microcode, as this page used to say;
  the AMD patches in `matchbox/README.md` describe an earlier build. Upgrades must
  use the matching `factory.talos.dev/metal-installer/<schematic>:<ver>` image or
  the extension is dropped. Download commands are in
  `matchbox/assets/talos/<ver>/<arch>/commands.md`. The binaries themselves are
  gitignored.
- Rendered machine configs (`controlplane{1,2,3}.yaml`, `worker.yaml`,
  `talosconfig`, `kubeconfig`) are gitignored because they embed cluster CA
  keys and tokens — only the patch fragments are committed.
- `matchbox/sync-to-matchbox.sh` rsyncs `assets/`, `profiles/`, `groups/` to
  the Matchbox host (an Unraid server, default `root@10.0.0.12`, serving from
  `/mnt/user/appdata/matchbox/`) with `--delete`. That includes the gitignored
  local machine configs, so it undoes `regen-served-configs.sh`.
- `matchbox/ipxe-builder/build-ipxe.sh` builds the chainloading iPXE images:
  `.lkrn`/`.efi`/`.usb` for x86_64 and a hand-rolled MBR disk image for
  Raspberry Pi (arm64 cross-compile).

Related: [Talos nodes](/infrastructure/talos-nodes.md),
[Terraform bootstrap](/infrastructure/terraform-bootstrap.md).

[^matchbox-dir]: matchbox/ directory (groups, profiles, sync script, ipxe-builder)
[^matchbox-host-1030]: docker inspect and docker logs Matchbox on 10.0.0.12, 2026-10-30
[^served-vs-live-1030]: redacted diff of served controlplane{1,2,3}.yaml against talosctl get machineconfig, 2026-10-30
[^usb-sticks]: matchbox/usb-sticks/README.md
[^explicit-route-1101]: talosctl get routespecs/resolverspecs on 10.0.4.101-103 and dig against all three resolvers for every HTTPRoute hostname, 2026-11-01
