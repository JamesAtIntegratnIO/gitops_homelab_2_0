# Operations

> **Official Documentation References:**
> - [ArgoCD Operations Guide](https://argo-cd.readthedocs.io/en/stable/operator-manual/) - Production operations
> - [Kubernetes Production Best Practices](https://kubernetes.io/docs/setup/best-practices/) - Cluster operations
> - [Prometheus Alerting](https://prometheus.io/docs/alerting/latest/overview/) - Alert management
> - [Grafana Dashboards](https://grafana.com/docs/grafana/latest/dashboards/) - Visualization

## Overview

This guide covers **day-to-day operations** for the GitOps homelab platform, including:

- Common workflows (addon changes, vCluster management, troubleshooting)
- Incident response procedures
- Backup and recovery
- Upgrade procedures
- Monitoring and alerting
- Performance tuning

**Operational Principles:**
1. **Git is source of truth** - All changes flow through Git commits
2. **Immutability** - Replace rather than modify (GitOps way)
3. **Observability first** - Check metrics/logs before guessing
4. **Automate recovery** - ArgoCD self-heals most issues
5. **Document everything** - Runbooks capture tribal knowledge

## Quick Reference

### Core URLs

| Service | URL | Access Method |
|---------|-----|---------------|
| **ArgoCD** | https://argocd.cluster.integratn.tech | External via Gateway |
| **Grafana** | https://grafana.cluster.integratn.tech | External via Gateway ✓ |
| **Prometheus** | https://prometheus.cluster.integratn.tech | External via Gateway ✓ |
| **Alertmanager** | https://alertmanager.cluster.integratn.tech | External via Gateway ✓ |
| **Loki** | https://loki.cluster.integratn.tech | External via Gateway ✓ |
| **Prom Remote Write** | https://prom-remote.integratn.tech | Internal (vcluster federation) |

### Common Commands

```bash
# ArgoCD sync status
argocd app list
argocd app get <app-name>
argocd app sync <app-name>

# Check cluster health
kubectl get nodes
kubectl get pods --all-namespaces | grep -v Running
kubectl top nodes
kubectl top pods -A

# View application logs
kubectl logs -n <namespace> <pod> -f

# Check resource quotas
kubectl get resourcequota -A

# Debugging networking
kubectl run debug --rm -it --image=nicolaka/netshoot -- bash
```

## Routine Workflows

### Workflow 1: Modify Addon Configuration

**Scenario:** Change Grafana admin password in values file.

**Steps:**
```bash
# 1. Edit values file
cd ~/projects/gitops_homelab_2_0
vi addons/cluster-roles/control-plane/addons/kube-prometheus-stack/values.yaml

# Locate grafana section
grafana:
  adminPassword: new-secure-password-here

# 2. Commit and push
git add addons/cluster-roles/control-plane/addons/kube-prometheus-stack/values.yaml
git commit -m "Update Grafana admin password"
git push

# 3. Sync ArgoCD Application
argocd app sync kube-prometheus-stack-the-cluster

# 4. Wait for sync to complete
argocd app wait kube-prometheus-stack-the-cluster --health

# 5. Verify change applied
kubectl get secret -n monitoring kube-prometheus-stack-grafana -o jsonpath='{.data.admin-password}' | base64 -d

# 6. Test login
open https://grafana.cluster.integratn.tech
# Login with new password
```

**Expected Duration:** 2-3 minutes

**Rollback Procedure:**
```bash
# Revert Git commit
git revert HEAD
git push

# Sync ArgoCD
argocd app sync kube-prometheus-stack-the-cluster
```

### Workflow 2: Enable New Addon

**Scenario:** Enable Kyverno policy engine on control-plane cluster.

**Steps:**
```bash
# 1. Edit addons.yaml
vi addons/environments/production/addons/addons.yaml

# Add Kyverno entry
kyverno:
  enabled: true
  namespace: kyverno
  project: platform-services
  defaultVersion: "3.2.6"
  chartRepository: "https://kyverno.github.io/kyverno/"
  selector:
    matchExpressions:
      - key: cluster_role
        operator: In
        values: ['control-plane']
  syncPolicy:
    automated:
      selfHeal: true
      prune: true

# 2. Create base values file
mkdir -p addons/cluster-roles/control-plane/addons/kyverno
cat > addons/cluster-roles/control-plane/addons/kyverno/values.yaml <<EOF
replicaCount: 3  # HA for production

admissionController:
  replicas: 3
  
backgroundController:
  enabled: true

cleanupController:
  enabled: true

# ArgoCD-safe settings
installCRDs: true
webhooksCleanup:
  enabled: false  # Don't clean up on uninstall
EOF

# 3. Add required cluster label
kubectl label secret -n argocd the-cluster enable_kyverno=true --overwrite

# 4. Commit and push
git add addons/
git commit -m "Enable Kyverno policy engine on control-plane"
git push

# 5. Sync ApplicationSets
argocd app sync application-sets-control-plane

# 6. Wait for ApplicationSet to generate Application
kubectl get applicationset -n argocd kyverno
kubectl get application -n argocd kyverno-the-cluster

# 7. Verify Kyverno deployment
kubectl get pods -n kyverno
kubectl get validatingwebhookconfigurations | grep kyverno
```

**Expected Duration:** 3-5 minutes

### Workflow 3: Create New vCluster

**Scenario:** Development team needs isolated cluster for testing.

> **Philosophy:** Stop thinking "CRDs" — start thinking "Platform Product."
> The `hctl` CLI turns vCluster provisioning into a self-service developer
> experience. It generates validated YAML, commits to Git, and monitors
> readiness — all in one command.

#### Option A: One-liner (scripted / CI)

```bash
hctl vcluster create dev-team-1 --preset dev --auto-commit
```

This single command will:
1. **Generate** a validated `VClusterOrchestratorV2` resource with all defaults
2. **Write** it to `platform/vclusters/dev-team-1.yaml`
3. **Commit & push** to Git (triggers ArgoCD sync automatically)

#### Option B: Interactive wizard (recommended for first-time use)

```bash
hctl vcluster create
```

The wizard walks through each option:
- vCluster name
- Preset (`dev` — lightweight / `prod` — HA with etcd)
- Kubernetes version (v1.34.3, 1.33, 1.32)
- Isolation mode (standard / strict)
- External hostname (defaults to `<name>.cluster.integratn.tech`)
- ArgoCD environment (production / staging / development)
- NFS egress toggle
- Custom workload repository (URL, path, branch — for teams keeping workloads in a separate repo)
- Commit-and-push confirmation

#### Workloads from another repository

Teams often keep their Kubernetes manifests in their own repo rather than
the platform repo. The `--workload-repo-*` flags configure the ArgoCD
ApplicationSet to source workloads from that external repository:

```bash
hctl vcluster create team-api --preset dev \
  --workload-repo-url https://github.com/myorg/team-api-workloads \
  --workload-repo-path deploy/k8s \
  --workload-repo-revision main \
  --auto-commit
```

| Flag | Default | Description |
|------|---------|-------------|
| `--workload-repo-url` | *(this repo)* | Git URL for workload definitions |
| `--workload-repo-base-path` | *(empty)* | Prefix path in the repo (e.g. `clusters/dev`) |
| `--workload-repo-path` | `workloads` | Directory containing actual manifests |
| `--workload-repo-revision` | `main` | Git branch or tag to track |

#### Custom networking and egress

```bash
# Database + Redis access from the vCluster namespace
hctl vcluster create data-team --preset dev \
  --enable-nfs \
  --extra-egress postgres:10.0.1.50/32:5432 \
  --extra-egress redis:10.0.1.60/32:6379:TCP \
  --subnet 10.0.4.0/24 \
  --vip 10.0.4.215 \
  --auto-commit
```

#### Production preset (HA, etcd, persistence)

```bash
hctl vcluster create my-prod \
  --preset prod \
  --replicas 3 \
  --hostname my-prod.cluster.integratn.tech \
  --isolation strict \
  --persistence --persistence-size 20Gi \
  --cluster-label team=backend \
  --cluster-annotation owner=platform-team \
  --chart-version 0.31.1 \
  --auto-commit
```

#### Monitoring & access

```bash
# Watch provisioning progress (status contract + diagnostic chain)
hctl vcluster status dev-team-1

# Once ready, extract kubeconfig
hctl vcluster kubeconfig dev-team-1
# => Written to ~/.kube/hctl/dev-team-1.yaml

# Set KUBECONFIG and verify
export KUBECONFIG=$(hctl vcluster kubeconfig dev-team-1)
kubectl get nodes
kubectl get namespaces
```

**Expected Duration:** 5-10 minutes (including ArgoCD sync and pod readiness)

#### Share kubeconfig with team

```bash
# Upload to 1Password (secure sharing)
op document create ~/.kube/hctl/dev-team-1.yaml \
  --title "dev-team-1-kubeconfig" \
  --vault homelab

# Or share via secure channel (avoid email/Slack)
```

#### Advanced: force-sync after creation

```bash
# If ArgoCD apps need a nudge after provisioning
hctl vcluster sync dev-team-1
hctl vcluster sync dev-team-1 --force  # sync all apps, not just failed
```

#### Full flag reference

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--preset` | string | `dev` | Base sizing preset (`dev`, `prod`) |
| `--replicas` | int | preset | Control plane replica count |
| `--hostname` | string | `<name>.<domain>` | External API hostname |
| `--environment` | string | `production` | ArgoCD environment label |
| `--k8s-version` | string | `v1.34.3` | Kubernetes version |
| `--isolation` | string | `standard` | Isolation mode (`standard`, `strict`) |
| `--subnet` | string | | CIDR subnet for VIP allocation |
| `--vip` | string | | Static VIP for vCluster API |
| `--api-port` | int | `443` | API port |
| `--persistence` | bool | preset | Enable control plane persistence |
| `--persistence-size` | string | | Volume size (e.g. `10Gi`) |
| `--storage-class` | string | | Storage class for volumes |
| `--enable-nfs` | bool | `false` | Enable NFS egress |
| `--extra-egress` | string[] | | `name:cidr:port[:protocol]` (repeatable) |
| `--coredns-replicas` | int | preset | CoreDNS replicas |
| `--workload-repo-url` | string | *(this repo)* | Workload Git URL |
| `--workload-repo-base-path` | string | | Prefix path in workload repo |
| `--workload-repo-path` | string | `workloads` | Manifest directory |
| `--workload-repo-revision` | string | `main` | Branch/tag to track |
| `--cluster-label` | string[] | | ArgoCD cluster label `key=value` (repeatable) |
| `--cluster-annotation` | string[] | | ArgoCD cluster annotation `key=value` (repeatable) |
| `--chart-version` | string | `0.31.0` | vCluster Helm chart version |
| `--auto-commit` | bool | `false` | Commit and push immediately |

<details>
<summary>Platform engineer reference: what hctl generates under the hood</summary>

The CLI produces a `VClusterOrchestratorV2` resource equivalent to:

```yaml
apiVersion: platform.integratn.tech/v1alpha1
kind: VClusterOrchestratorV2
metadata:
  name: dev-team-1
  namespace: platform-requests
spec:
  name: dev-team-1
  targetNamespace: dev-team-1
  projectName: dev-team-1
  vcluster:
    preset: dev
    isolationMode: standard
    coredns:
      replicas: 1
  integrations:
    certManager:
      clusterIssuerSelectorLabels:
        integratn.tech/cluster-issuer: letsencrypt-prod
    externalSecrets:
      clusterStoreSelectorLabels:
        integratn.tech/cluster-secret-store: onepassword-store
    argocd:
      environment: production
      workloadRepo:
        url: https://github.com/myorg/team-api-workloads
        path: deploy/k8s
        revision: main
  exposure:
    hostname: dev-team-1.cluster.integratn.tech
    apiPort: 443
  argocdApplication:
    repoURL: https://charts.loft.sh
    chart: vcluster
    targetRevision: 0.31.0
  networkPolicies:
    enableNFS: false
```

You can always inspect the generated file at `platform/vclusters/<name>.yaml`
before it is committed (use `gitMode: prompt` in your `hctl` config).

</details>

### Workflow 4: Update Promise Pipeline

**Scenario:** Fix bug in vCluster orchestrator v2 pipeline.

**Steps:**
```bash
# 1. Edit pipeline code (v2 uses single pipeline image)
vi promises/vcluster-orchestrator-v2/workflows/...

# Make code changes
# ...

# 2. Commit and push
git add promises/vcluster-orchestrator-v2/
git commit -m "Fix vCluster orchestrator v2 pipeline bug"
git push

# 3. Wait for GitHub Actions to build new image
gh run watch
# ✓ Build promise pipeline images (main) 3m 45s

# 4. Refresh Promise (forces image pull)
kubectl annotate promise vcluster-orchestrator-v2 \
  kratix.io/refresh-at="$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  --overwrite

# 5. Trigger re-execution of existing ResourceRequests
kubectl annotate vclusterorchestratorv2 vcluster-media -n platform-requests \
  platform.integratn.tech/reconcile-at="$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  --overwrite

# 6. Watch new pipeline pod (v2 single pipeline)
kubectl get pods -n platform-requests | grep vcluster-media-vco-v2-configure
kubectl logs -n platform-requests <new-pipeline-pod> -f

# 7. Verify fix applied
kubectl get vclustercore media -n platform-requests -o yaml
# Check rendered output matches expected values
```

**Expected Duration:** 10-15 minutes (includes image build time)

## Incident Response

### Incident: ArgoCD Application OutOfSync

**Symptoms:**
- ArgoCD dashboard shows application OutOfSync
- Resources in cluster don't match Git state

**Triage Steps:**
```bash
# 1. Check application status
argocd app get <app-name>

# Output shows:
# Status:       OutOfSync
# Sync Status:  OutOfSync
# Health Status: Healthy

# 2. View differences
argocd app diff <app-name>

# 3. Check last sync time
argocd app get <app-name> -o json | jq '.status.operationState.finishedAt'
```

**Resolution Option 1: Git Changed (Expected)**
```bash
# Sync to apply Git changes
argocd app sync <app-name>

# Enable auto-sync if desired
argocd app set <app-name> --sync-policy automated
```

**Resolution Option 2: Manual Drift (Unexpected)**
```bash
# Someone `kubectl apply`'d directly - bad practice!

# Hard sync (replace cluster state with Git)
argocd app sync <app-name> --force --replace

# Enable self-heal to prevent future drift
argocd app set <app-name> --self-heal
```

**Resolution Option 3: Resource Externally Modified**
```bash
# Example: Kubernetes operator modified resource

# Add ignoreDifferences to Application
cat > addons/clusters/the-cluster/addons/my-app/ignore-diff-patch.yaml <<EOF
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: my-app-the-cluster
  namespace: argocd
spec:
  ignoreDifferences:
    - group: apps
      kind: Deployment
      jsonPointers:
        - /spec/replicas  # Ignore HPA modifications
EOF

git add addons/
git commit -m "Ignore HPA-managed replica count for my-app"
git push

argocd app sync application-sets-control-plane
```

### Incident: Pods CrashLooping

**Symptoms:**
- Application unhealthy in ArgoCD
- Pods show CrashLoopBackOff status

**Triage Steps:**
```bash
# 1. Identify failing pods
kubectl get pods -n <namespace>

# NAME                     READY   STATUS             RESTARTS   AGE
# myapp-abc123-xyz         0/1     CrashLoopBackOff   5          3m

# 2. Check pod logs
kubectl logs -n <namespace> myapp-abc123-xyz

# 3. Check previous container logs (if restarted)
kubectl logs -n <namespace> myapp-abc123-xyz --previous

# 4. Describe pod for events
kubectl describe pod -n <namespace> myapp-abc123-xyz

# Look for:
# - Image pull errors
# - Resource limit issues (OOMKilled)
# - Volume mount failures
# - Liveness/readiness probe failures

# 5. Check resource usage
kubectl top pod -n <namespace> myapp-abc123-xyz
```

**Resolution by Cause:**

**OOMKilled (Out of Memory):**
```bash
# Increase memory limit
yq eval '.resources.limits.memory = "2Gi"' -i \
  addons/clusters/the-cluster/addons/my-app/values.yaml

git commit -am "Increase my-app memory limit to 2Gi"
git push
argocd app sync my-app-the-cluster
```

**Image Pull Error:**
```bash
# Check ImagePullSecrets
kubectl describe pod -n <namespace> myapp-abc123-xyz | grep -A5 Events
# Warning  Failed     12s   kubelet  Failed to pull image "ghcr.io/private/image:latest": failed to authorize

# Verify secret exists
kubectl get secret ghcr-login-secret -n <namespace>

# If missing, create from GHCR credentials
kubectl create secret docker-registry ghcr-login-secret \
  --docker-server=ghcr.io \
  --docker-username=<username> \
  --docker-password=<PAT> \
  -n <namespace>

# Add to ServiceAccount
kubectl patch serviceaccount default -n <namespace> \
  -p '{"imagePullSecrets": [{"name": "ghcr-login-secret"}]}'
```

**Liveness Probe Failing:**
```bash
# Check probe configuration
kubectl get pod -n <namespace> myapp-abc123-xyz -o yaml | yq eval '.spec.containers[0].livenessProbe'

# Adjust probe settings if too aggressive
yq eval '.livenessProbe.initialDelaySeconds = 60' -i values.yaml
yq eval '.livenessProbe.timeoutSeconds = 10' -i values.yaml

git commit -am "Adjust my-app liveness probe timing"
git push
argocd app sync my-app-the-cluster
```

### Incident: Service Unreachable

**Symptoms:**
- Service exists but connections time out
- HTTPRoute shows "Accepted: False"

**Triage Steps:**
```bash
# 1. Check service exists
kubectl get svc -n <namespace> my-service

# 2. Check endpoints (pods backing service)
kubectl get endpoints -n <namespace> my-service

# If no endpoints, service selector might be wrong
kubectl get svc -n <namespace> my-service -o yaml | yq eval '.spec.selector'
kubectl get pods -n <namespace> -l app=myapp --show-labels

# 3. Check HTTPRoute status
kubectl get httproute -n <namespace> my-service-route -o yaml

# Look at status.conditions
# - Accepted: True/False (Gateway accepts route)
# - ResolvedRefs: True/False (Backend service exists)

# 4. Test service from within cluster
kubectl run debug --rm -it --image=nicolaka/netshoot -- bash
curl http://my-service.<namespace>.svc.cluster.local
```

**Resolution by Cause:**

**Service selector mismatch:**
```bash
# Fix selector in values
yq eval '.service.selector.app = "correct-label"' -i values.yaml
git commit -am "Fix my-service selector"
git push
argocd app sync my-app-the-cluster
```

**HTTPRoute not accepted:**
```bash
# Check Gateway status
kubectl get gateway -n nginx-gateway-fabric cluster-gateway

# Verify HTTPRoute references correct Gateway
kubectl get httproute -n <namespace> my-service-route -o yaml | \
  yq eval '.spec.parentRefs'

# Should reference:
# - name: cluster-gateway
#   namespace: nginx-gateway-fabric

# Fix if wrong
kubectl patch httproute -n <namespace> my-service-route --type=json -p='[
  {
    "op": "replace",
    "path": "/spec/parentRefs/0/name",
    "value": "cluster-gateway"
  },
  {
    "op": "replace",
    "path": "/spec/parentRefs/0/namespace",
    "value": "nginx-gateway-fabric"
  }
]'
```

**TLS certificate not ready:**
```bash
# Check certificate status
kubectl get certificate -n <namespace>

# NAME                READY   SECRET              AGE
# my-service-cert     False   my-service-tls      2m

kubectl describe certificate -n <namespace> my-service-cert
# Check status for errors

# Common issues:
# - DNS validation failing (check external-dns logs)
# - ClusterIssuer not found (check cert-manager)
# - Rate limit exceeded (use staging issuer)

# Force renewal
kubectl delete certificaterequest -n <namespace> <request-name>
kubectl annotate certificate -n <namespace> my-service-cert \
  cert-manager.io/issue-temporary-certificate="true" \
  --overwrite
```

### Incident: Prometheus Disk Full

**Symptoms:**
- Prometheus pod restarting
- Metrics queries timing out
- Alert: "PrometheusDiskSpaceUsage"

**Triage Steps:**
```bash
# 1. Check PVC usage
kubectl exec -n monitoring prometheus-kube-prometheus-stack-prometheus-0 -- \
  df -h /prometheus

# Filesystem      Size  Used Avail Use% Mounted on
# /dev/sdb        30G   29G   1.0G  97% /prometheus

# 2. Check retention settings
kubectl get prometheus -n monitoring kube-prometheus-stack-prometheus \
  -o yaml | yq eval '.spec.retention'
# 15d

# 3. Check data size
kubectl exec -n monitoring prometheus-kube-prometheus-stack-prometheus-0 -- \
  du -sh /prometheus
# 28G
```

**Resolution Option 1: Increase PVC Size**
```bash
# Edit values
yq eval '.prometheus.prometheusSpec.storageSpec.volumeClaimTemplate.spec.resources.requests.storage = "50Gi"' -i \
  addons/cluster-roles/control-plane/addons/kube-prometheus-stack/values.yaml

git commit -am "Increase Prometheus storage to 50Gi"
git push
argocd app sync kube-prometheus-stack-the-cluster

# Note: Requires StorageClass with allowVolumeExpansion: true
# May need to restart pod for resize to take effect
kubectl delete pod -n monitoring prometheus-kube-prometheus-stack-prometheus-0
```

**Resolution Option 2: Reduce Retention**
```bash
# Shorten retention period
yq eval '.prometheus.prometheusSpec.retention = "7d"' -i \
  addons/cluster-roles/control-plane/addons/kube-prometheus-stack/values.yaml

git commit -am "Reduce Prometheus retention to 7 days"
git push
argocd app sync kube-prometheus-stack-the-cluster

# Wait for Prometheus to clean up old data
kubectl logs -n monitoring prometheus-kube-prometheus-stack-prometheus-0 -f | grep compact
```

**Resolution Option 3: Reduce Cardinality**
```bash
# Find high-cardinality metrics
kubectl port-forward -n monitoring svc/kube-prometheus-stack-prometheus 9090:9090
open http://localhost:9090/tsdb-status

# Look at "Top 10 label names with value count"
# Example: pod_name has 1000+ unique values

# Drop expensive metrics via relabel config
cat > metric-drop-config.yaml <<EOF
prometheus:
  prometheusSpec:
    additionalScrapeConfigs:
      - job_name: 'kubernetes-pods'
        metric_relabel_configs:
          - source_labels: [__name__]
            regex: 'expensive_metric_.*'
            action: drop
EOF

yq eval-all 'select(fileIndex == 0) * select(fileIndex == 1)' \
  addons/cluster-roles/control-plane/addons/kube-prometheus-stack/values.yaml \
  metric-drop-config.yaml > values-new.yaml

mv values-new.yaml addons/cluster-roles/control-plane/addons/kube-prometheus-stack/values.yaml
git commit -am "Drop expensive metrics from Prometheus"
git push
argocd app sync kube-prometheus-stack-the-cluster
```

## Backup and Recovery

### Backup Strategy

**What to Backup:**
1. **Git repositories** (most critical)
   - gitops_homelab_2_0 (this repo)
   - kratix-platform-state (Kratix outputs)
2. **Kubernetes secrets** (if not in 1Password)
3. **Persistent volumes** (stateful applications)
4. **Terraform state** (infrastructure config)

**Backup Frequency:**
- Git: Continuous (GitHub provides backup)
- Secrets: Weekly (via Velero or 1Password export)
- PVs: Daily (via Velero or snapshot)
- Terraform state: After every `apply` (PostgreSQL backend with versioning)

### Velero Backup (Future Implementation)

**Install Velero:**
```yaml
# Add to addons.yaml
velero:
  enabled: true
  namespace: velero
  defaultVersion: "7.2.1"
  chartRepository: "https://vmware-tanzu.github.io/helm-charts"
  selector:
    matchExpressions:
      - key: cluster_role
        operator: In
        values: ['control-plane']
```

**Create Backup Schedule:**
```bash
velero schedule create daily-backup \
  --schedule="0 2 * * *" \
  --include-namespaces "*" \
  --exclude-namespaces "kube-system,kube-public" \
  --ttl 720h0m0s  # 30 days retention

# Verify schedule created
velero schedule get
```

**Manual Backup:**
```bash
velero backup create manual-backup-$(date +%Y%m%d) \
  --include-namespaces vcluster-media,monitoring

# Check backup status
velero backup describe manual-backup-20260123
```

**Restore from Backup:**
```bash
# List available backups
velero backup get

# Restore specific backup
velero restore create --from-backup daily-backup-20260123

# Monitor restore
velero restore describe <restore-name>
```

### Disaster Recovery Scenario: Complete Cluster Loss

**Scenario:** Control plane nodes failed, cluster unrecoverable.

**Recovery Steps:**

**Phase 1: Rebuild Cluster (60 minutes)**
```bash
# 1. Rebuild Talos nodes via PXE boot
# (Follow bootstrap.md procedures)

# 2. Bootstrap etcd
talosctl --nodes 10.0.4.101 bootstrap

# 3. Verify cluster healthy
kubectl get nodes
# All nodes Ready

# 4. Verify PVs available (if using external storage)
kubectl get pv
```

**Phase 2: Bootstrap GitOps (30 minutes)**
```bash
# 1. Clone gitops repo
git clone https://github.com/jamesatintegratnio/gitops_homelab_2_0
cd gitops_homelab_2_0

# 2. Run Terraform bootstrap
cd terraform/cluster/
tofu init -backend-config=backend.hcl
tofu apply

# 3. Wait for ArgoCD ready
kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=argocd-server -n argocd --timeout=300s

# 4. Sync all applications
argocd app sync -l app.kubernetes.io/instance=argocd
```

**Phase 3: Restore Data (Variable)**
```bash
# 1. Restore from Velero backup (if available)
velero restore create disaster-recovery --from-backup daily-backup-20260122

# 2. Restore secrets from 1Password
# ExternalSecrets will auto-sync from 1Password

# 3. Restore PV data from snapshots/backups
# (Storage-specific procedure)

# 4. Verify applications healthy
argocd app list
kubectl get pods -A
```

**Total Recovery Time:** ~2-3 hours

## Upgrade Procedures

### Upgrade Kubernetes Version (Talos)

**Preparation:**
```bash
# 1. Check current version
talosctl --nodes 10.0.4.101 version
# Client: v1.11.5
# Server: v1.11.5

# 2. Review release notes
open https://www.talos.dev/v1.12/introduction/what-is-new/

# 3. Backup cluster state
velero backup create pre-k8s-upgrade-backup --wait
```

**Upgrade (Rolling):**
```bash
# 1. Upgrade one control plane node at a time
talosctl --nodes 10.0.4.101 upgrade-k8s --to 1.35.0

# 2. Wait for node to rejoin
kubectl wait --for=condition=ready node controlplane1 --timeout=600s

# 3. Verify etcd cluster healthy
talosctl --nodes 10.0.4.101,10.0.4.102,10.0.4.103 etcd members

# 4. Repeat for remaining control plane nodes
talosctl --nodes 10.0.4.102 upgrade-k8s --to 1.35.0
# ... wait ...
talosctl --nodes 10.0.4.103 upgrade-k8s --to 1.35.0

# 5. Verify cluster health
kubectl get nodes
# All nodes should show v1.35.0

kubectl get pods -A | grep -v Running
# Should be empty
```

**Rollback:**
```bash
# Downgrade is not supported
# Restore from pre-upgrade backup if issues occur
velero restore create --from-backup pre-k8s-upgrade-backup
```

### Upgrade ArgoCD Version

**Preparation:**
```bash
# 1. Check current version
kubectl get deployment -n argocd argocd-server \
  -o jsonpath='{.spec.template.spec.containers[0].image}'
# quay.io/argoproj/argocd:v2.13.0

# 2. Review upgrade notes
open https://argo-cd.readthedocs.io/en/stable/operator-manual/upgrading/overview/

# 3. Check for breaking changes
# Especially CRD schema changes
```

**Upgrade:**
```bash
# 1. Update Terraform chart version
vi terraform/cluster/main.tf

module "argocd" {
  argocd = {
    chart_version = "9.1.0"  # Changed from 9.0.3
  }
}

# 2. Plan and apply
cd terraform/cluster/
tofu plan
tofu apply

# 3. Watch rollout
kubectl rollout status deployment -n argocd argocd-server

# 4. Verify ArgoCD operational
argocd version
# argocd: v2.14.0+unknown
# argocd-server: v2.14.0+unknown

argocd app list
# Should show all applications
```

**Rollback:**
```bash
# Revert Terraform change
git revert HEAD
cd terraform/cluster/
tofu apply

# ArgoCD will rollback to previous version
kubectl rollout status deployment -n argocd argocd-server
```

## Monitoring and Alerting

### Key Metrics to Watch

**Cluster Health:**
- Node CPU/memory usage (`node_cpu_seconds_total`, `node_memory_MemAvailable_bytes`)
- Pod restart rate (`kube_pod_container_status_restarts_total`)
- Failed pods (`kube_pod_status_phase{phase="Failed"}`)

**GitOps Health:**
- ArgoCD sync failures (`argocd_app_sync_total{phase="Error"}`)
- Application out-of-sync count (`argocd_app_info{sync_status="OutOfSync"}`)

**Resource Saturation:**
- Disk usage (`node_filesystem_avail_bytes`)
- Network errors (`node_network_transmit_errs_total`)
- PVC usage (`kubelet_volume_stats_used_bytes` / `kubelet_volume_stats_capacity_bytes`)

### Critical Alerts

**Alert: PodCrashLooping**
```yaml
alert: PodCrashLooping
expr: rate(kube_pod_container_status_restarts_total[15m]) > 0
for: 5m
annotations:
  summary: "Pod {{ $labels.namespace }}/{{ $labels.pod }} is crash looping"
  description: "Pod has restarted {{ $value }} times in the last 15 minutes"
```

**Response:**
```bash
kubectl logs -n {{ $labels.namespace }} {{ $labels.pod }} --previous
kubectl describe pod -n {{ $labels.namespace }} {{ $labels.pod }}
```

**Alert: NodeDiskPressure**
```yaml
alert: NodeDiskPressure
expr: node_filesystem_avail_bytes{mountpoint="/"} / node_filesystem_size_bytes{mountpoint="/"} < 0.10
for: 5m
annotations:
  summary: "Node {{ $labels.instance }} disk usage above 90%"
  description: "Only {{ $value | humanizePercentage }} disk space remaining"
```

**Response:**
```bash
# Find large directories
kubectl debug node/{{ $labels.instance }} -it --image=ubuntu -- bash
du -sh /* | sort -h

# Common culprits:
# - /var/lib/containerd (old images)
# - /var/log (large log files)
# - /tmp (forgotten temp files)

# Clean up
crictl rmi --prune  # Remove unused images
journalctl --vacuum-time=7d  # Trim old logs
```

## Performance Tuning

### Optimize Prometheus Performance

**Reduce scrape interval:**
```yaml
prometheus:
  prometheusSpec:
    scrapeInterval: "60s"  # From default 30s
    evaluationInterval: "60s"
```

**Limit metric retention:**
```yaml
prometheus:
  prometheusSpec:
    retention: "7d"  # From default 15d
    retentionSize: "20GB"  # Hard limit
```

**Sample metrics (reduce precision):**
```yaml
prometheus:
  prometheusSpec:
    query:
      maxSamples: 50000000  # Limit query complexity
```

### Optimize ArgoCD Performance

**Increase resource limits:**
```yaml
# In Terraform argocd module
argocd = {
  values = {
    controller:
      resources:
        limits:
          cpu: "2000m"
          memory: "4Gi"
    repoServer:
      resources:
        limits:
          cpu: "1000m"
          memory: "2Gi"
  }
}
```

**Enable application sharding:**
```yaml
controller:
  replicas: 3
  env:
    - name: ARGOCD_CONTROLLER_REPLICAS
      value: "3"
```

### Optimize Kratix Pipeline Execution

**Increase pipeline concurrency:**
```yaml
# In Kratix values
kratix:
  platformController:
    resources:
      limits:
        cpu: "2000m"
        memory: "2Gi"
    env:
      - name: MAX_CONCURRENT_PIPELINES
        value: "5"  # From default 3
```

## Matrix Alerting

### Alert Flow Architecture

Alerts flow through the following path:

```
Prometheus → Alertmanager → matrix-alertmanager-receiver → Matrix Room
```

1. **Prometheus** evaluates alert rules every 60s
2. **Alertmanager** groups and routes firing alerts to the receiver webhook
3. **matrix-alertmanager-receiver** (deployed as a Deployment in `monitoring` namespace) converts alerts to Matrix messages with rich HTML formatting
4. Alert messages include:
   - Alert name, severity, and description
   - Source link to Prometheus
   - **Logs link** - clickable URL to Grafana Explore with pre-filtered Loki query (see [Loki Log Correlation](#loki-log-correlation))

### Checking Receiver Health

```bash
# Verify receiver pod is running
kubectl get pods -n monitoring -l app=matrix-alertmanager-receiver

# Check receiver logs for delivery status
kubectl logs -n monitoring -l app=matrix-alertmanager-receiver --tail=50

# Verify ExternalSecret for Matrix credentials
kubectl get externalsecret -n monitoring matrix-alertmanager-receiver-secrets
# Should show READY: True

# Test connectivity to Matrix server
kubectl exec -n monitoring deploy/matrix-alertmanager-receiver -- \
  wget -qO- --spider https://matrix.integratn.tech/_matrix/client/versions
```

### Troubleshooting Alert Delivery

**Alerts not appearing in Matrix room:**

```bash
# 1. Check Alertmanager is routing to receiver
kubectl port-forward -n monitoring svc/kube-prometheus-stack-alertmanager 9093:9093
# Open http://localhost:9093/#/status - check receiver config

# 2. Check for firing alerts
kubectl port-forward -n monitoring svc/kube-prometheus-stack-prometheus 9090:9090
# Open http://localhost:9090/alerts - verify alerts are firing

# 3. Check receiver logs for errors
kubectl logs -n monitoring -l app=matrix-alertmanager-receiver -f
# Look for: HTTP errors, auth failures, room ID issues

# 4. Verify Matrix credentials are current
kubectl get secret -n monitoring matrix-alertmanager-receiver-secrets \
  -o jsonpath='{.data.MATRIX_HOMESERVER}' | base64 -d
```

**Common issues:**
- ExternalSecret not synced → Check 1Password Connect is healthy
- Matrix token expired → Rotate token in 1Password, delete ExternalSecret to force re-sync
- Room ID wrong → Verify room ID in 1Password matches the intended Matrix room

## Loki Log Correlation

### How It Works

All 25 Prometheus alert rules include a `logs_url` annotation that generates a clickable link to Grafana Explore with a pre-filtered Loki query. When an alert fires:

1. Prometheus templates the `logs_url` with the alert's namespace/pod labels
2. Alertmanager passes the URL to the Matrix receiver
3. The Matrix message includes a **"Logs"** link
4. Clicking opens Grafana Explore with a LogQL query scoped to the relevant namespace/pod

### URL Pattern

Alert rules use URL-encoded `logs_url` annotations:

```yaml
annotations:
  logs_url: >-
    https://grafana.cluster.integratn.tech/explore?schemaVersion=1&panes=%7B%22pane%22%3A%7B%22datasource%22%3A%22loki%22%2C%22queries%22%3A%5B%7B%22refId%22%3A%22A%22%2C%22expr%22%3A%22%7Bnamespace%3D%5C%22{{ $labels.namespace }}%5C%22%7D%22%2C%22queryType%22%3A%22range%22%7D%5D%7D%7D&orgId=1
```

The `left=` parameter is **URL-encoded** so that Alertmanager and Matrix clients correctly auto-detect the full URL as a single clickable link.

### Verifying Log Correlation

```bash
# Check that alert rules have logs_url annotations
kubectl get prometheusrule -n monitoring kube-prometheus-stack-custom-alerts \
  -o yaml | grep -c logs_url
# Should return 25 (all rules have logs_url)

# Verify Loki datasource is available in Grafana
kubectl port-forward -n monitoring svc/kube-prometheus-stack-grafana 3000:80
# Open http://localhost:3000/connections/datasources - verify "loki" datasource exists with UID "loki"
```

### Adding logs_url to New Alert Rules

When creating new PrometheusRules, include the `logs_url` annotation:

```yaml
- alert: MyNewAlert
  expr: my_metric > threshold
  for: 5m
  labels:
    severity: warning
  annotations:
    summary: "Description of {{ $labels.pod }}"
    logs_url: >-
      https://grafana.cluster.integratn.tech/explore?schemaVersion=1&panes=%7B%22pane%22%3A%7B%22datasource%22%3A%22loki%22%2C%22queries%22%3A%5B%7B%22refId%22%3A%22A%22%2C%22expr%22%3A%22%7Bnamespace%3D%5C%22{{ $labels.namespace }}%5C%22%7D%22%2C%22queryType%22%3A%22range%22%7D%5D%7D%7D&orgId=1
```

> **Important:** The `left=` JSON value must be URL-encoded. Raw JSON characters (`{`, `}`, `"`, etc.) break URL auto-detection in Alertmanager UI and Matrix HTML messages.

## Key Files Reference

- **ArgoCD Configuration**: [terraform/cluster/main.tf](../terraform/cluster/main.tf)
- **Prometheus Values**: [addons/cluster-roles/control-plane/addons/kube-prometheus-stack/values.yaml](../addons/cluster-roles/control-plane/addons/kube-prometheus-stack/values.yaml)
- **Grafana Dashboards**: [addons/cluster-roles/control-plane/addons/kube-prometheus-stack/dashboards/](../addons/cluster-roles/control-plane/addons/kube-prometheus-stack/dashboards/)
- **Alert Rules (Custom)**: Check `kube-prometheus-stack` addon values (`additionalPrometheusRulesMap`)
- **Matrix Alertmanager Receiver**: [addons/cluster-roles/control-plane/addons/matrix-alertmanager-receiver/](../addons/cluster-roles/control-plane/addons/matrix-alertmanager-receiver/)
- **Loki Values**: [addons/cluster-roles/control-plane/addons/loki/values.yaml](../addons/cluster-roles/control-plane/addons/loki/values.yaml)
- **Promtail Values**: [addons/cluster-roles/control-plane/addons/promtail/values.yaml](../addons/cluster-roles/control-plane/addons/promtail/values.yaml)
- **Backup Schedules**: (Future: Velero addon configuration)
