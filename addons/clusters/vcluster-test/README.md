# vCluster Test Cluster

This directory will contain ArgoCD configuration for applications deployed to the vcluster-test virtual cluster.

## Setup

1. After the vCluster is provisioned, the ArgoCD cluster secret is created automatically from 1Password
2. Add applications to `addons.yaml` using the same pattern as other clusters

## Structure

```
vcluster-test/
  addons.yaml           # Applications to deploy to this vcluster
  cluster-secret.yaml   # (optional) override labels/annotations if needed
  addons/               # Addon-specific configuration
```
