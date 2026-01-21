# GitOps Homelab Platform Architecture & Workflow

## Overview
This repository defines a GitOps-driven homelab platform built on Talos Linux and Kubernetes, orchestrated by Argo CD, and extended with Kratix promises. It delivers:

- Cluster lifecycle and bootstrap configuration (Terraform + Matchbox)
- Addons and platform services via Argo CD ApplicationSets
- Platform workflows via Kratix promises (including vcluster provisioning)
- Secure secret management through ExternalSecrets backed by 1Password

## Core Architecture

### Control-plane components
- **Talos + Kubernetes**: Immutable OS and control-plane for the cluster
- **Argo CD**: GitOps engine that installs addons and reconciles platform state
- **Kratix**: Platform promises and pipelines that fulfill ResourceRequests
- **ExternalSecrets**: Syncs secrets from 1Password into Kubernetes

### Git repositories
- **This repo**: Desired state for addons, platform requests, promises, and infrastructure
- **Kratix state repo**: Rendered resources from Kratix pipelines used by the state reconciler

### High-level flow
1. Argo CD bootstraps addon ApplicationSets from Git
2. Addons are installed by per-cluster Applications
3. Kratix promises are installed and run pipelines for platform requests
4. Pipelines render resources into the state repo
5. Argo CD state reconciler applies those resources to the cluster

## GitOps Layers & Responsibilities

### 1) Addons layer (ApplicationSets)
The addons layer declares cluster-wide services (e.g., Argo CD, cert-manager, external-secrets, Kratix).

- Entry point: [addons/README.md](addons/README.md)
- Bootstrap ApplicationSet: [terraform/cluster/bootstrap/addons.yaml](terraform/cluster/bootstrap/addons.yaml)
- ApplicationSets chart: [addons/charts/application-sets](addons/charts/application-sets)

**Workflow**
- A bootstrap ApplicationSet points Argo CD at this repo.
- The ApplicationSets chart renders one ApplicationSet per addon.
- Each addon ApplicationSet creates per-cluster Applications based on cluster labels.

### 2) Platform requests layer
Platform requests are Git-managed CRs that Kratix fulfills.

- vcluster requests live in: [platform/vclusters](platform/vclusters)
- vcluster request flow: [platform/vclusters/README.md](platform/vclusters/README.md)

### 3) Kratix promises & pipelines
Kratix promises define APIs and workflows to fulfill platform requests.

- Promises root: [promises](promises)
- vcluster orchestrator: [promises/vcluster-orchestrator/README.md](promises/vcluster-orchestrator/README.md)
- kubeconfig sync promise: [promises/vcluster-kubeconfig-sync/README.md](promises/vcluster-kubeconfig-sync/README.md)

**Pipeline images**
- Pipeline images are built by GitHub Actions on changes under promises/*/pipelines or promises/*/internal.
- Workflow: [.github/workflows/build-promise-images.yaml](.github/workflows/build-promise-images.yaml)

## vcluster Workflow (End-to-End)
The vcluster flow is the reference platform workflow.

1. **Request**: Add or update a vcluster request under [platform/vclusters](platform/vclusters).
2. **Reconcile**: Argo CD applies the request into the platform-requests namespace.
3. **Orchestrate**: The vcluster orchestrator promise renders sub-requests:
   - vcluster core (helm chart)
   - coredns
   - argocd project & application
   - kubeconfig sync job
   - external secret for kubeconfig
   - Argo CD cluster registration
4. **State output**: Pipelines render manifests into the Kratix state repo.
5. **Apply**: The Kratix state reconciler app applies those manifests to the cluster.
6. **Access**: Kubeconfig is synced into 1Password and exposed via ExternalSecrets.

## Secret Management Model
Secrets never live in Git. All credentials and kubeconfigs are sourced from 1Password and synced via ExternalSecrets.

- ExternalSecrets are the only supported secret mechanism for promises.
- Promises must render ExternalSecret resources, not Secret resources.

## Repository Structure (Key Paths)
- Addons: [addons](addons)
- Platform requests: [platform](platform)
- Promises: [promises](promises)
- Terraform bootstrap and infrastructure: [terraform](terraform)
- Talos assets: [matchbox](matchbox)

## Operational Workflow

### Add or change an addon
1. Update addon config under [addons/clusters](addons/clusters)
2. Commit and push
3. Argo CD reconciles and applies the addon

### Add or change a vcluster
1. Update a vcluster request in [platform/vclusters](platform/vclusters)
2. Commit and push
3. Argo CD applies request → Kratix fulfills → state reconciler applies outputs

### Update a promise pipeline
1. Modify scripts under [promises](promises)
2. Commit and push
3. GitHub Actions builds a new pipeline image
4. Refresh the Kratix promises app to pick up the new image
5. Re-trigger the request reconcile to re-run pipelines

## Notes
- Argo CD is the source of truth for cluster state.
- Kratix pipelines are idempotent and designed to be re-run safely.
- State reconciliation is fully GitOps-driven via the Kratix state repo.
