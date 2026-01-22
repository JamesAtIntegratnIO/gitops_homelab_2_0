# VCluster Core Promise

Creates the target namespace and a ConfigMap containing Helm values for the vcluster chart.

## ResourceRequest Example

```yaml
apiVersion: platform.integratn.tech/v1alpha1
kind: VClusterCore
metadata:
  name: media
  namespace: platform-requests
spec:
  name: media
  targetNamespace: vcluster-media
  valuesYaml: |
    controlPlane:
      service:
        enabled: true
```

## Required values

| Path | Type | Description |
| --- | --- | --- |
| spec.name | string | Vcluster instance name. |
| spec.targetNamespace | string | Namespace where the vcluster will be deployed. |
| spec.valuesYaml | string | Helm values YAML content for the vcluster chart. |# VCluster Core Promise

Creates the target namespace and a ConfigMap containing Helm values for the vcluster chart.
