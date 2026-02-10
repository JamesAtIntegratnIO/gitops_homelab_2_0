# ArgoCD Cluster Registration Promise

Registers any Kubernetes cluster with ArgoCD for GitOps management via the
1Password Connect → ExternalSecret pattern.

This promise is **not** specific to vclusters — it works for any cluster that
has its kubeconfig stored as a Kubernetes secret.

## What It Creates

| # | Resource | Purpose |
|---|----------|---------|
| 1 | ExternalSecret | Fetches 1Password Connect token for the sync job |
| 2 | ServiceAccount | Identity for the kubeconfig sync job |
| 3 | Role | Grants read access to the kubeconfig and 1Password token secrets |
| 4 | RoleBinding | Binds the role to the service account |
| 5 | Job | Reads kubeconfig, extracts TLS certs, writes to 1Password |
| 6 | ExternalSecret | Pulls kubeconfig back from 1Password |
| 7 | ExternalSecret | Creates the ArgoCD cluster secret from 1Password data |

## API

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `spec.name` | string | Yes | | Cluster name in ArgoCD |
| `spec.targetNamespace` | string | Yes | | Namespace for RBAC/sync resources |
| `spec.kubeconfigSecret` | string | Yes | | K8s secret containing the kubeconfig |
| `spec.kubeconfigKey` | string | No | `config` | Key within the secret |
| `spec.externalServerURL` | string | Yes | | Cluster API URL for ArgoCD |
| `spec.onePasswordItem` | string | No | `{name}-kubeconfig` | 1Password item name |
| `spec.onePasswordConnectHost` | string | No | `https://connect.integratn.tech` | 1Password Connect URL |
| `spec.environment` | string | No | `development` | ArgoCD environment label |
| `spec.baseDomain` | string | No | `integratn.tech` | Base domain for naming |
| `spec.baseDomainSanitized` | string | No | derived | Dots → dashes |
| `spec.clusterLabels` | map | No | | ArgoCD cluster secret labels |
| `spec.clusterAnnotations` | map | No | | ArgoCD cluster secret annotations |
| `spec.syncJobName` | string | No | `{name}-kubeconfig-sync` | Override for reconciliation |

## Example

```yaml
apiVersion: platform.integratn.tech/v1alpha1
kind: ArgoCDClusterRegistration
metadata:
  name: media-cluster-registration
  namespace: platform-requests
spec:
  name: vcluster-media
  targetNamespace: vcluster-media
  kubeconfigSecret: vc-vcluster-media
  externalServerURL: https://media.integratn.tech:443
  environment: production
  clusterLabels:
    cluster_role: vcluster
    enable_argocd: "true"
    enable_cert_manager: "true"
    enable_external_secrets: "true"
  clusterAnnotations:
    addons_repo_url: https://github.com/jamesatintegratnio/gitops_homelab_2_0.git
    workload_repo_url: https://github.com/jamesatintegratnio/gitops_homelab_2_0
```
