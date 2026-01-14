# HTTPRoute Pattern for App Teams

## Overview
This pattern allows app teams to own their HTTPRoutes without modifying the shared nginx-gateway infrastructure.

## How It Works

### Gateway (Owned by Network Team)
- Located in `nginx-gateway` namespace
- Managed by the network team
- Has a `ReferenceGrant` that allows HTTPRoutes from any namespace to reference it

### HTTPRoutes (Owned by App Teams)
- Located in each app's namespace (e.g., `argocd`, `myapp`)
- Deployed alongside the app as part of its addon
- References the shared gateway using cross-namespace references

## Creating Routes for Your App

### 1. Create your HTTPRoute manifests
Add an `httproutes.yaml` file in your app's addon directory:

```yaml
# addons/clusters/the-cluster/addons/myapp/httproutes.yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: myapp
  namespace: myapp
spec:
  parentRefs:
    - name: nginx-gateway
      namespace: nginx-gateway
      sectionName: https  # or 'http' for non-TLS
  hostnames:
    - myapp.cluster.integratn.tech
  rules:
    - matches:
        - path:
            type: PathPrefix
            value: /
      backendRefs:
        - name: myapp-service
          port: 80
```

### 2. Update addons.yaml
Configure your addon to deploy the HTTPRoute manifests:

```yaml
# addons/clusters/the-cluster/addons.yaml
myapp:
  additionalResources:
    type: manifests
    path: clusters
    manifestPath: addons/myapp
  ignoreDifferences:
    - group: gateway.networking.k8s.io
      kind: HTTPRoute
      jqPathExpressions:
        - .status
```

### 3. Optional: HTTP to HTTPS Redirect
Add a redirect HTTPRoute for automatic HTTPS upgrades:

```yaml
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: myapp-http-redirect
  namespace: myapp
spec:
  parentRefs:
    - name: nginx-gateway
      namespace: nginx-gateway
      sectionName: http
  hostnames:
    - myapp.cluster.integratn.tech
  rules:
    - filters:
        - type: RequestRedirect
          requestRedirect:
            scheme: https
            statusCode: 301
```

## Benefits
- **Separation of Concerns**: Network team manages gateway, app teams manage routes
- **Namespace Isolation**: Each app's routes live in their own namespace
- **GitOps Friendly**: Routes are version controlled with the app
- **No Gateway Modifications**: Adding new apps doesn't require changes to gateway config
- **Team Autonomy**: App teams can deploy and update routes independently

## Example: ArgoCD
See `addons/clusters/the-cluster/addons/argo-cd/httproutes.yaml` for a complete example.
