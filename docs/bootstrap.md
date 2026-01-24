# Bootstrap & Bare‑metal Talos

This guide captures the full bare‑metal bootstrap flow using Matchbox + iPXE and Talos.

## Where Things Live
- Talos assets: `matchbox/assets/talos/<version>/`
- Matchbox groups: `matchbox/groups/`
- Matchbox profiles: `matchbox/profiles/`
- Talos machine configs: `matchbox/assets/talos/<version>/*.yaml`
- iPXE builder: `ipxe-builder/`

## Talos Config Generation
Machine config patches live under:
- `matchbox/talos-machineconfigs/all.yaml`
- `matchbox/talos-machineconfigs/cp.yaml`
- `matchbox/talos-machineconfigs/work.yaml`

Generate configs into the assets directory, then copy and customize per‑node configs (e.g., `controlplane1.yaml`, `controlplane2.yaml`, `controlplane3.yaml`).

## Matchbox: Groups & Profiles
Groups map MAC addresses to profiles. Profiles define boot kernel/initrd and point to a Talos config URL. Update these files whenever hardware MACs or IPs change:
- `matchbox/groups/*.json`
- `matchbox/profiles/*.json`

## iPXE Boot Media
Use the helper script to build boot artifacts:
- `ipxe-builder/build-ipxe.sh <matchbox_host> [--port] [--arch both]`

Artifacts are placed in `ipxe-builder/build/`.

## Sync to a Matchbox Host
If Matchbox runs on another host (e.g., Unraid), use:
- `matchbox/sync-to-matchbox.sh`

This mirrors `assets/`, `profiles/`, and `groups/` to the remote Matchbox instance.

## Control Plane Bootstrap
After the first control plane node comes up, bootstrap etcd and retrieve kubeconfig using `talosctl`. The repo’s Talos configs and matchbox profiles are designed to match the disk label `TALOS_INSTALL`.

## Key Risks / Common Pitfalls
- Talos version mismatch between `talosctl` and the node images.
- Wrong IP/MAC mappings in Matchbox groups and profiles.
- Forgetting to update `talos.config=` in profile boot args.
- DHCP / NIC driver limitations in iPXE (notably some Realtek chipsets).

Refer to `matchbox/README.md` for the full, step‑by‑step procedure and examples.
