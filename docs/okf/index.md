---
okf_version: '0.2'
---

# GitOps Homelab 2.0 Knowledge Bundle

An [OKF](https://github.com/GoogleCloudPlatform/knowledge-catalog) bundle
describing this repository and the live cluster it manages (`the-cluster`).
Start with [Getting started](getting-started.md); consult
[known issues](cluster/known-issues.md) for the current warts.

# Start here

* [Getting started](getting-started.md) - orientation: what this platform is, how to navigate the bundle, and how it was produced.

# Platform design

* [Platform architecture](platform/architecture.md) - the big picture: Talos + ArgoCD + Kratix + vclusters, and the ADRs behind them.
* [GitOps flow, end to end](platform/gitops-layers.md) - how a change travels through bootstrap, addons, promises, and workloads.
* [Secret management](platform/secret-management.md) - the 1Password → ExternalSecrets design and its enforcement layers.
* [Networking](platform/networking.md) - Cilium, MetalLB, Gateway API, DNS, TLS, and the full address plan.
* [Storage](platform/storage.md) - NFS-backed storage classes, who uses them, accepted limits.
* [Security posture](platform/security-posture.md) - enforced policies, scanning, SSO, resource governance.
* [Resilience](platform/resilience.md) - how the cluster heals itself: the seven layers, the one rule, and what is still outside git's reach.
* [Observability](platform/observability.md) - hub-and-spoke metrics, Loki logging, Matrix alerting with log deep-links.
* [AI stack](platform/ai-stack.md) - Open WebUI, Qdrant, MCP servers, and the retired llmkube/git-indexer.

# Live cluster (the-cluster)

* [the-cluster](cluster/the-cluster.md) - identity, namespaces, access, health at snapshot time.
* [Workload inventory](cluster/workload-inventory.md) - everything running, by namespace (snapshot).
* [Component versions](cluster/component-versions.md) - running image versions vs chart pins (snapshot).
* [vcluster-media](cluster/vcluster-media.md) - the one tenant vcluster and its media stack.
* [Known issues & drift](cluster/known-issues.md) - broken, orphaned, cosmetic, and docs-vs-reality findings.

# Addon layer

* [Addon system](addons/addon-system.md) - the ApplicationSet factory chart and its merge/templating rules.
* [Addon inventory](addons/addon-inventory.md) - every addon, per layer, with versions and purpose.

# Kratix promises

* [Kratix platform layer](promises/kratix.md) - pipelines, the public state repo, destination, status reconciler.
* [vcluster-orchestrator-v2](promises/vcluster-orchestrator-v2.md) - the composite vcluster promise.
* [http-service](promises/http-service.md) - the HTTP app product promise.
* [argocd-application](promises/argocd-application.md) - leaf: renders an ArgoCD Application.
* [argocd-project](promises/argocd-project.md) - leaf: renders an AppProject.
* [argocd-cluster-registration](promises/argocd-cluster-registration.md) - the kubeconfig→1Password→ArgoCD loop.
* [gateway-route](promises/gateway-route.md) - leaf: renders HTTPRoute (+ redirect).
* [external-secret](promises/external-secret.md) - leaf: renders 1Password-backed ExternalSecrets.

# Infrastructure

* [Talos nodes](infrastructure/talos-nodes.md) - the three bare-metal nodes and their machine configs.
* [Matchbox PXE](infrastructure/matchbox-pxe.md) - network-boot provisioning and MAC→node mapping.
* [Terraform bootstrap](infrastructure/terraform-bootstrap.md) - the one imperative step that installs ArgoCD.

# Tooling

* [hctl](tooling/hctl.md) - the homelab control CLI.
* [Nix dev shell](tooling/nix-dev-shell.md) - the pinned toolchain and helper scripts.
* [CI workflows](tooling/ci-workflows.md) - GitHub Actions, the no-Secrets gate, repo guardrails.
