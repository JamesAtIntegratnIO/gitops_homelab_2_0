# ArgoCD Application Promise

Creates an ArgoCD `Application` resource from a platform ResourceRequest.

## API

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `spec.name` | string | Yes | Name of the Application |
| `spec.namespace` | string | No | Target namespace (default: `argocd`) |
| `spec.project` | string | Yes | ArgoCD AppProject name |
| `spec.annotations` | map[string]string | No | Annotations on the Application |
| `spec.labels` | map[string]string | No | Labels on the Application |
| `spec.finalizers` | []string | No | Finalizers on the Application |
| `spec.source.repoURL` | string | Yes | Helm chart or git repository URL |
| `spec.source.chart` | string | No | Helm chart name |
| `spec.source.targetRevision` | string | Yes | Chart version or git revision |
| `spec.source.helm.releaseName` | string | No | Helm release name |
| `spec.source.helm.valuesObject` | object | No | Helm values |
| `spec.destination.server` | string | Yes | Target cluster API server |
| `spec.destination.namespace` | string | Yes | Target namespace |
| `spec.syncPolicy` | object | No | ArgoCD sync policy |

## Example

```yaml
apiVersion: platform.integratn.tech/v1alpha1
kind: ArgoCDApplication
metadata:
  name: vcluster-media
  namespace: platform-requests
spec:
  name: vcluster-media
  namespace: argocd
  project: vcluster-media
  annotations:
    argocd.argoproj.io/sync-wave: "0"
  finalizers:
    - resources-finalizer.argocd.argoproj.io
  source:
    repoURL: https://charts.loft.sh
    chart: vcluster
    targetRevision: 0.31.0
    helm:
      releaseName: vcluster-media
      valuesObject:
        controlPlane:
          distro:
            k8s:
              enabled: true
  destination:
    server: https://kubernetes.default.svc
    namespace: vcluster-media
  syncPolicy:
    automated:
      selfHeal: true
      prune: true
```
