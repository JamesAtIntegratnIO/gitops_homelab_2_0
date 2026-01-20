# VCluster Orchestrator Promise

This promise orchestrates the full vcluster stack by rendering mandatory ResourceRequests for sub-promises:
- `vcluster-core`
- `vcluster-coredns`
- `argocd-project`
- `argocd-application`
- `vcluster-kubeconfig-sync`
- `vcluster-kubeconfig-external-secret`
- `vcluster-argocd-cluster-registration`

All inputs are structured; raw manifest inputs are not supported.
