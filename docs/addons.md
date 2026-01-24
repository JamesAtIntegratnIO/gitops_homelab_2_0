# Addons

> **Official Documentation References:**
> - [ArgoCD ApplicationSets](https://argo-cd.readthedocs.io/en/stable/user-guide/application-set/) - Multi-cluster application templating
> - [Helm](https://helm.sh/docs/) - Kubernetes package manager
> - [Stakater Application Chart](https://github.com/stakater/application) - Reusable Helm chart pattern
> - [ArgoCD Multi-Source Applications](https://argo-cd.readthedocs.io/en/stable/user-guide/multiple_sources/) - Multiple repo sources

## Overview

The **addons layer** is the heart of the GitOps platform's service deployment mechanism. It uses a custom Helm chart ([addons/charts/application-sets](../addons/charts/application-sets/)) to dynamically generate ArgoCD ApplicationSets, which in turn generate Applications for each cluster based on label selectors.

**Why This Pattern:**
- **Single source of truth**: One `addons.yaml` file defines all platform services
- **Multi-cluster by default**: Automatically targets clusters based on labels
- **Value overlay system**: Environment/cluster-specific overrides without duplication
- **Type-safe**: Helm templates validate configuration at render time
- **GitOps native**: Everything flows through Git → ArgoCD → Kubernetes

## Architecture Diagram

```
┌──────────────────────────────────────────────────────────────────┐
│  Git Repo: gitops_homelab_2_0                                     │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │ addons/environments/production/addons/addons.yaml          │  │
│  │                                                             │  │
│  │ argocd:                                                     │  │
│  │   enabled: true                                            │  │
│  │   defaultVersion: "9.0.3"                                  │  │
│  │   selector:                                                │  │
│  │     matchExpressions:                                      │  │
│  │       - key: enable_argocd                                 │  │
│  │         operator: In                                       │  │
│  │         values: ['true']                                   │  │
│  └────────────────────────────────────────────────────────────┘  │
└──────────────────┬───────────────────────────────────────────────┘
                   │ ArgoCD syncs bootstrap app
                   ▼
┌──────────────────────────────────────────────────────────────────┐
│  Kubernetes: the-cluster (ArgoCD namespace)                       │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │ Helm Chart: application-sets                               │  │
│  │ (renders ApplicationSet CRDs from addons.yaml)             │  │
│  └────────────────────────────┬───────────────────────────────┘  │
│                                │ Helm renders template
│                                ▼
│  ┌────────────────────────────────────────────────────────────┐  │
│  │ ApplicationSet: argocd                                      │  │
│  │                                                             │  │
│  │ generators:                                                 │  │
│  │   - clusters:                                              │  │
│  │       selector:                                            │  │
│  │         matchLabels:                                       │  │
│  │           argocd.argoproj.io/secret-type: cluster          │  │
│  │         matchExpressions:                                  │  │
│  │           - key: enable_argocd                             │  │
│  │             operator: In                                   │  │
│  │             values: ['true']                               │  │
│  │       values:                                              │  │
│  │         addonChartVersion: "9.0.3"                         │  │
│  │                                                             │  │
│  │ template:                                                   │  │
│  │   spec:                                                     │  │
│  │     sources:                                               │  │
│  │       - repoURL: https://argoproj.github.io/argo-helm     │  │
│  │         chart: argo-cd                                     │  │
│  │         targetRevision: '{{.values.addonChartVersion}}'    │  │
│  │         helm:                                              │  │
│  │           valueFiles:                                      │  │
│  │             - $values/addons/default/addons/argocd/values.yaml │
│  │             - $values/addons/cluster-roles/control-plane/addons/argocd/values.yaml │
│  │       - repoURL: https://github.com/...                    │  │
│  │         ref: values                                        │  │
│  └────────────────────────────┬───────────────────────────────┘  │
│                                │ ApplicationSet controller
│                                │ evaluates cluster selectors
│                                ▼
│  ┌────────────────────────────────────────────────────────────┐  │
│  │ Cluster Secret: the-cluster                                │  │
│  │   labels:                                                   │  │
│  │     argocd.argoproj.io/secret-type: cluster                │  │
│  │     enable_argocd: "true"                                  │  │
│  │     cluster_role: control-plane                            │  │
│  │     environment: production                                │  │
│  └────────────────────────────┬───────────────────────────────┘  │
│                                │ MATCH! Cluster matches selector
│                                ▼
│  ┌────────────────────────────────────────────────────────────┐  │
│  │ Application: argocd-the-cluster                            │  │
│  │                                                             │  │
│  │ spec:                                                       │  │
│  │   project: platform                                        │  │
│  │   sources:                                                  │  │
│  │     - repoURL: https://argoproj.github.io/argo-helm       │  │
│  │       chart: argo-cd                                       │  │
│  │       targetRevision: "9.0.3"                              │  │
│  │       helm:                                                 │  │
│  │         valueFiles:                                        │  │
│  │           - $values/addons/default/addons/argocd/values.yaml │
│  │           - $values/addons/cluster-roles/control-plane/addons/argocd/values.yaml │
│  │     - repoURL: https://github.com/...                      │  │
│  │       targetRevision: main                                 │  │
│  │       ref: values                                          │  │
│  │   destination:                                             │  │
│  │     server: https://kubernetes.default.svc                 │  │
│  │     namespace: argocd                                      │  │
│  └────────────────────────────┬───────────────────────────────┘  │
│                                │ ArgoCD syncs
│                                ▼
│  ┌────────────────────────────────────────────────────────────┐  │
│  │ Helm Release: argo-cd                                       │  │
│  │ (Deployed in argocd namespace with merged values)          │  │
│  └────────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────────┘
```

## How the Addons Layer Works (Step-by-Step)

### Step 1: Bootstrap ApplicationSet Creation (Terraform)

Terraform creates the initial ApplicationSet that points ArgoCD at this repo:

```terraform
# terraform/cluster/main.tf
module "argocd" {
  source = "git::https://github.com/jamesAtIntegratnIO/terraform-helm-gitops-bridge.git?ref=homelab"
  
  apps = {
    addons-control-plane = file("${path.module}/bootstrap/addons-control-plane.yaml")
    addons-vcluster      = file("${path.module}/bootstrap/addons-vcluster.yaml")
  }
}
```

**Bootstrap ApplicationSet** ([terraform/cluster/bootstrap/addons-control-plane.yaml](../terraform/cluster/bootstrap/addons-control-plane.yaml)):
```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: application-sets-control-plane
  namespace: argocd
spec:
  generators:
    - clusters:
        selector:
          matchLabels:
            cluster_role: control-plane
  template:
    spec:
      source:
        repoURL: https://github.com/jamesatintegratnio/gitops_homelab_2_0
        path: addons/charts/application-sets
        targetRevision: main
        helm:
          valueFiles:
            - ../../environments/production/addons/addons.yaml
      destination:
        namespace: argocd
```

### Step 2: Helm Chart Renders ApplicationSets

The [addons/charts/application-sets](../addons/charts/application-sets/) Helm chart processes `addons.yaml` and renders one **ApplicationSet per enabled addon**.

**Key Template Logic** ([addons/charts/application-sets/templates/application-set.yaml](../addons/charts/application-sets/templates/application-set.yaml)):

```helm
{{- range $chartName, $chartConfig := .Values }}
{{- if and (kindIs "map" $chartConfig) (hasKey $chartConfig "enabled") }}
{{- if eq (toString $chartConfig.enabled) "true" }}
  
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: {{ $chartName }}
  namespace: argocd
spec:
  goTemplate: true
  generators:
    - clusters:
        selector:
          matchLabels:
            argocd.argoproj.io/secret-type: cluster
          {{- if $chartConfig.selector }}
          {{- toYaml $chartConfig.selector | nindent 10 }}
          {{- end }}
        values:
          addonChartVersion: {{ $chartConfig.defaultVersion | quote }}
```

### Step 3: Cluster Matching via Label Selectors

ApplicationSets use **cluster generators** to find matching clusters based on Cluster Secret labels.

**Cluster Secret Example:**
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: the-cluster
  namespace: argocd
  labels:
    argocd.argoproj.io/secret-type: cluster
    cluster_name: the-cluster
    environment: production
    cluster_role: control-plane
    enable_argocd: "true"
    enable_cert_manager: "true"
    enable_kratix: "true"
type: Opaque
```

**Addon Selector (from addons.yaml):**
```yaml
argocd:
  enabled: true
  selector:
    matchExpressions:
      - key: enable_argocd
        operator: In
        values: ['true']
      - key: cluster_role
        operator: NotIn
        values: ['vcluster']  # Exclude vclusters
```

**Matching Logic:**
1. ApplicationSet controller lists all Secrets with `argocd.argoproj.io/secret-type: cluster`
2. Applies `matchLabels` and `matchExpressions` filters
3. For each matching cluster, renders an Application from the template

### Step 4: Multi-Source Value Files

Applications use **multi-source** to pull Helm chart from upstream and values from this repo.

**Multi-Source Configuration:**
```yaml
spec:
  sources:
    # Source 1: Helm chart from upstream repository
    - repoURL: https://prometheus-community.github.io/helm-charts
      chart: kube-prometheus-stack
      targetRevision: '58.2.1'
      helm:
        valueFiles:
          - $values/addons/default/addons/kube-prometheus-stack/values.yaml
          - $values/addons/cluster-roles/control-plane/addons/kube-prometheus-stack/values.yaml
          - $values/addons/clusters/the-cluster/addons/kube-prometheus-stack/values.yaml
    
    # Source 2: Value files from this repository
    - repoURL: https://github.com/jamesatintegratnio/gitops_homelab_2_0
      targetRevision: main
      ref: values  # Referenced as $values in Source 1
```

**Why Multi-Source:**
- ✅ Helm charts stay upstream (easy updates)
- ✅ Values stay in Git (version controlled)
- ✅ No need to mirror/fork upstream charts
- ✅ Clear separation: chart (what) vs values (how)

## Folder Structure

```
addons/
├── charts/
│   └── application-sets/          # Helm chart that renders ApplicationSets
│       ├── Chart.yaml
│       ├── values.yaml
│       └── templates/
│           ├── application-set.yaml      # Main template
│           └── _application_set.tpl      # Helper templates
│
├── default/
│   └── addons/
│       └── <addon-name>/
│           └── values.yaml        # Base values (lowest precedence)
│
├── cluster-roles/
│   ├── control-plane/
│   │   └── addons/
│   │       └── <addon-name>/
│   │           └── values.yaml    # Control-plane role overlay
│   └── vcluster/
│       └── addons/
│           └── <addon-name>/
│               └── values.yaml    # vCluster role overlay
│
├── environments/
│   ├── production/
│   │   └── addons/
│   │       ├── addons.yaml        # Addon definitions (enabled, selector, etc.)
│   │       └── <addon-name>/
│   │           └── values.yaml    # Production overlay
│   ├── staging/
│   └── development/
│
└── clusters/
    └── the-cluster/
        └── addons/
            └── <addon-name>/
                └── values.yaml    # Cluster-specific overrides (highest precedence)
```

## Value File Precedence (Deep Dive)

Helm merges value files in **order**, with later files overriding earlier ones:

**Precedence Order (lowest → highest):**
1. Chart's `values.yaml` (from upstream Helm chart)
2. `addons/default/addons/<addon>/values.yaml`
3. `addons/cluster-roles/<role>/addons/<addon>/values.yaml`
4. `addons/environments/<environment>/addons/<addon>/values.yaml`
5. `addons/clusters/<cluster>/addons/<addon>/values.yaml`

**Example: kube-prometheus-stack values for the-cluster**

```yaml
# 1. Chart defaults (from prometheus-community/kube-prometheus-stack chart)
prometheus:
  prometheusSpec:
    retention: 10d

# 2. addons/default/addons/kube-prometheus-stack/values.yaml
# (Empty - no base values set)

# 3. addons/cluster-roles/control-plane/addons/kube-prometheus-stack/values.yaml
prometheus:
  prometheusSpec:
    retention: 15d  # Override: extended retention for control-plane
    storageSpec:
      volumeClaimTemplate:
        spec:
          storageClassName: config-nfs-client
          resources:
            requests:
              storage: 30Gi

# 4. addons/environments/production/addons/kube-prometheus-stack/values.yaml
# (Empty - no production-specific overrides)

# 5. addons/clusters/the-cluster/addons/kube-prometheus-stack/values.yaml
prometheus:
  prometheusSpec:
    retention: 20d  # FINAL: Cluster-specific override wins
```

**Final Merged Values:**
```yaml
prometheus:
  prometheusSpec:
    retention: 20d  # From cluster-specific values
    storageSpec:
      volumeClaimTemplate:
        spec:
          storageClassName: config-nfs-client  # From role values
          resources:
            requests:
              storage: 30Gi  # From role values
```

**Key Insight:** `ignoreMissingValueFiles: true` means you only create value files where you need overrides.

## Cluster Selection Mechanics

### Basic Label Matching

**Simple label match (AND logic):**
```yaml
external-secrets:
  enabled: true
  selector:
    matchLabels:
      enable_external_secrets: "true"
```

Matches clusters with **exactly** `enable_external_secrets: "true"` label.

### Match Expressions (Advanced)

**In operator (OR logic):**
```yaml
argocd:
  enabled: true
  selector:
    matchExpressions:
      - key: cluster_role
        operator: In
        values: ['control-plane', 'management']  # Matches either value
```

**NotIn operator (exclusion):**
```yaml
argocd:
  enabled: true
  selector:
    matchExpressions:
      - key: cluster_role
        operator: NotIn
        values: ['vcluster']  # Deploy everywhere except vclusters
```

**Exists operator (presence check):**
```yaml
kyverno:
  enabled: true
  selector:
    matchExpressions:
      - key: enable_kyverno
        operator: Exists  # Matches if label exists (any value)
```

**DoesNotExist operator:**
```yaml
legacy-addon:
  enabled: true
  selector:
    matchExpressions:
      - key: modern-cluster
        operator: DoesNotExist  # Only deploy to clusters without this label
```

### Combining Selectors (AND logic)

All selectors must match:
```yaml
kube-prometheus-stack-agent:
  enabled: true
  selector:
    matchLabels:
      environment: production  # Must be production
    matchExpressions:
      - key: cluster_role
        operator: In
        values: ['vcluster']  # AND must be a vcluster
      - key: enable_monitoring
        operator: In
        values: ['true']  # AND must have monitoring enabled
```

### Environment-Specific Overrides

Use `environments` to target same cluster with different values:

```yaml
my-addon:
  enabled: true
  defaultVersion: "1.0.0"
  selector:
    matchLabels:
      enable_my_addon: "true"
  environments:
    - selector:
        environment: staging
      chartVersion: "1.1.0-rc1"  # Staging gets pre-release
    - selector:
        environment: production
      chartVersion: "1.0.0"  # Production stays stable
```

## Addon Definition Schema

### Required Fields

```yaml
addon-name:
  enabled: true | false  # Feature flag
  defaultVersion: "X.Y.Z"  # Chart version (can be overridden per environment)
```

### Common Optional Fields

```yaml
addon-name:
  enabled: true
  defaultVersion: "1.0.0"
  
  # Helm chart source
  chartRepository: "https://charts.example.com"
  chartName: "my-chart"  # Defaults to addon-name
  chartNamespace: "example"  # For OCI registries (public.ecr.aws/namespace/chart)
  
  # ArgoCD configuration
  namespace: target-namespace  # Where to deploy
  project: argocd-project-name  # ArgoCD project
  releaseName: custom-release-name  # Helm release name
  
  # Cluster targeting
  selector:
    matchLabels:
      key: value
    matchExpressions:
      - key: cluster_role
        operator: In
        values: ['control-plane']
  
  # ArgoCD sync behavior
  syncPolicy:
    automated:
      selfHeal: true
      prune: true
  annotationsApp:
    argocd.argoproj.io/sync-wave: "0"
  
  # Value templating from cluster metadata
  valuesObject:
    someKey: '{{.metadata.labels.cluster_label}}'
    nested:
      value: '{{.metadata.annotations.annotation_key}}'
  
  # Multi-environment support
  environments:
    - selector:
        environment: staging
      chartVersion: "1.1.0"
      values:
        customKey: staging-value
```

### Manifest-Type Addons (Non-Helm)

For raw YAML manifests instead of Helm charts:

```yaml
argocd-projects:
  enabled: true
  type: manifest
  namespace: argocd
  path: addons/environments/production/addons/argocd-projects
  selector:
    matchExpressions:
      - key: enable_argocd
        operator: In
        values: ['true']
```

## Key Addons in This Repo

| Addon | Purpose | Cluster Role | Version | Chart Repository |
|-------|---------|--------------|---------|------------------|
| **argocd** | GitOps CD engine | control-plane | 9.0.3 | https://argoproj.github.io/argo-helm |
| **cert-manager** | TLS automation | all | v1.16.2 | https://charts.jetstack.io |
| **external-secrets** | 1Password sync | all | 0.10.3 | https://charts.external-secrets.io |
| **nginx-gateway-fabric** | Gateway API | control-plane | latest | https://github.com/nginxinc/nginx-gateway-fabric |
| **external-dns** | Cloudflare DNS | control-plane | latest | https://kubernetes-sigs.github.io/external-dns/ |
| **metallb** | LoadBalancer IPs | control-plane | latest | https://metallb.github.io/metallb |
| **kyverno** | Policy engine | all | 3.2.6 | https://kyverno.github.io/kyverno/ |
| **kube-prometheus-stack** | Full observability | control-plane | 58.2.1 | https://prometheus-community.github.io/helm-charts |
| **kube-prometheus-stack-agent** | Metrics agent | vcluster | 58.2.1 | https://prometheus-community.github.io/helm-charts |
| **loki** | Log aggregation | control-plane | 6.9.0 | https://grafana.github.io/helm-charts |
| **promtail** | Log collection | control-plane | 6.9.0 | https://grafana.github.io/helm-charts |
| **kratix** | Platform API | control-plane | latest | https://github.com/syntasso/kratix |

## Adding a New Addon (Complete Walkthrough)

### Scenario: Add Velero (Backup Tool)

**Step 1: Add addon definition to addons.yaml**

Edit [addons/environments/production/addons/addons.yaml](../addons/environments/production/addons/addons.yaml):

```yaml
velero:
  enabled: true
  namespace: velero
  project: platform-services
  chartName: velero
  defaultVersion: "7.2.1"
  chartRepository: "https://vmware-tanzu.github.io/helm-charts"
  selector:
    matchExpressions:
      - key: enable_velero
        operator: In
        values: ['true']
      - key: cluster_role
        operator: In
        values: ['control-plane']  # Only control-plane needs backups
  syncPolicy:
    automated:
      selfHeal: true
      prune: true
  valuesObject:
    configuration:
      backupStorageLocation:
        - name: default
          provider: aws
          bucket: '{{.metadata.annotations.velero_backup_bucket}}'
          config:
            region: '{{.metadata.annotations.aws_region}}'
```

**Step 2: Create base values file**

Create [addons/cluster-roles/control-plane/addons/velero/values.yaml](../addons/cluster-roles/control-plane/addons/velero/values.yaml):

```yaml
initContainers:
  - name: velero-plugin-for-aws
    image: velero/velero-plugin-for-aws:v1.10.0
    volumeMounts:
      - mountPath: /target
        name: plugins

configuration:
  provider: aws
  backupStorageLocation:
    - name: default
      provider: aws
      default: true

credentials:
  useSecret: true
  existingSecret: velero-aws-credentials

snapshotsEnabled: false  # No volume snapshots for now
```

**Step 3: Add cluster labels**

Update cluster Secret (Terraform or manually):
```yaml
metadata:
  labels:
    enable_velero: "true"
  annotations:
    velero_backup_bucket: "homelab-velero-backups"
    aws_region: "us-east-1"
```

**Step 4: Create ExternalSecret for credentials**

Create [addons/cluster-roles/control-plane/addons/velero/external-secret-aws-credentials.yaml](../addons/cluster-roles/control-plane/addons/velero/external-secret-aws-credentials.yaml):

```yaml
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: velero-aws-credentials
  namespace: velero
spec:
  secretStoreRef:
    name: onepassword-connect
    kind: ClusterSecretStore
  target:
    name: velero-aws-credentials
  data:
    - secretKey: cloud
      remoteRef:
        key: velero-aws-credentials
        property: credentials
```

**Step 5: Commit and sync**

```bash
git add addons/
git commit -m "Add Velero backup addon for control-plane clusters"
git push

# Sync the application-sets ApplicationSet
argocd app sync application-sets-control-plane

# Verify ApplicationSet created
kubectl get applicationset -n argocd velero

# Verify Application generated
kubectl get application -n argocd velero-the-cluster

# Check deployment
kubectl get pods -n velero
```

## Troubleshooting

### ApplicationSet Not Generating Applications

**Symptom:** ApplicationSet exists but no Applications are created.

**Diagnosis:**
```bash
# Check ApplicationSet status
kubectl get applicationset -n argocd <addon-name> -o yaml

# Look for conditions
kubectl get applicationset -n argocd <addon-name> -o jsonpath='{.status.conditions}'

# Check if any clusters match selector
kubectl get secrets -n argocd -l argocd.argoproj.io/secret-type=cluster --show-labels
```

**Common Causes:**
- ❌ Cluster Secret missing required label
- ❌ `matchExpressions` operator syntax error
- ❌ Label values in quotes when they shouldn't be (or vice versa)

**Fix:**
```bash
# Add missing label to cluster
kubectl label secret -n argocd the-cluster enable_my_addon=true

# Verify ApplicationSet regenerates
kubectl get application -n argocd | grep my-addon
```

### Application OutOfSync Despite No Changes

**Symptom:** Application shows OutOfSync, but Git hasn't changed.

**Diagnosis:**
```bash
# Check diff
argocd app diff <app-name>

# Check ApplicationSet parameters
argocd appset get <appset-name>
```

**Common Causes:**
- ❌ `ignoreMissingValueFiles: true` not set (Application fails if optional value file missing)
- ❌ Cluster annotation changed (templated in `valuesObject`)
- ❌ Upstream Helm chart repository changed default values

**Fix:**
```yaml
# Ensure ignoreMissingValueFiles is set
helm:
  ignoreMissingValueFiles: true
  valueFiles:
    - $values/addons/default/addons/my-addon/values.yaml
```

### Values Not Being Applied

**Symptom:** Deployed resources don't have expected values.

**Diagnosis:**
```bash
# Get rendered manifest from Application
argocd app manifests <app-name> > /tmp/rendered.yaml

# Compare with values file
yq eval '.spec.sources[0].helm.valueFiles' /tmp/rendered.yaml

# Check if value file exists in repo
ls -la addons/cluster-roles/control-plane/addons/my-addon/values.yaml
```

**Common Causes:**
- ❌ Value file path typo
- ❌ Value file not committed to Git
- ❌ `$values` reference missing (must have `ref: values` source)
- ❌ Incorrect YAML indentation (Helm silently ignores malformed values)

**Fix:**
```bash
# Verify file exists
git ls-files | grep addons/cluster-roles/control-plane/addons/my-addon

# Check YAML syntax
yq eval addons/cluster-roles/control-plane/addons/my-addon/values.yaml

# Verify multi-source setup
argocd app get <app-name> -o yaml | yq eval '.spec.sources'
```

### Application Stuck in Progressing

**Symptom:** Application sync'd but stays "Progressing", never becomes "Healthy".

**Diagnosis:**
```bash
# Check application resources
argocd app get <app-name>

# Check pod status
kubectl get pods -n <namespace> -l app=<app>

# Check events
kubectl get events -n <namespace> --sort-by='.lastTimestamp'
```

**Common Causes:**
- ❌ Missing dependent resources (CRDs not installed)
- ❌ Resource limits too low (OOMKilled)
- ❌ PVC stuck in Pending (storage class doesn't exist)
- ❌ Image pull failures
- ❌ Webhook validation blocking deployment

**Fix:**
```bash
# Check if CRDs exist
kubectl get crd | grep <addon-name>

# Check PVC status
kubectl get pvc -n <namespace>

# Check pod logs
kubectl logs -n <namespace> <pod-name>

# Force sync with replace
argocd app sync <app-name> --replace
```

### Addon Deployed to Wrong Cluster

**Symptom:** Addon appears on unexpected cluster (e.g., vcluster when it should only be control-plane).

**Diagnosis:**
```bash
# Check which clusters the ApplicationSet targets
kubectl get applicationset -n argocd <addon-name> -o yaml | yq eval '.spec.generators[0].clusters.selector'

# Check cluster labels
kubectl get secret -n argocd <cluster-name> -o yaml | yq eval '.metadata.labels'
```

**Common Causes:**
- ❌ Missing `NotIn` exclusion for vcluster
- ❌ Cluster has unexpected label
- ❌ Selector logic error (forgot `matchExpressions`)

**Fix:**
```yaml
# Add exclusion to addon definition
addon-name:
  selector:
    matchExpressions:
      - key: cluster_role
        operator: In
        values: ['control-plane']
      - key: cluster_role
        operator: NotIn
        values: ['vcluster']  # Explicitly exclude
```

## Operational Best Practices

### Value File Organization

**DO:**
- ✅ Keep values at the lowest precedence level that makes sense
- ✅ Use cluster-specific values only for truly unique configs
- ✅ Document why cluster-specific overrides exist (comments in YAML)
- ✅ Use `valuesObject` for dynamic values from cluster metadata

**DON'T:**
- ❌ Duplicate values across multiple precedence levels
- ❌ Put environment-specific values in cluster-specific files
- ❌ Hardcode values that could be templated from cluster labels/annotations

### Addon Naming

- Use lowercase with hyphens: `kube-prometheus-stack`, not `kubePrometheusStack`
- Match Helm chart name when possible: `argocd` (chart) → `argocd:` (addon)
- Use descriptive names for custom addons: `observability-secrets`, not `secrets`

### Selector Patterns

**Simple feature flag:**
```yaml
selector:
  matchLabels:
    enable_my_addon: "true"
```

**Role-based deployment:**
```yaml
selector:
  matchExpressions:
    - key: cluster_role
      operator: In
      values: ['control-plane']
```

**Complex targeting:**
```yaml
selector:
  matchLabels:
    environment: production
  matchExpressions:
    - key: cluster_role
      operator: In
      values: ['control-plane', 'worker']
    - key: enable_my_addon
      operator: In
      values: ['true']
```

### Service Exposure Strategy

- ✅ **ClusterIP services** behind Gateway HTTPRoutes (default)
- ✅ **LoadBalancer** only for Gateway itself (MetalLB)
- ❌ **NodePort** (avoid - not cloud-native)
- ❌ **LoadBalancer** per service (wastes IPs)

### Testing Addon Changes

**Test in staging first:**
```yaml
my-addon:
  enabled: true
  environments:
    - selector:
        environment: staging
      chartVersion: "2.0.0-beta"  # Test new version
    - selector:
        environment: production
      chartVersion: "1.5.0"  # Stable version
```

**Dry run before committing:**
```bash
# Render ApplicationSet locally
helm template addons/charts/application-sets \
  -f addons/environments/production/addons/addons.yaml \
  | yq eval 'select(.kind == "ApplicationSet")' - \
  | yq eval 'select(.metadata.name == "my-addon")' -
```

## Key Files Reference

- **ApplicationSet chart**: [addons/charts/application-sets/](../addons/charts/application-sets/)
- **Main template**: [addons/charts/application-sets/templates/application-set.yaml](../addons/charts/application-sets/templates/application-set.yaml)
- **Helper templates**: [addons/charts/application-sets/templates/_application_set.tpl](../addons/charts/application-sets/templates/_application_set.tpl)
- **Production addons**: [addons/environments/production/addons/addons.yaml](../addons/environments/production/addons/addons.yaml)
- **Control-plane values**: [addons/cluster-roles/control-plane/addons/](../addons/cluster-roles/control-plane/addons/)
- **vCluster values**: [addons/cluster-roles/vcluster/addons/](../addons/cluster-roles/vcluster/addons/)
- **Bootstrap files**: [terraform/cluster/bootstrap/](../terraform/cluster/bootstrap/)
