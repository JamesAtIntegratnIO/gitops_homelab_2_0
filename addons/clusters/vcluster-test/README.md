# vCluster Test Cluster

This directory will contain ArgoCD configuration for applications deployed to the vcluster-test virtual cluster.

## Setup

1. After the vCluster is provisioned, retrieve its kubeconfig from 1Password (item: `vcluster-test-kubeconfig`)
2. Create an ArgoCD cluster secret to register it as a deployment target
3. Add applications to `addons.yaml` using the same pattern as other clusters

## Structure

```
vcluster-test/
  addons.yaml           # Applications to deploy to this vcluster
  cluster-secret.yaml   # ArgoCD cluster registration (requires kubeconfig from 1Password)
  addons/               # Addon-specific configuration
```
