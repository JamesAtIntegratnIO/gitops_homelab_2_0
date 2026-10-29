---
type: Provisioning System
title: Matchbox PXE provisioning
description: How bare-metal nodes network-boot into Talos via iPXE and Matchbox, and which MACs map to which nodes.
tags: [matchbox, pxe, ipxe, talos, bare-metal]
status: stable
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-29T17:30:00Z }
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
---

# Boot flow

1. Node boots from a custom iPXE image (USB/EFI) whose embedded script runs
   `dhcp` then `chain http://10.0.112.2:8080/boot.ipxe` — this avoids having to
   reconfigure the DHCP server for network boot.
2. **Matchbox** (HTTP on `10.0.112.2:8080`) matches the node's MAC against
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
  `/mnt/user/appdata/matchbox/`) with `--delete`.
- `matchbox/ipxe-builder/build-ipxe.sh` builds the chainloading iPXE images:
  `.lkrn`/`.efi`/`.usb` for x86_64 and a hand-rolled MBR disk image for
  Raspberry Pi (arm64 cross-compile).

Related: [Talos nodes](/infrastructure/talos-nodes.md),
[Terraform bootstrap](/infrastructure/terraform-bootstrap.md).

[^matchbox-dir]: matchbox/ directory (groups, profiles, sync script, ipxe-builder)
