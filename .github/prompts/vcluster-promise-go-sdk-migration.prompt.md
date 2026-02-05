# VCluster Promise Migration to Go SDK - Implementation Plan

## Context

### Current Architecture Analysis
The vcluster promise stack currently executes **8 promises for a single vcluster deployment**:

1. `vcluster-orchestrator` (user-facing) - Creates 7 sub-promise ResourceRequests
2. `vcluster-core` - Core vcluster Helm chart deployment
3. `vcluster-coredns` - CoreDNS configuration
4. `argocd-project` - Creates ArgoCD AppProject
5. `argocd-application` - Creates ArgoCD Application
6. `vcluster-kubeconfig-sync` - Syncs kubeconfig to parent cluster
7. `vcluster-kubeconfig-external-secret` - Creates ExternalSecret for kubeconfig from 1Password
8. `vcluster-argocd-cluster-registration` - Registers vcluster as ArgoCD cluster

### Current Implementation Antipatterns

**File Structure Issues:**
```
promises/vcluster-orchestrator/
├── internal/configure-pipeline/scripts/
│   ├── render.sh                      # Entry point, sources 4 lib files
│   └── lib/
│       ├── render-resources.sh        # 7 functions with cat <<EOF heredocs (200+ lines each)
│       ├── ip-utils.sh               # IP calculation utilities
│       ├── read-inputs.sh            # Input parsing
│       └── defaults.sh               # Default values
```

**Identified Antipatterns:**
1. **YAML buried in bash heredocs**: 200+ line `cat <<EOF` blocks generating YAML with variable substitution
2. **Deep script nesting**: render.sh sources 4 lib files, difficult to trace execution
3. **No official tooling usage**: Not using Kratix CLI or official SDK
4. **Testing difficulty**: Cannot unit test YAML generation without running full pipeline
5. **Maintainability issues**: No IDE support for YAML in heredocs, syntax errors caught at runtime
6. **Type safety**: Bash provides no type checking or validation

**Example Current Pattern:**
```bash
# From render-resources.sh
write_vcluster_core_request() {
    cat <<EOF > "${OUTPUT_DIR}/vcluster-core-request.yaml"
apiVersion: platform.kratix.io/v1alpha1
kind: vclustercore
metadata:
  name: ${VCLUSTER_NAME}
  namespace: ${NAMESPACE}
spec:
  name: ${VCLUSTER_NAME}
  chart:
    version: ${CHART_VERSION}
  values:
    # ... 180 more lines of heredoc YAML
EOF
}
```

### Official Kratix Best Practices

**From Kratix Documentation and Marketplace:**
1. **Use Kratix CLI**: `kratix init`, `kratix update api`, `kratix add container`
2. **Use Official SDKs**: Python (`kratix-sdk`) or Go (`github.com/syntasso/kratix-go`)
3. **YAML as actual files**: Templates should be real `.yaml` files, not heredocs
4. **Recommended structure**: `workflows/resource/configure/<container-name>/`
5. **Patterns from marketplace**:
   - **Simple promises** (nginx-ingress, istio): `cp /resources/* /kratix/output/`
   - **Complex promises** (mongodb, app-as-a-service): Use SDK with templates

### Go SDK API Reference

**Package:** `github.com/syntasso/kratix-go`

**Core SDK Methods:**
```go
sdk := kratix.New()                              // Initialize SDK
resource, _ := sdk.ReadResourceInput()           // Read /kratix/input/object.yaml
promise, _ := sdk.ReadPromiseInput()             // Read promise (for promise workflows)
name := resource.GetName()                       // Get resource name
namespace := resource.GetNamespace()             // Get namespace
value, _ := resource.GetValue("spec.field")      // Extract spec values using path
status, _ := resource.GetStatus()                // Get resource status
sdk.WriteOutput("file.yaml", []byte("..."))      // Write to /kratix/output/
status := kratix.NewStatus()                     // Create new status
status.Set("phase", "Ready")                     // Set status field
sdk.WriteStatus(status)                          // Write to /kratix/metadata/status.yaml
sdk.PublishStatus(resource, status)              // Update resource status in K8s API
selectors := []kratix.DestinationSelector{...}   // Configure destination
sdk.WriteDestinationSelectors(selectors)         // Write destination selectors
```

**Environment Variables Available:**
- `KRATIX_WORKFLOW_ACTION`: "configure" or "delete"
- `KRATIX_WORKFLOW_TYPE`: "resource" or "promise"
- `KRATIX_PROMISE_NAME`: Name of the promise
- `KRATIX_PIPELINE_NAME`: Name of the pipeline
- `KRATIX_CRD_PLURAL`: Plural form of CRD for API calls

**Helper Methods:**
```go
sdk.WorkflowAction()      // Returns KRATIX_WORKFLOW_ACTION
sdk.WorkflowType()        // Returns KRATIX_WORKFLOW_TYPE
sdk.PromiseName()         // Returns KRATIX_PROMISE_NAME
sdk.PipelineName()        // Returns KRATIX_PIPELINE_NAME
sdk.IsPromiseWorkflow()   // true if promise workflow
sdk.IsResourceWorkflow()  // true if resource workflow
sdk.IsConfigureAction()   // true if configure action
sdk.IsDeleteAction()      // true if delete action
```

## Proposed Solution: Go SDK Refactoring

### New Directory Structure
```
promises/vcluster-orchestrator-v2/
├── promise.yaml
├── example-resource-request.yaml.example
├── README.md
├── workflows/
│   └── resource/
│       └── configure/
│           ├── main.go                          # Pipeline logic (Go SDK)
│           ├── go.mod                           # Go module definition
│           ├── go.sum                           # Dependency lock
│           ├── Dockerfile                       # Multi-stage Go build
│           └── templates/                       # YAML templates as actual files
│               ├── vcluster-core.yaml
│               ├── vcluster-coredns.yaml
│               ├── argocd-project.yaml
│               ├── argocd-application.yaml
│               ├── kubeconfig-sync.yaml
│               ├── kubeconfig-external-secret.yaml
│               └── argocd-cluster-registration.yaml
```

### Implementation Code

#### workflows/resource/configure/main.go
```go
package main

import (
    "embed"
    "log"
    "text/template"
    "bytes"
    "fmt"
    
    kratix "github.com/syntasso/kratix-go"
    "sigs.k8s.io/yaml"
)

//go:embed templates/*.yaml
var templatesFS embed.FS

// VClusterConfig holds all configuration for template rendering
type VClusterConfig struct {
    Name            string
    Namespace       string
    VCluster        VClusterSpec
    Exposure        ExposureSpec
    Integrations    IntegrationsSpec
    WorkflowContext WorkflowContext
}

type VClusterSpec struct {
    ChartVersion string
    Distro       string
    Sync         SyncConfig
    // Add other vcluster spec fields as needed
}

type ExposureSpec struct {
    Ingress IngressConfig
    // Add other exposure fields
}

type IntegrationsSpec struct {
    ArgoCD ArgoCDConfig
    // Add other integration fields
}

type SyncConfig struct {
    ConfigMaps bool
    Secrets    bool
}

type IngressConfig struct {
    Enabled bool
    Host    string
}

type ArgoCDConfig struct {
    Enabled bool
}

type WorkflowContext struct {
    WorkflowAction string
    WorkflowType   string
    PromiseName    string
    PipelineName   string
}

func main() {
    // Initialize SDK
    sdk := kratix.New()
    
    log.Printf("=== VCluster Orchestrator v2 Pipeline ===")
    log.Printf("Action: %s", sdk.WorkflowAction())
    log.Printf("Type: %s", sdk.WorkflowType())
    log.Printf("Promise: %s", sdk.PromiseName())
    
    // Read user's VClusterOrchestrator ResourceRequest
    resource, err := sdk.ReadResourceInput()
    if err != nil {
        log.Fatalf("ERROR: Failed to read resource input: %v", err)
    }
    
    log.Printf("Processing resource: %s in namespace: %s", 
        resource.GetName(), resource.GetNamespace())
    
    // Extract spec values
    config, err := buildConfig(sdk, resource)
    if err != nil {
        log.Fatalf("ERROR: Failed to build config: %v", err)
    }
    
    // Execute workflow based on action
    if sdk.IsConfigureAction() {
        if err := handleConfigure(sdk, config); err != nil {
            log.Fatalf("ERROR: Configure failed: %v", err)
        }
    } else if sdk.IsDeleteAction() {
        if err := handleDelete(sdk, config); err != nil {
            log.Fatalf("ERROR: Delete failed: %v", err)
        }
    } else {
        log.Fatalf("ERROR: Unknown workflow action: %s", sdk.WorkflowAction())
    }
    
    log.Println("=== Pipeline completed successfully ===")
}

func buildConfig(sdk *kratix.KratixSDK, resource kratix.Resource) (*VClusterConfig, error) {
    // Extract spec values using SDK
    name, err := resource.GetValue("spec.name")
    if err != nil {
        return nil, fmt.Errorf("spec.name not found: %w", err)
    }
    
    // Extract nested specs (with proper type assertions)
    vclusterSpec, _ := resource.GetValue("spec.vcluster")
    exposureSpec, _ := resource.GetValue("spec.exposure")
    integrationsSpec, _ := resource.GetValue("spec.integrations")
    
    // Build typed config (add proper unmarshaling here)
    config := &VClusterConfig{
        Name:      name.(string),
        Namespace: resource.GetNamespace(),
        // TODO: Unmarshal vclusterSpec, exposureSpec, integrationsSpec into typed structs
        WorkflowContext: WorkflowContext{
            WorkflowAction: sdk.WorkflowAction(),
            WorkflowType:   sdk.WorkflowType(),
            PromiseName:    sdk.PromiseName(),
            PipelineName:   sdk.PipelineName(),
        },
    }
    
    return config, nil
}

func handleConfigure(sdk *kratix.KratixSDK, config *VClusterConfig) error {
    log.Println("--- Rendering resource templates ---")
    
    // List of templates to render (order matters for dependencies)
    templates := []string{
        "vcluster-core.yaml",
        "vcluster-coredns.yaml",
        "argocd-project.yaml",
        "argocd-application.yaml",
        "kubeconfig-sync.yaml",
        "kubeconfig-external-secret.yaml",
        "argocd-cluster-registration.yaml",
    }
    
    for _, tmplName := range templates {
        if err := renderTemplate(sdk, tmplName, config); err != nil {
            return fmt.Errorf("failed to render %s: %w", tmplName, err)
        }
        log.Printf("✓ Rendered: %s", tmplName)
    }
    
    // Update status
    status := kratix.NewStatus()
    status.Set("phase", "Scheduled")
    status.Set("message", "VCluster sub-resources scheduled for creation")
    status.Set("templatesRendered", len(templates))
    
    if err := sdk.WriteStatus(status); err != nil {
        return fmt.Errorf("failed to write status: %w", err)
    }
    
    log.Println("✓ Status updated")
    return nil
}

func renderTemplate(sdk *kratix.KratixSDK, tmplName string, config *VClusterConfig) error {
    // Read template from embedded filesystem
    content, err := templatesFS.ReadFile("templates/" + tmplName)
    if err != nil {
        return fmt.Errorf("read template: %w", err)
    }
    
    // Parse Go template
    tmpl, err := template.New(tmplName).Parse(string(content))
    if err != nil {
        return fmt.Errorf("parse template: %w", err)
    }
    
    // Execute template with config
    var buf bytes.Buffer
    if err := tmpl.Execute(&buf, config); err != nil {
        return fmt.Errorf("execute template: %w", err)
    }
    
    // Validate YAML before writing (optional but recommended)
    var yamlCheck interface{}
    if err := yaml.Unmarshal(buf.Bytes(), &yamlCheck); err != nil {
        return fmt.Errorf("invalid YAML generated: %w", err)
    }
    
    // Write to output directory (Kratix will deploy these)
    outputPath := "resources/" + tmplName
    if err := sdk.WriteOutput(outputPath, buf.Bytes()); err != nil {
        return fmt.Errorf("write output: %w", err)
    }
    
    return nil
}

func handleDelete(sdk *kratix.KratixSDK, config *VClusterConfig) error {
    log.Printf("--- Handling delete for vcluster: %s ---", config.Name)
    
    // Kratix handles deletion of resources created in configure
    // Add any custom cleanup logic here if needed
    
    status := kratix.NewStatus()
    status.Set("phase", "Deleting")
    status.Set("message", "VCluster resources scheduled for deletion")
    
    if err := sdk.WriteStatus(status); err != nil {
        return fmt.Errorf("failed to write status: %w", err)
    }
    
    return nil
}
```

#### workflows/resource/configure/go.mod
```go
module github.com/jamesatintegratnio/vcluster-orchestrator-pipeline

go 1.24

require (
    github.com/syntasso/kratix-go v0.1.0
    k8s.io/api v0.34.0
    k8s.io/apimachinery v0.34.0
    sigs.k8s.io/yaml v1.4.0
)
```

#### workflows/resource/configure/Dockerfile
```dockerfile
FROM --platform=$BUILDPLATFORM golang:1.24 AS builder
ARG TARGETARCH
ARG TARGETOS

WORKDIR /workspace

ENV GO111MODULE=on
ENV CGO_ENABLED=0
ENV GOOS=${TARGETOS}
ENV GOARCH=${TARGETARCH}

# Copy go mod files and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code and embedded templates
COPY main.go ./
COPY templates/ ./templates/

# Build static binary
RUN go build -a -ldflags '-extldflags "-static"' -o /out/pipeline main.go

# Runtime stage (distroless for security)
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /out/pipeline .
USER 65532:65532

ENTRYPOINT ["/pipeline"]
```

#### workflows/resource/configure/templates/vcluster-core.yaml (example)
```yaml
apiVersion: platform.kratix.io/v1alpha1
kind: vclustercore
metadata:
  name: {{ .Name }}
  namespace: {{ .Namespace }}
  labels:
    kratix.io/promise-name: {{ .WorkflowContext.PromiseName }}
    vcluster.io/orchestrator: "true"
spec:
  name: {{ .Name }}
  chart:
    version: {{ .VCluster.ChartVersion }}
  values:
    sync:
      configmaps:
        enabled: {{ .VCluster.Sync.ConfigMaps }}
      secrets:
        enabled: {{ .VCluster.Sync.Secrets }}
    controlPlane:
      distro:
        k3s:
          enabled: {{ eq .VCluster.Distro "k3s" }}
        k8s:
          enabled: {{ eq .VCluster.Distro "k8s" }}
```

### Migration Strategy: Zero-Downtime

**CRITICAL**: The media vcluster is currently running in production. We must guarantee zero downtime.

#### Phase 1: Create Parallel v2 Implementation
**Objective**: Build and test new Go SDK implementation without touching existing promises

**Steps:**
1. Create `promises/vcluster-orchestrator-v2/` directory structure
2. Implement Go SDK pipeline as documented above
3. Extract YAML templates from current bash heredocs to `templates/*.yaml`
4. Build and push container image to registry
5. Install v2 promise to Kratix platform (does NOT affect existing vclusters)

**Commands:**
```bash
# Create v2 structure
cd promises/
cp -r vcluster-orchestrator vcluster-orchestrator-v2
cd vcluster-orchestrator-v2/

# Implement Go SDK (as documented above)
# ...

# Build container
cd workflows/resource/configure/
docker build -t <registry>/vcluster-orchestrator-v2:latest .
docker push <registry>/vcluster-orchestrator-v2:latest

# Update promise.yaml to reference new image
# Install to cluster
kubectl apply -f promises/vcluster-orchestrator-v2/promise.yaml
```

**Validation:**
- [ ] v2 promise shows "Available" in Kratix
- [ ] Media vcluster continues running with v1 promise
- [ ] No changes to existing resources

#### Phase 2: Test with New Test VCluster
**Objective**: Validate v2 implementation with a disposable test vcluster

**Steps:**
1. Create `platform/vclusters/test.yaml` using v2 promise
2. Monitor resource creation (7 sub-promise ResourceRequests)
3. Validate vcluster functionality:
   - VCluster pod running and healthy
   - Kubeconfig accessible
   - ArgoCD registration successful
   - CoreDNS resolution working
   - Ingress/exposure configured
4. Test delete workflow
5. Confirm cleanup is complete

**Test Vcluster Definition:**
```yaml
# platform/vclusters/test.yaml
apiVersion: platform.kratix.io/v1alpha1
kind: VClusterOrchestratorV2  # Note: V2!
metadata:
  name: test
  namespace: vcluster-test
spec:
  name: test
  vcluster:
    chartVersion: "0.20.0"
    distro: k3s
    sync:
      configmaps: true
      secrets: true
  exposure:
    ingress:
      enabled: true
      host: test-vcluster.cluster.integratn.tech
  integrations:
    argocd:
      enabled: true
```

**Validation Checklist:**
- [ ] All 7 sub-promise ResourceRequests created
- [ ] VCluster pod reaches Running state
- [ ] `kubectl get pods -n vcluster-test` shows healthy pods
- [ ] Kubeconfig ExternalSecret created and synced from 1Password
- [ ] ArgoCD cluster registration successful
- [ ] `kubectl --context test get nodes` works
- [ ] Ingress resolves to vcluster API
- [ ] Delete workflow removes all resources
- [ ] No orphaned resources after deletion

#### Phase 3: Migrate Media VCluster to v2
**Objective**: Switch production media vcluster to v2 promise

**Pre-Migration Safety Checks:**
1. [ ] Backup media vcluster kubeconfig
2. [ ] Document current media vcluster resources (`kubectl get all -n vcluster-media`)
3. [ ] Verify test vcluster completed successfully in Phase 2
4. [ ] Confirm rollback plan (revert to v1 promise)
5. [ ] Schedule migration during maintenance window (if applicable)

**Migration Steps:**
1. Update `platform/vclusters/media.yaml`:
   ```yaml
   apiVersion: platform.kratix.io/v1alpha1
   kind: VClusterOrchestratorV2  # Changed from VClusterOrchestrator
   # ... rest stays the same
   ```

2. Commit and push change:
   ```bash
   git add platform/vclusters/media.yaml
   git commit -m "feat: migrate media vcluster to orchestrator v2 (Go SDK)"
   git push
   ```

3. Monitor ArgoCD sync:
   ```bash
   argocd app get vcluster-media --watch
   kubectl get pods -n vcluster-media --watch
   ```

4. Validate media vcluster functionality:
   - [ ] VCluster pod remains Running (should NOT restart)
   - [ ] Workloads inside vcluster unaffected
   - [ ] ArgoCD applications in vcluster continue syncing
   - [ ] Ingress still resolves correctly
   - [ ] No error events: `kubectl get events -n vcluster-media --sort-by='.lastTimestamp'`

**Expected Behavior:**
- Kratix detects CRD kind change (VClusterOrchestrator → VClusterOrchestratorV2)
- New v2 promise pipeline executes
- Sub-promise ResourceRequests regenerated (should reconcile to same state)
- **VCluster itself should NOT be deleted/recreated** (StatefulSet persists)

**Rollback Plan (if issues occur):**
```bash
# Revert to v1 promise
git revert HEAD
git push

# OR manually edit media.yaml
# Change kind back to: VClusterOrchestrator
kubectl apply -f platform/vclusters/media.yaml
```

#### Phase 4: Cleanup Old Promises
**Objective**: Remove v1 bash-based promises after successful migration

**Timeline:** Wait 1-2 weeks after Phase 3 to ensure stability

**Steps:**
1. Verify media vcluster stable for 1-2 weeks
2. Confirm no v1 promise ResourceRequests remain:
   ```bash
   kubectl get vclusterorchestrator -A
   # Should show nothing or only v2
   ```

3. Delete old v1 promise:
   ```bash
   kubectl delete promise vcluster-orchestrator
   ```

4. Remove old promise directory:
   ```bash
   git rm -r promises/vcluster-orchestrator/
   git commit -m "chore: remove legacy bash-based vcluster orchestrator"
   git push
   ```

5. (Optional) Rename v2 to canonical name:
   ```bash
   git mv promises/vcluster-orchestrator-v2 promises/vcluster-orchestrator
   # Update promise.yaml CRD name back to VClusterOrchestrator
   git commit -m "chore: promote v2 orchestrator to canonical version"
   git push
   ```

### Sub-Promise Consolidation (Future Phase)

**Recommendation**: Keep sub-promises separate initially, consolidate later after v2 is proven stable.

**Current Sub-Promises:**
1. `vcluster-core` - Keep separate (complex Helm logic)
2. `vcluster-coredns` - **Consolidate candidate** (simple ConfigMap)
3. `argocd-project` - Keep separate (general-purpose, reusable)
4. `argocd-application` - Keep separate (general-purpose, reusable)
5. `kubeconfig-sync` - **Consolidate candidate** (simple Job)
6. `kubeconfig-external-secret` - **Consolidate candidate** (simple ExternalSecret)
7. `argocd-cluster-registration` - **Consolidate candidate** (simple Job)

**Consolidation Strategy (Future):**
- Move vcluster-specific logic (#2, #5, #6, #7) directly into orchestrator v2 main.go
- Generate resources inline instead of creating ResourceRequests
- Keep ArgoCD promises separate for reusability
- Reduces 8 promise executions → 3 (orchestrator + 2 ArgoCD promises)

## Implementation Checklist

### Phase 1: Build v2 Parallel Implementation
- [ ] Create `promises/vcluster-orchestrator-v2/` directory
- [ ] Create `workflows/resource/configure/` structure
- [ ] Implement `main.go` with Go SDK
- [ ] Extract YAML templates from bash heredocs to `templates/*.yaml`
- [ ] Create `go.mod`, `go.sum`
- [ ] Create multi-stage `Dockerfile`
- [ ] Update `promise.yaml` with new CRD `VClusterOrchestratorV2`
- [ ] Build container image
- [ ] Push to container registry
- [ ] Install v2 promise: `kubectl apply -f promise.yaml`
- [ ] Verify v1 promise still active
- [ ] Verify media vcluster unaffected

### Phase 2: Test with Disposable VCluster
- [ ] Create `platform/vclusters/test.yaml` with `kind: VClusterOrchestratorV2`
- [ ] Commit and push test vcluster
- [ ] Monitor ArgoCD sync
- [ ] Verify 7 sub-promise ResourceRequests created
- [ ] Check vcluster pod running
- [ ] Test kubeconfig access
- [ ] Verify ArgoCD registration
- [ ] Test CoreDNS resolution
- [ ] Test ingress/exposure
- [ ] Delete test vcluster
- [ ] Verify cleanup completed
- [ ] Document any issues found

### Phase 3: Migrate Media VCluster
- [ ] Backup media vcluster kubeconfig
- [ ] Document current resources
- [ ] Update `platform/vclusters/media.yaml` to use `VClusterOrchestratorV2`
- [ ] Commit with clear message
- [ ] Push and monitor ArgoCD sync
- [ ] Verify vcluster pod stable
- [ ] Test vcluster functionality
- [ ] Check workloads inside vcluster
- [ ] Monitor for 24 hours
- [ ] Document any issues

### Phase 4: Cleanup (After 1-2 Weeks)
- [ ] Confirm media vcluster stable
- [ ] Delete v1 promise from cluster
- [ ] Remove `promises/vcluster-orchestrator/` directory
- [ ] (Optional) Rename v2 to canonical name
- [ ] Update documentation

## Testing Strategy

### Unit Tests (Go)
```go
// main_test.go
package main

import (
    "testing"
    kratix "github.com/syntasso/kratix-go"
)

func TestBuildConfig(t *testing.T) {
    // Mock SDK and resource
    // Test config building logic
}

func TestRenderTemplate(t *testing.T) {
    // Test template rendering with known inputs
    // Verify YAML validity
}
```

### Integration Tests
```bash
# Test pipeline locally before building container
export KRATIX_WORKFLOW_ACTION=configure
export KRATIX_WORKFLOW_TYPE=resource
export KRATIX_PROMISE_NAME=vcluster-orchestrator-v2
export KRATIX_PIPELINE_NAME=resource-configure

# Create test input
mkdir -p /tmp/kratix/{input,output,metadata}
cp test-resource.yaml /tmp/kratix/input/object.yaml

# Run pipeline
go run main.go

# Verify output
ls /tmp/kratix/output/resources/
kubectl apply --dry-run=client -f /tmp/kratix/output/resources/
```

## Benefits Summary

1. **Maintainability**: Clean Go code vs 200+ line bash heredocs
2. **Type Safety**: Compile-time checks vs runtime bash errors
3. **Testability**: Unit tests for template rendering
4. **IDE Support**: Full IDE features for Go and YAML templates
5. **Debugging**: Stack traces and better error messages
6. **Official Pattern**: Follows Kratix best practices
7. **Performance**: Compiled Go binary vs interpreted bash
8. **Readability**: Clear separation of logic and templates

## Risks and Mitigations

| Risk | Impact | Mitigation |
|------|--------|-----------|
| Media vcluster downtime during migration | HIGH | Parallel v2, test thoroughly, rollback plan |
| Template conversion errors | MEDIUM | Validate YAML, test with disposable vcluster |
| Go SDK bugs | LOW | SDK is official and tested, report issues upstream |
| Container build failures | LOW | Multi-stage Dockerfile tested in SDK repo |
| Migration introduces breaking changes | MEDIUM | Keep v1 installed for quick rollback |

## Success Criteria

- [ ] Media vcluster migrated to v2 with zero downtime
- [ ] All vcluster functionality preserved
- [ ] Pipeline execution time < 60 seconds (current: ~90s)
- [ ] YAML templates readable and maintainable
- [ ] Go code passes `go vet` and `golangci-lint`
- [ ] Unit tests achieve >80% coverage
- [ ] Documentation updated

## Timeline Estimate

- **Phase 1 (Build v2)**: 2-3 days
- **Phase 2 (Test)**: 1-2 days
- **Phase 3 (Migrate)**: 1 day + monitoring
- **Phase 4 (Cleanup)**: 1 day (after 1-2 week soak period)

**Total**: ~1 week active development + 1-2 weeks validation

## References

- Kratix Go SDK: https://github.com/syntasso/kratix-go
- Kratix Documentation: https://docs.kratix.io
- Go Templates: https://pkg.go.dev/text/template
- Current promise: `promises/vcluster-orchestrator/`
- Media vcluster: `platform/vclusters/media.yaml`
