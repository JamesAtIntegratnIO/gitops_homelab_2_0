# Platform VClusters

This directory contains Kratix ResourceRequests for provisioning virtual clusters.

## How it works

1. Add a VCluster ResourceRequest YAML file to this directory
2. ArgoCD creates the `platform-requests` namespace from `00-namespace.yaml`
3. ArgoCD applies requests via the `platform-vclusters` Application
4. Kratix Promise pipeline provisions the vCluster into the `spec.targetNamespace`
5. Pipeline syncs kubeconfig to 1Password (vault: homelab, item: `vcluster-<name>-kubeconfig`)
6. Pipeline registers the vCluster as an ArgoCD deployment target

## ResourceRequest Example

```yaml
apiVersion: platform.integratn.tech/v1alpha1
kind: VCluster
metadata:
  name: my-cluster
  namespace: platform-requests
spec:
  name: my
  targetNamespace: vcluster-my-namespace
  k8sVersion: "1.34"
  isolationMode: standard
  resources:
    requests:
      cpu: "200m"
      memory: "512Mi"
    limits:
      cpu: "1000m"
      memory: "1Gi"
```

## Available Kubernetes Versions

- 1.34 (default)
- 1.33
- 1.32

## Isolation Modes

- `standard` (default): Standard workload isolation
- `strict`: Enhanced security with restricted pod security standards
