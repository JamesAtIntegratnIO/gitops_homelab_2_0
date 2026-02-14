# Authentik SSO Platform

Authentik is deployed as a platform-wide identity provider (IdP) for SSO/OIDC authentication across all cluster services.

## Architecture

- **Deployment**: Control-plane addon via ArgoCD ApplicationSet
- **Database**: External PostgreSQL (credentials from 1Password)
- **Cache**: Redis (subchart, NFS-backed PVC)
- **Configuration**: Blueprints (native YAML) for flows, providers, and policies
- **Domain**: `auth.cluster.integratn.tech`
- **Ingress**: Gateway API HTTPRoute via nginx-gateway-fabric
- **Monitoring**: Prometheus ServiceMonitor + Grafana dashboard

## Required 1Password Items

Before deploying Authentik, create the following items in your 1Password vault:

### 1. `authentik-main` (Secure Note or Password)

Core Authentik secrets:

- **secret-key**: Django SECRET_KEY (50+ character random string)
  ```bash
  # Generate with:
  openssl rand -base64 60 | tr -d '\n'
  ```
- **postgres-host**: PostgreSQL hostname (e.g., `postgres.example.com`)
- **postgres-port**: PostgreSQL port (default: `5432`)
- **postgres-database**: Database name (e.g., `authentik`)
- **postgres-user**: Database username
- **postgres-password**: Database password

### 2. `authentik-bootstrap-admin` (Password)

Initial admin account:

- **username**: Admin username (e.g., `admin`)
- **password**: Admin password (strong password, 16+ chars)
- **email**: Admin email address

### 3. `authentik-argocd-oidc` (Password)

OAuth2/OIDC credentials for ArgoCD integration:

- **client-id**: OAuth client ID (e.g., `argocd`)
- **client-secret**: OAuth client secret (random 32+ char string)
  ```bash
  # Generate with:
  openssl rand -base64 32
  ```

### 4. `authentik-grafana-oidc` (Password)

OAuth2/OIDC credentials for Grafana integration:

- **client-id**: OAuth client ID (e.g., `grafana`)
- **client-secret**: OAuth client secret (random 32+ char string)

## Authentik Configuration (Blueprints)

Blueprints are YAML files that declaratively configure Authentik:

- `00-admin-group.yaml`: Superuser admin group
- `01-argocd-provider.yaml`: OAuth2 provider for ArgoCD
- `02-grafana-provider.yaml`: OAuth2 provider for Grafana
- `03-default-authentication-flow.yaml`: Standard login flow

Blueprints are mounted as ConfigMap and auto-applied on startup. All provider configuration is code-driven; no manual UI configuration required.

## Service Integration

### ArgoCD

OIDC configuration in `argo-cd/values.yaml`:

```yaml
configs:
  cm:
    oidc.config: |
      name: Authentik
      issuer: https://auth.cluster.integratn.tech/application/o/argocd/
      clientID: $argocd-oidc-config:client-id
      clientSecret: $argocd-oidc-config:client-secret
      requestedScopes: ["openid", "profile", "email", "groups"]
  rbac:
    policy.csv: |
      g, authentik Admins, role:admin
```

### Grafana

OAuth2 configuration in `kube-prometheus-stack/values.yaml`:

```yaml
grafana:
  auth.generic_oauth:
    enabled: true
    name: Authentik
    client_id: $grafana-oidc-config:client-id
    client_secret: $grafana-oidc-config:client-secret
    scopes: openid profile email groups
    auth_url: https://auth.cluster.integratn.tech/application/o/grafana/authorize
    token_url: https://auth.cluster.integratn.tech/application/o/grafana/token
    api_url: https://auth.cluster.integratn.tech/application/o/grafana/userinfo
    role_attribute_path: contains(groups[*], 'authentik Admins') && 'Admin' || 'Viewer'
```

## Deployment Steps

1. **Create 1Password items** (see above)
2. **Enable addon** in `addons/clusters/the-cluster/addons.yaml`:
   ```yaml
   authentik:
     enabled: true
   ```
3. **Commit and push** - ArgoCD will sync automatically
4. **Wait for sync** - Check `kubectl get pods -n authentik`
5. **Access UI** - Navigate to https://auth.cluster.integratn.tech
6. **Login** - Use bootstrap admin credentials from 1Password
7. **Verify blueprints** - Check Applications → Applications → see ArgoCD and Grafana providers
8. **Test SSO** - Login to ArgoCD/Grafana via Authentik

## Troubleshooting

### Pods not starting

```bash
kubectl get pods -n authentik
kubectl logs -n authentik -l app.kubernetes.io/name=authentik-server
kubectl logs -n authentik -l app.kubernetes.io/name=authentik-worker
```

### Database connection issues

```bash
kubectl get externalsecret -n authentik authentik-secrets
kubectl describe externalsecret -n authentik authentik-secrets
kubectl get secret -n authentik authentik-secrets -o yaml
```

### HTTPRoute not working

```bash
kubectl get httproute -n authentik
kubectl describe httproute -n authentik authentik
kubectl get gateway -n nginx-gateway nginx-gateway
```

### Blueprints not applying

```bash
kubectl logs -n authentik -l app.kubernetes.io/name=authentik-worker | grep -i blueprint
kubectl exec -n authentik deploy/authentik-worker -- ak list_blueprints
```

## Monitoring

Prometheus metrics exposed on port 9300:

- `authentik_system_tasks_*`: Background task execution
- `authentik_models_*`: Database object counts
- `django_*`: Django application metrics

Grafana dashboard: **Authentik Dashboard** (ID 14837)

## Security Considerations

- All secrets via ExternalSecret → 1Password (never in git)
- Repository is PUBLIC - no `kind: Secret` resources allowed
- Pre-commit hooks validate no secrets in promise directories
- Database credentials rotate in 1Password, ExternalSecret syncs automatically
- Bootstrap admin credentials should be rotated after initial setup
- Consider enabling MFA for all admin users
- Review audit logs regularly: **Events** → **Logs** in Authentik UI

## References

- [Authentik Documentation](https://goauthentik.io/docs/)
- [Authentik Helm Chart](https://github.com/goauthentik/helm)
- [Authentik Blueprints](https://goauthentik.io/docs/flow/blueprints/)
- [ArgoCD OIDC Configuration](https://argo-cd.readthedocs.io/en/stable/operator-manual/user-management/#existing-oidc-provider)
- [Grafana OAuth Configuration](https://grafana.com/docs/grafana/latest/setup-grafana/configure-security/configure-authentication/generic-oauth/)
