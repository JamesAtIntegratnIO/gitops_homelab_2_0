# GitOps Homelab Documentation

This folder is the **comprehensive, deep-dive knowledge base** for the homelab platform. It goes beyond surface-level descriptions to explain *how the system actually works*, *why architectural decisions were made*, and *how to operate it safely and effectively*.

## What Makes This Documentation Different

- **📚 External References**: Links to official documentation for every major component
- **🎯 Concrete Examples**: Real YAML/code from this repository, not generic snippets
- **🔍 Deep Technical Detail**: Data flow diagrams, timing information, actual kubectl output
- **🏗️ Architecture Decisions**: Explains WHY choices were made, not just WHAT was chosen
- **🔧 Operational Runbooks**: Step-by-step procedures for common tasks and troubleshooting
- **📊 Diagrams**: ASCII art and visual representations of complex flows
- **🐛 Real Troubleshooting**: Actual error messages with root cause analysis and fixes
- **⚡ Performance Guidance**: Resource limits, query optimization, scaling considerations

## Documentation Structure

### Core Architecture & Design
- **[Architecture](architecture.md)**: System architecture, ADRs, GitOps layers, security model, data flows
  - Includes: Architecture Decision Records, trust boundaries, drift detection
  - Diagrams: GitOps flow, cluster architecture, networking
  - External refs: ArgoCD, Kratix, Talos, Gateway API, vcluster

### Infrastructure & Bootstrap
- **[Bootstrap & Bare-metal Talos](bootstrap.md)**: PXE boot, Matchbox, Talos machine configs
  - Includes: iPXE boot sequence, control plane bootstrap, machine config generation
  - External refs: Talos Linux docs, Matchbox docs, iPXE
  
- **[Terraform & Infrastructure](terraform.md)**: Infrastructure as Code, module usage, PostgreSQL backend config
  - Includes: Prerequisites, backend setup, variable management, workflow
  - External refs: OpenTofu/Terraform docs, Helm provider, 1Password provider

### Platform Services
- **[Addons & ApplicationSets](addons.md)**: How platform services are deployed via ArgoCD
  - Includes: ApplicationSet mechanics, value precedence, cluster targeting
  - External refs: ArgoCD ApplicationSets, Helm, Stakater chart
  
- **[vCluster Platform Requests](vclusters.md)**: Requesting and managing virtual Kubernetes clusters
  - Includes: VClusterOrchestratorV2 CRD, promise workflow, networking, v1→v2 evolution
  - External refs: vcluster docs, Kratix docs
  
- **[Kratix Promises & Pipelines](promises.md)**: Promise development, pipeline execution, state repo
  - Includes: Pipeline mechanics, secret management rules, image builds
  - External refs: Kratix docs, Kubernetes Jobs

### Observability
- **[Observability](observability.md)**: Metrics, logs, dashboards, alerting
  - Includes: Data flow diagrams, PromQL/LogQL examples, troubleshooting scenarios
  - External refs: Prometheus, Grafana, Loki, kube-prometheus-stack

### Operations
- **[Operations & Runbooks](operations.md)**: Day-to-day operations, incident response, maintenance
  - Includes: Routine workflows, troubleshooting checklists, upgrade procedures
  - External refs: ArgoCD CLI, kubectl, talosctl

## Quick Navigation by Use Case

**I want to...**

- **Understand the overall system**: Start with [Architecture](architecture.md)
- **Bootstrap a new bare-metal cluster**: See [Bootstrap](bootstrap.md)
- **Add a new platform service**: See [Addons](addons.md)
- **Create a vcluster for a team**: See [vClusters](vclusters.md)
- **Develop a new Kratix promise**: See [Promises](promises.md)
- **View metrics and logs**: See [Observability](observability.md)
- **Troubleshoot an issue**: See [Operations](operations.md)
- **Provision infrastructure**: See [Terraform](terraform.md)

## Key Concepts

### GitOps Workflow
All changes flow through Git → ArgoCD → Kubernetes. Manual cluster edits are temporary and should be codified in Git for persistence.

### Promise-Based Platform
Kratix provides self-service infrastructure via "promises". Users submit resource requests (e.g., VClusterOrchestrator), Kratix pipelines render resources to a state repo, ArgoCD applies them.

### Hub-and-Spoke Observability
Host cluster runs full Prometheus + Grafana + Loki. vClusters run Prometheus agent mode and ship metrics/logs to the hub.

### Zero Secrets in Git
All secrets live in 1Password. ExternalSecrets Operator pulls them into Kubernetes as Secret resources.

## Repository Anchors
- Root overview: [../README.md](../README.md)
- Promise directory: [../promises/](../promises/)
- Addons directory: [../addons/](../addons/)
- Platform requests: [../platform/](../platform/)

## Additional Documentation
- **[Monitoring Summary](MONITORING_SUMMARY.md)**: Overview of observability stack implementation
- **[Kratix Troubleshooting](kratix-troubleshooting.md)**: Kratix-specific debugging guide
- **[Phase 2 Verification](phase2-verification.md)**: Platform phase 2 verification checklist

## Non‑Negotiables

These are hard rules enforced by pre-commit hooks, CI, or operational practice:

1. **Git is the only source of truth** - No manual cluster edits without Git commits
2. **No secrets in Git** - Use ExternalSecrets + 1Password, never commit `kind: Secret`
3. **Gateway API for HTTP** - Use HTTPRoute, not Ingress annotations
4. **Kratix promises must not output Secrets** - Will be blocked by CI validation
5. **All changes are small, reviewed, and reversible** - Atomic commits, test in staging when possible

## How to Read This Documentation

Each documentation file follows a similar structure:

1. **Overview & External References**: Links to official docs
2. **Concepts**: What the technology is and why we use it
3. **Configuration Examples**: Real YAML from this repo
4. **Data Flows**: Diagrams showing how requests/data moves through the system
5. **Operational Procedures**: How to perform common tasks
6. **Troubleshooting**: Common issues with root cause and resolution

**Pro Tip**: Use your browser's "Find in Page" (Ctrl+F / Cmd+F) to search for specific error messages, components, or concepts across documentation files.

## Contributing to Documentation

When updating documentation:
- ✅ Add external references to official docs
- ✅ Include concrete examples from this repository
- ✅ Explain WHY decisions were made (ADRs)
- ✅ Add troubleshooting scenarios with actual error messages
- ✅ Update diagrams when architecture changes
- ❌ Don't just list features, explain how they work
- ❌ Don't use generic examples, use actual repo code
- ❌ Don't skip the "why" - future you will thank you

## External Resources

### Core Technologies
- [Kubernetes Documentation](https://kubernetes.io/docs/)
- [ArgoCD Documentation](https://argo-cd.readthedocs.io/)
- [Kratix Documentation](https://kratix.io/docs/)
- [Talos Linux Documentation](https://www.talos.dev/)
- [vcluster Documentation](https://www.vcluster.com/docs/)
- [Gateway API Specification](https://gateway-api.sigs.k8s.io/)

### Observability
- [Prometheus Documentation](https://prometheus.io/docs/)
- [Grafana Documentation](https://grafana.com/docs/)
- [Loki Documentation](https://grafana.com/docs/loki/)
- [kube-prometheus-stack Chart](https://github.com/prometheus-community/helm-charts/tree/main/charts/kube-prometheus-stack)

### Security & Secrets
- [External Secrets Operator](https://external-secrets.io/)
- [1Password Connect](https://developer.1password.com/docs/connect/)
- [cert-manager Documentation](https://cert-manager.io/docs/)

### Infrastructure
- [OpenTofu Documentation](https://opentofu.org/docs/)
- [Terraform Documentation](https://developer.hashicorp.com/terraform/docs)
- [Matchbox Documentation](https://matchbox.psdn.io/)

---

**Last Updated**: February 2026  
**Maintainer**: Platform Team  
**Repository**: https://github.com/jamesatintegratnio/gitops_homelab_2_0
