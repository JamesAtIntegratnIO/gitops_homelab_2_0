# VCluster Orchestrator Promise

Orchestrates the full vcluster stack by rendering ResourceRequests for:
- `vcluster-core`
- `vcluster-coredns`
- `argocd-project`
- `argocd-application`
- `vcluster-kubeconfig-sync`
- `vcluster-kubeconfig-external-secret`
- `vcluster-argocd-cluster-registration`

All inputs are structured; raw manifest inputs are not supported.

## ResourceRequest Example

```yaml
apiVersion: platform.integratn.tech/v1alpha1
kind: VClusterOrchestrator
metadata:
  name: my-cluster
  namespace: platform-requests
spec:
  name: my
  targetNamespace: vcluster-my-namespace
  projectName: vcluster-my-namespace
  vcluster:
    preset: dev
  exposure:
    hostname: my.integratn.tech
    apiPort: 443
  integrations:
    certManager:
      clusterIssuerSelectorLabels:
        integratn.tech/cluster-issuer: letsencrypt-prod
    externalSecrets:
      clusterStoreSelectorLabels:
        integratn.tech/cluster-secret-store: onepassword-store
    argocd:
      environment: production
  argocdApplication:
    repoURL: https://charts.loft.sh
    chart: vcluster
    targetRevision: 0.30.4
```

## Required values

| Path | Type | Description |
| --- | --- | --- |
| spec.name | string | Name of the vcluster instance. |
| spec.targetNamespace | string | Namespace where the vcluster will be deployed. |
| spec.projectName | string | ArgoCD project name for the vcluster. |
| spec.vcluster.preset | string | Base sizing preset for the vcluster (`dev` or `prod`). |
| spec.integrations.certManager.clusterIssuerSelectorLabels | object | Label selector for ClusterIssuers to sync from host. |
| spec.integrations.externalSecrets.clusterStoreSelectorLabels | object | Label selector for ClusterSecretStores to sync from host. |
| spec.integrations.argocd.environment | string | Environment label used for ArgoCD cluster secret selectors. |
| spec.argocdApplication.repoURL | string | Helm repo URL for the vcluster chart. |
| spec.argocdApplication.chart | string | Helm chart name. |
| spec.argocdApplication.targetRevision | string | Helm chart version. |

## Optional values (with defaults)

| Path | Type | Default | Purpose |
| --- | --- | --- | --- |
| spec.vcluster.k8sVersion | string | v1.34.3 | Kubernetes version for the virtual cluster. Allowed: v1.34.3, 1.34, 1.33, 1.32. |
| spec.vcluster.replicas | integer | - | Override vcluster control plane replicas. |
| spec.vcluster.isolationMode | string | standard | Workload isolation mode (`standard` or `strict`). |
| spec.vcluster.resources.requests.cpu | string | 200m | Control plane CPU request. |
| spec.vcluster.resources.requests.memory | string | 512Mi | Control plane memory request. |
| spec.vcluster.resources.limits.cpu | string | 1000m | Control plane CPU limit. |
| spec.vcluster.resources.limits.memory | string | 1Gi | Control plane memory limit. |
| spec.vcluster.persistence.enabled | boolean | true | Enable control plane persistence. |
| spec.vcluster.persistence.size | string | 5Gi | Persistent volume size. |
| spec.vcluster.persistence.storageClass | string | - | Storage class for persistence. |
| spec.vcluster.coredns.replicas | integer | 1 | CoreDNS replicas. |
| spec.vcluster.networking.clusterDomain | string | cluster.local | Cluster domain for the virtual cluster. |
| spec.vcluster.backingStore | object | - | Backing store configuration (passed through). |
| spec.vcluster.exportKubeConfig | object | - | Kubeconfig export overrides. `server` is derived from exposure when hostname is set. |
| spec.vcluster.helmOverrides | object | - | Additional Helm values (avoid for standard clusters). |
| spec.exposure.hostname | string | - | DNS hostname for the vcluster API endpoint. |
| spec.exposure.subnet | string | - | CIDR subnet for VIP allocation. |
| spec.exposure.vip | string | - | VIP for the vcluster API (defaults to .100 in subnet). |
| spec.exposure.apiPort | integer | 8443 | API port exposed by the vcluster service. |
| spec.integrations.argocd.clusterLabels | object | - | Extra labels for the ArgoCD cluster secret. |
| spec.integrations.argocd.clusterAnnotations | object | - | Extra annotations for the ArgoCD cluster secret. |
| spec.integrations.argocd.workloadRepo.url | string | https://github.com/jamesatintegratnio/gitops_homelab_2_0 | Workloads repo URL for vcluster ArgoCD. |
| spec.integrations.argocd.workloadRepo.basePath | string | "" | Base path inside workloads repo. |
| spec.integrations.argocd.workloadRepo.path | string | workloads | Path inside workloads repo. |
| spec.integrations.argocd.workloadRepo.revision | string | main | Git revision for workloads repo. |
| spec.argocdApplication.destinationServer | string | https://kubernetes.default.svc | Destination server for ArgoCD vcluster install. |
| spec.argocdApplication.syncPolicy | object | - | Override ArgoCD sync policy for vcluster install. |# VCluster Orchestrator Promise

This promise orchestrates the full vcluster stack by rendering mandatory ResourceRequests for sub-promises:
- `vcluster-core`
- `vcluster-coredns`
- `argocd-project`
- `argocd-application`
- `vcluster-kubeconfig-sync`
- `vcluster-kubeconfig-external-secret`
- `vcluster-argocd-cluster-registration`

All inputs are structured; raw manifest inputs are not supported.
