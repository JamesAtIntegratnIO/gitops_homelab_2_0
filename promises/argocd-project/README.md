# ArgoCD Project Promise

Creates an ArgoCD `AppProject` resource from a platform ResourceRequest.

## API

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `spec.name` | string | Yes | Name of the AppProject |
| `spec.namespace` | string | No | Target namespace (default: `argocd`) |
| `spec.description` | string | No | Project description |
| `spec.annotations` | map[string]string | No | Annotations on the AppProject |
| `spec.labels` | map[string]string | No | Labels on the AppProject |
| `spec.sourceRepos` | []string | Yes | Allowed source repositories |
| `spec.destinations` | []object | Yes | Allowed deployment destinations |
| `spec.clusterResourceWhitelist` | []object | No | Allowed cluster-scoped resources |
| `spec.namespaceResourceWhitelist` | []object | No | Allowed namespace-scoped resources |

## Example

```yaml
apiVersion: platform.integratn.tech/v1alpha1
kind: ArgoCDProject
metadata:
  name: my-project
  namespace: platform-requests
spec:
  name: my-project
  namespace: argocd
  description: "My application project"
  sourceRepos:
    - https://charts.loft.sh
  destinations:
    - server: https://kubernetes.default.svc
      namespace: my-namespace
  clusterResourceWhitelist:
    - group: "*"
      kind: "*"
```
