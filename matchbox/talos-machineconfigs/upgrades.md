# Upgrading Talos and Kubernetes on the-cluster

Written 2026-10-29, when the cluster was Talos **1.11.5** / Kubernetes **v1.34.1**
and both were out of support.

Talos configuration is outside the GitOps loop, so none of this is applied by
ArgoCD. Every `talosctl upgrade` here is a manual step that reboots a node and
replaces its OS image.

## Scripted route

The steps below explain each stage. These scripts run them, each behind gates:

| Script | Does |
|---|---|
| `preflight.sh <target>` | Checks a target before anything moves |
| `ladder.sh [--dry-run] <talos-version>` | Upgrades every node to one Talos version, one rung per run; resumes if stopped |
| `k8s-upgrade.sh [--dry-run] <k8s-version>` | Moves Kubernetes one minor |
| `regen-served-configs.sh [--dry-run] <talos-version>` | Rebuilds the configs Matchbox serves from the live nodes |

If a node has to be rebuilt along the way, follow [replace-disk.md](replace-disk.md).
Run `regen-served-configs.sh` when the ladder finishes, so a later reinstall does
not bring back an old node.

## Why this is necessary

| | Running | |
|---|---|---|
| Kubernetes | v1.34.1 | **EOL 2026-10-27** — no further patches, including security |
| Talos | v1.11.5 | the 1.11 line no longer appears in the Sidero support matrix; its last patch was **v1.11.6** (2025-12-16) |

And the coupling that decides the order:

**A Talos minor caps the Kubernetes maximum.** Talos 1.11 supports Kubernetes
1.29–1.34 and nothing above it, so there is no route off EOL Kubernetes without
a Talos minor upgrade first. Neither project's changelog states this; it lives
only in the support matrix.

| Talos | Supported Kubernetes |
|---|---|
| 1.11 | 1.29 – 1.34 |
| 1.12 | 1.30 – 1.35 |
| 1.13 | 1.31 – 1.36 |
| 1.14 | 1.33 – 1.37 |

The useful consequence: **1.12, 1.13 and 1.14 all still support 1.34.** So Talos
can move the whole ladder with Kubernetes held constant, and Kubernetes moves
afterwards as a separate change. Two steps with one variable each beats one step
with two — when something breaks you know which half did it.

`preflight.sh` encodes that table and refuses a target whose range excludes the
running Kubernetes, or a jump that skips a minor.

## Step 0 — renew the os:admin certificate

**Done 2026-10-29.** The old cert had one day left (`notAfter Oct 30 17:10:25
2026 GMT`); the renewed one runs to **Oct 29 2027**, on context
`admin@the-cluster`. `preflight.sh` FAILs under 7 days remaining and WARNs
under 30, so the next one will not arrive with a day's notice.

This gates everything. `talosctl upgrade` needs a valid `os:admin` cert, and if
it lapses mid-ladder you cannot finish *or* roll back without regenerating from
the CA in `matchbox/assets/talos/<ver>/controlplane1.yaml` (gitignored, present
on the workstation that generated it).

### Once, now: drop the stale context

The 2026-10-29 renewal left two contexts. `the-cluster` still holds the cert
that expires on Oct 30; remove it so nothing reaches for it afterwards:

```bash
talosctl config remove the-cluster --dry-run
talosctl config remove the-cluster -y
talosctl config contexts                     # admin@the-cluster, current, alone
```

### Next time: the renewal procedure

`config merge` is not a replace, and three of its behaviours combine badly. All
three were tested against throwaway configs on 2026-10-29:

- On a **name collision** it renames the *incoming* context
  (`admin@the-cluster` → `admin@the-cluster-1`) and makes that current. The
  canonical name keeps the **old** cert.
- `config remove` **refuses to remove the current context** — it prints
  "skipping" and **exits 0**, so an `&&` chain carries straight on.
- A freshly generated context has **no default node**, so commands against it
  need `-n`.

Together, the obvious sequence — remove the old, merge the new, select it — ends
on the old cert, current, under the canonical name, with every step exiting 0.
This order works and leaves any other contexts alone:

```bash
# Absolute path on purpose: the trailing argument is written relative to your
# CWD, and a freshly minted talosconfig is a credential.
NEW="$HOME/.talos/talosconfig-new"
talosctl --context admin@the-cluster config new \
  --roles os:admin --crt-ttl 8760h "$NEW"

# Prove the new cert works BEFORE touching the one that does.
talosctl --talosconfig "$NEW" -n 10.0.4.101 version --short

talosctl config merge "$NEW"                    # lands as admin@the-cluster-1, current
talosctl config remove admin@the-cluster -y     # the old one: no longer current, so allowed
talosctl config merge "$NEW"                    # no collision now: admin@the-cluster, current
talosctl config remove admin@the-cluster-1 -y   # the interim copy
rm "$NEW"
```

Then check the result rather than the exit codes, which have already lied once:

```bash
./matchbox/talos-machineconfigs/preflight.sh    # "context: admin@the-cluster", "valid for 364d"
```

`preflight.sh` reads the current context, prints its name, and checks *that*
context's cert; override with `TALOS_CTX=...`. Before 2026-10-29 it read the
first `crt:` in the file while its etcd calls hard-coded `the-cluster`, so it
could report one cert healthy while the commands used another.

The first draft of this runbook verified the renewal with
`talosctl --context the-cluster version` — the *old* context. It passed because
the old cert still had a day left, so it could not have caught a bad renewal.

### Which of these commands care about your directory

Worth stating because this repo has both conventions:

| | CWD matters? |
|---|---|
| `talosctl ... upgrade` / `version` / `etcd members` | **No.** `TALOSCONFIG` is unset here, so talosctl reads `~/.talos/config` and `--context` selects within it. Run from anywhere. |
| `talosctl config new <out>` / `config merge <in>` | **Only for the file argument.** Use absolute paths, as above. |
| `preflight.sh` | **No.** It resolves the repo root from its own location; verified from `/tmp`. |
| the commands in `matchbox/README.md` | **Yes.** Those use `--talosconfig talosconfig` — a bare relative filename — so they must be run from inside `matchbox/assets/talos/<ver>/`. That is the cluster-build convention, not this one. |

## Step 0b — the talosctl client follows the nodes

Sidero's upgrade FAQ, verbatim: *"Which version of `talosctl` should I use to
update a cluster? We recommend using the version that matches the current
running version of the cluster."* — the version the nodes run **now**, not the
target. (The first draft of this runbook said the opposite.)

`flake.nix` pins it in a `talosctlOverlay`, apart from nixpkgs, so bumping it
moves nothing else in the shell. It is **1.11.5** as of 2026-10-29, matching the
nodes. During the ladder it trails the nodes by one rung: after every node
reports a new version, bump `version` in the overlay and replace its four
hashes — the overlay's comment has the two commands. Then:

```bash
nix build .#talosctl --no-link --print-out-paths   # fetch + verify the hash
direnv reload                                      # or re-enter `nix develop`
talosctl version --client --short                  # the new version
```

`preflight.sh` compares the client with the running version: a patch mismatch
WARNs, a minor mismatch FAILs. So a forgotten bump stops you at the next rung.

## Step 1 — patch within the line first: 1.11.5 → 1.11.6

Same minor, no Kubernetes implications. Its purpose is to prove the mechanics
on the least dangerous change available: that the image is right, that a node
comes back *with its extensions*, that etcd re-joins, that `preflight.sh` goes
green again. If anything about the procedure is wrong, learn it here.

```bash
./matchbox/talos-machineconfigs/preflight.sh 1.11.6
```

## Step 2 — the Talos ladder, one minor at a time, latest patch of each

```
1.11.5  →  1.11.6  →  1.12.12  →  1.13.12  →  (1.14.5)
```

Latest patches on 2026-10-29. Sidero: *"the recommended upgrade path is to
always upgrade to the latest patch release of all intermediate minor
releases"*, because the migrations are *"only tested between adjacent minor
releases"*. `preflight.sh` FAILs a skipped minor and a version that does not
exist, and WARNs on one that is not the latest patch — so re-check the numbers
on the day rather than trusting this table.

Kubernetes stays on 1.34 throughout. Stop at 1.13 if you want a shorter hop; it
supports Kubernetes up to 1.36, which is enough to get back into support.

Between rungs: bump the talosctl overlay to the version the nodes now run
(Step 0b), *then* start the next rung.

## Step 3 — per node, in this order

Workers first if there were any; this cluster is three control-plane nodes, so
go **non-VIP-holder first and the VIP holder last**. Find the VIP holder:

```bash
talosctl --context admin@the-cluster -n 10.0.4.101,10.0.4.102,10.0.4.103 get addresses \
  | grep 10.0.4.100
```

For each node, in turn:

```bash
# 1. Gate. It also prints the exact --image for this target and verifies
#    that the factory has it -- copy it from there rather than typing a hash.
./matchbox/talos-machineconfigs/preflight.sh <TARGET>    # must exit 0

# 2. Upgrade one node. --image is NOT optional -- see the traps below.
talosctl --context admin@the-cluster -n <NODE_IP> upgrade \
  --image factory.talos.dev/metal-installer/c9078f9419961640c712a8bf2bb9174933dfcf1da383fd8ea2b7dc21493f8bac:v<TARGET>

#    --wait defaults to true, so this blocks and streams progress.
#    If it hangs on kexec, retry that node with: -m powercycle

# 3. Confirm it rejoined. Not just Ready -- etcd too.
kubectl get nodes -w                                     # that node Ready
talosctl --context admin@the-cluster -n 10.0.4.101 etcd members # 3 members, no learner
talosctl --context admin@the-cluster -n 10.0.4.101 etcd alarm list   # empty

# 4. Confirm the extension survived. An upgrade installs exactly what the
#    image carries; a node without iscsi-tools here got the wrong image.
talosctl --context admin@the-cluster -n <NODE_IP> get extensions   # iscsi-tools + schematic c9078f94...

# 5. Gate again before touching the next one.
./matchbox/talos-machineconfigs/preflight.sh <TARGET>
```

Four things about this command, verified against `talosctl upgrade --help`, the
nodes and the Image Factory rather than remembered:

- **The image is the schematic's, not the plain installer.** These nodes run
  Image Factory schematic `c9078f94…`, which adds `siderolabs/iscsi-tools`. An
  upgrade installs what the image carries, so `ghcr.io/siderolabs/installer`
  would boot each node without the extension. Nothing uses iSCSI today (storage
  is NFS only), so it would not have broken anything — but dropping an
  extension is a separate decision, not a side effect of an OS upgrade. The
  factory's own docs call `factory.talos.dev/installer/…` the *legacy* name;
  bare metal is `metal-installer`.
- **`--image` is mandatory in practice.** Its default is the plain installer at
  the *client's* version (`ghcr.io/siderolabs/installer:v1.11.5` today). With
  the client matching the nodes, a bare `talosctl upgrade` reinstalls the same
  version *without the schematic*. Always pass the image.
- **Never use `-f/--force`.** Its own help says it skips the etcd health and
  member checks and "might lead to data loss". Those checks are the thing
  standing between a one-node reboot and a lost quorum.
- **There is no `--preserve` flag.** Older Talos had one; modern `upgrade`
  preserves `EPHEMERAL` (`/var`) by default, which is what keeps etcd data and
  this cluster's PVs. If you find `--preserve` in an old note, it is from a
  different era and will fail as an unknown flag.

Between nodes, `preflight.sh` will report mixed Talos versions as a FAIL — that
is expected *during* the ladder and is the one FAIL you knowingly walk past.
It exists to stop you starting a **new** upgrade on top of an unfinished one.

Do not proceed to the next node until etcd has three voting members. With three
members quorum is two, so exactly one may be down; starting a second reboot
early is how a maintenance window becomes an outage.

## Step 4 — Kubernetes, only once Talos is settled

```bash
talosctl --context admin@the-cluster --nodes 10.0.4.101 upgrade-k8s --to <latest 1.35 patch>
```

`upgrade-k8s` drives the whole control plane from one node; it is not per-node.
Then repeat for 1.36 if you went to Talos 1.13+. Same rule as the Talos ladder:
the latest patch, looked up on the day — the Talos steps were drafted with a
`.0` and a version that did not exist, so do not trust a number written here.

Afterwards the client has to follow. The flake's kubectl comes from the nixpkgs
input (1.34.1 on 2026-10-29), so it needs the same treatment talosctl got — an
overlay, or a deliberate input bump — and then `tools.sh`'s pin moves with it:

```bash
# .claude/skills/homelab-toolchain/scripts/tools.sh
export KUBECTL_WANT_MINOR="${KUBECTL_WANT_MINOR:-1.36}"
```

The tooling follows the cluster; it never drives it.

## Step 5 — separately, the vcluster

`vcluster-media` runs its own Kubernetes (v1.34.3 on 2026-10-29). That is a
vcluster chart version bump through the normal addon path, not a Talos
operation, and it belongs in its own PR.

## Rollback

Talos keeps the previous image and will roll back on its own if the new one
fails to boot. To force it:

```bash
talosctl --context admin@the-cluster -n <NODE_IP> rollback
```

If a node is lost rather than merely unhappy, the recovery is a Matchbox
reinstall from `matchbox/assets/talos/<ver>/` plus an etcd member replace — see
`matchbox/README.md`. That is why Step 0 of every window is a fresh etcd
snapshot, and why `preflight.sh` FAILs without one newer than 24h.

## Traps specific to this cluster

- **etcd shares a disk with the OS and containerd on all three nodes.**
  Controllers drop leases (`leaderelection lost`) for no local reason. During an
  upgrade this looks exactly like the upgrade's fault. `preflight.sh` WARNs when
  leader-election events are already elevated, so check it *before* believing
  the upgrade caused one.
- **The machine config is multi-document** once `watchdog.yaml` is on, so JSON
  6902 patches are refused. Use the `live-*.yaml` strategic-merge patches.
- **Never `patch mc` with `all.yaml` / `cp.yaml` / `work.yaml`.** Those are
  generation-time fragments and `patch mc` *appends* list items — a dry-run on
  2026-08-21 showed it duplicating `certSANs`, `extraKernelArgs` and the entire
  network-interface/VIP block.
- **`dns.yaml` is withdrawn.** Do not apply it; both of its premises were
  disproved on 2026-08-21.
- **A Talos upgrade no longer bumps CoreDNS** — see
  `docs/okf/cluster/known-issues.md`. Check it separately afterwards.
- **`/var` layout is not uniform**: `sda4` on .101/.102, `sda6` on .103. Harmless,
  but do not pattern-match a disk path across nodes in a script.
- **`machine.install.image` is stale on every node.** On 2026-10-29 it read
  `ghcr.io/siderolabs/installer:v1.11.3` — an older version *and* the plain
  installer — while the nodes ran 1.11.5 from schematic `c9078f94…`.
  `talosctl upgrade --image` ignores the field, so the ladder is unaffected, but
  anything that installs *from the machine config* — a reset, a node rebuilt
  from Matchbox — would come back on 1.11.3 without iscsi-tools. After the
  ladder, set it to the final `metal-installer` image with a `live-*.yaml`
  strategic-merge patch.
- **The schematic is `iscsi-tools` only.** Older notes say it carries AMD GPU
  firmware and microcode; the factory's definition of `c9078f94…` lists only
  `siderolabs/iscsi-tools`. The AMD patches in `matchbox/README.md` describe an
  earlier build. Read a schematic with
  `curl https://factory.talos.dev/schematics/<id>` before reasoning about it.

## Sources

- [Kubernetes 1.34 release page](https://kubernetes.io/releases/1.34/) — EOL 2026-10-27
- [Talos v1.14 support matrix](https://docs.siderolabs.com/talos/v1.14/getting-started/support-matrix)
- [Talos v1.12 support matrix](https://docs.siderolabs.com/talos/v1.12/getting-started/support-matrix)
- [Talos v1.11 support matrix](https://www.talos.dev/v1.11/introduction/support-matrix)
- [Upgrading Talos (v1.12)](https://docs.siderolabs.com/talos/v1.12/configure-your-talos-cluster/lifecycle-management/upgrading-talos) — latest-patch path; the FAQ on which `talosctl` to use
- [Image Factory API](https://github.com/siderolabs/image-factory/blob/main/docs/api.md) — `<platform>-installer` naming; `installer/` is the legacy form
- Release tags: `gh api repos/siderolabs/talos/git/matching-refs/tags/v1.11.` — how v1.11.10 was found not to exist
