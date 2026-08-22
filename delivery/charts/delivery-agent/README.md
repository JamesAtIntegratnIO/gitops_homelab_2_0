# delivery-agent (chart)

Runs [`delivery-agent`](../../images/delivery-agent) in-cluster: Deployment,
Service, RBAC and NetworkPolicy.

> **Status: not yet implemented.** This README is the contract.

## Shape

- **Read-only RBAC.** `get`/`list`/`watch` on Kargo CRDs, ArgoCD Applications
  and AnalysisRuns, pods and events. No `create`, `update`, `patch` or `delete`
  anywhere — the agent observes the cluster and writes to pull requests, never
  to the cluster.
- **Not exposed.** No Ingress or HTTPRoute. Only Kargo calls it, in-cluster.
  Publishing it would be gratuitous exposure of something that can spend money
  and write to your repository.
- **Two halves of the network path.** The agent's namespace must admit Kargo's
  controller, *and* the controller's own egress policy must permit the agent.
  Missing the second half presents as a hang with zero bytes, not an error.
- **Secrets by reference.** The chart takes the name of an existing Secret. How
  it gets there — ExternalSecret, Vault Agent, SOPS, `kubectl create` — belongs
  to whoever installs this.
- **No default model provider.** `llm.provider` must be set explicitly. See
  [`../../adr/0004-provider-interfaces.md`](../../adr/0004-provider-interfaces.md).
