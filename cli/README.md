# hctl — Homelab Platform CLI

`hctl` is the CLI for the [integratn.tech](https://integratn.tech) homelab platform. It provides self-service operations for vClusters, workload deployment (via [Score](https://score.dev)), addon management, platform diagnostics, and day-to-day operational tasks.

## Installation

### Nix (recommended)

`hctl` is built and provided by the project's Nix flake. If you're using the dev shell, it's already available:

```bash
nix develop   # hctl is on PATH, bash completions loaded automatically
```

### From source

```bash
cd cli
make build        # → bin/hctl
make install      # → $GOPATH/bin/hctl
```

### Build flags

Version and commit are injected at build time:

```bash
go build -ldflags "-X github.com/jamesatintegratnio/hctl/cmd.Version=1.0.0 \
  -X github.com/jamesatintegratnio/hctl/cmd.Commit=$(git rev-parse --short HEAD)" -o hctl .
```

## Quick Start

```bash
# 1. Initialize — detects repo, checks cluster, writes config
hctl init

# 2. Check your environment
hctl doctor

# 3. View platform status
hctl status

# 4. Deploy a workload
cd workloads/my-app
hctl deploy init --template api   # scaffold a score.yaml
hctl deploy run                   # translate + commit + push
hctl deploy status                # check ArgoCD sync
```

## Commands

### Platform Operations

| Command | Description |
|---------|-------------|
| `hctl init` | Detect git repo, validate cluster access, write config |
| `hctl status` | Platform health dashboard (nodes, ArgoCD, Kratix, vClusters, workloads, addons) |
| `hctl status --watch` | Continuously refresh status with `--interval` control |
| `hctl doctor` | Validate prerequisites: config, kubectl, git, cluster, ArgoCD, Kratix CRDs |
| `hctl context` | Show current platform context |
| `hctl alerts` | Display active platform alerts |
| `hctl version` | Print version and commit |

### Workload Deployment (`deploy`)

Score-based workload deployment to vClusters. Translates `score.yaml` → Stakater Application Helm chart values, manages git workflow, and monitors ArgoCD sync.

| Command | Description |
|---------|-------------|
| `hctl deploy init` | Scaffold a new `score.yaml` (templates: `--template web\|api\|worker\|cron`) |
| `hctl deploy run` | Translate score.yaml, write to repo, commit & push |
| `hctl deploy run --watch` | Deploy and poll ArgoCD until synced/healthy (with `--timeout`) |
| `hctl deploy render` | Preview generated manifests without writing (supports `--output json\|yaml`) |
| `hctl deploy diff` | Show diff between rendered output and on-disk files |
| `hctl deploy status` | Check deployment sync status in ArgoCD |
| `hctl deploy list` | List all deployed workloads |
| `hctl deploy remove` | Remove a workload from the repo |

### Troubleshooting

| Command | Description |
|---------|-------------|
| `hctl diagnose <resource>` | Walk the resource lifecycle chain (CR → Pipeline → Work → ArgoCD) |
| `hctl diagnose <resource> --bundle out.json` | Export full diagnostic data as JSON |
| `hctl trace <resource>` | Trace a resource through 5 lifecycle stages with tree-style output |
| `hctl reconcile <resource>` | Force Kratix pipeline re-execution via reconcile-at annotation |

### Convenience Commands

Quick operations that infer context from `score.yaml` in the current directory or accept a workload name as argument.

| Command | Description |
|---------|-------------|
| `hctl up [workload]` | Scale workload to desired replicas (default 1), re-enable ArgoCD sync |
| `hctl down [workload]` | Scale to 0, disable ArgoCD auto-sync |
| `hctl logs [workload]` | Stream pod logs (`-f` follow, `-t` tail lines, `-c` container) |
| `hctl open [workload]` | Open workload URL in browser (from score.yaml route or ArgoCD annotation) |

### vCluster Management (`vcluster`)

| Command | Description |
|---------|-------------|
| `hctl vcluster create` | Create a new vCluster via Kratix ResourceRequest |
| `hctl vcluster delete` | Delete a vCluster |
| `hctl vcluster list` | List active vClusters |

### Addon Management (`addon`)

| Command | Description |
|---------|-------------|
| `hctl addon list` | List available addons |
| `hctl addon enable` | Enable an addon for a cluster role/environment |
| `hctl addon disable` | Disable an addon |

### Other

| Command | Description |
|---------|-------------|
| `hctl scale` | Scale namespace workloads up/down |
| `hctl secret` | Manage ExternalSecret resources |
| `hctl ai` | AI-assisted operations |
| `hctl completion [bash\|zsh\|fish]` | Generate shell completion scripts |

## Configuration

Config lives at `~/.config/hctl/config.yaml` (or `$XDG_CONFIG_HOME/hctl/config.yaml`). Created automatically on first `hctl init`.

```yaml
repoPath: /home/user/projects/gitops_homelab_2_0
defaultCluster: vcluster-media
gitMode: prompt          # auto | prompt | generate | stage-only
argocdURL: https://argocd.cluster.integratn.tech
interactive: true
kubeContext: ""           # empty = current context
outputFormat: ""          # text (default) | json | yaml
platform:
  domain: cluster.integratn.tech
  clusterSubnet: 10.0.4.0/24
  metalLBPool: 10.0.4.200-253
  platformNamespace: platform-requests
```

### Git Modes

| Mode | Behavior |
|------|----------|
| `prompt` | Ask before committing/pushing (default, interactive) |
| `auto` | Commit and push automatically after every change |
| `generate` | Write files only, no git operations |
| `stage-only` | Stage files (`git add`) but don't commit or push |

### Global Flags

```
--config string       Config file path (default: ~/.config/hctl/config.yaml)
--non-interactive     Disable interactive prompts
--output, -o string   Output format: text, json, yaml
--verbose, -v         Enable debug output
--quiet, -q           Suppress informational output
```

## Output Formats

All commands support `--output json` and `--output yaml` for machine-readable output, making `hctl` scriptable:

```bash
# JSON status for monitoring integration
hctl status -o json | jq '.vclusters'

# YAML deploy preview
hctl deploy render -o yaml

# Pipe diagnostics
hctl diagnose my-app --bundle /tmp/diag.json
```

## Shell Completions

Dynamic completions are provided for workload and vCluster names:

```bash
# Bash (add to .bashrc or loaded automatically in nix shell)
source <(hctl completion bash)

# Zsh
hctl completion zsh > "${fpath[1]}/_hctl"

# Fish
hctl completion fish | source
```

## Project Structure

```
cli/
├── main.go                    # Entrypoint
├── Makefile                   # Build targets
├── go.mod
├── cmd/
│   ├── root.go                # Root command, global flags, config init
│   ├── commands.go            # init, status, diagnose, reconcile, context, alerts
│   ├── convenience.go         # up, down, open, logs
│   ├── doctor.go              # Environment health checks
│   ├── trace.go               # Resource lifecycle tracing
│   ├── completions.go         # Dynamic shell completions
│   ├── alerts.go              # Alert display
│   ├── deploy/                # Score-based workload deployment
│   ├── vcluster/              # vCluster management
│   ├── addon/                 # Addon management
│   ├── scale/                 # Namespace scaling
│   ├── secret/                # ExternalSecret management
│   └── ai/                    # AI-assisted operations
├── internal/
│   ├── config/                # Config loading, validation, defaults
│   ├── deploy/                # Score → Stakater translation engine
│   ├── git/                   # Git commit/push workflow
│   ├── kube/                  # Kubernetes client (Clientset + dynamic)
│   ├── platform/              # Platform status types + collectors
│   ├── score/                 # Score spec types + loader
│   └── tui/                   # Structured output, logging, theming
├── pkg/
│   └── provisioners/          # Resource provisioners (postgres, redis, route, volume, dns)
└── vendor/                    # Vendored dependencies
```

## Testing

```bash
cd cli
make test          # go test ./... -v
go test -count=1 ./...   # skip cache
```

## Development

The nix dev shell provides all tooling. Just open a terminal in the repo and everything is available:

```bash
# Build and test locally
cd cli && make build && ./bin/hctl version

# After making changes, rebuild the nix package
nix build .#hctl

# Run the full suite
make test
```
