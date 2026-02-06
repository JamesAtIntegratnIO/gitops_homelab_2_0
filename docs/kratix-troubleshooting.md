# Kratix Resource Troubleshooting Guide

## Overview

This guide covers manual troubleshooting of stuck Kratix resources including ResourceRequests, Work, WorkPlacements, and GitStateStore issues.

## Resource Hierarchy

Understanding the flow helps identify where issues occur:

```
ResourceRequest (e.g., VClusterOrchestratorV2)
  ↓ triggers
Pipeline Job (configure/delete)
  ↓ creates
Work resource
  ↓ schedules to
WorkPlacement (per destination)
  ↓ writes to
GitStateStore (git repository)
  ↓ synced by
ArgoCD Applications
```

## Common Symptoms & Solutions

### 1. Pipeline Job Stuck or Failing

**Symptoms:**
- Job shows `Failed` or `CrashLoopBackOff` status
- ResourceRequest status shows `ConfigureWorkflowFailed`

**Investigation:**
```bash
# Find jobs for a specific resource
kubectl get jobs -n platform-requests -l kratix.io/resource-name=<resource-name>

# Get job details
kubectl describe job <job-name> -n platform-requests

# Find the pod
kubectl get pods -n platform-requests -l job-name=<job-name>

# Check init container statuses
kubectl get pod <pod-name> -n platform-requests \
  -o jsonpath='{range .status.initContainerStatuses[*]}{.name}: {.state}{"\n"}{end}'

# Check logs for each container
kubectl logs pod/<pod-name> -n platform-requests -c reader
kubectl logs pod/<pod-name> -n platform-requests -c configure  # or delete
kubectl logs pod/<pod-name> -n platform-requests -c work-writer
```

**Common Issues:**
- **Template errors**: Check configure container logs for template function errors
- **Missing dependencies**: Ensure all required CRDs and promises are installed
- **Image pull errors**: Verify `imagePullPolicy: Always` and container registry access
- **Security context**: Pods require restricted security context (runAsNonRoot, drop capabilities)

**Resolution:**
```bash
# Delete failed jobs to clean up
kubectl delete job -n platform-requests -l kratix.io/resource-name=<resource-name>

# Force reconciliation with annotation
kubectl annotate <promise-resource> <resource-name> -n platform-requests \
  kratix.io/manual-reconcile="$(date +%s)" --overwrite
```

### 2. Work Resources Not Creating

**Symptoms:**
- Pipeline job completes but no Work resources appear
- work-writer container shows errors

**Investigation:**
```bash
# List Work resources for a resource
kubectl get work -n platform-requests -l kratix.io/resource-name=<resource-name>

# Check Work status
kubectl get work <work-name> -n platform-requests -o jsonpath='{.status}' | jq

# Check work-writer logs
kubectl logs pod/<pod-name> -n platform-requests -c work-writer
```

**Common Issues:**
- **Empty output**: Pipeline didn't write files to `/kratix/output/`
- **Invalid YAML**: Work contains malformed Kubernetes manifests
- **Label mismatch**: Resources missing required Kratix labels

**Resolution:**
```bash
# Check what files the pipeline wrote
kubectl logs pod/<pod-name> -n platform-requests -c configure | grep "Rendered:"

# Manually inspect Work spec
kubectl get work <work-name> -n platform-requests -o yaml | less
```

### 3. WorkPlacement Failing to Write to GitStateStore

**Symptoms:**
- Work shows `Ready` but WorkPlacement shows `Failing`
- Error message: "no files changed" or git errors
- WorkPlacement status: `WriteSucceeded: False`

**Investigation:**
```bash
# Find WorkPlacements for a resource
kubectl get workplacement -n platform-requests | grep <resource-name>

# Check WorkPlacement status
kubectl get workplacement <workplacement-name> -n platform-requests \
  -o jsonpath='{.status.conditions}' | jq

# Check Kratix controller logs for git errors
kubectl logs -n kratix-platform-system \
  -l app.kubernetes.io/name=kratix-platform --tail=100 | \
  grep -A 10 "WorkPlacement.*<resource-name>"
```

**Common Issues:**

#### GitStateStore `/tmp/kratix-repo` Directory Corruption

**Error:** `fatal: cannot use .git/info/exclude as an exclude file`

This is a known Kratix bug where the `/tmp/kratix-repo*` directory becomes corrupted after multiple operations.

**Resolution:**
```bash
# Restart the Kratix controller to force fresh git clone
kubectl rollout restart deployment kratix-platform-controller-manager \
  -n kratix-platform-system

# Wait for rollout to complete
kubectl rollout status deployment kratix-platform-controller-manager \
  -n kratix-platform-system --timeout=60s

# Verify new pod is running
kubectl get pods -n kratix-platform-system \
  -l app.kubernetes.io/name=kratix-platform
```

#### No Files Changed Error

**Symptom:** `"message": "no files changed"`

This can occur when:
1. GitStateStore directory is corrupted (see above)
2. Output files have wrong extensions (e.g., `.yaml.tmpl` instead of `.yaml`)
3. Resources are identical to previous commit

**Resolution:**
```bash
# Check what the pipeline actually wrote
kubectl logs pod/<pipeline-pod> -n platform-requests -c work-writer

# Verify GitStateStore repository manually
# Clone the repo and check if files exist at expected path:
# clusters/<destination-name>/resources/<namespace>/<promise-name>/<resource-name>/...
```

### 4. ResourceRequest Stuck in Pending

**Symptoms:**
- ResourceRequest shows `Pending` status indefinitely
- No jobs created

**Investigation:**
```bash
# Check ResourceRequest status
kubectl get <promise-resource> <resource-name> -n platform-requests \
  -o jsonpath='{.status}' | jq

# Check promise installation
kubectl get promise <promise-name> -o jsonpath='{.status.conditions}' | jq

# Check for controller errors
kubectl logs -n kratix-platform-system \
  -l app.kubernetes.io/name=kratix-platform --tail=100 | \
  grep -i "error\|failed"
```

**Common Issues:**
- **Promise not installed**: Promise must have `Available` status
- **Invalid resource spec**: Check against promise CRD schema
- **Controller not running**: Verify Kratix controller pod is healthy

**Resolution:**
```bash
# Verify promise is available
kubectl get promise <promise-name>

# Check CRD exists
kubectl get crd | grep <promise-name>

# Force reconciliation
kubectl annotate <promise-resource> <resource-name> -n platform-requests \
  kratix.io/manual-reconcile="$(date +%s)" --overwrite
```

### 5. ArgoCD Not Creating Applications

**Symptoms:**
- WorkPlacement shows `Ready` but ArgoCD apps don't appear
- Files exist in GitStateStore but not synced

**Investigation:**
```bash
# Check if files exist in git state repo
git clone <gitstatestore-repo>
cd <repo>/clusters/<destination>/resources/...

# Check ArgoCD ApplicationSet
kubectl get applicationset -n argocd

# Check ArgoCD generator logs
kubectl logs -n argocd -l app.kubernetes.io/name=argocd-applicationset-controller --tail=50
```

**Common Issues:**
- **Wrong file extensions**: ArgoCD expects `.yaml`, not `.yaml.tmpl`
- **Path structure**: Must match ApplicationSet generator path pattern
- **ApplicationSet not watching path**: Check generator config
- **ArgoCD sync disabled**: Verify auto-sync policy

**Resolution:**
```bash
# Manually sync ApplicationSet
kubectl annotate applicationset <appset-name> -n argocd \
  argocd.argoproj.io/refresh=normal --overwrite

# Force ArgoCD to re-scan git repo
kubectl exec -n argocd <argocd-server-pod> -- \
  argocd app list --refresh hard
```

## Quick Diagnostic Checklist

Run these commands to get a complete picture:

```bash
RESOURCE_NAME="<your-resource-name>"
PROMISE_NAME="<promise-name>"
NAMESPACE="platform-requests"

echo "=== ResourceRequest Status ==="
kubectl get ${PROMISE_NAME} ${RESOURCE_NAME} -n ${NAMESPACE} -o jsonpath='{.status}' | jq

echo -e "\n=== Pipeline Jobs ==="
kubectl get jobs -n ${NAMESPACE} -l kratix.io/resource-name=${RESOURCE_NAME}

echo -e "\n=== Latest Job Pod ==="
POD=$(kubectl get pods -n ${NAMESPACE} \
  -l kratix.io/resource-name=${RESOURCE_NAME} \
  --sort-by=.metadata.creationTimestamp \
  -o jsonpath='{.items[-1].metadata.name}')
echo "Pod: $POD"
kubectl get pod $POD -n ${NAMESPACE}

echo -e "\n=== Work Resources ==="
kubectl get work -n ${NAMESPACE} -l kratix.io/resource-name=${RESOURCE_NAME}

echo -e "\n=== WorkPlacements ==="
kubectl get workplacement -n ${NAMESPACE} | grep ${RESOURCE_NAME}

echo -e "\n=== WorkPlacement Status ==="
WP=$(kubectl get workplacement -n ${NAMESPACE} -o jsonpath="{.items[?(@.metadata.labels.kratix\.io/resource-name=='${RESOURCE_NAME}')].metadata.name}" | head -1)
kubectl get workplacement ${WP} -n ${NAMESPACE} -o jsonpath='{.status.conditions}' | jq

echo -e "\n=== Recent Kratix Controller Logs ==="
kubectl logs -n kratix-platform-system \
  -l app.kubernetes.io/name=kratix-platform --tail=20 | \
  grep -i "${RESOURCE_NAME}\|error"
```

## Force Reconciliation Techniques

### Delete and Recreate Work/WorkPlacement

When resources are truly stuck:

```bash
# Delete Work (will be recreated by controller)
kubectl delete work -n platform-requests -l kratix.io/resource-name=<resource-name>

# WorkPlacement will be automatically recreated
# Monitor recreation:
watch kubectl get work,workplacement -n platform-requests | grep <resource-name>
```

### Annotate for Manual Reconciliation

```bash
# Add timestamp annotation to force reconciliation
kubectl annotate <promise-resource> <resource-name> -n platform-requests \
  kratix.io/manual-reconcile="$(date +%s)" --overwrite
```

### Delete Failed Jobs

```bash
# Clean up old failed jobs
kubectl delete job -n platform-requests -l kratix.io/resource-name=<resource-name>

# This allows fresh job creation on next reconciliation
```

### Restart Kratix Controller

**When to use:** GitStateStore corruption, controller in bad state, mysterious reconciliation failures

```bash
# Restart controller (forces fresh git clones)
kubectl rollout restart deployment kratix-platform-controller-manager \
  -n kratix-platform-system

# Wait for restart
kubectl rollout status deployment kratix-platform-controller-manager \
  -n kratix-platform-system --timeout=60s

# Verify new pod is running
kubectl get pods -n kratix-platform-system -l app.kubernetes.io/name=kratix-platform
```

## Validation After Fixes

Verify the complete flow:

```bash
RESOURCE_NAME="<your-resource-name>"

# 1. ResourceRequest should show success
kubectl get <promise> ${RESOURCE_NAME} -n platform-requests -o jsonpath='{.status.conditions[?(@.type=="ConfigureWorkflowCompleted")]}' | jq

# 2. Job should be Complete
kubectl get job -n platform-requests -l kratix.io/resource-name=${RESOURCE_NAME} \
  --sort-by=.metadata.creationTimestamp | tail -1

# 3. Work should be Ready
kubectl get work -n platform-requests -l kratix.io/resource-name=${RESOURCE_NAME} \
  -o jsonpath='{.items[0].status.conditions[?(@.type=="Ready")]}' | jq

# 4. WorkPlacement should have WriteSucceeded=True
WP=$(kubectl get workplacement -n platform-requests -o name | grep ${RESOURCE_NAME} | head -1)
kubectl get ${WP} -n platform-requests -o jsonpath='{.status.conditions[?(@.type=="WriteSucceeded")]}' | jq

# 5. Check GitStateStore commit
# Look for recent Kratix controller logs showing "pushing changes"
kubectl logs -n kratix-platform-system -l app.kubernetes.io/name=kratix-platform --tail=50 | \
  grep "pushing changes.*${RESOURCE_NAME}"

# 6. Verify ArgoCD sees the resources
# Check ApplicationSet or Applications were created
kubectl get application -n argocd | grep ${RESOURCE_NAME}
```

## Common Patterns and Solutions

### Pattern: CrashLoopBackOff Init Container

**Cause:** Pipeline code error, missing template function, invalid resource spec

**Fix:**
1. Check pipeline logs for exact error
2. Fix pipeline code or resource spec
3. Rebuild container image
4. Delete job and pod to pull new image
5. Force reconciliation

### Pattern: "No Files Changed" Loop

**Cause:** GitStateStore directory corruption

**Fix:**
1. Restart Kratix controller
2. Wait for fresh git clone
3. Resources should write successfully on next reconciliation

### Pattern: Files with .tmpl Extension in GitStateStore

**Cause:** Pipeline writing template filenames instead of stripping extension

**Fix:**
1. Update pipeline to strip `.tmpl` from output filenames
2. Rebuild and redeploy pipeline container
3. Delete Work/WorkPlacement to trigger fresh render
4. Verify `.yaml` files appear in GitStateStore

### Pattern: WorkPlacement Failing After Successful Write

**Cause:** Subsequent reconciliations hitting "no files changed" (harmless if initial write succeeded)

**Fix:**
- If resources deployed successfully, ignore WorkPlacement "Failing" status
- This is a Kratix controller quirk, doesn't affect actual deployments
- Or restart controller to clear the error state

## GitStateStore Repository Structure

Expected directory layout:

```
<gitstatestore-repo>/
└── clusters/
    └── <destination-name>/        # e.g., "the-cluster"
        └── resources/
            └── <namespace>/        # e.g., "platform-requests"
                └── <promise-name>/ # e.g., "vcluster-orchestrator-v2"
                    └── <resource-name>/  # e.g., "media"
                        └── <pipeline-name>/  # e.g., "vco-v2-configure"
                            └── <workplacement-suffix>/  # e.g., "5058f"
                                └── resources/
                                    ├── resource1.yaml
                                    ├── resource2.yaml
                                    └── ...
```

## Useful kubectl Aliases

Add to your shell config for faster troubleshooting:

```bash
# Kratix aliases
alias kgw='kubectl get work -n platform-requests'
alias kgwp='kubectl get workplacement -n platform-requests'
alias kgjob='kubectl get jobs -n platform-requests'
alias kdw='kubectl describe work -n platform-requests'
alias kdwp='kubectl describe workplacement -n platform-requests'

# Get Kratix controller logs
alias kratix-logs='kubectl logs -n kratix-platform-system -l app.kubernetes.io/name=kratix-platform --tail=100'

# Restart Kratix controller
alias kratix-restart='kubectl rollout restart deployment kratix-platform-controller-manager -n kratix-platform-system'

# Check specific resource troubleshooting
kratix-check() {
  local resource=$1
  echo "Jobs:"
  kubectl get jobs -n platform-requests -l kratix.io/resource-name=${resource}
  echo -e "\nWork:"
  kubectl get work -n platform-requests -l kratix.io/resource-name=${resource}
  echo -e "\nWorkPlacements:"
  kubectl get workplacement -n platform-requests | grep ${resource}
}
```

## Known Issues & Workarounds

### Issue: GitStateStore /tmp Directory Loss

**Symptom:** Recurring "no files changed" or git errors after restarts

**Workaround:** Restart Kratix controller to force fresh git clone (not a permanent fix)

**Tracking:** This is a known issue with Kratix v0.1.0 GitStateStore implementation

### Issue: WorkPlacement Shows Failing Despite Successful Deployment

**Symptom:** WorkPlacement condition `Ready=False` but ArgoCD apps are syncing fine

**Impact:** Cosmetic - doesn't affect actual deployments

**Workaround:** Ignore WorkPlacement status if resources deployed correctly

### Issue: Pipeline Image Not Updating

**Symptom:** Code changes not reflected in pipeline execution

**Cause:** `imagePullPolicy: IfNotPresent` or image cached

**Fix:**
1. Ensure promise uses `imagePullPolicy: Always`
2. Delete pod to force fresh pull
3. Or tag images with commit SHA instead of `latest`

## Additional Resources

- [Kratix Documentation](https://kratix.io/docs)
- [Kratix GitHub Issues](https://github.com/syntasso/kratix/issues)
- [ArgoCD Troubleshooting](https://argo-cd.readthedocs.io/en/stable/user-guide/troubleshooting/)
