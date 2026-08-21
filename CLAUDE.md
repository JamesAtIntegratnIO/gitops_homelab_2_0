# CLAUDE.md

Guidance for Claude Code (and any coding agent) working in this repository.

This file is the **operating manual**: how to work here, what will bite you, and
where the real information lives. It deliberately does not restate the
architecture — [AGENTS.md](AGENTS.md) already does that in depth.

## What this repo is

A self-service internal developer platform on bare metal: Talos Linux →
Kubernetes 1.34 → ArgoCD → Kratix promises → vclusters. One physical cluster,
`the-cluster` (API VIP `https://10.0.4.100:6443`, kube context
`admin@the-cluster`), and one tenant vcluster, `vcluster-media`.

**ArgoCD reconciles from `origin/main`.** A commit on a branch changes nothing in
the cluster until it lands on main. Plan verification accordingly.

## Read order

| When | Read |
|---|---|
| **Picking up in-flight work** | **[docs/handoff-2026-08-21.md](docs/handoff-2026-08-21.md)** — what is open, what needs a human, and the numbers to re-measure |
| First time in the repo | [README.md](README.md) → [AGENTS.md](AGENTS.md) |
| Anything about the live cluster | [docs/okf/](docs/okf/index.md) — the knowledge bundle |
| Before touching a broken thing | [docs/okf/cluster/known-issues.md](docs/okf/cluster/known-issues.md) |
| Addons / ApplicationSets | [docs/addons.md](docs/addons.md), [addons/README.md](addons/README.md) |
| Kratix promises | [docs/promises.md](docs/promises.md) |
| MCP servers | [docs/mcp.md](docs/mcp.md) |
| Runbooks | [docs/operations.md](docs/operations.md), [docs/game-day.md](docs/game-day.md) |

`docs/` is the hand-written deep-dive documentation. `docs/okf/` is the
[OKF](https://github.com/GoogleCloudPlatform/knowledge-catalog) knowledge bundle
— concept-per-file, with provenance and freshness dates, covering both the repo
and the *live* cluster. When the two disagree, the OKF bundle is usually newer.

## Non-negotiables

1. **No `kind: Secret` anywhere near `promises/`.** The Kratix state repo
   (`kratix-platform-state`) is **public**. Use `ExternalSecret` against the
   `onepassword-store` `ClusterSecretStore`. Enforced by
   [.githooks/pre-commit](.githooks/pre-commit) and the
   `validate-promises` GitHub Action.
2. **Git is the source of truth.** Fix things with commits that ArgoCD
   reconciles. Direct `kubectl` mutation is reserved for orphans — resources
   nothing in git manages any more — and you say so explicitly when you do it.
3. **Never push to `main`.** Push a branch and hand the merge to James. The repo
   is otherwise trunk-based and single-committer.
4. **Never read or print `secrets.env`.** It is gitignored and contains live
   credentials.
5. **Gateway API, not Ingress.** HTTPRoute everywhere.
6. Small, focused, conventional commits scoped like the existing history
   (`fix(mcp-system): ...`, `docs(okf): ...`).

## Environment

Tooling comes from the Nix flake ([flake.nix](flake.nix)) — `kubectl`, `helm`,
`argocd`, `tofu`, `talosctl`, `k9s`, `kubecm`, `yq`, and the `hctl` CLI. `direnv`
loads it via [.envrc](.envrc).

```bash
nix develop
```

**Gotchas when running as an agent:**

- These tools are **not on the default PATH**. In an agent worktree under
  `.claude/worktrees/*` there is no `secrets.env`, so `nix develop` either warns
  or stalls building `hctl`. The fast escape hatch:
  ```bash
  K=$(ls -d /nix/store/*-kubectl-*/bin/kubectl | sort -V | tail -1)
  ```
  The same glob works for `jq`, `yq`, `talosctl`, `kubecm`, `kubernetes-helm`.
- The Bash tool runs **zsh** on macOS: unquoted `$var` does not word-split (use
  `${=var}`), and there is no `timeout` binary.
- There is **no talosconfig on this workstation**. Anything needing
  `talosctl apply-config` / `patch mc` has to be run by James. The pending
  patches live in [matchbox/talos-machineconfigs/](matchbox/talos-machineconfigs/).
- Merging a kubeconfig? Use `kubecm add -cf <file> --context-name admin@<cluster>`
  from the flake. The `-c/--cover` flag is required in non-TTY sessions.

## Repository map (short)

```
addons/       ArgoCD addon definitions -> ApplicationSets
  charts/application-sets/   the factory chart
  environments/ cluster-roles/ clusters/   value layers, in precedence order
platform/     Kratix ResourceRequests (vclusters, http-services)
promises/     Kratix Promise definitions + Go pipelines (_shared/kratixutil)
workloads/    apps deployed *inside* vclusters
terraform/    the one imperative bootstrap step (ArgoCD + Cloudflare DNS)
matchbox/     PXE/Talos machine configs
cli/          hctl (Go, Cobra + Bubbletea)
images/       container image sources (kubectl, git-indexer, status reconciler)
docs/         deep-dive docs + docs/okf/ knowledge bundle
```

Addon value files resolve `environments/{env}` → `cluster-roles/{role}` →
`clusters/{name}`, and **by chart name, not addon key**.

## Traps that have already caused incidents

- **Addon targeting leaks into vclusters.** Two independent mechanisms:
  (1) value folders resolve by `chartName`, so `cert-manager-vcluster` reads the
  *host* `cert-manager/values.yaml` unless you set `valuesFolderName`;
  (2) several production-layer addons (`kyverno`, `external-secrets`) have no
  `cluster_role` exclusion and therefore target every production cluster,
  vclusters included. After any addon change, render both bootstraps and diff
  which clusters each app targets — do not reason from the addon definition alone.
- **The Kyverno `generate-default-deny-netpol` ClusterPolicy owns every
  namespace's `default-deny-all` NetworkPolicy** with `synchronize: true`.
  Deleting or replacing it without `generateExisting: true` in the desired state
  cascade-deletes every default-deny with nothing to regenerate them.
- **etcd is I/O-starved.** All three nodes put `/var/lib/etcd` on the same disk
  as the OS and containerd. Leader-elected controllers drop their leases
  (`leaderelection lost`) for no local reason. Suspect this before you suspect
  the controller.
- **Pin images.** An unpinned `:latest` in `mcp-system` silently jumped
  grafana-mcp 0.14.0 → 1.1.0 on a rollout. Everything is digest-pinned now; keep
  it that way. And never point a *liveness* probe at an external dependency.

## Working with the OKF bundle

The bundle at [docs/okf/](docs/okf/index.md) is the platform's long-term memory —
what is actually running, what is broken, and why decisions were made. Skills
`/okf:okf`, `/okf:validate`, `/okf:visualize` operate on it.

After a change that alters reality (a fix that lands, a version bump, a newly
discovered issue):

1. Update the affected concept file(s) — keep `sources`, `generated`, and
   `stale_after` frontmatter honest.
2. Add a line to [docs/okf/log.md](docs/okf/log.md) under today's date.
3. Validate:
   ```bash
   /okf:validate docs/okf --strict
   ```

Do not let the bundle drift; a stale bundle is worse than no bundle because it
gets trusted.

## MCP servers

Two unrelated things share the name. See [docs/mcp.md](docs/mcp.md) for the full
picture.

- **In-cluster (`mcp-system` namespace)** — MCP tool servers behind
  `mcp.cluster.integratn.tech`, consumed by Open WebUI (via the `mcpo`
  MCP→OpenAPI bridge) and any external MCP client. Managed as the `mcp-system`
  addon in `addons/cluster-roles/control-plane/addons/mcp-system/`.
- **Editor-side** — [.vscode/mcp.json](.vscode/mcp.json) configures MCP servers
  for VS Code/Copilot. It contains hard-coded `/home/boboysdadda/...` paths from
  the original Linux workstation and does **not** work on macOS as written.

Claude Code in this repo does not need either: use `kubectl` and `argocd` from
the flake directly.

## Verifying a change

```bash
# render an addon's ApplicationSet the way ArgoCD will
helm template addons/charts/application-sets -f <values...>

# promise pipelines (Go) -- no tests here, so it is a build gate
cd promises/vcluster-orchestrator-v2/workflows/resource/configure && go build ./...

# CLI (has real tests)
cd cli && go build -o /dev/null . && go test ./...

# status reconciler
cd images/platform-status-reconciler && go test ./...

# what ArgoCD thinks
kubectl -n argocd get applications
argocd app get <name>
```

A change is not verified because it renders. It is verified when it is on
`main`, ArgoCD reports the app `Synced`/`Healthy`, and the thing it was supposed
to fix stopped happening.
