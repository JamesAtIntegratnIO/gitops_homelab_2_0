# Bootstrap & Bare-Metal Talos

> **Official Documentation References:**
> - [Talos Linux](https://www.talos.dev/latest/) - Immutable Kubernetes OS
> - [Matchbox](https://matchbox.psdn.io/) - Network boot server
> - [iPXE](https://ipxe.org/docs) - Network boot firmware
> - [DHCP Network Booting](https://ipxe.org/howto/chainloading) - iPXE chainloading guide
> - [Talosctl CLI Reference](https://www.talos.dev/latest/reference/cli/) - Talos management commands

## Overview

The bootstrap process transforms bare metal machines into a fully operational Talos Linux Kubernetes cluster using **network boot** (PXE/iPXE) and declarative machine configurations. This approach provides:

**Benefits:**
- ✅ **Immutable infrastructure**: OS changes require reboot (no configuration drift)
- ✅ **Declarative machine configs**: Entire node config in YAML (version controlled)
- ✅ **Rapid provisioning**: New nodes boot and join cluster automatically
- ✅ **Easy upgrades**: Change boot profile, reboot, done
- ✅ **Consistent hardware**: All nodes run identical OS image

**Tradeoffs:**
- ❌ Network boot dependency (DHCP + Matchbox must be operational)
- ❌ More complex initial setup vs traditional installers
- ❌ Requires USB-labeled disks or disk path discovery

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│  Bare Metal Node (Powered On)                                   │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │ BIOS/UEFI: PXE Boot Enabled (Network Boot Priority)       │  │
│  └─────────────────────┬─────────────────────────────────────┘  │
│                        │ 1. DHCP Discover (MAC: 00:23:24:e7:29:40)
│                        ▼
│  ┌───────────────────────────────────────────────────────────┐  │
│  │ DHCP Server (Unraid/Router)                               │  │
│  │   - IP: 10.0.4.101                                        │  │
│  │   - Next-server: 10.0.112.2 (Matchbox)                    │  │
│  │   - Filename: undionly.kpxe (iPXE binary)                 │  │
│  └─────────────────────┬─────────────────────────────────────┘  │
│                        │ 2. DHCP Offer + Boot Instructions
│                        ▼
│  ┌───────────────────────────────────────────────────────────┐  │
│  │ Firmware loads undionly.kpxe from Matchbox TFTP           │  │
│  │ iPXE chainloads to: http://10.0.112.2/boot.ipxe?mac=...   │  │
│  └─────────────────────┬─────────────────────────────────────┘  │
│                        │ 3. HTTP GET /boot.ipxe
│                        ▼
│  ┌───────────────────────────────────────────────────────────┐  │
│  │ Matchbox Server (10.0.112.2)                              │  │
│  │   - Matches MAC → Group (cp-1.11.3.1.json)                │  │
│  │   - Returns Profile (cp-1.11.3-1.json)                    │  │
│  │   - iPXE script loads:                                    │  │
│  │       kernel: /assets/talos/1.11.3/amd64/vmlinuz          │  │
│  │       initrd: /assets/talos/1.11.3/amd64/initramfs.xz     │  │
│  │       args: talos.config=http://...controlplane1.yaml     │  │
│  └─────────────────────┬─────────────────────────────────────┘  │
│                        │ 4. Boot Talos Linux kernel
│                        ▼
│  ┌───────────────────────────────────────────────────────────┐  │
│  │ Talos Kernel + InitRAMFS                                  │  │
│  │   - Fetch machine config: controlplane1.yaml              │  │
│  │   - Validate machine config against schema                │  │
│  │   - Install Talos to /dev/disk/by-label/TALOS_INSTALL     │  │
│  │   - Apply machine config (kubelet, etcd, certs, etc.)     │  │
│  └─────────────────────┬─────────────────────────────────────┘  │
│                        │ 5. Reboot from disk
│                        ▼
│  ┌───────────────────────────────────────────────────────────┐  │
│  │ Talos Linux Running (Control Plane Node 1)                │  │
│  │   - etcd: 10.0.4.101:2380                                 │  │
│  │   - kube-apiserver: 10.0.4.101:6443                       │  │
│  │   - Ready to bootstrap cluster                            │  │
│  └───────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

## Bootstrap Flow (Detailed Steps)

### Phase 1: Generate Machine Configs

**Command:**
```bash
cd matchbox/talos-machineconfigs/

# Generate base configs with patches
talosctl gen config the-cluster https://10.0.4.100 \
  --config-patch @all.yaml \
  --config-patch-control-plane @cp.yaml \
  --config-patch-worker @work.yaml
```

**What Happens:**
1. `talosctl` generates:
   - `controlplane.yaml` - Base control-plane config
   - `worker.yaml` - Base worker config
   - `talosconfig` - Admin credentials file
2. Applies patches:
   - `all.yaml` - Settings for all nodes (disk, certSANs, kernel args)
   - `cp.yaml` - Control-plane specific settings
   - `work.yaml` - Worker specific settings

**Generated Patches Example** ([matchbox/talos-machineconfigs/all.yaml](../matchbox/talos-machineconfigs/all.yaml)):
```yaml
machine:
  install:
    wipe: true
    disk: /dev/disk/by-label/TALOS_INSTALL  # USB-labeled disk
    extraKernelArgs:
      - talos.platform=metal
      - reboot=k  # Kernel reboot (faster)
  certSANs:
    - 10.0.4.100  # VIP for API server
    - 10.0.4.101  # Control plane node 1
    - 10.0.4.102  # Control plane node 2
    - 10.0.4.103  # Control plane node 3
  network:
    interfaces:
      - interface: eth0
        dhcp: true
  time:
    servers:
      - time.cloudflare.com
```

**Output Files:**
- `controlplane.yaml` - Full rendered control-plane config
- `worker.yaml` - Full rendered worker config
- `talosconfig` - CLI credentials (merge into ~/.talos/config)

### Phase 2: Customize Per-Node Configs

**Copy and customize configs for each control-plane node:**
```bash
# Copy base config three times
cp controlplane.yaml ../assets/talos/1.11.3/controlplane1.yaml
cp controlplane.yaml ../assets/talos/1.11.3/controlplane2.yaml
cp controlplane.yaml ../assets/talos/1.11.3/controlplane3.yaml

# Edit each file to set unique hostname and IP
yq eval '.machine.network.hostname = "controlplane1"' -i ../assets/talos/1.11.3/controlplane1.yaml
yq eval '.machine.network.interfaces[0].addresses = ["10.0.4.101/24"]' -i ../assets/talos/1.11.3/controlplane1.yaml

yq eval '.machine.network.hostname = "controlplane2"' -i ../assets/talos/1.11.3/controlplane2.yaml
yq eval '.machine.network.interfaces[0].addresses = ["10.0.4.102/24"]' -i ../assets/talos/1.11.3/controlplane2.yaml

yq eval '.machine.network.hostname = "controlplane3"' -i ../assets/talos/1.11.3/controlplane3.yaml
yq eval '.machine.network.interfaces[0].addresses = ["10.0.4.103/24"]' -i ../assets/talos/1.11.3/controlplane3.yaml
```

**Why Per-Node Configs:**
- Each node needs unique hostname and IP
- etcd requires static IPs for cluster members
- Future: Use Talos machine config server for dynamic configs

### Phase 3: Create Matchbox Groups (MAC → Profile Mapping)

**Group Example** ([matchbox/groups/cp-1.11.3.1.json](../matchbox/groups/cp-1.11.3.1.json)):
```json
{
    "id": "cp-1.11.3-1",
    "name": "Control Plane Node 1",
    "profile": "cp-1.11.3-1",
    "selector": {
        "mac": "00:23:24:e7:29:40"  # Hardware MAC address
    }
}
```

**Matchbox Matching Logic:**
1. Node boots → DHCP assigns IP → iPXE chainloads → Matchbox receives `GET /boot.ipxe?mac=00:23:24:e7:29:40`
2. Matchbox scans all groups in `matchbox/groups/` directory
3. Matches `selector.mac` against request MAC
4. Returns iPXE script referencing profile `cp-1.11.3-1`

**How to Find MAC Addresses:**
```bash
# Option 1: From existing machine
ip link show | grep ether

# Option 2: From DHCP server logs
grep "DHCP DISCOVER" /var/log/syslog

# Option 3: From router admin UI (DHCP leases)
```

### Phase 4: Create Matchbox Profiles (Boot Config)

**Profile Example** ([matchbox/profiles/cp-1.11.3-1.json](../matchbox/profiles/cp-1.11.3-1.json)):
```json
{
    "id": "cp-1.11.3-1",
    "name": "Talos Control Plane v1.11.3 Node 1",
    "boot": {
        "kernel": "/assets/talos/1.11.3/amd64/vmlinuz",
        "initrd": [
            "/assets/talos/1.11.3/amd64/initramfs.xz"
        ],
        "args": [
            "initrd=initramfs.xz",
            "init_on_alloc=1",         # Security: zero-init allocations
            "init_on_free=1",          # Security: zero-init freed memory
            "slab_nomerge",            # Security: prevent slab merging exploits
            "pti=on",                  # Security: page table isolation
            "console=tty0",            # Console output to VGA
            "console=ttyS0",           # Console output to serial port
            "printk.devkmsg=on",       # Kernel messages to dmesg
            "talos.platform=metal",    # Platform type
            "talos.config=http://10.0.112.2/assets/talos/1.11.3/controlplane1.yaml"
        ]
    }
}
```

**Kernel Args Explained:**
- `talos.config=URL`: Where Talos fetches machine config during boot
- `talos.platform=metal`: Tells Talos it's running on bare metal (not cloud)
- Security flags (`init_on_*`, `pti=on`): Hardening against speculative execution attacks
- Console args: Enable output to both VGA and serial (useful for IPMI SOL debugging)

### Phase 5: Download Talos Assets

```bash
cd matchbox/assets/talos/1.11.3/

# Download kernel and initramfs for amd64
curl -LO https://github.com/siderolabs/talos/releases/download/v1.11.3/vmlinuz-amd64
curl -LO https://github.com/siderolabs/talos/releases/download/v1.11.3/initramfs-amd64.xz

# Rename to match profile paths
mkdir -p amd64
mv vmlinuz-amd64 amd64/vmlinuz
mv initramfs-amd64.xz amd64/initramfs.xz

# Verify checksums (highly recommended)
curl -LO https://github.com/siderolabs/talos/releases/download/v1.11.3/sha512sum.txt
sha512sum --check sha512sum.txt --ignore-missing
```

**Why These Files:**
- `vmlinuz`: Linux kernel optimized for Talos
- `initramfs.xz`: Root filesystem with Talos init system, containerd, kubelet

### Phase 6: Build iPXE Boot Media

**Script Usage:**
```bash
cd ipxe-builder/

# Build for both BIOS and UEFI
./build-ipxe.sh 10.0.112.2 --port 8080 --arch both

# Output files in ipxe-builder/build/:
#   - undionly.kpxe (BIOS)
#   - ipxe.efi (UEFI)
```

**What This Does:**
1. Clones iPXE source from GitHub
2. Embeds chainload script pointing to Matchbox
3. Compiles iPXE binaries with embedded script
4. Resulting binaries automatically boot from network and chainload to Matchbox

**Embedded Chainload Script:**
```ipxe
#!ipxe
dhcp
chain http://10.0.112.2:8080/boot.ipxe?mac=${net0/mac}
```

### Phase 7: Sync to Matchbox Server

**Script:**
```bash
cd matchbox/
./sync-to-matchbox.sh
```

**What Gets Synced:**
```bash
rsync -avz --delete \
  assets/ user@10.0.112.2:/var/lib/matchbox/assets/ \
  profiles/ user@10.0.112.2:/var/lib/matchbox/profiles/ \
  groups/ user@10.0.112.2:/var/lib/matchbox/groups/
```

**Matchbox Directory Structure:**
```
/var/lib/matchbox/
├── assets/
│   └── talos/
│       └── 1.11.3/
│           ├── amd64/
│           │   ├── vmlinuz
│           │   └── initramfs.xz
│           ├── controlplane1.yaml
│           ├── controlplane2.yaml
│           └── controlplane3.yaml
├── groups/
│   ├── cp-1.11.3.1.json
│   ├── cp-1.11.3.2.json
│   └── cp-1.11.3.3.json
└── profiles/
    ├── cp-1.11.3-1.json
    ├── cp-1.11.3-2.json
    └── cp-1.11.3-3.json
```

### Phase 8: First Boot (Control Plane Node 1)

**Power on the first control plane machine. Boot sequence:**

1. **BIOS/UEFI**: Network boot (PXE)
2. **DHCP**: Receives IP + Matchbox server address
3. **iPXE**: Loads `undionly.kpxe` → chainloads to Matchbox
4. **Matchbox**: Returns profile → iPXE loads `vmlinuz` + `initramfs.xz`
5. **Talos**: Fetches `controlplane1.yaml` → installs to disk → reboots
6. **Node Ready**: Talos running from disk, waiting for cluster bootstrap

**Watch Boot Progress:**
```bash
# Matchbox logs (server side)
journalctl -u matchbox -f

# Talos logs (node side - requires serial console or IPMI)
# Watch for:
#   [talos] fetching machine config
#   [talos] installing to /dev/sda
#   [talos] machine is ready
```

### Phase 9: Bootstrap etcd and Kubernetes

**On your workstation with talosctl:**

```bash
# Set endpoint to first control plane node
export TALOSCONFIG=~/projects/gitops_homelab_2_0/matchbox/talos-machineconfigs/talosconfig
talosctl config endpoint 10.0.4.101

# Bootstrap etcd (creates initial cluster)
talosctl bootstrap

# Wait for control plane to come up (~2-3 minutes)
talosctl dashboard

# Expected output:
#   NAMESPACE   NAME                 READY
#   system      etcd                 Healthy
#   system      kube-apiserver       Healthy
#   system      kube-controller-manager  Healthy
#   system      kube-scheduler       Healthy

# Generate kubeconfig
talosctl kubeconfig ~/.kube/config-the-cluster --force

# Verify cluster access
export KUBECONFIG=~/.kube/config-the-cluster
kubectl get nodes

# Expected output:
#   NAME            STATUS   ROLES           AGE   VERSION
#   controlplane1   Ready    control-plane   2m    v1.34.1
```

### Phase 10: Add Additional Control Plane Nodes

**Power on controlplane2 and controlplane3:**

```bash
# Nodes auto-join via machine config (no manual bootstrap)
# Watch nodes join:
kubectl get nodes -w

# Expected sequence:
#   controlplane1   Ready    control-plane   5m    v1.34.1
#   controlplane2   Ready    control-plane   1m    v1.34.1
#   controlplane3   Ready    control-plane   30s   v1.34.1

# Verify etcd cluster health
talosctl --nodes 10.0.4.101,10.0.4.102,10.0.4.103 etcd members

# Expected output:
#   NODE          ID              ROLE      ERRORS
#   10.0.4.101    member-1        leader    none
#   10.0.4.102    member-2        follower  none
#   10.0.4.103    member-3        follower  none
```

### Phase 11: Bootstrap GitOps (Terraform)

**Once cluster is operational:**

```bash
cd terraform/cluster/

# Initialize Terraform with remote backend
terraform init -backend-config=backend.hcl

# Review planned changes
terraform plan

# Apply Terraform (installs ArgoCD + bootstrap ApplicationSets)
terraform apply

# Verify ArgoCD is running
kubectl get pods -n argocd

# Access ArgoCD UI
kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 -d

# Port-forward to access UI
kubectl port-forward svc/argocd-server -n argocd 8080:443

# Open browser: https://localhost:8080
# Username: admin
# Password: <from secret above>
```

**What Terraform Deploys:**
- ArgoCD Helm chart (v9.0.3)
- Bootstrap ApplicationSets:
  - `application-sets-control-plane` → Syncs control-plane addons
  - `application-sets-vcluster` → Syncs vCluster addons
- Cluster Secret with labels (for ApplicationSet selectors)
- GHCR pull secret (for private images)

## File Reference

### Machine Config Files
- **Patch templates**: [matchbox/talos-machineconfigs/all.yaml](../matchbox/talos-machineconfigs/all.yaml), [cp.yaml](../matchbox/talos-machineconfigs/cp.yaml), [work.yaml](../matchbox/talos-machineconfigs/work.yaml)
- **Generated configs**: `matchbox/assets/talos/1.11.3/controlplane{1,2,3}.yaml`
- **Commands reference**: [matchbox/talos-machineconfigs/commands.md](../matchbox/talos-machineconfigs/commands.md)

### Matchbox Files
- **Groups** (MAC mapping): [matchbox/groups/](../matchbox/groups/)
- **Profiles** (boot config): [matchbox/profiles/](../matchbox/profiles/)
- **Assets** (kernels, configs): [matchbox/assets/talos/](../matchbox/assets/talos/)
- **Sync script**: [matchbox/sync-to-matchbox.sh](../matchbox/sync-to-matchbox.sh)

### Terraform Files
- **Main config**: [terraform/cluster/main.tf](../terraform/cluster/main.tf)
- **Variables**: [terraform/cluster/variables.tf](../terraform/cluster/variables.tf), [terraform.tfvars](../terraform/cluster/terraform.tfvars)
- **Bootstrap manifests**: [terraform/cluster/bootstrap/](../terraform/cluster/bootstrap/)

### iPXE Files
- **Build script**: [matchbox/ipxe-builder/build-ipxe.sh](../matchbox/ipxe-builder/build-ipxe.sh)
- **Output artifacts**: `matchbox/ipxe-builder/build/undionly.kpxe`, `ipxe.efi`

## Troubleshooting

### Node Won't PXE Boot

**Symptoms:**
- Node shows "No bootable device"
- DHCP times out
- PXE boot not in BIOS boot order

**Diagnosis:**
```bash
# Check DHCP server logs
grep "DHCP DISCOVER" /var/log/syslog | grep <MAC address>

# Verify Matchbox HTTP server is running
curl http://10.0.112.2:8080/boot.ipxe

# Check TFTP service (for iPXE binary delivery)
tftp 10.0.112.2 -c get undionly.kpxe /tmp/test.kpxe
```

**Common Fixes:**
- ✅ Enable network boot in BIOS/UEFI
- ✅ Set network boot as first priority
- ✅ Verify DHCP next-server points to Matchbox IP
- ✅ Check firewall allows TFTP (port 69) and HTTP (port 8080)
- ✅ Ensure Matchbox TFTP directory contains iPXE binaries

### Matchbox Returns Wrong Profile

**Symptoms:**
- Node boots but gets wrong kernel
- Machine config URL 404
- Wrong hostname applied

**Diagnosis:**
```bash
# Check Matchbox logs for MAC match
journalctl -u matchbox -f | grep <MAC>

# Manually test Matchbox API
curl "http://10.0.112.2:8080/boot.ipxe?mac=00:23:24:e7:29:40"

# Verify group selector matches MAC
cat matchbox/groups/cp-1.11.3.1.json | jq '.selector.mac'
```

**Common Fixes:**
- ✅ Verify MAC address is correct (check `ip link` on node)
- ✅ Ensure group JSON uses lowercase MAC address
- ✅ Check profile referenced in group exists in `profiles/` directory
- ✅ Resync files to Matchbox server after changes

### Talos Fails to Fetch Machine Config

**Symptoms:**
- Node boots but hangs at "fetching machine config"
- Error: "failed to download config: 404 Not Found"

**Diagnosis:**
```bash
# Verify machine config URL is accessible
curl http://10.0.112.2/assets/talos/1.11.3/controlplane1.yaml

# Check Matchbox asset paths
ls -la /var/lib/matchbox/assets/talos/1.11.3/

# View boot args from profile
cat matchbox/profiles/cp-1.11.3-1.json | jq '.boot.args'
```

**Common Fixes:**
- ✅ Ensure machine config file exists at path specified in profile
- ✅ Verify machine config YAML is valid: `talosctl validate --mode metal controlplane1.yaml`
- ✅ Check Matchbox HTTP server is serving assets correctly
- ✅ Confirm node can reach Matchbox server (no firewall blocking)

### etcd Cluster Won't Form

**Symptoms:**
- Bootstrap command hangs
- `talosctl dashboard` shows etcd unhealthy
- Nodes stuck in NotReady state

**Diagnosis:**
```bash
# Check etcd logs
talosctl --nodes 10.0.4.101 logs etcd

# Verify control plane endpoint connectivity
talosctl --nodes 10.0.4.101 get members

# Check machine config has correct initial cluster members
talosctl --nodes 10.0.4.101 get machineconfig | yq eval '.cluster.etcd.initialCluster'
```

**Common Fixes:**
- ✅ Ensure all control plane IPs in `machine.certSANs` are correct
- ✅ Verify nodes can reach each other on port 2380 (etcd peer communication)
- ✅ Check no other etcd cluster is running (old data in `/var/lib/etcd`)
- ✅ Only bootstrap on **one** node - never run `talosctl bootstrap` on multiple nodes
- ✅ If retry needed: `talosctl reset --graceful --reboot` to wipe state

### Disk Installation Fails

**Symptoms:**
- Error: "failed to install: no disk found at /dev/disk/by-label/TALOS_INSTALL"
- Node reboots back to network boot

**Diagnosis:**
```bash
# Check available disks from Talos
talosctl --nodes 10.0.4.101 disks

# List disk labels
talosctl --nodes 10.0.4.101 ls /dev/disk/by-label/
```

**Common Fixes:**
- ✅ Create USB filesystem label: `mkfs.ext4 -L TALOS_INSTALL /dev/sdX`
- ✅ Update machine config to use disk path: `/dev/sda` instead of label
- ✅ Verify disk is not in use (unmount any partitions)
- ✅ Use `wipe: true` in machine config to force wipe existing partitions

### Talos Version Mismatch

**Symptoms:**
- Error: "unsupported Talos version"
- `talosctl` commands fail with protocol errors

**Diagnosis:**
```bash
# Check talosctl version
talosctl version

# Check node Talos version
talosctl --nodes 10.0.4.101 version
```

**Common Fixes:**
- ✅ Install matching `talosctl`: `brew install siderolabs/tap/talosctl` (macOS) or download from releases
- ✅ Update Matchbox assets to match node version
- ✅ Regenerate machine configs with matching `talosctl` version

### Network Driver Issues (Realtek NICs)

**Symptoms:**
- iPXE loads but network times out during kernel boot
- DHCP works in iPXE but Talos gets no IP

**Diagnosis:**
```bash
# Check network interfaces from Talos
talosctl --nodes 10.0.4.101 get links

# View kernel messages
talosctl --nodes 10.0.4.101 dmesg | grep eth
```

**Common Fixes:**
- ✅ Some Realtek NICs require firmware blobs not in Talos kernel
- ✅ Use Intel NICs if possible (better Linux driver support)
- ✅ Try kernel arg `talos.network.interface.ignore=eth1` to skip problematic interface
- ✅ Build custom Talos image with required firmware (advanced)

## Upgrade Procedures

### Upgrading Talos Version

**Steps:**
1. Download new Talos assets:
   ```bash
   cd matchbox/assets/talos/
   mkdir 1.12.0
   cd 1.12.0
   curl -LO https://github.com/siderolabs/talos/releases/download/v1.12.0/vmlinuz-amd64
   curl -LO https://github.com/siderolabs/talos/releases/download/v1.12.0/initramfs-amd64.xz
   ```

2. Regenerate machine configs with new version:
   ```bash
   talosctl gen config the-cluster https://10.0.4.100 \
     --talos-version v1.12.0 \
     --config-patch @all.yaml \
     --config-patch-control-plane @cp.yaml
   ```

3. Update Matchbox profiles to point to new assets:
   ```json
   {
     "kernel": "/assets/talos/1.12.0/amd64/vmlinuz",
     "initrd": ["/assets/talos/1.12.0/amd64/initramfs.xz"],
     "args": ["talos.config=http://10.0.112.2/assets/talos/1.12.0/controlplane1.yaml"]
   }
   ```

4. Sync to Matchbox and reboot nodes (one at a time):
   ```bash
   ./matchbox/sync-to-matchbox.sh
   talosctl --nodes 10.0.4.101 reboot
   # Wait for node to rejoin cluster before proceeding to next node
   ```

### Upgrading Kubernetes Version

**Using in-place upgrade (no reboot):**
```bash
# Upgrade to Kubernetes 1.35.0
talosctl --nodes 10.0.4.101,10.0.4.102,10.0.4.103 upgrade-k8s --to 1.35.0

# Monitor upgrade progress
kubectl get nodes -w
```

**Why This Works:**
- Talos manages Kubernetes as static pods
- Upgrade downloads new kubelet/apiserver/etc. images
- Gracefully restarts control plane components
- No machine reboot required

## Best Practices

### Disk Labeling Strategy
- ✅ **USB label approach**: Label installation disk with `TALOS_INSTALL` during hardware prep
- ✅ **Consistent across fleet**: Same label on all nodes simplifies machine config
- ❌ **Avoid `/dev/sda` paths**: Device names change based on boot order and attached USB drives

### Machine Config Management
- ✅ **Keep patches in Git**: All machine configs generated from versioned patches
- ✅ **Per-environment patches**: Separate `production/`, `staging/` patch directories
- ✅ **Test config generation**: Run `talosctl validate` before deploying
- ❌ **Don't manually edit generated configs**: Always regenerate from patches

### Matchbox Organization
- ✅ **Naming convention**: `cp-<version>-<node number>` (e.g., `cp-1.11.3-1`)
- ✅ **Version in paths**: Include Talos version in asset paths for easy rollback
- ✅ **Backup groups/profiles**: Keep copy in Git (this repo does this)

### Network Boot Reliability
- ✅ **Redundant DHCP**: Run DHCP failover if possible
- ✅ **Matchbox HA**: Consider running Matchbox on multiple servers
- ✅ **Local iPXE cache**: Some switches can cache iPXE binaries

### Security Considerations
- ⚠️ **Machine configs over HTTP**: Anyone on network can read configs (contains secrets)
  - **Mitigation**: Use network isolation (dedicated PXE VLAN)
  - **Future**: Use Talos machine config server with authentication
- ⚠️ **No encryption during boot**: Kernel/initramfs fetched over HTTP
  - **Mitigation**: Sign kernel/initramfs and verify in UEFI Secure Boot
- ✅ **Rotate Talos API credentials**: Regenerate `talosconfig` periodically
- ✅ **Limit Matchbox access**: Firewall rules to only allow cluster network
