# Authentik SSO Platform

Authentik is deployed as a platform-wide identity provider (IdP) for SSO/OIDC authentication across all cluster services.

## Architecture

- **Deployment**: Control-plane addon via ArgoCD ApplicationSet
- **Database**: External PostgreSQL (credentials from 1Password)
- **Cache**: Redis (subchart, NFS-backed PVC)
- **Configuration**: Blueprints (native YAML) for flows, providers, and policies
- **Domain**: `auth.cluster.integratn.tech`
- **Ingress**: Gateway API HTTPRoutes via nginx-gateway-fabric (`auth.` for sign-in/OIDC, LAN-only `auth-admin.` for the admin interface)
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

Initial admin account (only used on an empty database):

- **password**: Admin password (strong password, 16+ chars)
- **email**: Admin email address

(The `token` field is no longer read.)

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

Blueprints are YAML files that declaratively configure Authentik. They live in
exactly one place — as the `data` keys of
[authentik-blueprints-configmap.yaml](../../../../clusters/the-cluster/addons/authentik/authentik-blueprints-configmap.yaml),
which `blueprints.configMaps` in [values.yaml](values.yaml) mounts. Edit them
there; a loose copy under this directory is applied by nothing.

- `00-admin-group.yaml`: Superuser admin group
- `01-argocd-provider.yaml`: OAuth2 provider for ArgoCD
- `02-grafana-provider.yaml`: OAuth2 provider for Grafana
- `03-authentication-flows.yaml`: own sign-in flows (password then required MFA; Google variant), the brand's authentication flow, and a deny-all on the stock `default-authentication-flow`
- `04-google-oauth-source.yaml`: Google sign-in for existing users only (no enrollment)
- `05-openwebui-provider.yaml`: OAuth2 provider for Open WebUI
- `06-qdrant-proxy-provider.yaml`: forward-auth proxy provider for the Qdrant dashboard
- `07-kargo-provider.yaml`: PKCE public client for Kargo
- `08*-…-proxy-provider.yaml`, `09-embedded-outpost.yaml`: forward-auth apps and the outpost's provider list
- App icons: every application's `meta_icon` points at `/static/dist/assets/images/apps/<slug>.<ext>`, from ConfigMap `authentik-app-icons` built by `hack/authentik-app-icons/build.sh`. A new app needs an icon there too.
- `10-hermes-provider.yaml`: PKCE public client for Hermes (authentik Admins only)
- `11-access-groups.yaml`: groups and the binding that opens each app to them
- `12-invitation-enrollment.yaml`: the invite-only enrollment flow
- `13-retire-bootstrap-token.yaml`: deletes the old bootstrap API token

Blueprints are auto-applied on startup. All provider configuration is code-driven; no manual UI configuration required.

## Users, groups and sign-up

**There is no sign-up.** Accounts are created only from invitations; Google
sign-in only links to an account that already exists (same email). Password
sign-in needs a second factor (TOTP or a passkey), enrolled on first sign-in;
Google sign-in relies on the Google account's own 2FA instead, so invitees who
use Google should have 2FA on their Google account.

**Password sign-in works only from the home network** (LAN or Tailscale).
Requests from anywhere else, or that arrive through Cloudflare (any `Cf-Ray`
header), stop after the username with "use Sign in with Google". So anyone who
needs access from outside must have a Google account linked (same email as
their authentik account).

### Groups

| Group | Opens |
|---|---|
| `authentik Admins` | authentik admin (superuser), Hermes, cq; admin role in ArgoCD, Grafana, Kargo, Open WebUI, Mealie |
| `homelab` | ArgoCD (admin), Kargo (admin), Grafana (Editor), Bosun, Specmarshal, Qdrant, Umami, the landing page, Open WebUI |
| `media-management` | Sonarr / Radarr / Bazarr / SABnzbd on vcluster-media, by forward-auth (`08d-media-proxy-providers.yaml`) |
| `media-viewer` | nothing bound yet: for Jellyfin once it uses OIDC |
| `open-webui-users` | Open WebUI only |
| `mealie-users` | Mealie only (`authentik-blueprints-mealie`) |
| `home-family` | everything `media-viewer`, `open-webui-users` and `mealie-users` open (bound to each app directly, not nested) |

Invited accounts start in `media-viewer`.

A user in no group can sign in to the portal and reach nothing. Apps see
only direct group memberships, so `home-family` is bound to each family app
itself; when you bind an app to `media-viewer`, `open-webui-users` or
`mealie-users`, bind it to `home-family` too. Every new
application must get a binding in `11-access-groups.yaml` (or its own
blueprint) -- an application with no binding is open to every account.

### Adding a user

1. Admin interface (`https://auth-admin.cluster.integratn.tech`) ->
   Directory -> Invitations -> Create. Flow `integratn-invitation`, single
   use, expires in a few days (check the date the form shows: left at its
   default it is already expired). Fixed data `{"email": "<their address>"}`.
2. Either:
   - **Google** (works away from home): tell them to open
     `https://auth.integratn.tech` (away from home) or
     `https://auth.cluster.integratn.tech` (on the LAN) and click **Google** (or
     **IntegratnIO Workspace** for an @integratn.io / @integratn.tech
     account) with the Google account whose email is in the invite. Their
     account is created with that email as the username, linked to Google,
     and they are signed in. A Google account with no matching invite is
     refused. Blueprint `14-google-invite-enrollment.yaml`.
   - **Password** (home network only): send the invitation link. The admin
     page builds it from the host you are on, so change `auth-admin.` to
     `auth.` first. They pick a username and password (14+ characters, not
     breached) and enrol TOTP or a passkey.
3. They start in `media-viewer`. Directory -> Groups -> add them to anything
   else they need (`home-family` for family, `homelab` to run the platform).

### Admin interface

`/if/admin` on `auth.cluster.integratn.tech` redirects to
`auth-admin.cluster.integratn.tech`, which only private (LAN/Tailscale)
addresses can open. Never publish `auth-admin` on a Cloudflare tunnel; only
`auth.` may ever be public.

### Retiring akadmin

The bootstrap `akadmin` is a shared, guessable name. Replace it with a named
admin, once, by hand:

1. Create an invitation (above) and enrol your own account through it, with
   MFA.
2. Add that account to `authentik Admins`, `homelab` and whatever else you use.
3. Sign out, sign in as the new account, confirm the admin interface and an
   app or two work.
4. Deactivate `akadmin` (Directory -> Users -> akadmin -> Edit -> uncheck
   Active), and clear its email so Google sign-in can never match it.
   **Do not** set it an unusable password or delete it: the `initial-setup`
   flow stays closed only while akadmin has a usable password.

`AUTHENTIK_BOOTSTRAP_PASSWORD`/`EMAIL` stay in values.yaml for the same
reason (an empty database must not come up with an open setup flow). The
bootstrap *token* is gone.

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
- MFA is required for password sign-in (`integratn-mfa` stage); Google sign-in trusts Google's 2FA
- Review audit logs regularly: **Events** → **Logs** in Authentik UI

## References

- [Authentik Documentation](https://goauthentik.io/docs/)
- [Authentik Helm Chart](https://github.com/goauthentik/helm)
- [Authentik Blueprints](https://goauthentik.io/docs/flow/blueprints/)
- [ArgoCD OIDC Configuration](https://argo-cd.readthedocs.io/en/stable/operator-manual/user-management/#existing-oidc-provider)
- [Grafana OAuth Configuration](https://grafana.com/docs/grafana/latest/setup-grafana/configure-security/configure-authentication/generic-oauth/)
