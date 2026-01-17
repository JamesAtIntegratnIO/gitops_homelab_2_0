# vCluster Promise

This Promise provides virtual Kubernetes clusters using [vcluster](https://www.vcluster.com/) by Loft Labs.

## Features

- **On-demand virtual clusters**: Create isolated Kubernetes clusters within your host cluster
- **Multiple K8s versions**: Support for Kubernetes 1.32, 1.33, and 1.34
- **Isolation modes**: Choose between standard and strict isolation
- **Resource control**: Configure CPU and memory requests/limits per vcluster
- **GitOps integration**: Uses ArgoCD Application for declarative management
- **Secure kubeconfig storage**: Automatically syncs kubeconfig to 1Password via External Secrets Operator
- **External access**: kubeconfig available in 1Password for external access and backup

## Installation

The Promise will be installed through Kratix platform management.

```bash
kubectl apply -f promise.yaml
```

## Usage

Create a vcluster by applying a ResourceRequest:

```yaml
apiVersion: platform.integratn.tech/v1alpha1
kind: VCluster
metadata:
  name: my-vcluster
  namespace: my-team
spec:
  name: my-cluster
  targetNamespace: my-team
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

## Access the vcluster

After creation, you can connect using the vcluster CLI:

```bash
vcluster connect my-cluster --namespace my-team
```

Or use kubectl with the generated kubeconfig:

```bash
kubectl --kubeconfig <path-to-kubeconfig> get nodes
```

## Pipeline

The Promise includes two pipelines:

- **Configure**: Creates namespace, ArgoCD Application, kubeconfig sync Job, and ExternalSecret for kubeconfig
  - ArgoCD Application deploys vcluster using Helm chart
  - Job waits for vcluster kubeconfig and syncs it to 1Password
  - ExternalSecret references the kubeconfig from 1Password for external access
- **Delete**: Cleans up resources when the ResourceRequest is deleted

## Architecture

1. ResourceRequest creates ArgoCD Application
2. ArgoCD deploys vcluster via Helm
3. vcluster generates kubeconfig in secret `vc-<name>`
4. Sync Job reads kubeconfig and pushes to 1Password (vault: homelab)
5. ExternalSecret pulls kubeconfig from 1Password for external reference
6. Kubeconfig available both in-cluster and in 1Password

## Building the Pipeline Image

```bash
cd pipelines
docker build -t ghcr.io/jamesatintegratnio/vcluster-promise-pipeline:v0.1.0 .
docker push ghcr.io/jamesatintegratnio/vcluster-promise-pipeline:v0.1.0
```

## Security Considerations

- vclusters run with standard Kubernetes RBAC
- Strict isolation mode enables Pod Security Standards
- All vclusters run in their own namespace
- Network policies can be applied at the host cluster level
- Kubeconfig stored securely in 1Password
- RBAC limits kubeconfig sync job to read-only access to vcluster secret
- 1Password Connect token required for kubeconfig sync

## Prerequisites

- 1Password Connect deployed and configured
- External Secrets Operator with ClusterSecretStore `onepassword-store`
- 1Password vault named `homelab` (or update vault name in pipeline)
- Secret `onepassword-connect-token` in `external-secrets` namespace
