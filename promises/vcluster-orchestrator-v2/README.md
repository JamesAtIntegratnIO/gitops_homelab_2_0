# VCluster Orchestrator V2 Promise

The orchestrator promise for provisioning vclusters with full ArgoCD integration. It delegates to 3 reusable sub-promises via Kratix ResourceRequests, plus generates direct Kubernetes resources for namespace setup, CoreDNS, and etcd certificates.

## Architecture

```
VClusterOrchestratorV2 (ResourceRequest)
│
├── ResourceRequests (delegated to sub-promises)
│   ├── ArgoCDProject         → creates ArgoCD AppProject
│   ├── ArgoCDApplication     → creates ArgoCD Application (vcluster Helm chart)
│   └── ArgoCDClusterRegistration → 1P token, RBAC, kubeconfig sync Job,
│                                    kubeconfig ExternalSecret, ArgoCD cluster secret
│
└── Direct Resources (written to state store)
    ├── Namespace
    ├── CoreDNS ConfigMap
    └── Etcd Certificates (if external etcd enabled)
```

The orchestrator emits ResourceRequests that Kratix writes to the state store. ArgoCD syncs them into the cluster as CRs, triggering the sub-promise pipelines.

## Project Structure

```
promises/vcluster-orchestrator-v2/
├── promise.yaml                     # CRD + pipeline definitions
├── README.md
└── workflows/resource/configure/
    ├── main.go                      # Entry point, config parsing, action routing
    ├── types.go                     # Shared Go types (Resource, specs)
    ├── builders_argocd.go           # Builds ArgoCD sub-promise ResourceRequests
    ├── builders_namespace_coredns.go # Builds Namespace + CoreDNS ConfigMap
    ├── builders_etcd.go             # Builds etcd Certificate/Issuer resources
    ├── builders_common.go           # Shared builder helpers
    ├── writers.go                   # YAML serialization + SDK output helpers
    ├── Dockerfile                   # Multi-stage build
    ├── go.mod / go.sum
    └── .gitignore
```

## Technology Stack

- **Language**: Go 1.24
- **Framework**: Kratix Go SDK v0.1.0
- **Build**: Multi-stage Docker (golang → distroless)
- **CI**: GitHub Actions builds `ghcr.io/jamesatintegratnio/vcluster-orchestrator-v2-configure:latest`

## Security Model

No `kind: Secret` resources are generated. All credentials flow through `ExternalSecret` with 1Password Connect via `ClusterSecretStore`.

## Resource Generation

### Configuration
- Extracts 40+ spec fields from the ResourceRequest
- Applies preset-based defaults (`dev` vs `prod`)
- Calculates VIP from CIDR subnet if not specified
- Validates VIP within subnet boundaries
- Builds complete Helm values for the vcluster chart

### Output Resources

| Resource | Type | Destination |
|----------|------|-------------|
| ArgoCDProject request | ResourceRequest | Kratix → state store → `platform-requests` |
| ArgoCDApplication request | ResourceRequest | Kratix → state store → `platform-requests` |
| ArgoCDClusterRegistration request | ResourceRequest | Kratix → state store → `platform-requests` |
| Namespace | Direct | Target namespace |
| CoreDNS ConfigMap | Direct | Target namespace |
| Etcd Certificates | Direct (conditional) | Target namespace |

### Delete Handling

The same image handles both `configure` and `delete` actions. The SDK's `WorkflowAction()` determines the action from Kratix environment variables. On delete, the pipeline logs cleanup but Kratix handles actual resource removal through the state store.

## Preset Defaults

| Setting | Dev | Prod |
|---------|-----|------|
| Replicas | 1 | 3 |
| CPU Request | 200m | 500m |
| Memory Request | 512Mi | 1Gi |
| CPU Limit | 1000m | 2 |
| Memory Limit | 1Gi | 2Gi |
| Persistence | Disabled | Enabled (10Gi) |
| CoreDNS Replicas | 1 | 2 |

## Usage

### Deploy the Promise

The promise is deployed automatically via ArgoCD from the `promises/` directory. To manually apply:

```bash
kubectl apply -f promises/vcluster-orchestrator-v2/promise.yaml
```

### Create a VCluster

Create a `VClusterOrchestratorV2` resource in `platform/vclusters/`:

```yaml
apiVersion: platform.integratn.tech/v1alpha1
kind: VClusterOrchestratorV2
metadata:
  name: my-vcluster
  namespace: platform-requests
spec:
  name: my-vcluster
  preset: prod
  hostname: my-vcluster.integratn.tech
  # See promise.yaml CRD for full spec options
```

### Verify

```bash
# Check orchestrator status
kubectl get vclusterorchestratorv2 -A

# Check sub-promise ResourceRequests
kubectl get argocdproject,argocdapplication,argocdclusterregistration -A

# Check pipeline pods
kubectl get pods -n platform-requests -l kratix.io/promise-name=vcluster-orchestrator-v2
```

## Troubleshooting

### Pipeline Failures
```bash
kubectl logs -n platform-requests -l kratix.io/promise-name=vcluster-orchestrator-v2 -c configure
```

### Sub-Promise Issues
```bash
# Check if sub-promise ResourceRequests were created
kubectl get argocdproject,argocdapplication,argocdclusterregistration -n platform-requests

# Check sub-promise pipeline logs
kubectl logs -n platform-requests -l kratix.io/promise-name=argocd-cluster-registration -c configure
```

### Kubeconfig Sync Issues
```bash
kubectl get jobs -n <namespace> | grep kubeconfig-sync
kubectl logs -n <namespace> job/vcluster-<name>-kubeconfig-sync
```

### ArgoCD Cluster Registration
```bash
# Check the ExternalSecret-generated cluster secret
kubectl get secret -n argocd -l argocd.argoproj.io/secret-type=cluster

# Verify in ArgoCD
kubectl get app -n argocd | grep <name>
```

## Development

### Building Locally

```bash
cd promises/vcluster-orchestrator-v2/workflows/resource/configure
go vet ./...
go build -o /dev/null .
```

CI automatically builds and pushes the Docker image on changes to `promises/vcluster-orchestrator-v2/**`.

### IP Utilities

- `defaultVIPFromCIDR(cidr, offset)` — calculates default VIP (e.g., .100) in subnet
- `ipInCIDR(ip, cidr)` — validates IP falls within subnet boundaries

## Related Promises

- [argocd-project](../argocd-project/) — creates ArgoCD AppProject resources
- [argocd-application](../argocd-application/) — creates ArgoCD Application resources
- [argocd-cluster-registration](../argocd-cluster-registration/) — handles full cluster registration (1P, RBAC, kubeconfig sync, ArgoCD secret)

## Related Documentation

- [Architecture Overview](../../docs/architecture.md)
- [Promise Development Guide](../../docs/promises.md)
- [VCluster Operations](../../docs/vclusters.md)
