# Update Log

## 2026-08-21
* **Update**: Applied the quick wins from [known issues](cluster/known-issues.md): 6 GitOps commits on `claude/repo-cluster-learning-8dcc81` (pre-commit hook fix, llmkube/git-indexer config removal, mcp-system addon adoption + secret-ref fix, etcd-secret-writer applyability fix, GatewayRoute ignoreDifferences, generate-policy unblock) plus out-of-band deletion of orphans (git-indexer stack, stuck trivy jobs, stale suspended VCO pipeline job). Trivy scanning and the vcluster-media status contract recovered immediately; remaining steps execute after the branch merges to main. Added a remediation-status section to the known-issues concept.

## 2026-08-20
* **Creation**: Initial population of the bundle from a deep review of the repository and a live sweep of `the-cluster` (kubectl, context `admin@the-cluster`). Added 25 concepts across [platform/](platform/index.md), [cluster/](cluster/index.md), [addons/](addons/index.md), [promises/](promises/index.md), [infrastructure/](infrastructure/index.md), and [tooling/](tooling/index.md), including dated snapshots ([workload inventory](cluster/workload-inventory.md), [component versions](cluster/component-versions.md)) and a consolidated [known issues & drift](cluster/known-issues.md) report.
* **Creation**: Scaffolded the GitOps Homelab 2.0 Knowledge Bundle with `okf_init.py` — see [getting started](getting-started.md).
