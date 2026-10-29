# Replacing a control-plane node's disk

Written for **10.0.4.102 (`talos-4kg-ymz`)** on 2026-10-29, when its SSD failed
in the middle of the Talos ladder. The steps are the same for any of the three;
the values in the table are .102's.

| | |
|---|---|
| node | 10.0.4.102, `talos-4kg-ymz`, ThinkCentre M900 Tiny |
| failed disk | `SAMSUNG MZ7TY256HDHP-000L7` (PM871b, 256 GB SATA), fw `MAT04L6Q` |
| replacement | PNY CS900 250 GB SATA (ordered 2026-10-29). DRAM-less: watch `EtcdWalFsyncSlow` on .102 after the rebuild |
| matchbox group / profile | `cp-1.11.5-2`, MAC `00:23:24:e7:25:10` |
| machine config served | `http://10.0.112.2/assets/talos/1.11.5/controlplane2.yaml` (on the matchbox host -- it holds the cluster's secrets and is not in this repo) |
| etcd member id (old disk) | `49f2ae2cf0da92b7` |
| schematic | `c9078f9419961640c712a8bf2bb9174933dfcf1da383fd8ea2b7dc21493f8bac` (iscsi-tools) |

## Why the disk, and not the kernel or the cable

The drive reports the failure itself:

```
ata1.00: failed command: READ FPDMA QUEUED
ata1.00: error: { UNC }
sd 0:0:0:0: [sda] Sense Key : Medium Error [current]
sd 0:0:0:0: [sda] Add. Sense: Unrecovered read error - auto reallocate failed
XFS (sda4): metadata I/O error in "xfs_imap_to_bp+0x66/0xe0" ... error 5
```

`UNC` plus *auto reallocate failed* is the drive saying it could not read the
sector and had nowhere to remap it. A cable or port fault shows `ICRC` and link
errors instead; a driver regression does not make a drive return a medium
error. The SCSI timeout counter told the same story before any log was read:
`/sys/block/sda/device/iotmo_cnt` was 0x10b (267) after seven hours and 0x6d
(109) sixteen seconds after a clean power cycle, against 0 on .101 and .103.

## The cost of waiting

With one etcd member on a disk like this, the cluster has no spare: if .102's
disk goes while .101 or .103 is rebooting, etcd loses quorum. That happened
once already -- 17:08-17:11Z on 2026-10-29, .102 died while .103 was rebooting
for 1.11.6. **Do not reboot .101 or .103 until .102 has a good disk.**
`ladder.sh` now refuses to (`g_disks`).

## 1. Fix the machine config on the matchbox host -- before the new disk boots

A reinstall reads three things from `controlplane2.yaml` that are wrong or
missing for a fresh disk:

```yaml
machine:
  install:
    # Was ghcr.io/siderolabs/installer:v1.11.3 -- the plain installer, an older
    # patch, and no iscsi-tools. The ladder never noticed because it always
    # passes --image; a PXE install has nothing else to go on.
    image: factory.talos.dev/metal-installer/c9078f9419961640c712a8bf2bb9174933dfcf1da383fd8ea2b7dc21493f8bac:v1.11.6
    # Unchanged, and it is why step 2 labels the new disk: Talos installs to the
    # disk carrying this label and nothing else.
    disk: /dev/disk/by-label/TALOS_INSTALL
  network:
    # talos-4kg-ymz is generated from a machine id kept on the disk, so a new
    # disk means a new name. Pinning it keeps the Node, the CiliumNode, and the
    # Trivy scanJobAffinity in addons/.../trivy-operator/values.yaml pointing at
    # the right machine.
    hostname: talos-4kg-ymz
```

Use the version the other nodes run at the time (`talosctl ... version`); it
is v1.11.6 today with .101 still on v1.11.5. Edit the file in place on the
matchbox host; do not copy it into this repo.

## 2. Label the replacement SSD

On any Linux machine, per [../README.md](../README.md) "Prepare Talos install disk":

```bash
lsblk -o NAME,SIZE,MODEL,SERIAL     # confirm the device -- this wipes it
sudo mkfs.ext4 -F -L TALOS_INSTALL /dev/sdX
```

## 3. Take .102 out of etcd and power it off

Snapshot first, through the nightly CronJob so the copy lands where the others
do:

```bash
kubectl --context admin@the-cluster -n talos-backup create job --from=cronjob/talos-etcd-snapshot pre-disk-swap-$(date +%s)
kubectl --context admin@the-cluster -n talos-backup wait --for=condition=complete job/pre-disk-swap-<ts> --timeout=600s
```

Then:

```bash
kubectl --context admin@the-cluster drain talos-4kg-ymz --ignore-daemonsets --delete-emptydir-data --timeout=15m
talosctl --context admin@the-cluster -n 10.0.4.102 etcd leave
talosctl --context admin@the-cluster -n 10.0.4.101 etcd members     # expect 2
talosctl --context admin@the-cluster -n 10.0.4.102 shutdown
```

If `etcd leave` errors (the disk may refuse), remove the member from a healthy
node instead -- the new node cannot join while a member with its peer URL
exists:

```bash
talosctl --context admin@the-cluster -n 10.0.4.101 etcd remove-member 49f2ae2cf0da92b7
```

From here until step 5 etcd runs on two members and tolerates no failure.
Keep the window short.

## 4. Swap the disk and boot

Fit the labeled SSD, power on. With an empty disk the box falls through to
PXE; matchbox matches the MAC, boots the 1.11.5 kernel, and the installer
image from step 1 decides what lands on disk (v1.11.6 + iscsi-tools). Talos
joins etcd as a learner and promotes itself.

## 5. Verify, then resume

```bash
talosctl --context admin@the-cluster -n 10.0.4.102 version --short        # v1.11.6
talosctl --context admin@the-cluster -n 10.0.4.102 get extensions         # iscsi-tools + schematic c9078f94...
talosctl --context admin@the-cluster -n 10.0.4.101 etcd members           # 3, LEARNER false
talosctl --context admin@the-cluster -n 10.0.4.102 read /sys/block/sda/device/iotmo_cnt   # 0x0
kubectl --context admin@the-cluster uncordon talos-4kg-ymz
LADDER_LOG=/tmp/dry.log ./matchbox/talos-machineconfigs/ladder.sh --dry-run 1.11.6   # every gate ok
```

Then the ladder carries on where it stopped: `ladder.sh 1.11.6` (only .101
left), `1.12.12`, `1.13.12`, then `k8s-upgrade.sh 1.35.10` and `1.36.6`. See
[upgrades.md](upgrades.md).

## Afterwards

- `machine.install.image` on .101 and .103 still names
  `ghcr.io/siderolabs/installer:v1.11.3`. Harmless until one of them is
  reinstalled; patch it to the final metal-installer once the ladder is done.
- .101 carries the same drive model (PM871b), bought with this one. Its
  counters are clean (0 timeouts over 1.67 billion I/Os), but the only early
  warning .102 gave was write latency: ~84 ms average against ~21 ms on .101
  in mid-October, which was read at the time as load rather than wear. Watch
  that ratio on .101.
