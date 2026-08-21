# Platform design

* [Platform architecture](architecture.md) - the big picture: Talos + ArgoCD + Kratix + vclusters, and the ADRs behind them.
* [GitOps flow, end to end](gitops-layers.md) - bootstrap → addons → promises → workloads, and reconciliation behavior.
* [Secret management](secret-management.md) - 1Password → ExternalSecrets, and the layers enforcing zero-secrets-in-git.
* [Networking](networking.md) - Cilium CNI, MetalLB, Gateway API routing, DNS and TLS, full address plan.
* [Storage](storage.md) - NFS storage classes, consumers, limits, recovery helper.
* [Security posture](security-posture.md) - enforced network policy, Kyverno, Trivy, Authentik SSO, VPA governance.
* [Observability](observability.md) - hub-and-spoke Prometheus, Loki, dashboards, Matrix alerting.
* [AI stack](ai-stack.md) - Open WebUI, Qdrant, MCP servers, retired components.
* [Kargo version updates](kargo.md) - how every image and chart pin is kept current: Warehouses, one Stage per artifact, PR-gated merges by policy.
