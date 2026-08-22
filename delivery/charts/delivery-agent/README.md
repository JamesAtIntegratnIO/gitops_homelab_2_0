# delivery-agent (chart)

Runs [`delivery-agent`](../../images/delivery-agent) in-cluster: Deployment,
Service, RBAC and NetworkPolicy.

## Install

Two Secrets and a values file. The chart creates neither Secret — how they get
there is yours to choose.

```yaml
image:
  repository: ghcr.io/you/delivery-agent
  digest: sha256:...          # prefer a digest to a moving tag
git:
  owner: you
  repo: platform
  repoURL: https://github.com/you/platform.git
  existingSecret: delivery-agent-git
llm:
  provider: openai            # no default; you must choose
  baseURL: http://model.internal:1234/v1
  model: your-model
triage:
  allowPaths: [addons/**]     # empty means it can fix nothing
networkPolicy:
  kargoNamespace: kargo
  egress:
    ipBlocks:
      - {cidr: 192.0.2.10/32, port: 1234}   # your model endpoint
    allowPublicHTTPS: true                   # your git host
```

Then point the pipelines chart's triage hook at it:

```yaml
triage:
  enabled: true
  url: http://<release>-delivery-agent.<namespace>.svc:8080/v1/promotion-opened
```

## The other half of the network path

This chart writes the policy governing what reaches the agent. It cannot write
the **Kargo controller's** egress policy, and that is the half people miss.

A controller allowed `0.0.0.0/0` with RFC1918 excepted — a common shape, since
it usually only needs to reach registries — cannot reach a ClusterIP at all.
The symptom is a hang with zero bytes, not an error, so it reads as a slow
agent rather than a blocked one. Add an explicit rule for this service's
namespace and port.

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
