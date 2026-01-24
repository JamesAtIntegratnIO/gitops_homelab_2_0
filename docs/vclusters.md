# vCluster Platform Requests

This section explains how vClusters are requested and fulfilled via Kratix.

## Request Location
- Requests live under `platform/vclusters/`
- The namespace `platform-requests` is created by `platform/vclusters/00-namespace.yaml`

## End‑to‑End Flow
1. Add a `VClusterOrchestrator` request in `platform/vclusters/`.
2. ArgoCD applies the request to `platform-requests`.
3. The `vcluster-orchestrator` promise renders sub‑requests.
4. Sub‑promises create namespaces, configs, and ArgoCD resources.
5. Kubeconfig is synced to 1Password and exposed via ExternalSecret.
6. The vcluster is registered into ArgoCD as a destination.

## Required Fields (VClusterOrchestrator)
- `spec.name`: vcluster name
- `spec.targetNamespace`: host namespace to deploy into
- `spec.projectName`: ArgoCD project name
- `spec.vcluster.preset`: `dev` or `prod`
- `spec.integrations.certManager.clusterIssuerSelectorLabels`
- `spec.integrations.externalSecrets.clusterStoreSelectorLabels`
- `spec.integrations.argocd.environment`
- `spec.argocdApplication.repoURL`, `chart`, `targetRevision`

## Optional Fields (Highlights)
- `spec.vcluster.k8sVersion` (default v1.34.3)
- `spec.vcluster.isolationMode` (`standard` / `strict`)
- `spec.vcluster.persistence.storageClass`
- `spec.exposure.hostname` and `spec.exposure.apiPort`
- `spec.integrations.argocd.clusterLabels` / `clusterAnnotations`
- `spec.integrations.argocd.workloadRepo.*`

## Where to Inspect Templates
- Request schema and examples: `platform/vclusters/README.md`
- Orchestration logic: `promises/vcluster-orchestrator/`

## Tips
- Prefer presets (`dev`/`prod`) and avoid raw `helmOverrides` unless necessary.
- Always use ExternalSecrets for kubeconfig delivery.
- Keep vcluster names and namespaces aligned for clarity.
