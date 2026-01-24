# Kratix Promises

Kratix promises define platform APIs and pipelines that render resources into the Kratix state repo. ArgoCD then applies those rendered resources to the target clusters.

## Location
- `promises/` contains schemas, CRDs, pipelines, and helper scripts.

## Non‑Negotiable Rules
- **Never** output `kind: Secret` from a promise pipeline.
- **Always** use ExternalSecrets backed by 1Password.
- Always set explicit namespaces on rendered resources.

## Core Promise Inventory
### vcluster orchestration
- `vcluster-orchestrator/`: orchestrates all vcluster sub‑promises.
- `vcluster-core/`: namespace + ConfigMap with vcluster Helm values.
- `vcluster-coredns/`: host namespace CoreDNS ConfigMap for vcluster.
- `vcluster-kubeconfig-sync/`: syncs kubeconfig to 1Password.
- `vcluster-kubeconfig-external-secret/`: exposes kubeconfig via ExternalSecret.
- `vcluster-argocd-cluster-registration/`: registers vcluster in ArgoCD.

### ArgoCD primitives
- `argocd-project/`: creates AppProject from structured inputs.
- `argocd-application/`: creates Application from structured inputs.

## Pipeline Mechanics
- Pipelines are idempotent and use `kubectl apply`.
- Outputs are labeled with `kratix.io/promise-name` and `app.kubernetes.io/managed-by: kratix`.
- Delete pipelines remove associated resources cleanly.

## vcluster Request Flow (Concrete)
1. User commits `VClusterOrchestrator` CR in `platform/vclusters/`.
2. ArgoCD applies it to `platform-requests`.
3. `vcluster-orchestrator` renders sub‑requests for each promise.
4. Sub‑promises render resources into the Kratix state repo.
5. State reconciler applies them to the host cluster.
6. Kubeconfig is synced to 1Password and exposed via ExternalSecret.

## Pipeline Image Builds
Pipeline container images are built by GitHub Actions whenever a promise pipeline changes. See:
- `.github/workflows/build-promise-images.yaml`

## Namespace Policy
vcluster namespaces include PodSecurity labels. Privileged labels are scoped only to namespaces that require hostPath access (e.g., log collectors).
