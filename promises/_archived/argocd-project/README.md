# ArgoCD Project Promise

Creates an ArgoCD AppProject from structured inputs.

## ResourceRequest Example

```yaml
apiVersion: platform.integratn.tech/v1alpha1
kind: ArgoCDProject
metadata:
  name: appteam-project
  namespace: platform-requests
spec:
  name: appteam
  namespace: argocd
  description: App team project
  sourceRepos:
    - https://github.com/example/*
  destinations:
    - server: https://kubernetes.default.svc
      namespace: appteam-*
```

## Required values

| Path | Type | Description |
| --- | --- | --- |
| spec.name | string | AppProject name. |
| spec.namespace | string | Namespace where the AppProject CR is created. |
| spec.sourceRepos | array | Allowed source repositories. |
| spec.destinations | array | Allowed destinations for Applications. |

## Optional values

| Path | Type | Purpose |
| --- | --- | --- |
| spec.description | string | Project description. |
| spec.annotations | object | Annotations on the AppProject CR. |
| spec.labels | object | Labels on the AppProject CR. |
| spec.clusterResourceWhitelist | array | Cluster-scoped resource whitelist. |
| spec.namespaceResourceWhitelist | array | Namespaced resource whitelist. |# ArgoCD Project Promise

Creates an ArgoCD AppProject from structured inputs.
