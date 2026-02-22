# Talos Bootstrap for bare metal clusters

Prerequisites:
- TalosCTL installed
- Docker Installed and Host networking is available
- Etcher or DD to burn the custom iPXE boot image. (Optional)
- Small flash drive or CD
- iPXE supported NIC.
- Kubectl Installed
- WSL if on Windows using Ubuntu

Recommended:
- Load balancer to split load across control plane nodes.

Warnings:
- Ensure your TalosCTL version matches the exact version of the Talos you are going to be deploying. This is critical as you will run into many weird issues if you don't follow this.
- Double check the IP settings on the matchbox control plane profile for the initial control plane node. Messing this up can cause your initial control plane node to be unreachable. Kernel args are not persisted on reboots.
- If you have freezing issues with Talos on startup and are using an AMD GPU you likely need to install the AMD driver patches. The patched images for this can be generated [here](https://factory.talos.dev/) The patches I needed for my specific AMD setup were: `siderolabs/amdgpu-firmware (20240513)` and `siderolabs/amd-ucode (20240513)`. This one in particular caused a lot of headaches.
- Since I don't feel like altering my DHCP server I opted to use iPXE with matchbox using a custom init script. 
- With my 2.5Gbe Realtek NICs they don't work for installation because iPXE lacks the needed drivers. You may need to bootstrap using the onboard NIC and switchover in the machine config.  
- If using IPV6 and IPV4 I highly recommend forcing the default IPV4 gateway in the machine config as I experienced an issue where Talos grabbed onto the IPV6 gateway address.

## Acknowledgements

This workflow started with the excellent groundwork in [dellathefella/talos-baremetal-install](https://github.com/dellathefella/talos-baremetal-install/tree/master). Huge thanks to the original author for openly sharing their approach—it provided the foundation that made these customizations and homelab tweaks possible.

## Prepare Talos install disk

Talos installs expect the target disk to be wiped, so double check the device path before you touch anything.

> If using WSL: https://learn.microsoft.com/en-us/windows/wsl/connect-usb#attach-a-usb-device

1. **Identify the USB disk**
    ```bash
    lsblk -o NAME,SIZE,MODEL,SERIAL,MOUNTPOINT
    ls -la /dev/disk/by-id/
    ```
    Compare the output before and after plugging in the Talos install USB. Match the size/model so you are 100% certain of the device (for example `/dev/sde`).
2. **Optional: back up existing data** if the drive was previously used.
3. **Format with an explicit label** once you are sure about the device. The Talos machineconfigs in this repo expect `/dev/disk/by-label/TALOS_INSTALL` (see `talos-machineconfigs/all.yaml`).
    ```bash
    sudo mkfs.ext4 -F -L TALOS_INSTALL /dev/sdX  # replace sdX with your confirmed device
    lsblk -o NAME,LABEL,SIZE /dev/sdX            # verify the label
    ```
    Using the label avoids surprises when multiple USB devices are attached—the matchbox configs will always target the labeled disk.

## Talos config setup

1. First thing we need to do is generate config files for our Talos Cluster. These patches contain the bare minimum to get the cluster up and running. Additional configuration may be required based on your setup for additional information on config options look [here](https://www.talos.dev/v1.7/reference/configuration/v1alpha1/config/). The following strategic merge patches are included:

```yaml
# all.yaml
machine:
  install:
    wipe: true
    disk: /dev/disk/by-label/TALOS_INSTALL
    extraKernelArgs:
      - talos.platform=metal
      - reboot=k
  certSANs:
    - ${HOST_IP}
---
# cp.yaml
machine:
  network:
    interfaces:
    - deviceSelector:
        physical: true
      addresses:
        - ${HOST_IP}/${CIDR}
      vip:
        ip: ${VIP_IP}
cluster:
  allowSchedulingOnControlPlanes: true
---
# work.yaml
machine:
  network:
    interfaces:
    - deviceSelector:
        physical: true
      dhcp: true
```

Let's generate the configurations for our cluster. The initial IP used to generate the configuration should match your load balancer or be mapped to the first control plane nodes IP. The latter is not recommended for production usage but makes homelab setup simpler.
```bash
cd talos-machineconfigs &&
talosctl gen config test-cluster https://<FIRST_NODE_IP>:6443 \
    --config-patch @all.yaml \
    --config-patch-control-plane @cp.yaml \
    --config-patch-worker @work.yaml \
    --output-dir ../assets/talos/1.11.5
```

The command above drops `controlplane.yaml`, `worker.yaml`, and `talosconfig` directly into `assets/talos/1.11.5/`. Copy `controlplane.yaml` to `controlplane1.yaml`, `controlplane2.yaml`, and `controlplane3.yaml`, then adjust the per-node details (IP addresses, VIP, etc.) to match your environment. The repository already contains populated examples you can tweak in place.

## matchbox setup

Identify the MAC addresses for each node so matchbox can assign the correct profile during PXE boot.

This repository includes ready-to-edit selectors under `groups/`:

- `groups/cp-1.11.5.1.json`
- `groups/cp-1.11.5.2.json`
- `groups/cp-1.11.5.3.json`
- `groups/worker-1.11.5-amd64.json`
- `groups/worker-1.11.5.-arm64.json`
- `groups/default.json`

Update the `selector.mac` values to match your hardware. Example `groups/cp-1.11.5.1.json`:

```json
{
    "id": "cp-1.11.5-1",
    "name": "cp-1.11.5-1",
    "profile": "cp-1.11.5-1",
    "selector": {
        "mac": "dc:a6:32:c3:f6:ad"
    }
}
```

Workers use arrays so you can list multiple MACs per architecture:

```json
{
    "id": "worker-1.11.5-amd64",
    "name": "worker-1.11.5-amd64",
    "profile": "worker-1.11.5-amd64",
    "selector": {
        "mac": [
            "dc:a6:32:c3:f6:xx"
        ]
    }
}
```

Profiles live under `profiles/` and point at the versioned Talos assets in `assets/talos/1.11.5/`. Replace the hard-coded matchbox host (`10.0.112.2` in these examples) with your own IP or DNS name as needed.

`profiles/cp-1.11.5-1.json`
```json
{
    "id": "cp-1.11.5-1",
    "name": "cp-1.11.5-1",
    "boot": {
        "kernel": "/assets/talos/1.11.5/arm64/vmlinuz",
        "initrd": [
            "/assets/talos/1.11.5/arm64/initramfs.xz"
        ],
        "args": [
            "initrd=initramfs.xz",
            "init_on_alloc=1",
            "init_on_free=1",
            "slab_nomerge",
            "pti=on",
            "console=tty0",
            "console=ttyS0",
            "printk.devkmsg=on",
            "talos.platform=metal",
            "talos.config=http://10.0.112.2/assets/talos/1.11.5/controlplane1.yaml"
        ]
    }
}
```

`profiles/worker-1.11.5-amd64.json`
```json
{
    "id": "worker-1.11.5-amd64",
    "name": "worker-1.11.5-amd64",
    "boot": {
        "kernel": "/assets/talos/1.11.5/amd64/vmlinuz",
        "initrd": [
            "/assets/talos/1.11.5/amd64/initramfs.xz"
        ],
        "args": [
            "initrd=initramfs.xz",
            "init_on_alloc=1",
            "init_on_free=1",
            "slab_nomerge",
            "pti=on",
            "console=tty0",
            "console=ttyS0",
            "printk.devkmsg=on",
            "talos.platform=metal",
            "talos.config=http://10.0.112.2/assets/talos/1.11.5/worker.yaml"
        ]
    }
}
```

`profiles/worker-1.11.5-arm64.json` mirrors the same configuration but consumes the `arm64` kernel and initramfs in `assets/talos/1.11.5/arm64/`. A placeholder `groups/default.json` is included if you want to add a fallback profile—create a matching `profiles/default.json` or remove the group if you do not need it.

## iPXE loader setup

2. This step is entirely optional but you can build your own custom iPXE boot drive [docs](https://ipxe.org/embed). For our purposes we embed a small script that will point to the host IP of our matchbox server. In this case it's my desktop running Docker desktop with host networking enabled on port 8080. Alternatively you could just install the bare iPXE loader and type in the following commands listed in the script below. 

```ipxe
#!ipxe
echo Configure dhcp .... &&
dhcp &&
chain http://<MATCHBOX_SERVER_IP>:8080/boot.ipxe
```

```bash
# Install dependencies for building iPXE
sudo apt install mkisofs genisoimage isolinux syslinux mtools liblzma-dev -y
# Add gcc-aarch64-linux-gnu if you need to build ARM64 EFI binaries
git clone https://github.com/ipxe/ipxe.git
cd ipxe/src
make
# Embed boot script from above.
make bin/ipxe.lkrn bin-x86_64-efi/ipxe.efi EMBED=boot.ipxe
./util/genfsimg -o ipxe.usb bin/ipxe.lkrn bin-x86_64-efi/ipxe.efi
```

> Shortcut: run `./ipxe-builder/build-ipxe.sh <matchbox_host>` to generate the artifacts automatically. Use `--port` to override the Matchbox port and `--arch both` if you need the arm64 EFI image alongside x86_64 (requires `gcc-aarch64-linux-gnu`).
>
> The script creates the following artifacts inside the requested build directory (defaults to `ipxe-builder/build/`):
> - `boot.ipxe` – embedded script pointing to your Matchbox host/port
> - `ipxe-x86_64.usb` – USB image for legacy BIOS/UEFI x86_64 systems
> - `ipxe-x86_64.lkrn` – Linux kernel image for chainloading
> - `ipxe-x86_64.efi` – UEFI binary for x86_64
> - `ipxe-arm64.efi` – (optional) UEFI binary for arm64 when `--arch arm64|both` is used
> - `ipxe-arm64.usb` – FAT-formatted image containing `BOOTAA64.EFI`
> - `ipxe-arm64.img` – MBR-partitioned disk image with the FAT payload at 1 MiB, ready to `dd` onto Raspberry Pi media
>
> Copy the binaries you need (typically `ipxe-x86_64.efi` and/or `ipxe-arm64.efi`) into your Matchbox assets directory or onto removable media for bootstrapping.

For Raspberry Pi boot media, write `ipxe-arm64.img` to the USB device (it already includes the required FAT partition and `BOOTAA64.EFI`):

```bash
sudo dd if=ipxe-arm64.img of=/dev/sdX bs=4M status=progress conv=fsync
```

You are now ready to use Etcher or DD to write the ipxe.usb file to a flash drive.

### Talos image assets

This repository tracks the downloaded Talos PXE assets under `assets/talos/<version>/<arch>/`. Each architecture directory includes a `commands.md` file that documents the exact `wget` commands used to fetch `vmlinuz` and `initramfs.xz` so you can refresh them later.

Control-plane and worker machine configuration templates for Talos v1.11.5 live under `assets/talos/1.11.5/` (see `controlplane{1..3}.yaml`, `worker.yaml`, and `talosconfig`). Earlier examples for v1.10.3 are kept in `assets/talos/1.10.3/` for reference.

## Talos cluster creation

3. Let's begin the process of creating our cluster. First thing we need to is copy the `assets,profiles and groups` to the matchbox directory. The images specified below have the AMD patches applied to them to fix my freezing issue. You can use the image factory to generate images with the patches you need for your hardware. [Image factory](https://factory.talos.dev/)

```bash
# Copying configurations for matchbox to use.
sudo mkdir -p /var/lib/matchbox/assets/talos/1.11.5/{amd64,arm64} &&
sudo cp assets/talos/1.11.5/*.yaml /var/lib/matchbox/assets/talos/1.11.5/ &&
sudo cp assets/talos/1.11.5/amd64/* /var/lib/matchbox/assets/talos/1.11.5/amd64/ &&
sudo cp assets/talos/1.11.5/arm64/* /var/lib/matchbox/assets/talos/1.11.5/arm64/ &&
sudo mkdir -p /var/lib/matchbox/profiles &&
sudo cp profiles/*.json /var/lib/matchbox/profiles/ &&
sudo mkdir -p /var/lib/matchbox/groups &&
sudo cp groups/*.json /var/lib/matchbox/groups/ &&
# Starting matchbox with the mounts we created beforehand.
sudo docker run -d --net=host --rm -v /var/lib/matchbox:/var/lib/matchbox:Z -v /etc/matchbox:/etc/matchbox:Z,ro quay.io/poseidon/matchbox:v0.10.0 -address=0.0.0.0:8080 -log-level=debug
```

The `commands.md` files in `assets/talos/1.11.5/{amd64,arm64}/` capture the exact `wget` invocations used to download the current kernels and initramfs artifacts—rerun them when you need to refresh the images.

### Syncing repo changes to a persistent Matchbox host

If you are running Matchbox persistently on Unraid, use `sync-to-matchbox.sh` from this directory to mirror the repo’s `assets/`, `profiles/`, and `groups/` to the server:

```bash
chmod +x sync-to-matchbox.sh
./sync-to-matchbox.sh              # defaults to root@10.0.0.12
./sync-to-matchbox.sh user@host    # optional override
```

The script will create `/mnt/user/appdata/matchbox/{assets,profiles,groups}` on the remote host (if missing) and uses `rsync --delete --exclude='*example'` to keep the directories in sync. Example templates (`*.example`) stay local only.

On your cluster machines set the primary boot device to be your iPXE flash drive. The system will grab a DHCP address to start talking with the matchbox server. Based on the contents of the get request from the machine matchbox will serve up the profile it matches with. This can take a while so be patient.

## Control plane bootstrapping

4. The initial control plane will come online and begin in an unhealthy state as `etcd` is not bootstrapped. To fix this we need to run the following commands:

```bash
cd assets/talos/1.11.5
# etcd bootstrap
talosctl --talosconfig talosconfig config endpoint <FIRST_NODE_IP>
talosctl --talosconfig talosconfig config node <FIRST_NODE_IP>
talosctl --talosconfig talosconfig bootstrap
talosctl --talosconfig talosconfig kubeconfig .
# You should be able to see the nodes at this point
kubectl --kubeconfig kubeconfig get nodes
# Updating machine config if you have a typo
talosctl --talosconfig talosconfig -n <FIRST_NODE_IP> -e <FIRST_NODE_IP> apply machineconfig -f controlplane1.yaml --mode=no-reboot
```
