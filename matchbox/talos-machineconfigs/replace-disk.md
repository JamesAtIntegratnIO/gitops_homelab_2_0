# Replacing a control-plane node's disk

Written for **10.0.4.102 (`talos-4kg-ymz`)** on 2026-10-29, when its SSD failed
in the middle of the Talos ladder. The steps are the same for any of the three;
the values in the table are .102's.

| | |
|---|---|
| node | 10.0.4.102, `talos-4kg-ymz`, ThinkCentre M900 Tiny |
| failed disk | `SAMSUNG MZ7TY256HDHP-000L7` (PM871b, 256 GB SATA), fw `MAT04L6Q` |
| replacement | PNY CS900 250 GB SATA, fitted 2026-11-01. Reports its model as `PNY 250GB SATA S`. DRAM-less: watch `EtcdWalFsyncSlow` on .102 |
| matchbox group / profile | `cp-1.11.5-2`, MAC `00:23:24:e7:25:10` |
| machine config served | `http://10.0.112.2/assets/talos/1.11.5/controlplane2.yaml` (on the matchbox host -- it holds the cluster's secrets and is not in this repo) |
| etcd member id | old disk `49f2ae2cf0da92b7`, new disk `58e870fbfe9455f1` |
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

## 1. Serve a config built from the live one -- before the new disk boots

**Done for all three nodes on 2026-10-30.** Do it again before any reinstall that
comes after a config or version change, because nothing keeps the rendered files
on the matchbox host in step with the cluster. Until that day they dated from
February and no node had ever booted them.
Against .102's live config, `controlplane2.yaml` was missing:

- `cluster.network.cni: none`, `cluster.proxy.disabled`, `cluster.coreDNS.disabled`.
  Without them Talos renders flannel, kube-proxy and CoreDNS manifests on a
  Cilium cluster.
- etcd's `election-timeout` and `heartbeat-interval`
- the apiserver's `default-*-toleration-seconds`
- `kubernetesTalosAPIAccess`, which the etcd backup job needs
- the `WatchdogTimerConfig` document

`controlplane1.yaml` and `controlplane3.yaml` differed from their nodes the same
way. So rebuild them from the live configs, with the version the other nodes run:

```bash
./matchbox/talos-machineconfigs/regen-served-configs.sh --dry-run 1.11.6
./matchbox/talos-machineconfigs/regen-served-configs.sh 1.11.6
```

It changes two fields in each file and prints no config content:

- `install.image`: the live value lags, because `talosctl upgrade --image` does
  not update it. .102's still names v1.11.6, and .101 and .103 named the plain
  `installer:v1.11.3`, with no iscsi-tools, until 2026-11-01. A PXE install has
  nothing else to go on.
- `network.hostname`: the generated name comes from a machine id kept on the
  disk, so a new disk means a new name. Pinning it keeps the Node, the
  CiliumNode, and the Trivy `scanJobAffinity` pointing at the right machine.

The previous files go to `/mnt/user/appdata/matchbox/backup/` on the matchbox
host (`root@10.0.0.12`), outside what Matchbox serves.

**Do not run `sync-to-matchbox.sh` after this.** It rsyncs the gitignored local
`assets/` with `--delete`, which puts the old files back.

### The default route and DNS are in the config

All three live configs carry these, so a file built from them does too. Check that
they survived before a rebuild:

```yaml
machine:
  network:
    nameservers: [10.0.0.1, 1.1.1.1, 8.8.8.8]
    interfaces:
      - # ...the existing deviceSelector, addresses and vip...
        routes:
          - network: 0.0.0.0/0
            gateway: 10.0.0.1
```

.102 got them during its rebuild. .101 and .103 got them on 2026-11-01, by hand and
outside GitOps, because until then a config diff showed nothing was missing:

- Their default route came from `META` key `0x0a` on their disks, a `dhcp4` operator
  that a new disk does not have. Without it there is no route outside `10.0.0.0/9`,
  and on 2026-11-01 .102 failed its image pulls this way.
- `META` is still there on both. It now loses to the config: `get routespecs` and
  `get resolverspecs` show the `configuration` layer winning over `operator`.
- Their machine configs are multi-document and these fields are lists, so a
  strategic-merge patch would duplicate the interface block. The change was a full
  edited config, streamed to `talosctl apply-config --mode=auto`, after a `--dry-run`
  reported no reboot. Neither node rebooted and etcd kept its term.

**The router in that list is a choice, not an inheritance.** `META` lists the same
three resolvers, but at the `platform` layer, and DHCP's `1.1.1.1, 8.8.8.8` at the
`operator` layer beat it. So `10.0.0.1` was not in .101's or .103's path until
2026-11-01, when James chose to match .102. That reverses the reason
[dns.yaml](dns.yaml) was withdrawn: the router is a dnsmasq with its own LAN records.
Measured that day, all 23 HTTPRoute hostnames answered the same from the router,
1.1.1.1 and 8.8.8.8, so nothing flips today. It would if the router's records drift
from Cloudflare's. `tower` and the node names answer only from the router.

## 2. Select the replacement SSD

No label is needed. The served files for .101 and .103 already select the disk by
SATA port, so a new drive needs no edit and a USB stick cannot match. All three
M900s report the same path. Nothing has installed from this selector yet: it only
runs at install time, and neither node has been reinstalled since it was set.

```yaml
machine:
  install:
    wipe: true
    diskSelector:
      busPath: /pci0000:00/0000:00:17.0/ata1/host0/target0:0:0/0:0:0:0
```

.102's still says `model: 'PNY*'`, from its rebuild, which matches only that drive.
Change it to `busPath` before replacing .102's disk.

To select by model instead, use the string the drive actually reports. Read it in
step 4 if you do not already have it: the kernel truncates the model to 16
characters, and the brand on the box may not appear at all. `*CS900*` would never
have matched `PNY 250GB SATA S`.

The `by-label/TALOS_INSTALL` route and the `RELABEL` stick still exist, but the
stick was built with `--model CS900` on 2026-10-30 and stayed at its prompt on this
drive. A model mismatch is the likely reason; nobody saw the screen.

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

This is the route that worked on 2026-11-01: a plain Talos image on a stick, with
no iPXE and no Matchbox. Build it for the version the other nodes run and write it
on a Linux machine:

```bash
curl -fLo metal-amd64.iso https://factory.talos.dev/image/c9078f9419961640c712a8bf2bb9174933dfcf1da383fd8ea2b7dc21493f8bac/v1.11.6/metal-amd64.iso
./matchbox/usb-sticks/write.sh metal-amd64.iso <serial-prefix>
```

Fit the SSD, plug the node into a **Homelab** port, insert the stick, press F12 and
pick the `UEFI:` entry. Talos boots to maintenance mode and writes nothing until it
gets a config. Check the disk, then apply the served config. It holds the cluster's
secrets, so stream it rather than copying it:

```bash
talosctl get disks --insecure -n 10.0.4.102
talosctl get metakeys --insecure -n 10.0.4.102   # no 0x0a is normal: the route comes from the config in step 1
talosctl apply-config --insecure -n 10.0.4.102 \
  --file <(ssh root@10.0.0.12 cat /mnt/user/appdata/matchbox/assets/talos/1.11.5/controlplane2.yaml)
```

**This joins etcd as soon as it reboots.** Pull the stick while it installs, in case
the firmware boots it again after the reboot; pulling it mid-install did no harm on
2026-11-01. "extra kernel arguments are not supported" is expected under systemd-boot.

### What did not work

- **Port 1 on `RoutyMcRouteFace`, the troubleshooting port.** It is native Default,
  so the node gets `192.168.0.x`. Port 4 is native Homelab. Installing on the wrong
  network would register the etcd member at the wrong address.
- **The `IPXE` stick.** iPXE 2.0.0 hung on this M900 right after
  `file:/autoexec.ipxe... Not found`, before its script printed anything. Those
  lines are harmless on their own. The hardware probe is the best guess, unproven.
  The committed build (1.21.1-unstable) has not been tried on this hardware.
- **A blank disk with no stick.** The firmware netboots in UEFI mode and DHCP hands
  it `netboot.xyz.kpxe`, a BIOS file. It downloads, nothing follows, and it tries
  again every 4.6 minutes. The logs show only a successful TFTP transfer. Replacing
  the file with a BIOS chain loader failed the same way. A UEFI build is untried.
- **Installing on the bench with a throwaway worker config**, to rack it later. A
  worker needs a control plane to issue its API certificate, so the API never opens
  and the config cannot be wiped remotely. It sat at `booting` with port 50000
  refused. Its `STATE` then came up blank after a reboot and it booted the disk into
  maintenance mode, which is the right place to rack from, but nobody established
  why, so do not plan around it. If the bench is the only screen, use a throwaway
  **control-plane** config, never bootstrap it, and reset `STATE` and `EPHEMERAL`
  once racked. That is untested too.

### If etcd and kubelet stay Failed

`Failed to run pre stage: failed to pull image` means the 20-minute pull timed out,
and Talos does not retry it. Fix the network, then reboot the node. On the new node
that is safe: it is not an etcd member until the pull succeeds.

While the node has no default route, talk to it through a healthy node
(`-e 10.0.4.101 -n 10.0.4.102`). A workstation outside `10.0.0.0/9` gets no reply
from it directly.

## 5. Verify, then resume

```bash
talosctl --context admin@the-cluster -n 10.0.4.102 version --short        # v1.11.6
talosctl --context admin@the-cluster -n 10.0.4.102 get extensions         # iscsi-tools + schematic c9078f94...
talosctl --context admin@the-cluster -n 10.0.4.102 get routes | grep 10.0.0.1   # the default route
talosctl --context admin@the-cluster -n 10.0.4.101 etcd members           # 3, LEARNER false
talosctl --context admin@the-cluster -n 10.0.4.102 read /sys/block/sda/device/iotmo_cnt   # 0x0
kubectl --context admin@the-cluster uncordon talos-4kg-ymz
LADDER_LOG=/tmp/dry.log ./matchbox/talos-machineconfigs/ladder.sh --dry-run 1.11.6   # every gate ok
```

Then the ladder carries on where it stopped: `ladder.sh 1.11.6` (only .101
left), `1.12.12`, `1.13.12`, then `k8s-upgrade.sh 1.35.10` and `1.36.6`. See
[upgrades.md](upgrades.md).

## Afterwards

- Run `regen-served-configs.sh` again when the ladder finishes. They
  pin the v1.11.6 installer and Kubernetes v1.34.1, so a reinstall after the
  climb would bring back an old node.
- Stop Matchbox (`docker stop Matchbox` on the Unraid host) and take the iPXE
  stick out before the ladder resumes. It serves the machine configs, secrets
  included, over plain HTTP, and a node that netboots while it is up would get
  the 1.11.5 kernel and whatever config is on the host.
- .101 carries the same drive model (PM871b), bought with this one. Its
  counters are clean (0 timeouts over 1.67 billion I/Os), but the only early
  warning .102 gave was write latency: ~84 ms average against ~21 ms on .101
  in mid-October, which was read at the time as load rather than wear. Watch
  that ratio on .101.
