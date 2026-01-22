# VCluster Kubeconfig ExternalSecret Promise

Creates the ExternalSecret that exposes the vcluster kubeconfig from 1Password.

## ResourceRequest Example

```yaml
apiVersion: platform.integratn.tech/v1alpha1
kind: VClusterKubeconfigExternalSecret
metadata:
  name: media
  namespace: platform-requests
spec:
  name: media
  targetNamespace: vcluster-media
  onepasswordItem: vcluster-media-kubeconfig
```

## Required values

| Path | Type | Description |
| --- | --- | --- |
| spec.name | string | Vcluster instance name. |
| spec.targetNamespace | string | Namespace where the vcluster runs. |
| spec.onepasswordItem | string | 1Password item name for the kubeconfig. |# VCluster Kubeconfig ExternalSecret Promise

Creates the ExternalSecret that exposes the vcluster kubeconfig from 1Password.
