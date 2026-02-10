# ArgoCD Application Promise

Creates a single ArgoCD Application from a structured spec.

## ResourceRequest Example

```yaml
apiVersion: platform.integratn.tech/v1alpha1
kind: ArgoCDApplication
metadata:
  name: my-app
  namespace: platform-requests
spec:
  name: my-app
  namespace: argocd
  project: default
  source:
    repoURL: https://github.com/example/repo
    chart: my-chart
    targetRevision: 1.2.3
  destination:
    server: https://kubernetes.default.svc
    namespace: my-app
```

## Required values

| Path | Type | Description |
| --- | --- | --- |
| spec.name | string | ArgoCD Application name. |
| spec.namespace | string | Namespace where the Application CR is created. |
| spec.project | string | ArgoCD project name. |
| spec.source.repoURL | string | Git or Helm repository URL. |
| spec.source.chart | string | Helm chart name. |
| spec.source.targetRevision | string | Chart or Git revision. |
| spec.destination.server | string | Destination cluster API server URL. |
| spec.destination.namespace | string | Destination namespace. |

## Optional values

| Path | Type | Purpose |
| --- | --- | --- |
| spec.annotations | object | Annotations on the Application CR. |
| spec.labels | object | Labels on the Application CR. |
| spec.finalizers | array | Finalizers on the Application CR. |
| spec.source.helm.releaseName | string | Helm release name override. |
| spec.source.helm.valuesObject | object | Helm values object. |
| spec.syncPolicy | object | ArgoCD sync policy overrides. |# ArgoCD Application Promise

Creates a single ArgoCD Application from a structured spec.
