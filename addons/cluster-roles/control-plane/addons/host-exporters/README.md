# host-exporters

The machines outside the cluster, as metrics in the cluster's Prometheus.

| Machine | How it gets in | job / labels |
|---|---|---|
| Proxmox VE, 192.168.0.10 | `pve-exporter` here (prometheus-pve-exporter), scraped at `/pve?target=192.168.0.10&cluster=1&node=1` | `job="pve-exporter"`, `pve_*` keyed by `id` (`node/pve`, `qemu/100`, `storage/pve/local`) |
| UniFi console, 192.168.0.1 | `unpoller` here | `job="unpoller"`, `unpoller_site_*` by `subsystem`, `unpoller_device_*` by `name` |
| Tower (Unraid), 10.0.0.12 | node_exporter **on Tower**, scraped by `tower.yaml`'s ScrapeConfig | `job="host-node"`, `host="tower"` |
| holocron, pitdroid | push their own: nix_dots_2 `modules/{darwin,nixos}/host-metrics.nix` (node_exporter + a Prometheus agent remote-writing to `prom-remote.cluster.integratn.tech`) | `job="host-node"`, `host="<name>"`; runners as `github_runner_up` / `github_runner_busy{runner}` |

The alerts are `platform-alerts/host-rules.yaml`; the landing page's **Hosts**
tab shows all of it. The namespace and NetworkPolicies are in
`network-policies/host-exporters.yaml`; Prometheus's egress to Tower:9100 is
in `network-policies/monitoring.yaml`.

## Credentials

Both from 1Password through `externalsecret.yaml`, never in git:

- **Proxmox**: item `homepage-secret`, the same read-only API token the
  landing page used. `HOMEPAGE_VAR_PVE_USER` is the token id
  (`user@realm!tokenname`); the ExternalSecret template splits it into
  `PVE_USER` and `PVE_TOKEN_NAME`. The token needs `PVEAuditor` on `/`.
- **UniFi**: item `unifi-mcp-login` (`username`, `password`), the account the
  UniFi MCP uses. A dedicated View Only local admin would be tighter.

Both exporters answer a scrape with an error when their login is refused, so
`ProxmoxExporterDown` / `UniFiPollerDown` is what a bad credential looks like;
the pod log says which.

## Tower

Unraid is not Nix-managed, so the exporter is installed on Tower by hand:
Unraid > Apps, search "Prometheus Node Exporter" (the plugin, not a Docker
container), install. It listens on 9100 and survives reboots. Until then
`HostNotReporting{host="tower"}` fires and the Hosts tab says so.

Unraid's array disks are filled to the brim by design, so the disk alerts skip
`/mnt/disk*`; `/mnt/user` (the array as a whole) and the cache pools count.

## Why a ScrapeConfig for Tower

A selectorless Service with a hand-written Endpoints is the usual way to point
a ServiceMonitor outside the cluster, but ArgoCD excludes Endpoints and
EndpointSlices from sync by default, so it would never be applied. The
ScrapeConfig lives in `monitoring` with `release: kube-prometheus-stack`
because that is exactly what the chart's default ScrapeConfig selector
matches; kube-prometheus-stack's values did not change.
