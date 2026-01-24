# Kratix Promises

> **Official Documentation References:**
> - [Kratix Documentation](https://kratix.io/docs/) - Promise-based platform engineering
> - [Kratix Promise Structure](https://kratix.io/docs/main/reference/promises/intro) - Promise anatomy
> - [Kratix Pipelines](https://kratix.io/docs/main/reference/pipelines/intro) - Resource transformation
> - [Kratix Destinations](https://kratix.io/docs/main/reference/destinations/intro) - Multi-cluster scheduling
> - [GitStateStore](https://kratix.io/docs/main/reference/statestore/gitstatestore) - Git-based state management

## Overview

Kratix Promises define **platform APIs** that abstract complex infrastructure into simple, developer-friendly interfaces. Each Promise consists of:

1. **API Schema** (CustomResourceDefinition) - What developers request
2. **Pipelines** (Configure/Delete) - How the platform fulfills requests
3. **Dependencies** - What the Promise requires to function
4. **Destination Selectors** - Where resources get deployed

**Why Kratix:**
- ✅ **Self-service infrastructure**: Developers request without knowing underlying complexity
- ✅ **Consistency**: Same interface produces identical results every time
- ✅ **Auditability**: All requests are Kubernetes resources (version controlled)
- ✅ **Multi-cluster ready**: Destination selectors route resources to appropriate clusters
- ✅ **GitOps native**: Pipeline outputs go to Git → ArgoCD applies them

**Architecture Flow:**
```
Developer → ResourceRequest (CRD) → Kratix Promise Pipeline → GitStateStore → ArgoCD → Kubernetes
```

## Architecture Diagram

```
┌────────────────────────────────────────────────────────────────────┐
│  Git Repo: gitops_homelab_2_0                                      │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │ platform/vclusters/media.yaml                                │  │
│  │                                                               │  │
│  │ apiVersion: platform.integratn.tech/v1alpha1                 │  │
│  │ kind: VClusterOrchestrator                                   │  │
│  │ spec:                                                         │  │
│  │   name: media                                                │  │
│  │   targetNamespace: vcluster-media                            │  │
│  │   vcluster:                                                  │  │
│  │     preset: prod                                             │  │
│  │     replicas: 3                                              │  │
│  └──────────────────────────────────────────────────────────────┘  │
└───────────────────────┬────────────────────────────────────────────┘
                        │ ArgoCD syncs
                        ▼
┌────────────────────────────────────────────────────────────────────┐
│  Kubernetes: the-cluster (platform-requests namespace)             │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │ VClusterOrchestrator: media                                  │  │
│  │ (CustomResource created by ArgoCD)                           │  │
│  └────────────────────┬─────────────────────────────────────────┘  │
│                       │ Kratix controller detects new resource
│                       ▼
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │ Promise: vcluster-orchestrator                               │  │
│  │   workflows.resource.configure:                              │  │
│  │     - Pipeline: vcluster-orchestrator-configure              │  │
│  │       container: ghcr.io/.../vcluster-orchestrator:main      │  │
│  └────────────────────┬─────────────────────────────────────────┘  │
│                       │ Kratix schedules pipeline pod
│                       ▼
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │ Pod: media-configure-xyz (runs once)                         │  │
│  │   - Reads /kratix/input/object.yaml (ResourceRequest)        │  │
│  │   - Executes configure-pipeline.sh                           │  │
│  │   - Renders 6 sub-ResourceRequests:                          │  │
│  │       * VClusterCore                                         │  │
│  │       * VClusterCoreDNS                                      │  │
│  │       * VClusterKubeconfigSync                               │  │
│  │       * VClusterKubeconfigExternalSecret                     │  │
│  │       * VClusterArgocdClusterRegistration                    │  │
│  │       * ArgocdApplication                                    │  │
│  │   - Writes YAML to /kratix/output/                           │  │
│  └────────────────────┬─────────────────────────────────────────┘  │
│                       │ Kratix collects outputs
│                       ▼
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │ GitStateStore Controller                                     │  │
│  │   - Commits outputs to kratix-platform-state repo            │  │
│  │   - Path: clusters/the-cluster/media/*.yaml                  │  │
│  └────────────────────┬─────────────────────────────────────────┘  │
└────────────────────────┼─────────────────────────────────────────┘
                         │ Git push
                         ▼
┌────────────────────────────────────────────────────────────────────┐
│  Git Repo: kratix-platform-state (separate repo)                   │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │ clusters/the-cluster/media/                                  │  │
│  │   - vclustercore-media.yaml                                  │  │
│  │   - vclustercoredns-media.yaml                               │  │
│  │   - vclusterkubeconfigsync-media.yaml                        │  │
│  │   - vclusterkubeconfigexternalsecret-media.yaml              │  │
│  │   - vclusterargocdclusterregistration-media.yaml             │  │
│  │   - argocdapplication-media.yaml                             │  │
│  └──────────────────────────────────────────────────────────────┘  │
└───────────────────────┬────────────────────────────────────────────┘
                        │ ArgoCD kratix-state-reconciler app syncs
                        ▼
┌────────────────────────────────────────────────────────────────────┐
│  Kubernetes: the-cluster (platform-requests namespace)             │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │ Sub-ResourceRequests created (6 new CRs)                     │  │
│  └────────────────────┬─────────────────────────────────────────┘  │
│                       │ Each triggers its own Promise pipeline
│                       ▼
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │ VClusterCore Pipeline                                        │  │
│  │   - Renders namespace: vcluster-media                        │  │
│  │   - Renders ConfigMap: media-vcluster-values                 │  │
│  │   - Outputs to GitStateStore                                 │  │
│  └──────────────────────────────────────────────────────────────┘  │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │ ArgocdApplication Pipeline                                   │  │
│  │   - Renders Application: media                               │  │
│  │   - References ConfigMap for Helm values                     │  │
│  │   - Outputs to GitStateStore                                 │  │
│  └──────────────────────────────────────────────────────────────┘  │
│  ... (other sub-promise pipelines execute similarly)               │
└────────────────────────────────────────────────────────────────────┘
                        │ GitStateStore commits final resources
                        ▼
┌────────────────────────────────────────────────────────────────────┐
│  Git Repo: kratix-platform-state                                    │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │ clusters/the-cluster/media-vclustercore/                     │  │
│  │   - namespace-vcluster-media.yaml                            │  │
│  │   - configmap-media-vcluster-values.yaml                     │  │
│  │                                                               │  │
│  │ clusters/the-cluster/media-argocdapplication/                │  │
│  │   - application-media.yaml                                   │  │
│  └──────────────────────────────────────────────────────────────┘  │
└───────────────────────┬────────────────────────────────────────────┘
                        │ ArgoCD kratix-state-reconciler syncs
                        ▼
┌────────────────────────────────────────────────────────────────────┐
│  Kubernetes: the-cluster                                            │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │ Namespace: vcluster-media (created)                          │  │
│  │ ConfigMap: media-vcluster-values (created)                   │  │
│  │ Application: media (created in argocd namespace)             │  │
│  └────────────────────┬─────────────────────────────────────────┘  │
│                       │ ArgoCD syncs Application
│                       ▼
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │ Helm Release: vcluster (deployed in vcluster-media ns)       │  │
│  │   - StatefulSet: media (vcluster control plane)              │  │
│  │   - Service: media (vcluster API endpoint)                   │  │
│  │   - etcd StatefulSet (3 replicas for HA)                     │  │
│  └──────────────────────────────────────────────────────────────┘  │
└────────────────────────────────────────────────────────────────────┘
```

## Promise Inventory

### vCluster Orchestration Promises

#### vcluster-orchestrator
**Purpose:** Top-level Promise that orchestrates creation of a complete vCluster environment by rendering sub-ResourceRequests for all component Promises.

**Location:** [promises/vcluster-orchestrator/](../promises/vcluster-orchestrator/)

**ResourceRequest:** `VClusterOrchestrator`

**What It Does:**
1. Accepts high-level vCluster specification
2. Validates inputs and applies defaults
3. Renders 6 sub-ResourceRequests (one per component Promise)
4. Each sub-request triggers its own pipeline

**Key Fields:**
```yaml
spec:
  name: media                           # vCluster name
  targetNamespace: vcluster-media       # Host namespace
  projectName: vcluster-media           # ArgoCD project
  vcluster:
    preset: prod | dev                  # Resource sizing
    replicas: 3                         # Control plane HA
    k8sVersion: "v1.34.3"               # Kubernetes version
    backingStore:
      etcd:
        deploy:
          enabled: true                 # Embedded etcd
          statefulSet:
            highAvailability:
              replicas: 3               # etcd HA
  exposure:
    hostname: media.integratn.tech      # External access
    apiPort: 443                        # API port
  integrations:
    certManager:
      clusterIssuerSelectorLabels:      # TLS cert source
        integratn.tech/cluster-issuer: letsencrypt-prod
    externalSecrets:
      clusterStoreSelectorLabels:       # 1Password integration
        integratn.tech/cluster-secret-store: onepassword-store
    argocd:
      environment: production           # Environment tagging
      clusterLabels:                    # Custom cluster labels
        team: platform
      workloadRepo:                     # Optional workload sync
        repoURL: https://github.com/...
        path: apps/
  argocdApplication:
    repoURL: https://charts.loft.sh     # vCluster Helm chart
    chart: vcluster
    targetRevision: 0.30.4
```

**Pipeline Logic:**
1. Read VClusterOrchestrator resource
2. Apply defaults (k8sVersion, resources, etc.)
3. Generate unique IDs for sub-resources
4. Render 6 sub-ResourceRequests with templated values
5. Write to `/kratix/output/`

**Example Sub-ResourceRequest (VClusterCore):**
```yaml
apiVersion: platform.integratn.tech/v1alpha1
kind: VClusterCore
metadata:
  name: media
  namespace: platform-requests
spec:
  name: media
  targetNamespace: vcluster-media
  valuesYaml: |
    controlPlane:
      distro:
        k8s:
          enabled: true
          version: "v1.34.3"
      backingStore:
        etcd:
          deploy:
            enabled: true
            statefulSet:
              highAvailability:
                replicas: 3
```

#### vcluster-core
**Purpose:** Creates namespace and ConfigMap with vCluster Helm values.

**Location:** [promises/vcluster-core/](../promises/vcluster-core/)

**ResourceRequest:** `VClusterCore`

**Pipeline Actions:**
1. Render Namespace with PodSecurity labels:
   ```yaml
   apiVersion: v1
   kind: Namespace
   metadata:
     name: vcluster-media
     labels:
       app: vcluster
       instance: media
       pod-security.kubernetes.io/enforce: privileged  # vCluster needs hostPath
       pod-security.kubernetes.io/audit: privileged
       pod-security.kubernetes.io/warn: privileged
   ```

2. Render ConfigMap with Helm values:
   ```yaml
   apiVersion: v1
   kind: ConfigMap
   metadata:
     name: media-vcluster-values
     namespace: vcluster-media
   data:
     values.yaml: |
       <valuesYaml from spec>
   ```

**Why ConfigMap:**
- Decouples values from Application definition
- Allows updates without redeploying Application
- Single source of truth for vCluster configuration

#### vcluster-coredns
**Purpose:** Configures host cluster CoreDNS to resolve vCluster service DNS names.

**Location:** [promises/vcluster-coredns/](../promises/vcluster-coredns/)

**ResourceRequest:** `VClusterCoreDNS`

**Pipeline Actions:**
Patches CoreDNS ConfigMap in `kube-system` namespace:
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: coredns-custom
  namespace: kube-system
data:
  media.server: |
    media.svc.cluster.local:53 {
        errors
        cache 30
        forward . 10.43.0.10  # vCluster CoreDNS service IP
    }
```

**Why This Matters:**
- Pods in host cluster can resolve `myservice.media.svc.cluster.local`
- Enables cross-cluster service communication
- Required for Gateway API routes to vCluster services

#### vcluster-kubeconfig-sync
**Purpose:** Syncs vCluster kubeconfig to 1Password for secure storage and sharing.

**Location:** [promises/vcluster-kubeconfig-sync/](../promises/vcluster-kubeconfig-sync/)

**ResourceRequest:** `VClusterKubeconfigSync`

**Pipeline Actions:**
1. Creates CronJob that runs every 5 minutes:
   ```yaml
   apiVersion: batch/v1
   kind: CronJob
   metadata:
     name: media-kubeconfig-sync
     namespace: vcluster-media
   spec:
     schedule: "*/5 * * * *"
     jobTemplate:
       spec:
         template:
           spec:
             containers:
               - name: sync
                 image: vcluster:latest
                 command: ["/bin/sh"]
                 args:
                   - -c
                   - |
                     vcluster connect media --namespace vcluster-media --print-kubeconfig > /tmp/kubeconfig
                     op item create --vault homelab --category login \
                       --title "vcluster-media-kubeconfig" \
                       "kubeconfig=$(cat /tmp/kubeconfig)"
   ```

2. Kubeconfig stored in 1Password vault: `homelab`
3. Item name: `vcluster-media-kubeconfig`

**Security:**
- No kubeconfig stored in Git (public repo)
- 1Password Connect provides audited access
- CronJob ensures kubeconfig stays up-to-date after rotations

#### vcluster-kubeconfig-external-secret
**Purpose:** Exposes vCluster kubeconfig from 1Password as Kubernetes Secret.

**Location:** [promises/vcluster-kubeconfig-external-secret/](../promises/vcluster-kubeconfig-external-secret/)

**ResourceRequest:** `VClusterKubeconfigExternalSecret`

**Pipeline Actions:**
```yaml
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: media-kubeconfig
  namespace: vcluster-media
spec:
  secretStoreRef:
    name: onepassword-connect
    kind: ClusterSecretStore
  target:
    name: media-kubeconfig
  data:
    - secretKey: kubeconfig
      remoteRef:
        key: vcluster-media-kubeconfig
        property: kubeconfig
```

**Usage:**
- ArgoCD reads this Secret to connect to vCluster
- Developers use: `kubectl get secret media-kubeconfig -n vcluster-media -o jsonpath='{.data.kubeconfig}' | base64 -d > ~/.kube/media`

#### vcluster-argocd-cluster-registration
**Purpose:** Registers vCluster as an ArgoCD destination cluster.

**Location:** [promises/vcluster-argocd-cluster-registration/](../promises/vcluster-argocd-cluster-registration/)

**ResourceRequest:** `VClusterArgocdClusterRegistration`

**Pipeline Actions:**
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: media-cluster
  namespace: argocd
  labels:
    argocd.argoproj.io/secret-type: cluster
    cluster_name: media
    cluster_role: vcluster
    environment: production
    team: platform  # From VClusterOrchestrator.spec.integrations.argocd.clusterLabels
type: Opaque
data:
  name: bWVkaWE=  # base64("media")
  server: aHR0cHM6Ly9tZWRpYS52Y2x1c3Rlci1tZWRpYS5zdmMuY2x1c3Rlci5sb2NhbA==  # vCluster service URL
  config: <base64-encoded kubeconfig>
```

**Label Significance:**
- `argocd.argoproj.io/secret-type: cluster` - ArgoCD recognizes as cluster
- `cluster_role: vcluster` - ApplicationSet selectors target vclusters
- `environment: production` - Environment-based ApplicationSet routing
- Custom labels (team, etc.) - User-defined selectors

### ArgoCD Helper Promises

#### argocd-project
**Purpose:** Creates ArgoCD AppProject from structured inputs.

**Location:** [promises/argocd-project/](../promises/argocd-project/)

**ResourceRequest:** `ArgocdProject`

**Pipeline Actions:**
```yaml
apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: vcluster-media
  namespace: argocd
spec:
  description: "vCluster media platform project"
  sourceRepos:
    - https://charts.loft.sh
    - https://github.com/jamesatintegratnio/gitops_homelab_2_0
  destinations:
    - namespace: vcluster-media
      server: https://kubernetes.default.svc
  clusterResourceWhitelist:
    - group: '*'
      kind: '*'
```

**Benefits:**
- RBAC isolation (projects control permissions)
- Source repo whitelisting
- Destination namespace restrictions

#### argocd-application
**Purpose:** Creates ArgoCD Application from structured inputs with values reference.

**Location:** [promises/argocd-application/](../promises/argocd-application/)

**ResourceRequest:** `ArgocdApplication`

**Pipeline Actions:**
```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: media
  namespace: argocd
spec:
  project: vcluster-media
  sources:
    - repoURL: https://charts.loft.sh
      chart: vcluster
      targetRevision: 0.30.4
      helm:
        valueFiles:
          - $values/values.yaml  # Reference to second source
    - repoURL: https://kubernetes.default.svc  # ConfigMap hack
      targetRevision: HEAD
      ref: values
      path: vcluster-media  # Namespace where ConfigMap lives
  destination:
    server: https://kubernetes.default.svc
    namespace: vcluster-media
  syncPolicy:
    automated:
      selfHeal: true
      prune: true
```

**ConfigMap Values Pattern:**
- ArgoCD can't directly reference ConfigMaps as value sources
- Workaround: Use `$values/` reference with dummy git source
- Actual values come from `media-vcluster-values` ConfigMap

## Promise Development Workflow

### Creating a New Promise

**Step 1: Define the API (CRD)**

Create [promises/my-promise/promise.yaml](../promises/my-promise/promise.yaml):
```yaml
apiVersion: platform.kratix.io/v1alpha1
kind: Promise
metadata:
  name: my-promise
  labels:
    kratix.io/promise-type: platform
spec:
  destinationSelectors:
    - matchLabels:
        capability.my-promise: "true"  # Only clusters with this label

  api:
    apiVersion: apiextensions.k8s.io/v1
    kind: CustomResourceDefinition
    metadata:
      name: mypromises.platform.integratn.tech
    spec:
      group: platform.integratn.tech
      scope: Namespaced
      names:
        plural: mypromises
        singular: mypromise
        kind: MyPromise
      versions:
        - name: v1alpha1
          served: true
          storage: true
          schema:
            openAPIV3Schema:
              type: object
              properties:
                spec:
                  type: object
                  required:
                    - name
                  properties:
                    name:
                      type: string
                      description: Resource name
                      pattern: '^[a-z0-9]([-a-z0-9]*[a-z0-9])?$'
                    size:
                      type: string
                      description: Resource size
                      default: small
                      enum:
                        - small
                        - medium
                        - large

  workflows:
    resource:
      configure:
        - apiVersion: platform.kratix.io/v1alpha1
          kind: Pipeline
          metadata:
            name: my-promise-configure
            namespace: platform-requests
          spec:
            containers:
              - name: render
                image: ghcr.io/your-org/my-promise-pipeline:main
                imagePullPolicy: Always
      delete:
        - apiVersion: platform.kratix.io/v1alpha1
          kind: Pipeline
          metadata:
            name: my-promise-delete
            namespace: platform-requests
          spec:
            containers:
              - name: delete
                image: ghcr.io/your-org/my-promise-pipeline:main
                imagePullPolicy: Always
                command: ["/scripts/delete-pipeline.sh"]
```

**Step 2: Create Pipeline Scripts**

Create [promises/my-promise/pipelines/configure-pipeline.sh](../promises/my-promise/pipelines/configure-pipeline.sh):
```bash
#!/usr/bin/env bash
set -euo pipefail

# Read input ResourceRequest
NAME=$(yq eval '.spec.name' /kratix/input/object.yaml)
SIZE=$(yq eval '.spec.size' /kratix/input/object.yaml)

# Apply defaults
SIZE=${SIZE:-small}

# Determine resource allocation based on size
case $SIZE in
  small)
    CPU_REQUEST="100m"
    MEMORY_REQUEST="128Mi"
    ;;
  medium)
    CPU_REQUEST="500m"
    MEMORY_REQUEST="512Mi"
    ;;
  large)
    CPU_REQUEST="2000m"
    MEMORY_REQUEST="2Gi"
    ;;
esac

# Render Kubernetes resources
cat > /kratix/output/deployment.yaml <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ${NAME}
  namespace: default
  labels:
    app: ${NAME}
    kratix.io/promise-name: my-promise
    app.kubernetes.io/managed-by: kratix
spec:
  replicas: 1
  selector:
    matchLabels:
      app: ${NAME}
  template:
    metadata:
      labels:
        app: ${NAME}
    spec:
      containers:
        - name: app
          image: nginx:latest
          resources:
            requests:
              cpu: ${CPU_REQUEST}
              memory: ${MEMORY_REQUEST}
EOF

cat > /kratix/output/service.yaml <<EOF
apiVersion: v1
kind: Service
metadata:
  name: ${NAME}
  namespace: default
  labels:
    app: ${NAME}
    kratix.io/promise-name: my-promise
spec:
  selector:
    app: ${NAME}
  ports:
    - port: 80
      targetPort: 80
EOF
```

Create [promises/my-promise/pipelines/delete-pipeline.sh](../promises/my-promise/pipelines/delete-pipeline.sh):
```bash
#!/usr/bin/env bash
set -euo pipefail

# Read resource name from deleted ResourceRequest
NAME=$(yq eval '.spec.name' /kratix/input/object.yaml)

# Kratix automatically removes resources with matching labels:
#   kratix.io/promise-name: my-promise
#   kratix.io/resource-name: <NAME>
# No manual cleanup needed unless you have external resources

echo "Delete pipeline completed for: ${NAME}"
```

**Step 3: Create Dockerfile**

Create [promises/my-promise/pipelines/Dockerfile](../promises/my-promise/pipelines/Dockerfile):
```dockerfile
FROM alpine:3.19

# Install required tools
RUN apk add --no-cache bash yq

# Copy pipeline scripts
COPY configure-pipeline.sh /scripts/configure-pipeline.sh
COPY delete-pipeline.sh /scripts/delete-pipeline.sh

RUN chmod +x /scripts/*.sh

# Set default entrypoint
ENTRYPOINT ["/scripts/configure-pipeline.sh"]
```

**Step 4: Add GitHub Actions Workflow**

Ensure [.github/workflows/build-promise-images.yaml](../.github/workflows/build-promise-images.yaml) includes your promise:
```yaml
name: Build Promise Pipeline Images

on:
  push:
    paths:
      - 'promises/**'
      - '.github/workflows/build-promise-images.yaml'

jobs:
  build:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        promise:
          - my-promise
          - vcluster-core
          - vcluster-orchestrator
          # ... other promises
    steps:
      - uses: actions/checkout@v4
      - uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}
      - uses: docker/build-push-action@v5
        with:
          context: promises/${{ matrix.promise }}/pipelines
          push: true
          tags: ghcr.io/${{ github.repository_owner }}/${{ matrix.promise }}-promise-pipeline:main
```

**Step 5: Install Promise**

Add to [addons/cluster-roles/control-plane/addons/kratix/kratix-promises/my-promise.yaml](../addons/cluster-roles/control-plane/addons/kratix/kratix-promises/my-promise.yaml):
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-promise
  namespace: kratix-platform-system
  labels:
    kratix.io/promise: "true"
data:
  promise.yaml: |
    <paste contents of promises/my-promise/promise.yaml>
```

Kratix automatically installs Promises from ConfigMaps with `kratix.io/promise: "true"` label.

**Step 6: Test ResourceRequest**

Create [platform/my-resources/test.yaml](../platform/my-resources/test.yaml):
```yaml
apiVersion: platform.integratn.tech/v1alpha1
kind: MyPromise
metadata:
  name: test-app
  namespace: platform-requests
spec:
  name: test-app
  size: medium
```

Commit and push:
```bash
git add promises/my-promise platform/my-resources/test.yaml
git commit -m "Add my-promise and test request"
git push

# Watch pipeline execution
kubectl get pods -n platform-requests -w

# Check GitStateStore outputs
git clone <kratix-platform-state-repo>
ls -la clusters/the-cluster/test-app-my-promise/
```

## Promise Best Practices

### Pipeline Design

**DO:**
- ✅ Use `set -euo pipefail` in bash scripts (fail fast)
- ✅ Validate all inputs with meaningful error messages
- ✅ Add `kratix.io/promise-name` and `app.kubernetes.io/managed-by: kratix` labels to all outputs
- ✅ Use `yq` for YAML manipulation (installed in all pipeline images)
- ✅ Write idempotent scripts (safe to run multiple times)
- ✅ Log progress to stdout (visible in pod logs)
- ✅ Test pipelines locally with Docker before deploying

**DON'T:**
- ❌ Output `kind: Secret` - ALWAYS use ExternalSecret
- ❌ Assume default namespace - always set explicit namespaces
- ❌ Use kubectl/cluster API calls from pipelines (pure rendering only)
- ❌ Hardcode values - use inputs from ResourceRequest
- ❌ Create resources in unmanaged namespaces (create namespace in pipeline first)

### Resource Labeling

**Required labels on all pipeline outputs:**
```yaml
metadata:
  labels:
    kratix.io/promise-name: my-promise  # Promise name
    kratix.io/resource-name: test-app   # ResourceRequest name
    app.kubernetes.io/managed-by: kratix  # Standard managed-by label
```

**Why:**
- Kratix uses these labels to track resource ownership
- Delete pipelines can find resources to clean up
- Observability tools can group resources by promise

### Secret Management (CRITICAL)

**FORBIDDEN:**
```yaml
# ❌ NEVER output this from a pipeline
apiVersion: v1
kind: Secret
metadata:
  name: my-credentials
type: Opaque
data:
  password: bXlzZWNyZXRwYXNzd29yZA==
```

**REQUIRED:**
```yaml
# ✅ ALWAYS use ExternalSecret
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: my-credentials
  namespace: my-app
spec:
  secretStoreRef:
    name: onepassword-connect
    kind: ClusterSecretStore
  target:
    name: my-credentials
  data:
    - secretKey: password
      remoteRef:
        key: my-app-credentials  # Stored in 1Password
        property: password
```

**Pre-commit Validation:**
```bash
# .git/hooks/pre-commit
#!/bin/bash
if git diff --cached --name-only | grep -q '^promises/'; then
  if git diff --cached | grep -B2 "kind: Secret" | grep -v "kind: ExternalSecret"; then
    echo "ERROR: Promise pipelines cannot output kind: Secret"
    echo "Use ExternalSecret with 1Password ClusterSecretStore instead"
    exit 1
  fi
fi
```

### Namespace Management

**Always create target namespace in pipeline:**
```bash
cat > /kratix/output/namespace.yaml <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: ${TARGET_NAMESPACE}
  labels:
    app: my-app
    kratix.io/promise-name: my-promise
    # PodSecurity labels (adjust per requirements)
    pod-security.kubernetes.io/enforce: restricted
    pod-security.kubernetes.io/audit: restricted
    pod-security.kubernetes.io/warn: restricted
EOF
```

**PodSecurity Label Guide:**
- `restricted`: Default for most workloads (no privileged containers, no hostPath)
- `baseline`: Allows some host access (for DaemonSets that need it)
- `privileged`: vClusters only (requires hostPath for syncer)

### Error Handling

**Validate inputs before rendering:**
```bash
# Check required field exists
if ! yq eval '.spec.name' /kratix/input/object.yaml > /dev/null 2>&1; then
  echo "ERROR: spec.name is required"
  exit 1
fi

# Validate enum value
SIZE=$(yq eval '.spec.size' /kratix/input/object.yaml)
case $SIZE in
  small|medium|large) ;;
  *) 
    echo "ERROR: spec.size must be one of: small, medium, large"
    exit 1
    ;;
esac

# Validate pattern
NAME=$(yq eval '.spec.name' /kratix/input/object.yaml)
if ! echo "$NAME" | grep -qE '^[a-z0-9]([-a-z0-9]*[a-z0-9])?$'; then
  echo "ERROR: spec.name must match pattern: ^[a-z0-9]([-a-z0-9]*[a-z0-9])?$"
  exit 1
fi
```

## Troubleshooting

### Pipeline Pod Fails

**Symptom:** Pipeline pod exits with error, ResourceRequest stuck in provisioning state.

**Diagnosis:**
```bash
# Find pipeline pod
kubectl get pods -n platform-requests | grep <resource-name>

# Check logs
kubectl logs -n platform-requests <pod-name>

# Describe pod for events
kubectl describe pod -n platform-requests <pod-name>
```

**Common Causes:**
- ❌ Missing required input field (add validation)
- ❌ Bash syntax error (test script locally first)
- ❌ Image pull failure (check GHCR authentication)
- ❌ yq command failure (verify YAML path exists)

**Fix:**
```bash
# Test pipeline locally
docker run --rm -v $(pwd)/test-input.yaml:/kratix/input/object.yaml \
  -v $(pwd)/output:/kratix/output \
  ghcr.io/your-org/my-promise-pipeline:main

# Check outputs
ls -la output/
cat output/*.yaml
```

### Resources Not Appearing in GitStateStore

**Symptom:** Pipeline succeeds but resources don't appear in kratix-platform-state repo.

**Diagnosis:**
```bash
# Check GitStateStore controller logs
kubectl logs -n kratix-platform-system -l app=kratix-platform-controller -f

# Verify Promise destinationSelectors match cluster
kubectl get promise my-promise -o yaml | yq eval '.spec.destinationSelectors'
kubectl get clusters -n kratix-platform-system -o yaml | yq eval '.metadata.labels'

# Check Work resource created
kubectl get work -n kratix-platform-system
```

**Common Causes:**
- ❌ Destination selector doesn't match any cluster
- ❌ GitStateStore credentials invalid (can't push to repo)
- ❌ Pipeline outputs missing required labels

**Fix:**
```bash
# Add capability label to cluster
kubectl label cluster the-cluster capability.my-promise=true -n kratix-platform-system

# Verify GitStateStore secret
kubectl get secret kratix-platform-state-store-credentials -n kratix-platform-system -o yaml
```

### ResourceRequest Deleted But Resources Remain

**Symptom:** Deleted ResourceRequest but deployed resources still exist in cluster.

**Diagnosis:**
```bash
# Check if delete pipeline ran
kubectl get pods -n platform-requests | grep delete

# Check resources have correct labels
kubectl get all -A -l kratix.io/promise-name=my-promise
```

**Common Causes:**
- ❌ Delete pipeline not defined in Promise
- ❌ Resources missing `kratix.io/promise-name` label
- ❌ Delete pipeline failed (check logs)

**Fix:**
```bash
# Manual cleanup if delete pipeline missing
kubectl delete all -l kratix.io/resource-name=<name>

# Add delete pipeline to Promise
spec:
  workflows:
    resource:
      delete:
        - apiVersion: platform.kratix.io/v1alpha1
          kind: Pipeline
          ...
```

### Pipeline Image Not Updating

**Symptom:** Changed pipeline code but old behavior persists.

**Diagnosis:**
```bash
# Check image tag in Promise
kubectl get promise my-promise -o yaml | yq eval '.spec.workflows.resource.configure[0].spec.containers[0].image'

# Check when image was built
docker pull ghcr.io/your-org/my-promise-pipeline:main
docker inspect ghcr.io/your-org/my-promise-pipeline:main | jq '.[0].Created'
```

**Common Causes:**
- ❌ GitHub Actions build didn't trigger
- ❌ Image cached by Kubernetes (imagePullPolicy: IfNotPresent)
- ❌ Wrong branch pushed (should be main)

**Fix:**
```bash
# Force imagePullPolicy: Always in Promise
spec:
  workflows:
    resource:
      configure:
        - spec:
            containers:
              - imagePullPolicy: Always  # Add this

# Manually trigger rebuild
git commit --allow-empty -m "Rebuild promise pipeline"
git push

# Wait for GitHub Actions
gh run watch
```

## Key Files Reference

### Promise Definitions
- **vCluster orchestrator**: [promises/vcluster-orchestrator/promise.yaml](../promises/vcluster-orchestrator/promise.yaml)
- **vCluster core**: [promises/vcluster-core/promise.yaml](../promises/vcluster-core/promise.yaml)
- **CoreDNS**: [promises/vcluster-coredns/promise.yaml](../promises/vcluster-coredns/promise.yaml)
- **Kubeconfig sync**: [promises/vcluster-kubeconfig-sync/promise.yaml](../promises/vcluster-kubeconfig-sync/promise.yaml)
- **ExternalSecret**: [promises/vcluster-kubeconfig-external-secret/promise.yaml](../promises/vcluster-kubeconfig-external-secret/promise.yaml)
- **ArgoCD registration**: [promises/vcluster-argocd-cluster-registration/promise.yaml](../promises/vcluster-argocd-cluster-registration/promise.yaml)
- **ArgoCD project helper**: [promises/argocd-project/promise.yaml](../promises/argocd-project/promise.yaml)
- **ArgoCD application helper**: [promises/argocd-application/promise.yaml](../promises/argocd-application/promise.yaml)

### Pipeline Scripts
- **Orchestrator configure**: [promises/vcluster-orchestrator/pipelines/configure-pipeline.sh](../promises/vcluster-orchestrator/pipelines/configure-pipeline.sh)
- **Core render**: [promises/vcluster-core/pipelines/configure-pipeline.sh](../promises/vcluster-core/pipelines/configure-pipeline.sh)

### ResourceRequests
- **Platform requests namespace**: [platform/vclusters/00-namespace.yaml](../platform/vclusters/00-namespace.yaml)
- **Example vCluster**: [platform/vclusters/media.yaml](../platform/vclusters/media.yaml)
- **Request README**: [platform/vclusters/README.md](../platform/vclusters/README.md)

### GitHub Actions
- **Pipeline image builds**: [.github/workflows/build-promise-images.yaml](../.github/workflows/build-promise-images.yaml)

### Kratix Configuration
- **Promise installation**: [addons/cluster-roles/control-plane/addons/kratix/kratix-promises/](../addons/cluster-roles/control-plane/addons/kratix/kratix-promises/)
- **GitStateStore config**: Check `kratix` addon values for statestore configuration
