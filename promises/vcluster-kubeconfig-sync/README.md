# VCluster Kubeconfig Sync Promise

Creates the job that syncs vcluster kubeconfig to 1Password via External Secrets.

## ResourceRequest Example

```yaml
apiVersion: platform.integratn.tech/v1alpha1
kind: VClusterKubeconfigSync
metadata:
  name: media
  namespace: platform-requests
spec:
  name: media
  targetNamespace: vcluster-media
  kubeconfigSyncJobName: vcluster-media-kubeconfig-sync
  onepasswordItem: vcluster-media-kubeconfig
  hostname: media.integratn.tech
  apiPort: 443
```

## Required values

| Path | Type | Description |
| --- | --- | --- |
| spec.name | string | Vcluster instance name. |
| spec.targetNamespace | string | Namespace where the vcluster runs. |
| spec.kubeconfigSyncJobName | string | Job name for kubeconfig sync. |
| spec.onepasswordItem | string | 1Password item name for the kubeconfig. |

## Optional values

| Path | Type | Purpose |
| --- | --- | --- |
| spec.hostname | string | External hostname for the API server. |
| spec.apiPort | integer | External API port. |
| spec.serverUrl | string | Override server URL if not derived from hostname/port. |# VCluster Kubeconfig Sync Promise

Creates the job that syncs vcluster kubeconfig to 1Password via ExternalSecrets token.
