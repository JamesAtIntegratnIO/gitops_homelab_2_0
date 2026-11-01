# USB sticks for rebuilding a node

Two sticks that take a control-plane node from a blank SSD to a Talos install,
with no other machine needed to prepare the disk. The images are built here and
never committed: `build/` is gitignored.

**On real hardware on 2026-11-01, neither did the job.** The iPXE stick hung on an
M900 and the relabel stick never finished, most likely because of its model string.
.102 was rebuilt from a plain Talos ISO instead, with a disk selector and no label:
[replace-disk.md](../talos-machineconfigs/replace-disk.md) step 4. Start there. These
sticks had only passed in QEMU, and `write.sh` also writes that ISO.

| Stick | Volume | What it does |
|---|---|---|
| relabel | `RELABEL` | Boots Alpine, labels the node's new SSD `TALOS_INSTALL`, powers off |
| iPXE | `IPXE` | Runs DHCP and chains to Matchbox, which serves the node its kernel and config |

Talos installs only to the disk labeled `TALOS_INSTALL`, which is why a blank
drive needs the first stick. If you can plug the drive into a Linux machine, skip
it and run `mkfs.ext4 -F -L TALOS_INSTALL` there instead.

Both boot UEFI only, which is how these nodes boot. To check a node, see whether
`talosctl -n <ip> ls /sys/firmware/efi` lists anything. Build
`../ipxe-builder/build-ipxe.sh` for a legacy BIOS or arm64 machine.

The whole procedure, including taking the node out of etcd first, is in
[replace-disk.md](../talos-machineconfigs/replace-disk.md).

## Build, test, write

On an x86_64 Linux machine with Nix and `/dev/kvm`. Everything comes from the
nixpkgs in `flake.lock`, except the Alpine ISO, which is checked against the
sha256 in `build.sh`.

```bash
# The MAC comes from `talosctl -n <ip> get links`; matchbox/groups/ has the same list.
# The model is one word from the string the drive reports, not the name on the box.
./matchbox/usb-sticks/build.sh relabel --mac 00:23:24:e7:25:10 --model PNY
./matchbox/usb-sticks/build.sh ipxe

./matchbox/usb-sticks/test.sh relabel
./matchbox/usb-sticks/test.sh ipxe        # needs Matchbox running

./matchbox/usb-sticks/write.sh            # lists the sticks and their serials
./matchbox/usb-sticks/write.sh matchbox/usb-sticks/build/relabel.img <serial-prefix>
./matchbox/usb-sticks/write.sh matchbox/usb-sticks/build/ipxe.img <serial-prefix>
```

`write.sh` needs write access to the device, through the `disk` group or `sudo`.
A relabel image only works for the MAC and model it was built with, so rebuild it
for another node or drive.

## Why the relabel stick will not wipe the wrong disk

It boots and wipes without asking, so it refuses unless all of these hold:

1. The machine has the MAC it was built for.
2. Exactly one internal, non-USB disk has the model string it was built for.
3. That disk has no Talos partitions on it.

A refusal prints the reason, the disks and the NICs, writes nothing, and stops at
a login prompt. A disk that already has the label is left alone, so leaving the
stick in is harmless.

`test.sh relabel` boots the image in QEMU against each of those cases and compares
disk checksums. Run it after any change to `relabel.start.in`: a check that stops
matching looks exactly like a disk that is safe to wipe. The first version of
check 3 relied on `tr -d '\000'`, which did not strip anything on Alpine, and it
wiped its test disk.

## Things that are easy to get wrong

- **Matchbox listens on port 80 here.** `boot.ipxe.in` and the profiles use
  `http://10.0.112.2/`. Older docs in this repo say 8080.
- **Never test the iPXE stick with a node's real MAC.** Matchbox would hand the VM
  that node's kernel and real machine config, and it would try to join the cluster.
  `test.sh` uses a MAC that matches no group.
- **The kernel truncates a SATA model to 16 characters.** `SAMSUNG MZ7TY256HDHP-000L7`
  reads as `SAMSUNG MZ7TY256`. Pick a `--model` word from the start of the string.
- **The name on the box may not be in the model at all.** A PNY CS900 reports
  `PNY 250GB SATA S`. Boot a Talos ISO and run `talosctl get disks --insecure` to
  read it before building anything that matches on it.
- **Alpine needs a marker file.** Providing an apkovl turns off Alpine's default
  boot services, and with them driver loading, unless the overlay contains
  `etc/.default_boot_services`.
- **Both images are small on purpose.** `write.sh` wipes the stick and writes only
  the image, so most of the stick stays unallocated.
