# HTTP Service Promise

A **product Promise** that gives developers a one-CR experience to deploy an HTTP service with all platform wiring included: namespace, deployment, service, HTTPRoute (ingress + TLS + DNS), ExternalSecrets (1Password), NetworkPolicy, and optional Prometheus monitoring.

Under the hood it creates an **ArgoCD Application** pointing at the [Stakater `application`](https://github.com/stakater/application) Helm chart, so ArgoCD manages the full lifecycle.

## What You Get

From a single `HTTPService` CR, the pipeline generates:

| Resource | Source | Purpose |
|----------|--------|---------|
| **Namespace** | Pipeline output (sync-wave 0) | Isolated namespace for the app |
| **Deployment** | Stakater chart | Container with probes, resources, security context |
| **Service** | Stakater chart | ClusterIP service |
| **HTTPRoute** | Stakater chart | Gateway API route → `nginx-gateway` → TLS via cert-manager |
| **ServiceAccount** | Stakater chart | Dedicated SA for the workload |
| **ExternalSecret(s)** | Pipeline output | 1Password → K8s Secrets via `onepassword-connect` ClusterSecretStore |
| **NetworkPolicy** | Pipeline output | Default-deny + allow-gateway + allow-monitoring + allow-DNS |
| **ServiceMonitor** | Stakater chart (opt-in) | Prometheus scrape target |

## Quick Start

### Minimal — just a name and an image

```yaml
apiVersion: platform.integratn.tech/v1alpha1
kind: HTTPService
metadata:
  name: hello-world
  namespace: platform-requests
spec:
  name: hello-world
  image:
    repository: docker.io/nginxinc/nginx-unprivileged
    tag: latest
  port: 8080
```

This creates a deployment at `https://hello-world.cluster.integratn.tech` with sensible defaults (1 replica, 100m/128Mi requests, health checks on `/`, default-deny NetworkPolicy).

### With Security Hardening

```yaml
apiVersion: platform.integratn.tech/v1alpha1
kind: HTTPService
metadata:
  name: my-api
  namespace: platform-requests
spec:
  name: my-api
  image:
    repository: ghcr.io/my-org/my-api
    tag: v1.2.3
  securityContext:
    runAsNonRoot: true
    readOnlyRootFilesystem: true
    runAsUser: 65532
```

### Full-Featured

All available fields including secrets, monitoring, persistence, env vars, and `helmOverrides` escape hatch are documented in the API Reference below.

## API Reference

### Required Fields

| Field | Type | Description |
|-------|------|-------------|
| `spec.name` | string | App name (used for namespace, deployment, route) |
| `spec.image.repository` | string | Container image repository |

### Optional Fields

| Field | Default | Description |
|-------|---------|-------------|
| `spec.namespace` | `{name}` | Target namespace |
| `spec.team` | `platform` | Owning team (label) |
| `spec.image.tag` | `latest` | Image tag |
| `spec.image.pullPolicy` | `IfNotPresent` | Pull policy |
| `spec.replicas` | `1` | Replica count (1–10) |
| `spec.resources.requests.cpu` | `100m` | CPU request |
| `spec.resources.requests.memory` | `128Mi` | Memory request |
| `spec.resources.limits.cpu` | `500m` | CPU limit |
| `spec.resources.limits.memory` | `256Mi` | Memory limit |
| `spec.port` | `8080` | Container port |
| `spec.ingress.enabled` | `true` | Create HTTPRoute |
| `spec.ingress.hostname` | `{name}.cluster.integratn.tech` | FQDN |
| `spec.ingress.path` | `/` | URL path prefix |
| `spec.secrets` | `[]` | ExternalSecrets (1Password) |
| `spec.env` | `{}` | Plain env vars |
| `spec.healthCheck.path` | `/` | Probe path |
| `spec.healthCheck.port` | `{port}` | Probe port |
| `spec.monitoring.enabled` | `false` | Create ServiceMonitor |
| `spec.monitoring.path` | `/metrics` | Metrics path |
| `spec.monitoring.interval` | `30s` | Scrape interval |
| `spec.persistence.enabled` | `false` | Create PVC |
| `spec.persistence.size` | `1Gi` | Volume size |
| `spec.persistence.mountPath` | `/data` | Mount path |
| `spec.securityContext.runAsNonRoot` | `false` | Require non-root user |
| `spec.securityContext.readOnlyRootFilesystem` | `false` | Mount root fs read-only |
| `spec.securityContext.runAsUser` | _(unset)_ | Run as specific UID |
| `spec.securityContext.runAsGroup` | _(unset)_ | Run as specific GID |
| `spec.helmOverrides` | `{}` | Raw Stakater chart values (escape hatch) |

### Secrets Example

```yaml
spec:
  secrets:
    - name: my-app-db            # K8s Secret name (optional, auto-generated)
      onePasswordItem: my-db     # 1Password vault item name
      keys:
        - secretKey: DB_PASSWORD # Key in K8s Secret
          property: password     # Property in 1Password item
```

The pipeline generates an `ExternalSecret` backed by the `onepassword-connect` ClusterSecretStore. **No `kind: Secret` is ever written to git.**

## Security & Image Compatibility

### Cluster Security Policies

This cluster enforces security baselines via **Kyverno mutating policies** that are applied to all pods:

| Policy | Effect |
|--------|--------|
| `mutate-restrict-escalation` | Injects `allowPrivilegeEscalation: false` and `capabilities.drop: ["ALL"]` |
| `mutate-seccomp-default` | Injects `seccompProfile.type: RuntimeDefault` |
| `restrict-image-registries` | Warns (audit) on images from unapproved registries |

Because **all Linux capabilities are dropped**, images that require capabilities like `CHOWN`, `SETUID`, `SETGID`, or `NET_BIND_SERVICE` will fail at runtime.

### Choosing a Compatible Image

**Recommended:** Use images designed to run as non-root without special capabilities:

| Image | Works | Notes |
|-------|-------|-------|
| `docker.io/nginxinc/nginx-unprivileged` | ✅ | Runs as UID 101, listens on 8080 |
| `ghcr.io/your-org/your-app` | ✅ | Purpose-built images (best practice) |
| `docker.io/nginxdemos/hello` | ❌ | Runs as root, needs `CHOWN` capability |
| `docker.io/library/nginx` | ❌ | Runs as root, binds port 80, needs capabilities |

**Rule of thumb:** If an image runs as root or calls `chown`/`setuid` in its entrypoint, it won't work without adding capabilities back.

### Adding Capabilities (Escape Hatch)

If you must run an image that requires specific capabilities, use `helmOverrides`:

```yaml
spec:
  helmOverrides:
    deployment:
      containerSecurityContext:
        capabilities:
          add:
            - CHOWN
            - SETUID
            - SETGID
          drop:
            - ALL
```

> **Note:** The Kyverno `mutate-restrict-escalation` policy will still inject `allowPrivilegeEscalation: false` and `capabilities.drop: ALL`. Your `helmOverrides` capabilities are applied at the chart level, but the Kyverno mutation happens after admission. To truly override, consider a Kyverno policy exception for the namespace.

### Security Context Defaults

The pipeline defaults are **permissive** (`runAsNonRoot: false`, `readOnlyRootFilesystem: false`) to ensure standard Docker Hub images work out-of-the-box. The Stakater application chart itself defaults to hardened settings, but the pipeline explicitly overrides those chart defaults.

For production workloads, explicitly set `securityContext` in your CR:

```yaml
spec:
  securityContext:
    runAsNonRoot: true
    readOnlyRootFilesystem: true
    runAsUser: 65532
```

## Platform Conventions

- **Gateway**: Routes to `nginx-gateway` in namespace `nginx-gateway`
- **TLS**: Handled by cert-manager wildcard via the gateway
- **Secrets**: `ClusterSecretStore: onepassword-connect`
- **Monitoring**: ServiceMonitor label `release: kube-prometheus-stack`
- **Network**: Default-deny ingress + explicit allow from gateway and monitoring namespaces
- **GitOps**: ArgoCD auto-sync with self-heal and prune enabled

## Building the Pipeline Image

```bash
cd workflows/resource/configure

# Build locally
docker build -t ghcr.io/jamesatintegratnio/http-service-configure:latest .

# Push
docker push ghcr.io/jamesatintegratnio/http-service-configure:latest
```

## Deploying the Promise

```bash
kubectl apply -f promise.yaml
```

Then create an HTTPService:

```bash
kubectl apply -f examples/minimal.yaml
```

## Architecture

```
Developer CR (HTTPService)
    │
    ▼
Kratix Pipeline (Go binary)
    │
    ├─► ArgoCD Application (Stakater application chart)
    │       │
    │       ├─► Namespace
    │       ├─► Deployment + Service + ServiceAccount
    │       ├─► HTTPRoute (Gateway API)
    │       └─► ServiceMonitor (optional)
    │
    ├─► ExternalSecret(s) → 1Password
    │
    └─► NetworkPolicy (default-deny + allow-gateway + allow-dns)
```
