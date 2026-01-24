# Operations

## Principles
- Git is the source of truth.
- Avoid manual drift; use ArgoCD sync.
- Confirm before destructive actions.

## Core URLs
- ArgoCD: https://argocd.cluster.integratn.tech
- Grafana: https://grafana.cluster.integratn.tech

## Routine Workflows
### Addons change
1. Modify values under `addons/`.
2. Commit and push.
3. Sync the relevant ApplicationSet/Application.

### vcluster change
1. Modify a request under `platform/vclusters/`.
2. Commit and push.
3. Sync `platform-vclusters` and `kratix-state-reconciler` apps.

### Promise pipeline change
1. Modify `promises/<promise>/`.
2. Commit and push.
3. Wait for pipeline image build in GitHub Actions.
4. Refresh/sync `kratix-promises` and re‑sync the platform requests.

## Troubleshooting Checklist
1. Check ArgoCD application health and sync status.
2. Inspect the failing namespace for pod status and events.
3. Validate ExternalSecrets resolution (1Password connectivity).
4. Confirm Gateway HTTPRoutes and TLS cert status.
5. Verify storage class availability if pods are Pending.

## Observability Triage
- If metrics are missing: confirm vcluster agent remote‑write and ServiceMonitors.
- If logs are missing: confirm host promtail running and Loki gateway reachable.

## DNS/TLS
- Gateway uses wildcard cert for `*.cluster.integratn.tech`.
- HTTPRoutes bind hostnames to services.
