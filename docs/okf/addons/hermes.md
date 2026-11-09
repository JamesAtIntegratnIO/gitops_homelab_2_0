---
type: Workload
title: Hermes Agent
description: Nous Research's Hermes Agent running as a service on the-cluster's Raspberry Pi 5 worker, on the Anthropic API, with its state on local NVMe.
tags: [ai, agent, hermes, raspberry-pi, arm64]
status: draft
generated: { by: claude-code, at: 2026-11-09T01:50:00Z }
stale_after: 2027-02-07
sources:
  - id: manifests
    resource: ../../../addons/cluster-roles/control-plane/addons/hermes/hermes-agent.yaml
    title: Hermes addon manifests
  - id: policies
    resource: ../../../addons/cluster-roles/control-plane/addons/network-policies/hermes.yaml
    title: Hermes namespace and network policies
  - id: mcp-policies
    resource: ../../../addons/cluster-roles/control-plane/addons/network-policies/mcp-system.yaml
    title: mcp-system network policies
  - id: mcp-doc
    resource: ../../mcp.md
    title: In-cluster MCP servers
  - id: skills
    resource: ../../../addons/cluster-roles/control-plane/addons/hermes/skills.yaml
    title: Hermes cluster skills
  - id: backup
    resource: ../../../addons/cluster-roles/control-plane/addons/hermes/hermes-backup.yaml
    title: Hermes nightly backup CronJob
  - id: cq-backup
    resource: ../../../addons/cluster-roles/control-plane/addons/cq/cq-backup.yaml
    title: cq-server nightly backup CronJob
  - id: design-skill
    resource: ../../../hack/hermes-design/SKILL.md
    title: integratn-design skill (source; generated into design-skill.yaml)
  - id: github-mcp
    resource: ../../../addons/cluster-roles/control-plane/addons/mcp-system/github-mcp-hermes.yaml
    title: github-mcp-hermes
  - id: arr-mcp
    resource: ../../../addons/cluster-roles/control-plane/addons/mcp-system/arr-mcp.yaml
    title: arr-mcp config and ExternalSecret
  - id: arr-mcp-upstream
    resource: https://github.com/bardesss/arr-mcp/tree/v1.73.0
    title: bardesss/arr-mcp v1.73.0
  - id: optimization-guide
    resource: https://github.com/OnlyTerp/hermes-optimization-guide/tree/f865f8a
    title: OnlyTerp hermes-optimization-guide (pinned to Hermes v0.21.4)
  - id: upstream-docker
    resource: https://github.com/NousResearch/hermes-agent/blob/v0.23.0/website/docs/user-guide/docker.md
    title: Hermes Agent Docker guide (v0.23.0)
---

# What runs

One pod, `hermes-agent` in namespace `hermes`, pinned to `talos-pi5-1` (it
tolerates the node's arm64 taint). The image is
`ghcr.io/jamesatintegratnio/gitops_homelab_2_0/hermes-agent:v0.23.0-stt-cq0.19.2`,
digest-pinned: the official `nousresearch/hermes-agent:v0.23.0` with the
`stt-whisper` extra (faster-whisper 1.2.1) installed beside its venv, in `/opt/hermes-stt` on `PYTHONPATH`, pinned from Hermes's
own `uv.lock`, so speech-to-text runs locally on the Pi's CPU, plus the
`cq` CLI at `/usr/local/bin/cq` (`images/hermes-agent/`). It runs `gateway run`; with
no chat platform configured, the service it offers is the OpenAI-compatible API
server on port 8642 (ClusterIP `hermes-agent`, no ingress from other namespaces
yet). Open WebUI (namespace `ai`) may reach it, with `API_SERVER_KEY`.[^manifests]

# Browser UIs

Both are internal only, behind nginx-gateway (the gateway VIP is on the LAN;
reach it from home or over Tailscale):

| UI | URL | Port |
|---|---|---|
| Webapp (chat, Hermes Desktop in a browser) | `https://hermes.cluster.integratn.tech` | 9120 |
| Dashboard (sessions, config, logs, skills) | `https://hermes-admin.cluster.integratn.tech` | 9119 |

Hermes signs users in itself with its self-hosted OIDC plugin against authentik:
a public PKCE client `hermes` (blueprint `10-hermes-provider.yaml`, no client
secret, which is all the plugin supports), bound to the `authentik Admins`
group because every account that can sign in acts as the machine owner. There is
no authentik forward-auth in front. `dashboard.trusted_proxies` is the pod CIDR
so the gateway's `X-Forwarded-Proto` is honoured.[^manifests]

Open WebUI keeps its OpenAI connections in its own database, so adding Hermes
there is a one-time step in its admin settings (URL
`http://hermes-agent.hermes.svc.cluster.local:8642/v1`, key `API_SERVER_KEY`);
only the network path is in git.

The model is `claude-sonnet-5-5` through the native `anthropic` provider, billed
per token against the key in 1Password item `hermes-agent` (`homelab` vault,
fields `ANTHROPIC_API_KEY` and `API_SERVER_KEY`). The terminal backend is
`local`: shell commands run inside the pod.[^manifests]

# Constraints from the image

- s6-overlay `/init` starts as root, chowns `/opt/data`, then drops to uid
  10000. The pod must not set `runAsNonRoot` or a read-only root filesystem,
  and must not override the entrypoint.[^upstream-docker]
- `/opt/data` is all of Hermes's state. `state.db` is SQLite in WAL mode, which
  needs a local filesystem and one gateway per data directory: a 20 Gi
  `local-nvme` claim, one replica, `Recreate`.[^upstream-docker]
- `config.yaml` is copied from the `hermes-config` ConfigMap at every start, so
  settings changed inside the pod do not survive a restart.[^manifests]

# Network

Egress is DNS plus TCP 443/80 to anywhere outside RFC1918: Hermes reaches the
Anthropic API and the public web, but no LAN host and no in-cluster service,
with two exceptions: authentik through the gateway pods on 443 for OIDC
discovery and JWKS, and the three read-only MCP servers in `mcp-system`
(kubernetes-mcp 8080, argocd-mcp 3000, grafana-mcp 8000), which admit Hermes
through `allow-hermes-ingress`. Ingress: nginx-gateway to 9119/9120, and Open
WebUI to 8642.[^policies][^mcp-policies]

# Cluster access

Hermes inspects the cluster it runs on through the platform's own MCP servers,
called by Service DNS (`http://<server>.mcp-system.svc.cluster.local:<port>/mcp`),
not through the gateway. It holds no kube or ArgoCD credentials and `kubectl`
stays on its approval deny list: what it can see is what those servers' RBAC
allows.[^manifests][^mcp-doc]

| Server | Scope | Tools enabled |
|---|---|---|
| `kubernetes` | `read_only=true`, ClusterRole `kubernetes-mcp-viewer` (no secrets) | 12 list/get/log/top tools |
| `grafana` | `--disable-write` | 18: Prometheus and Loki queries and labels, alert rules and silences, datasources, dashboard search and summaries |
| `argocd` | `MCP_READ_ONLY=true`, account `mcp-readonly` | 6: applications, resource trees, events, workload logs |

kubernetes, grafana and github-mcp-hermes are `trust: untrusted`, so Hermes asks
before any tool that does not declare `readOnlyHint`. All kubernetes and grafana
tools declare it. No argocd tool does, so argocd is `trust: full` (2026-11-07):
the server cannot write, and a prompt on every call blocked scheduled health
checks, which have nobody to approve it. Each server has a `tools.include` list, so a tool added in a server
upgrade stays off until it is named in git. The shared `github-mcp` is not
used; GitHub goes through Hermes's own server (see GitOps fixes). The `*.cluster.integratn.tech` website
blocklist and `allow_private_urls` apply only to the web and browser tools, not to
MCP connections.

Config reaches the pod only at start (the init container copies the ConfigMap),
so a merged config change takes effect after the pod restarts.

# Backup

`hermes-backup`, a CronJob in `hermes` at 04:30 UTC, runs `hermes backup`
against the `hermes-data` claim on talos-pi5-1 and writes
`hermes-backup-<time>.zip` to datacron (10.0.3.5, VM 103) over NFSv4 at
`/srv/backups/hermes`, keeping the newest 14 (`-k 14`). It then tests the zip,
and renames a bad one to `.bad`. `hermes backup` snapshots each SQLite db with
`sqlite3.backup()`, so the gateway keeps running, and it skips caches and
installs: about 32 MB on 2026-11-08. The zip includes `.env` and
`auth.json`. The export is declared in nix_dots_2
`hosts/datacron/hermes-backup.nix`: it admits only 10.0.4.104, squashes every
uid to `hermes-backup` (10000), and the directory is 0700. Never Tower. The copy
is off the Pi, but it sits on the same Proxmox host as datacron's Postgres
dumps.[^backup]

Watched by `HermesBackupStale` (no success in 36h) and
`HermesBackupNeverSucceeded` in `platform.hermes`.

Restore: scale `hermes-agent` to 0 (ArgoCD self-heal will fight this; pause
auto-sync on `hermes-the-cluster` first), run a pod on talos-pi5-1 with the
claim and the export mounted as uid 10000, `hermes import <zip>`, then
re-enable sync. First run verified 2026-11-08 18:44Z (36 MB zip).

cq-server's knowledge lives in its own SQLite file (`/data/cq.db` on claim
`cq-server-data`), which `hermes backup` never sees. `cq-backup`, a CronJob in
`cq` at 04:40 UTC, copies it with the sqlite3 backup API, runs
`PRAGMA integrity_check`, and writes `cq-<time>.db` to the same export's `cq/`
subdirectory, keeping 14. The same two alerts watch both jobs. Restore: scale
`cq-server` to 0, copy a `cq-*.db` over `/data/cq.db`, scale back.[^cq-backup]

# Speech-to-text

Local faster-whisper (`stt.provider: local`, model `base`), from `/opt/hermes-stt` in our derived image; the model (~142 MB) is cached in `/opt/data/cache/huggingface`. On the Pi 5 with all 4 cores (`limits.cpu: 4`, `OMP_NUM_THREADS=4`) a 6 s clip takes about 5 s warm and 8 s on first use. Hermes fixes `beam_size` at 5. `tiny` is about twice as fast with the same result on clear speech; switch `stt.local.model` if latency matters more than accuracy.

# Cluster skills

Skills from git, in ConfigMap `hermes-skills`, mounted read-only under
`/opt/hermes-skills/homelab/` and listed in `skills.external_dirs`:
`cluster-health-check` (a sweep of ArgoCD, pods, nodes, etcd, storage and
firing alerts) and `cluster-triage` (this platform's failure shapes: one-sided
NetworkPolicies, etcd disk stalls, host-DNS restart storms, stale NFS after a
Tower reset, the stale nginx-gateway data plane). Both use only the read-only
MCP tools and treat every fix as a PR for James. Three more read-only skills
since 2026-11-08: `loki-log-triage` (scoping, patterns and the log lines that
have meant something here), `kargo-health` (the `kargo_*` kube-state-metrics
series, then Stage/Promotion/Freight objects, then orphaned
`kargo/promotion/*` PRs; remedies are reported for James) and
`talos-node-health` (conditions, versions, cross-node disk-latency
comparison, scrape gaps; no talosctl, so Talos-layer checks are named for
James). For kargo-health the `kubernetes-mcp-viewer` ClusterRole reads
`kargo.akuity.io` projects, projectconfigs, warehouses, stages, freights and
promotions. They are adapted from the
repo's `.claude/skills/cluster-triage` and [known issues](../cluster/known-issues.md).[^skills]

**Design system** (`integratn-design`, 2026-11-08). James rejected Hermes's
first architecture page as generic AI styling, so anything visual Hermes makes
now follows the IntegratnIO design system ("The Night Shift Ledger",
`DESIGN.md` in IntegratnIO/specmarshal, the same tokens as Glance and the
ArgoCD/Kargo/authentik themes). The skill carries `templates/base.css` (tokens,
components, SVG diagram classes, light ramp) and `templates/page.html` (the
cluster architecture page redone in the system) as linked files. Its source is
`hack/hermes-design/`; `gen.sh` writes ConfigMap `hermes-skill-design`
(`design-skill.yaml`), mounted with `subPath` under
`/opt/hermes-skills/homelab/integratn-design/`. Hermes's own UIs use the same system (2026-11-08): the
same ConfigMap carries a dashboard theme (`dashboard-themes/integratn.yaml`,
`dashboard.theme: integratn`: ledger palette pinned through `colorOverrides`,
system fonts in place of the Nous display faces, 6px radius, compact, no
grain, glow, shadows or blur) and a skin (`skins/integratn.yaml`,
`display.skin: integratn`) for the CLI/TUI, the dashboard's Chat tab and the
Webapp, whose `customCSS` pins the Webapp's surfaces. Hermes's own `/login` page (server-rendered, unthemed, one
button hard-coded as "Self-Hosted OIDC") is skipped: SnippetsFilter
`hermes-login-via-authentik` on both HTTPRoutes answers `/login` with a 302 to
`/auth/login?provider=self-hosted&<next>`, so sign-in is the authentik page
only. A theme or skin picked
in a UI lasts until the next restart, when config.yaml is copied back. Edit the source files and
rerun `gen.sh` (`--check` fails when the YAML is stale), never the YAML.[^design-skill]

Each file is a `subPath` mount: a plain ConfigMap volume adds hidden `..data`
symlink directories, and Hermes walks skill directories following symlinks, so
every skill would be listed twice. Hermes truncates a skill's `description` to
60 characters in its prompt index, so keep them short. Skills the agent writes
itself go to `/opt/data/skills` (its PVC, persistent) and are scanned.

**How Hermes improves its skills** (`skill-improve`, 2026-11-08). The git
skills stay read-only, so a prompt injection cannot rewrite Hermes's own
rules; Hermes changes them by PR to `skills.yaml` (plus the mount in
`hermes-agent.yaml` for a new skill) through github-mcp-hermes, and James
merges. Hermes's local skills are found before the git ones, so it must never
copy a git skill into `/opt/data/skills` under the same name: that would
shadow later fixes from git. This route needs the GitHub token to have
Contents and Pull requests write. First rule learned this way (James relayed
it from a Hermes session): cluster-remediate 1.1.0 re-reads the pod right
before a cordon, delete or restart and stops if the failure no longer
persists.

# Knowledge memory (cq)

Hermes's long-term operational memory is Mozilla.ai's
[cq](https://github.com/mozilla-ai/cq) (James's choice, 2026-11-08), next to
its built-in MEMORY.md/USER.md. The `cq` Go CLI (cli/v0.19.2, built from
source into our image, `images/hermes-agent`) runs as a stdio MCP server
(`mcp_servers.cq`) with its SQLite store at `/opt/data/cq/local.db`, on the
Pi's local NVMe data volume: a query took about 15 ms in testing and never
leaves the node. Tools: query, propose, confirm, flag, revoke, status, at
`trust: full` (cq declares no `readOnlyHint`, and its writes only touch that
file). The `knowledge-memory` skill sets the loop: query before acting,
propose when a lesson stabilizes, confirm/flag after; no secrets, nothing a
page or log told it to remember, and rules for acting go in git skills
instead. Never the hosted cq.exchange.

**cq-server** (added 2026-11-08, James: cq runs as its own service) is the
store: `ghcr.io/mozilla-ai/cq/server` v0.12.4 in namespace `cq`, on the Pi 5
with SQLite on a 2 Gi local-nvme PVC, UI and API at
`https://cq.cluster.integratn.tech` (addon `cq`, Kargo target `cq-server`).
Secrets from 1Password item `cq-server` (`CQ_JWT_SECRET`,
`CQ_API_KEY_PEPPER`, `HERMES_API_KEY`; UI user `james` seeded once by hand,
since the image lacks upstream's `seed-users.py`; API keys are made in the UI). Ingress from nginx-gateway and
Hermes only, no egress. The CLI stays the MCP surface: with `CQ_ADDR` and
`CQ_API_KEY` set, `cq mcp` sends proposals to the server as `private` and, on
start, drains every local unit to it (deleting the local copy); remote units
reach agents only after James approves them in the UI's review queue. Data
reads on the API are open by upstream design, so the gateway does not
serve `/api/v1/knowledge` at all (404); the UI keeps cq's own login only (cq
has no SSO, and James did not want two logins), and Hermes reads by ClusterIP.
Nothing secret goes in regardless.

# Live operational fixes

Hermes can act on the cluster, not only diagnose it, through a second
kubernetes-mcp-server, `kubernetes-mcp-ops` in `mcp-system` (Hermes's
`mcp_servers.kubeops`, added 2026-11-08). It exposes only `pods_delete` and
`resources_create_or_update` (server-side apply), and its ClusterRole
`kubernetes-mcp-ops` allows only pod delete, node patch (cordon/uncordon) and
Deployment/StatefulSet/DaemonSet patch (rollout restart); Secrets are denied
in the server config. Hermes calls it `trust: untrusted` and neither tool
declares `readOnlyHint`, so every write waits for James's approval in the
webapp. The `cluster-remediate` skill holds the rules (operational actions
only, config goes through gitops-fix, one node cordoned at a time, always
uncordon, never Tower) and the stale-NFS playbook (cordon -> delete ->
verify on another node -> uncordon). Node patch also permits labels and
taints and workload patch permits any template change: the approval prompt
and ArgoCD self-heal are the guards there.

ArgoCD syncs go through a third server, `argocd-mcp-ops` (Hermes's
`mcp_servers.argocdops`, 2026-11-08): mcp-for-argocd with writes on, but its
ArgoCD account `hermes-ops` may only `get` and `sync` (RBAC in
`addons/clusters/the-cluster/addons/argo-cd/values.yaml`), so
create/update/delete and resource actions are refused by ArgoCD. Hermes
includes only `sync_application`, `trust: untrusted`, and cluster-remediate
says when a sync is appropriate. Token: 1Password `argocd-hermes-ops`/`token`
(`argocd account generate-token --account hermes-ops`).

# Talos (read-only)

Hermes runs `talosctl` (built into the image, v1.13.12) with a talosconfig
that Talos mints for the `talos.dev` ServiceAccount `hermes-talos`
(`talos-serviceaccount.yaml`): role `os:reader` only, so Talos refuses every
write. The Secret is mounted `optional` at `/var/run/secrets/talos.dev` with
`TALOSCONFIG` set. The endpoint is `talos.default:50000` (control-plane apid);
CiliumNetworkPolicy `allow-talos-api-egress` allows it by entity. It needs
`live-talos-api-access-hermes.yaml` applied to the three control-plane nodes
by James (`matchbox/talos-machineconfigs/commands.md` step 2b); until then
the Secret doesn't exist and talosctl has no config.

# GitOps fixes

Hermes proposes manifest fixes as branch + PR on
`JamesAtIntegratnIO/gitops_homelab_2_0`, through its own GitHub MCP server,
`github-mcp-hermes` in `mcp-system` (port 8083, no HTTPRoute, ingress from
Hermes only). James merges. The `gitops-fix` skill carries the repo's rules
(never `main`, no secrets, pinned images, Kargo pin layout, both NetworkPolicy
halves).[^github-mcp][^skills]

- **The token never enters the Hermes pod.** The shared `github-mcp` runs in
  `http` mode, which takes the token from each client request; Hermes would
  have had to hold it, and its terminal (same uid) could read it.
  `github-mcp-hermes` runs the server in `stdio` mode with the token in its own
  environment, served over streamable HTTP by supergateway. The binary comes
  from the pinned `github-mcp-server` image as an image volume (it is
  distroless).
- **Tools are cut on the server**: toolsets `repos,pull_requests,issues,actions`
  with `--exclude-tools` for delete, repo create/fork, reviews, every
  issue/comment write and workflow trigger. Since 2026-11-08 Hermes has two
  entries on it: `github` (`trust: full`: reads, issue/CI reads,
  create_branch, create_pull_request, update_pull_request,
  update_pull_request_branch and, since James asked for Hermes to fix and
  merge Kargo bump PRs without approval, merge_pull_request) needs no
  approval; `githubwrite` (`trust: untrusted`: create_or_update_file,
  push_files) asks James on every call. The `gitops-fix` skill's "Merging" section sets the
  rules: only when asked, gate green on the reviewed head sha, squash, one
  at a time.
- **Token**: 1Password item `hermes-agent`, field `GITHUB_TOKEN` (added
  2026-11-08): a fine-grained PAT on James's own account, which is an admin.
  The Hermes pod's ExternalSecret maps only the Anthropic and API-server keys
  from that item, so the token stays in `github-mcp-hermes`. The `main`
  ruleset (required `addons-gate`, no force-push or deletion) has no
  pull-request rule and lets the Admin role bypass it, so
  `create_or_update_file`/`push_files` could name `main` as the branch. What
  stops that today is per-call approval (`trust: untrusted`) and the
  `gitops-fix` skill. A "require a pull request" rule without the Admin
  bypass, or a non-admin bot account, would make it GitHub's guard.
- Hermes strips `GITHUB_TOKEN`/`GH_TOKEN` from every terminal command by
  design, so git with a token in the Hermes pod was never the route.

# Inside the media vcluster

The host's kubernetes-mcp sees vcluster-media only as synced pods
(`<pod>-x-media-x-vcluster-media`). Hermes's `kubemedia` server is a second
read-only kubernetes-mcp-server running **inside** the vcluster (added
2026-11-08), deployed by the vcluster's own ArgoCD from
`workloads/vcluster-media/addons/kubernetes-mcp`, so its in-cluster
ServiceAccount token is a vcluster token. It is bound to the built-in `view`
ClusterRole plus get/list/watch on ArgoCD's `applications`, `applicationsets`
and `appprojects`; Secrets are also denied in the server config. The only
kubeconfig the host holds for the vcluster (`vc-vcluster-media`) is
cluster-admin, which is why nothing crosses the boundary instead. Hermes calls
the synced Service `kubernetes-mcp-x-media-x-vcluster-media.vcluster-media:8080`
(egress `allow-vcluster-media-mcp-egress`, ingress
`allow-hermes-kubernetes-mcp-ingress`). Its image is on the `kubernetes-mcp`
Kargo target (key `extraObjects.0...image`, so the Deployment stays the first
extraObject).

# Inside the media apps

Hermes works inside Sonarr, Radarr, Bazarr, SABnzbd and Jellyfin, not only on
their manifests, through `arr-mcp` (github.com/bardesss/arr-mcp v1.73.0) in
`mcp-system` (Hermes's `mcp_servers.arr`, port 6060, no HTTPRoute, ingress from
Hermes only; added 2026-11-08). Chosen over the other arr MCP servers on
GitHub for activity, coverage of our whole stack (Bazarr and SABnzbd
included), a multi-arch image and `readOnlyHint` on every read tool.[^arr-mcp][^arr-mcp-upstream]

- **Writes are gated three times.** arr-mcp grants `safe_write` (search, grab,
  monitor, rescan, retry, pause, subtitle search, profile change) on the four
  arr apps and `destructive` nowhere; Jellyfin is read-only. Hermes's token is
  tier `write`, which caps it below `destructive` whatever the file says.
  Every write is a preview, then a confirm with the preview's token, audited to
  `audit.db`; under `trust: untrusted` that is two approvals in Hermes per
  write. `tools.include` lists the 18 reads and 10 safe writes Hermes uses.
- **Config is git's.** arr-mcp reads only `/config/config.yaml`, with no env
  substitution, so ExternalSecret `arr-mcp-config` renders the whole file from
  1Password `homepage-secret` (the apps' keys) and `arr-mcp` (`MCP_TOKEN`,
  written only as its sha256; `UI_PASSWORD_HASH`, so the config UI starts
  claimed instead of handing itself to the first visitor). An init container
  copies it onto the PVC at every start; changes made in the UI last until the
  next restart.
- **Runs on the Pi** beside Hermes, on `local-nvme` (1 Gi, its SQLite audit,
  log and session files). Not Tower NFS: Bazarr's SQLite corrupted there.
- **The token is in the Hermes pod** (`ARR_MCP_TOKEN`, Secret
  `hermes-arr-mcp`), because Hermes sends it as a header. Its terminal could
  read it and call arr-mcp directly, skipping Hermes's approval prompts; the
  server's own limits (no destructive tier, preview + confirm) still hold.
- **House rules** are the `media-house-rules` skill: TV max 1080p (replace,
  never delete first), no bulk writes (Tower), OpenSubtitles' 1,000/day
  shared with Jellyfin, the anime-specials numbering gap.
- To look at the config UI: `kubectl --context admin@the-cluster -n mcp-system
  port-forward svc/arr-mcp 6060`, sign in as `admin` with `arr-mcp` field `password`.

[^manifests]: Hermes addon manifests
[^policies]: Hermes namespace and network policies
[^upstream-docker]: Hermes Agent Docker guide (v0.23.0)
[^mcp-policies]: mcp-system network policies
[^mcp-doc]: In-cluster MCP servers
[^skills]: Hermes cluster skills
[^github-mcp]: github-mcp-hermes
[^backup]: Hermes nightly backup CronJob
[^cq-backup]: cq-server nightly backup CronJob
[^arr-mcp]: arr-mcp config and ExternalSecret
[^arr-mcp-upstream]: bardesss/arr-mcp v1.73.0

# Tuning

Applied 2026-11-07 from the third-party optimization guide, which targets
v0.21.4, with every key checked against v0.23.0's built-in defaults first.[^optimization-guide]

- **Cost**: the side tasks (compression, titles, background review, approval
  classifier, curator, goal judge) run on `claude-haiku-5-5` instead of the main
  Sonnet model. `tts` is disabled, at most 4 concurrent subagents, and
  exact-repeat tool loops hard-stop. Since 2026-11-08 a run stops at 60 model
  turns (`agent.max_turns`), with a one-time warning to the model at 80%
  (`agent.budget_warning_ratio: 0.8`), and `prompt_caching.cache_ttl` is
  `auto` (1-hour cache for chats, 5-minute for cron, API and subagents).
- **One-shot runs skip every approval.** `hermes -z` sets
  `HERMES_YOLO_MODE=1` (v0.23.0 `hermes_cli/oneshot.py`), and yolo also skips
  the `trust: untrusted` MCP gate, so a one-shot run can call kubeops and
  GitHub writes unprompted. The terminal runs in the same pod as the same uid,
  so `approvals.deny` blocks `hermes -z`, `--oneshot`, `--yolo`,
  `HERMES_YOLO_MODE`, `hermes config set` and `approvals.mode`. Deny rules
  are fnmatch over shell commands and a Python subprocess gets past them, so
  the real limit is still kubeops's RBAC. Cron and `hermes chat -q` keep the
  default `deny` for unattended approvals.
- **Security**: smart approvals with a deny list (`sudo`, piping `curl`/`wget`
  to a shell, `dd` to devices, force-push, `authorized_keys`, `kubectl`), no
  private URLs, secret redaction, no runtime pip installs, internal hosts on the
  website blocklist, agent-written skills scanned, no borrowed external logins,
  no vault CLIs. The guide's `terminal.backend: docker` is impossible in a pod;
  the pod and its network policy are the sandbox.
- **Ops**: sessions are pruned after 90 days, `/dev/shm` is a 1 Gi memory
  volume for Chromium, and a liveness probe on `/health` restarts a wedged
  gateway.
- **Monitoring**: Hermes has no Prometheus endpoint; its gateway-health export
  is OTLP push only and is not wired up. Alert `HermesGatewayDown` (platform
  alerts, group `platform.hermes`) uses kube-state-metrics instead. There is no
  cron-health signal without OTLP.
- **Not used**: `security.tirith_fail_open` does not exist in v0.23.0.
  Delegation and cron stay on the main model.

[^optimization-guide]: OnlyTerp hermes-optimization-guide (pinned to Hermes v0.21.4)
[^design-skill]: integratn-design skill (source; generated into design-skill.yaml)
