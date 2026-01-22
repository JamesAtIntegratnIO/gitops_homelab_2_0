# VCluster ArgoCD Cluster Registration Promise

Creates the ExternalSecret that registers a vcluster as an ArgoCD cluster.

## ResourceRequest Example

```yaml
apiVersion: platform.integratn.tech/v1alpha1
kind: VClusterArgoCDClusterRegistration
metadata:
  name: media
  namespace: platform-requests
spec:
  name: media
  argocdNamespace: argocd
  onepasswordItem: vcluster-media-kubeconfig
  labels:
    environment: production
  annotations:
    managed-by: kratix
```

## Required values

| Path | Type | Description |
| --- | --- | --- |
| spec.name | string | Vcluster instance name. |
| spec.argocdNamespace | string | ArgoCD namespace where the cluster secret is created. |
| spec.onepasswordItem | string | 1Password item name containing kubeconfig. |
| spec.labels | object | Labels applied to the ArgoCD cluster secret. |
| spec.annotations | object | Annotations applied to the ArgoCD cluster secret. |# VCluster ArgoCD Cluster Registration Promise

Creates the ExternalSecret that registers a vcluster as an ArgoCD cluster.
