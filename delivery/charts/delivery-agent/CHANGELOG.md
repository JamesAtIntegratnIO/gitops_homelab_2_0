# Changelog

All notable changes to the `delivery-agent` chart. Format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versioning is semver.

## [Unreleased]

### Added

- Deployment, Service, ServiceAccount, read-only ClusterRole and NetworkPolicy
  for the in-cluster triage agent.
- `networkPolicy.flavor: cilium` additionally emits a `CiliumNetworkPolicy`
  with `toFQDNs`, which names the hosts the agent may reach rather than a range
  that happens to contain them.
- `values.schema.json`, and template-level `fail` for the cross-field
  requirements a schema cannot express.

### Notes

- **No Ingress or HTTPRoute is rendered, by design.** Only Kargo calls this
  service, and publishing something that can spend money and write to your
  repository would be gratuitous exposure.
- **RBAC is read-only.** No create, update, patch or delete verb appears
  anywhere. The agent observes the cluster and writes to pull requests.
- **The chart never creates a Secret.** It takes the name of an existing one,
  so ExternalSecret, Vault Agent, SOPS or `kubectl create` all work and none is
  assumed.
- **`llm.provider` has no default** and the chart refuses to render without it.
- **`triage.allowPaths` must be non-empty** — an empty allowlist means the
  agent could never apply a fix, and failing at render time is better than
  discovering it when a fix is silently refused.
- The Kargo controller's **own** egress policy must permit this service. A
  controller allowed `0.0.0.0/0 except RFC1918` cannot reach a ClusterIP, and
  the symptom is a hang with zero bytes rather than an error.
