# Architecture

## Platform Overview
This repository defines a GitOps‑first homelab platform built on Talos + Kubernetes. ArgoCD and ApplicationSets provide continuous reconciliation, while Kratix provides a promise‑based platform API (notably for vcluster lifecycle).

**Core components**
- **Talos + Kubernetes**: immutable OS, declarative machine configs.
- **ArgoCD**: syncs all addons and platform requests.
- **Kratix**: promise pipelines generate state into a separate repo and apply it.
- **ExternalSecrets**: pulls secrets from 1Password into Kubernetes.
- **Gateway API**: nginx‑gateway‑fabric terminates TLS and routes HTTP.
- **Storage**: NFS provisioner (`config-nfs-client`).

## GitOps Layers (How Changes Flow)
1. **Bootstrap** (Terraform) creates a bootstrap ApplicationSet that points ArgoCD at this repo.
2. **Addons layer** renders per‑addon ApplicationSets and per‑cluster Applications.
3. **Platform requests** (e.g., vcluster requests) are committed under `platform/`.
4. **Kratix** pipelines render resources into the Kratix state repo.
5. **State reconciler** applies those rendered resources to the cluster.

## Responsibility Boundaries
- **addons/**: shared platform services and cluster components.
- **platform/**: platform requests (vcluster CRs).
- **promises/**: promise schemas and pipelines.
- **terraform/**: infrastructure provisioning and bootstrap.
- **matchbox/**: bare‑metal boot assets and Talos configs.

## Cluster Roles & Targeting
Addons are targeted by label selectors on ArgoCD cluster Secrets. Two major roles are used:
- **control‑plane**: host cluster addons (full services)
- **vcluster**: addons to be installed within virtual clusters

Environment overlays are selected by labels such as `environment=production`.

## Secrets & Credentials
All credentials are ExternalSecrets backed by 1Password.
- Promise pipelines are forbidden from creating `kind: Secret`.
- Requests must specify selectors to locate ClusterSecretStore and ClusterIssuer.

## Networking
- **Gateway**: nginx‑gateway‑fabric exposes services with HTTPRoute.
- **DNS**: external‑dns manages records; Gateway TLS uses wildcard certs.
- **MetalLB**: L2 address pool for LoadBalancers.

## Storage
The storage class `config-nfs-client` is the default for platform persistence (Grafana, Prometheus, Loki).

## Ownership & Drift
Manual edits in the cluster should be avoided. Drift is resolved through Git commits + ArgoCD sync.
