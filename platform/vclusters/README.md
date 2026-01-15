# Platform VClusters

This directory contains Kratix ResourceRequests for provisioning virtual clusters.

## How it works

1. Add a VCluster ResourceRequest YAML file to this directory
2. ArgoCD automatically applies it via the `platform-vclusters` Application
3. Kratix Promise pipeline provisions the vCluster
4. Pipeline syncs kubeconfig to 1Password (vault: homelab, item: `vcluster-<name>-kubeconfig`)
5. Register the vCluster in ArgoCD as a deployment target (see addons/clusters/vcluster-<name>/)

## ResourceRequest Example

```yaml
apiVersion: platform.integratn.tech/v1alpha1
kind: VCluster
metadata:
  name: my-cluster
  namespace: vcluster-my-namespace
spec:
  name: my
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
