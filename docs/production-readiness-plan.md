# Production Readiness Plan

> **Assessment Date:** February 18, 2026
> **Framework:** Based on production readiness criteria covering Observability, Security, Infrastructure Lifecycling, Change Management, Team Readiness, Cost Controls, and Cultural Readiness.

## Overview

This plan captures the current maturity of the platform across seven production readiness criteria areas, identifies gaps, and defines actionable phases to close them. Each phase is ordered by **impact vs effort** — quick wins first, larger projects later.

**Current Maturity Summary:**

| Category | Rating | Key Strengths | Primary Gaps |
|----------|--------|---------------|--------------|
| Observability | Strong | Full metrics/logging/alerting, hub-spoke federation, dashboards with deep-links | No tracing, 15d retention, single alert channel |
| Security | Moderate | Kyverno, NetworkPolicies, ExternalSecrets, Talos, pre-commit hooks | Cilium audit-only, no image scanning, no runtime security |
| Infra Lifecycling | Moderate | Terraform/OpenTofu, DR plan documented, GitOps rollback | Velero not deployed, limited autoscaling |
| Change Management | Moderate | ArgoCD sync waves, selfHeal, retry policies, CI secret scanning | No progressive delivery, no approval gates, minimal CI |
| Team Readiness | Strong | 12 docs (1000s of lines), hctl CLI, Matrix alerts with Grafana links | Single notification channel, no structured playbooks |
| Cost Controls | Moderate | NFS SSD/HDD tiers, VPA Auto mode, Goldilocks dashboard | No ResourceQuota, LimitRange, or cost monitoring |
| Cultural Readiness | Strong | Git-first discipline, platform engineering patterns (Kratix) | — |

---

## Phase 1: Security Enforcement (Quick Wins)

**Goal:** Turn existing security infrastructure from logging to enforcing.
**Effort:** Low | **Impact:** High
**Timeline:** 1-2 sessions

### 1.1 Switch Cilium to Enforce Mode

**Current State:** `policyAuditMode: true` in [addons/environments/production/addons/cilium/values.yaml](../addons/environments/production/addons/cilium/values.yaml) — all 17 NetworkPolicy files log but don't block traffic.

**Steps:**
1. Review Hubble flow logs in Grafana to confirm no legitimate traffic would be blocked
   ```bash
   # Check for denied flows that would have been dropped
   kubectl exec -n kube-system ds/cilium -- hubble observe --verdict AUDIT --last 1000
   ```
2. Identify any namespaces missing a NetworkPolicy (Kyverno audit report)
   ```bash
   kubectl get polr -A -o json | jq '.items[] | select(.results[]?.result == "fail") | .scope.name'
   ```
3. Add missing NetworkPolicies for any uncovered namespaces
4. Switch to enforce mode:
   ```yaml
   # cilium values.yaml
   policyAuditMode: false
   ```
5. Monitor Hubble for unexpected drops over 24-48 hours
6. Roll back to audit if critical traffic is blocked

**Acceptance Criteria:**
- [x] `policyAuditMode: false` committed and synced _(commit ee491b2)_
- [x] No legitimate traffic dropped (verified via Hubble) _(zero drops post-enforce)_
- [x] All namespaces have at least a default-deny + allow-needed policy _(5 NetworkPolicy gaps fixed: commits 3444757, 4948f60, 3b7125f)_

### 1.2 Deploy Trivy Operator for Image Scanning

**Current State:** No vulnerability scanning. Images deploy without CVE checks.

**Steps:**
1. Add Trivy Operator as a control-plane addon:
   ```
   addons/cluster-roles/control-plane/addons/trivy-operator/
   ├── values.yaml
   ```
2. Add to control-plane addons.yaml with appropriate sync-wave
3. Configure scan targets: workload images, infra images
4. Add Grafana dashboard for vulnerability overview (community dashboard gnetId 17813)
5. Add PrometheusRule for critical CVE alerts:
   ```yaml
   - alert: CriticalVulnerabilityDetected
     expr: trivy_image_vulnerabilities{severity="Critical"} > 0
     for: 1h
     labels:
       severity: warning
   ```

**Acceptance Criteria:**
- [x] Trivy Operator scanning all namespaces _(excludes kube-system, kube-public, kube-node-lease)_
- [x] Grafana dashboard showing vulnerability summary _(Security/Trivy Operator Dashboard)_
- [x] Alert fires for Critical CVEs _(TrivyCriticalVulnerabilityDetected, TrivyHighVulnerabilityCount, TrivyOperatorDown)_

### 1.3 Strengthen Kyverno to Enforce Mode

**Current State:** Single ClusterPolicy in `Audit` mode only for namespace NetworkPolicy checks.

**Steps:**
1. Review current Kyverno policy reports for violations
   ```bash
   kubectl get polr -A --no-headers | wc -l
   kubectl get polr -A -o json | jq '[.items[].results[] | select(.result == "fail")] | length'
   ```
2. Add enforce-mode policies incrementally (one at a time, validate between each):
   - **Require resource limits** — block pods without `resources.requests` and `resources.limits`
   - **Disallow privileged containers** — block `securityContext.privileged: true`
   - **Require labels** — enforce `app.kubernetes.io/name` and `app.kubernetes.io/managed-by` labels
   - **Restrict image registries** — allow only known registries (ghcr.io, docker.io, quay.io, registry.k8s.io)
3. Exempt system namespaces (kube-system, kube-public, argocd) with `exclude` rules
4. Switch existing namespace-NetworkPolicy audit policy to `Enforce` after Phase 1.1

**Acceptance Criteria:**
- [x] At least 3 enforce-mode ClusterPolicies active _(require-default-deny-netpol, disallow-privileged-containers, disallow-default-namespace — commits e3e96ba, 6cdad3b)_
- [x] System namespaces exempted _(kube-system, kube-public, kube-node-lease, cilium-secrets, kratix-worker-system excluded per policy)_
- [x] No legitimate workloads blocked _(zero ClusterPolicyReport failures, restrict-image-registries in Audit mode for discovery)_

---

## Phase 2: Backup & Recovery — SKIPPED

**Reason:** All PVs persist on the NFS server, and the cluster is fully declarative via GitOps. The NFS server provides data persistence, and the cluster can be rebuilt from git. Velero adds complexity without proportional value for this homelab setup.

**Mitigations already in place:**
- PV data survives cluster rebuilds (NFS-backed storage classes)
- All cluster state is in git (ArgoCD + GitOps)
- Talos machine configs are versioned in git
- etcd can be rebuilt from Talos bootstrap

---

## Phase 2.5: Resource Right-Sizing with Goldilocks + VPA — COMPLETE

**Goal:** Deploy VPA in Auto mode with Goldilocks dashboard for resource right-sizing across all workloads. ArgoCD configured to ignore VPA-managed resource drift.
**Effort:** Low-Medium | **Impact:** High
**Timeline:** Completed February 18, 2026

### What Was Deployed

- **Goldilocks** (chart 10.2.0, app v4.14.1) — creates VPA objects for every workload, provides web dashboard
- **VPA** (all 3 components via subchart):
  - **Recommender**: Calculates resource recommendations from metrics
  - **Updater**: Evicts pods when resources need adjustment
  - **Admission Controller**: Mutates pod resource requests/limits on creation

### Configuration

- **VPA Auto mode**: All namespaces labeled `goldilocks.fairwinds.com/vpa-update-mode=auto` (except kube-system, kube-public, kube-node-lease, goldilocks)
- **On-by-default**: Controller monitors all namespaces without requiring opt-in labels
- **ArgoCD ignoreDifferences**: Global `resource.customizations.ignoreDifferences` configured for Deployment, StatefulSet, DaemonSet `.spec.template.spec.containers[].resources` and `.spec.template.spec.initContainers[]?.resources`
- **Kyverno mutate policy**: `mutate-ns-vpa-auto-mode` automatically labels new namespaces for VPA Auto mode
- **Network policies**: default-deny + allow-dns + allow-kube-api + allow-dashboard-ingress + allow-vpa-webhook
- **Dashboard**: `https://goldilocks.cluster.integratn.tech/` via Gateway API HTTPRoute

### Key Files

- [addons/cluster-roles/control-plane/addons/goldilocks/values.yaml](../addons/cluster-roles/control-plane/addons/goldilocks/values.yaml) — Helm values
- [addons/cluster-roles/control-plane/addons/network-policies/goldilocks.yaml](../addons/cluster-roles/control-plane/addons/network-policies/goldilocks.yaml) — Network policies
- [addons/cluster-roles/control-plane/addons/network-policies/kyverno-mutate-vpa-auto.yaml](../addons/cluster-roles/control-plane/addons/network-policies/kyverno-mutate-vpa-auto.yaml) — Auto-label namespaces
- [addons/clusters/the-cluster/addons/argo-cd/values.yaml](../addons/clusters/the-cluster/addons/argo-cd/values.yaml) — ignoreDifferences config

### Issues Encountered & Resolved

1. **Kyverno enforce chicken-and-egg**: `require-default-deny-netpol` blocked namespace creation (switched to Audit + generate policy)
2. **Dashboard CrashLoop**: `--exclude-namespaces` is controller-only flag, not supported by dashboard binary
3. **VPA update mode**: No `--vpa-update-mode` controller flag exists — uses namespace label instead
4. **nginx-gateway connectivity**: Controller pod listens on 8443 but NetworkPolicy specified 443 — needed both ports for Cilium DNAT handling

**Acceptance Criteria:**
- [x] 107 VPA objects created across all namespaces in Auto mode
- [x] VPA recommender generating resource recommendations
- [x] VPA updater + admission controller actively adjusting resources
- [x] ArgoCD not reverting VPA-managed resource changes
- [x] Goldilocks dashboard accessible at `https://goldilocks.cluster.integratn.tech/`
- [x] New namespaces automatically labeled for VPA Auto mode (Kyverno mutate policy)

---

## Phase 3: Resource Governance

**Goal:** Prevent noisy-neighbor problems and establish resource boundaries.
**Effort:** Low | **Impact:** Medium
**Timeline:** 1 session

### 3.1 Add ResourceQuota to VCluster Namespaces

**Current State:** No ResourceQuota manifests exist. Any namespace can consume unlimited resources.

**Steps:**
1. Define sensible defaults per vcluster namespace:
   ```yaml
   apiVersion: v1
   kind: ResourceQuota
   metadata:
     name: default-quota
     namespace: demo  # repeat for test, vcluster-media
   spec:
     hard:
       requests.cpu: "4"
       requests.memory: 8Gi
       limits.cpu: "8"
       limits.memory: 16Gi
       persistentvolumeclaims: "20"
       pods: "100"
   ```
2. Add as part of the vcluster-orchestrator promise pipeline output (so every new vcluster gets a quota automatically)
3. Add LimitRange for sane defaults when workloads omit resource specs:
   ```yaml
   apiVersion: v1
   kind: LimitRange
   metadata:
     name: default-limits
   spec:
     limits:
     - default:
         cpu: 500m
         memory: 512Mi
       defaultRequest:
         cpu: 100m
         memory: 128Mi
       type: Container
   ```
4. Add ResourceQuota Grafana dashboard or add panels to existing cluster dashboard

**Acceptance Criteria:**
- [ ] ResourceQuota applied to all vcluster namespaces
- [ ] LimitRange applied to all vcluster namespaces
- [ ] Promise pipeline generates quotas for new vclusters
- [ ] Dashboard shows quota utilization

### 3.2 Deploy OpenCost for Visibility

**Current State:** No cost monitoring tools.

**Steps:**
1. Add OpenCost as a control-plane addon (free, Prometheus-native):
   ```
   addons/cluster-roles/control-plane/addons/opencost/
   ├── values.yaml
   ```
2. Configure Prometheus integration (scrape OpenCost metrics)
3. Add Grafana dashboard (community dashboard gnetId 15714)
4. Set custom pricing for homelab hardware (electricity cost per node, amortized hardware cost)

**Acceptance Criteria:**
- [ ] OpenCost deployed and integrated with Prometheus
- [ ] Per-namespace cost visibility in Grafana
- [ ] Custom pricing model reflecting actual homelab costs

---

## Phase 4: Observability Gaps

**Goal:** Complete the observability stack with tracing, long-term storage, and log coverage.
**Effort:** Medium | **Impact:** Medium
**Timeline:** 2-3 sessions

### 4.1 Deploy OpenTelemetry Collector + Tempo for Tracing

**Current State:** Zero tracing infrastructure. As promise pipelines and multi-service workloads grow, debugging cross-service issues will require traces.

**Steps:**
1. Deploy Tempo as a control-plane addon (pairs naturally with existing Grafana/Loki):
   ```
   addons/cluster-roles/control-plane/addons/tempo/
   ├── values.yaml
   ```
2. Deploy OpenTelemetry Collector as a DaemonSet (or sidecar) to receive traces and forward to Tempo
3. Add Tempo as a Grafana datasource (alongside Prometheus and Loki)
4. Instrument promise pipeline containers with OTEL SDK (optional, per-service)
5. Enable trace-to-logs and trace-to-metrics correlation in Grafana

**Acceptance Criteria:**
- [ ] Tempo receiving and storing traces
- [ ] Grafana can query traces via Tempo datasource
- [ ] At least one service instrumented as proof-of-concept

### 4.2 Add Long-Term Metrics Storage (Thanos or Mimir)

**Current State:** Prometheus has 15-day retention. Capacity planning and historical analysis need months of data.

**Steps:**
1. Choose approach:
   - **Thanos Sidecar** — add to existing Prometheus, store blocks in object storage
   - **Grafana Mimir** — replace Prometheus remote-write target, multi-tenant
   - **Simple: increase retention** — if storage allows, bump to 90d (easiest, least scalable)
2. Configure object storage backend (MinIO or Backblaze B2, same as Velero)
3. Update Grafana datasource to query Thanos/Mimir for long-range queries
4. Keep Prometheus for real-time queries (last 15d), Thanos for historical

**Acceptance Criteria:**
- [ ] Metrics queryable beyond 15 days
- [ ] Grafana dashboards work seamlessly across short and long time ranges
- [ ] Storage costs documented

### 4.3 Centralize VCluster Logs

**Current State:** Promtail runs on host cluster only. VCluster container logs are visible via `kubectl logs` but not in Loki.

**Steps:**
1. Add Promtail to the vcluster cluster-role addons (so every vcluster gets log collection)
2. Configure Promtail to push to the host Loki instance via the Loki gateway
3. Add vcluster labels to log streams for filtering in Grafana

**Acceptance Criteria:**
- [ ] VCluster pod logs queryable in Grafana/Loki
- [ ] Logs tagged with vcluster name and namespace

### 4.4 Add Alert Notification Redundancy

**Current State:** Alertmanager routes only to Matrix webhook. If the Matrix receiver is down, alerts are lost.

**Steps:**
1. Add at least one additional receiver to Alertmanager config:
   - **Email** (SMTP via 1Password credentials) — good for async review
   - **Slack webhook** — if using Slack
   - **PagerDuty** — if wanting escalation (likely overkill for homelab)
   - **Pushover/Ntfy** — lightweight mobile push notifications (great for homelab)
2. Configure routing: critical alerts → both Matrix + secondary channel, warnings → Matrix only
3. Add a dead man's switch / Watchdog integration to detect alerting pipeline failures

**Acceptance Criteria:**
- [ ] At least 2 independent alert delivery channels
- [ ] Critical alerts route to both channels
- [ ] Watchdog alert confirms pipeline health

---

## Phase 5: Change Management Hardening

**Goal:** Reduce risk of bad changes and improve CI coverage.
**Effort:** Medium | **Impact:** Medium
**Timeline:** 2-3 sessions

### 5.1 Expand CI Pipeline

**Current State:** CI only does YAML syntax validation and secret scanning. No Helm template testing, no policy testing, no integration tests.

**Steps:**
1. **Helm template validation** — render all addon Helm charts in CI and validate output:
   ```yaml
   # .github/workflows/validate-helm.yaml
   - name: Template addons
     run: |
       for chart in addons/charts/*/; do
         helm template test "$chart" --values "$chart/ci-values.yaml" || exit 1
       done
   ```
2. **Kyverno policy testing** — test policies against sample resources using `kyverno apply`:
   ```yaml
   - name: Test Kyverno policies
     run: kyverno apply ./policies/ --resource ./test-resources/
   ```
3. **Promise pipeline tests** — build and lint Go code for promise pipelines:
   ```yaml
   - name: Test promise builders
     run: |
       cd promises/argocd-cluster-registration
       go test ./...
   ```
4. Add CI status badges to README

**Acceptance Criteria:**
- [ ] Helm template rendering in CI for all charts
- [ ] Kyverno policy tests with sample resources
- [ ] Go tests for promise pipeline code
- [ ] CI blocks merges on failure

### 5.2 Add Manual Approval Gates for Critical Addons

**Current State:** All ArgoCD sync policies are fully automated. Cilium, cert-manager, and core networking changes auto-apply.

**Steps:**
1. Identify critical addons that warrant manual sync:
   - Cilium (CNI — misconfiguration = cluster network outage)
   - cert-manager (TLS — bad config = all HTTPS broken)
   - MetalLB (load balancer — wrong config = external access lost)
2. Set `automated: {}` (empty, disabling auto-sync) for these addons
3. Require ArgoCD UI/CLI manual sync for changes to these addons
4. Document the approval workflow in operations.md

**Acceptance Criteria:**
- [ ] Critical addons require manual sync approval
- [ ] Documented workflow for reviewing and approving changes
- [ ] Non-critical addons remain fully automated

### 5.3 Progressive Delivery (Future Consideration)

**Current State:** No Argo Rollouts or canary strategies.

**Assessment:** For a homelab, progressive delivery adds significant complexity for limited benefit. Consider only if running production workloads where zero-downtime matters.

**If pursued:**
1. Deploy Argo Rollouts
2. Convert key Deployments to Rollouts with canary strategy
3. Integrate with Prometheus for automated rollback on error rate spikes

**Recommendation:** Defer until workloads justify the complexity. Document as a future option.

---

## Phase 6: Runtime Security (Stretch Goal)

**Goal:** Detect anomalous behavior inside containers at runtime.
**Effort:** High | **Impact:** Medium
**Timeline:** 2-3 sessions

### 6.1 Deploy Falco

**Steps:**
1. Add Falco as a control-plane addon (DaemonSet with eBPF driver for Talos compatibility)
2. Configure default rules + custom rules for homelab-specific patterns
3. Route Falco alerts to Alertmanager (via falcosidekick)
4. Add Grafana dashboard for security events
5. Tune rules to reduce noise (expect initial false positives)

**Acceptance Criteria:**
- [ ] Falco running on all nodes with eBPF driver
- [ ] Security events visible in Grafana
- [ ] Critical events route to Alertmanager → Matrix
- [ ] False positive rate manageable (< 10/day after tuning)

---

## Tracking & Progress

Use GitHub Issues to track each phase. Suggested labels:
- `production-readiness` — umbrella label
- `security`, `observability`, `backup`, `cost`, `ci-cd` — category labels
- `quick-win`, `medium-effort`, `stretch-goal` — effort labels

### Suggested Issue Breakdown

| Issue Title | Phase | Labels |
|-------------|-------|--------|
| Switch Cilium from audit to enforce mode | 1.1 | `security`, `quick-win` |
| Deploy Trivy Operator for image scanning | 1.2 | `security`, `quick-win` |
| Add Kyverno enforce-mode policies | 1.3 | `security`, `medium-effort` |
| Deploy Velero for automated backups | 2.1 | `backup`, `medium-effort` |
| Automate etcd snapshot exports | 2.2 | `backup`, `medium-effort` |
| Add ResourceQuota/LimitRange to vcluster namespaces | 3.1 | `cost`, `quick-win` |
| Deploy OpenCost for resource visibility | 3.2 | `cost`, `quick-win` |
| Deploy Tempo + OpenTelemetry for tracing | 4.1 | `observability`, `medium-effort` |
| Add long-term metrics storage (Thanos/Mimir) | 4.2 | `observability`, `medium-effort` |
| Centralize vcluster logs via Promtail | 4.3 | `observability`, `quick-win` |
| Add secondary alert notification channel | 4.4 | `observability`, `quick-win` |
| Expand CI with Helm/policy/Go tests | 5.1 | `ci-cd`, `medium-effort` |
| Add manual approval gates for critical addons | 5.2 | `ci-cd`, `quick-win` |
| Deploy Falco for runtime security | 6.1 | `security`, `stretch-goal` |

---

## Revision History

| Date | Change | Author |
|------|--------|--------|
| 2026-02-18 | Initial assessment and plan creation | — |
| 2026-02-18 | Phase 1.1 complete: Cilium enforce mode active, 5 NetworkPolicy gaps fixed | — |
| 2026-02-18 | Phase 1.2 complete: Trivy Operator deployed with dashboard + alerts | — |
| 2026-02-19 | Phase 1.3 complete: 3 enforce-mode + 1 audit-mode Kyverno policies deployed | — |
