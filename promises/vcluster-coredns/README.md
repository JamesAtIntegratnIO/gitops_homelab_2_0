# VCluster CoreDNS Promise

Creates the host namespace CoreDNS ConfigMap for a vcluster.

## ResourceRequest Example

```yaml
apiVersion: platform.integratn.tech/v1alpha1
kind: VClusterCoredns
metadata:
  name: media
  namespace: platform-requests
spec:
  name: media
  targetNamespace: vcluster-media
  clusterDomain: cluster.local
```

## Required values

| Path | Type | Description |
| --- | --- | --- |
| spec.name | string | Vcluster instance name. |
| spec.targetNamespace | string | Host namespace for the vcluster. |
| spec.clusterDomain | string | Cluster domain for the vcluster. |# VCluster CoreDNS Promise

Creates the host namespace CoreDNS ConfigMap for a vcluster.
