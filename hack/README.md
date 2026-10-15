# Hack

Development and testing utilities.

| File | Purpose |
|------|---------|
| `test-connect-api.sh` | Test 1Password Connect API connectivity |
| `test-pipeline-job.yaml` | Manual Kubernetes Job for testing vcluster-orchestrator-v2 pipeline locally |

## dashboards/

`homelab_overview.py` generates the Homelab Overview Grafana dashboard
(`addons/cluster-roles/control-plane/addons/platform-landing-zone/homelab-overview.yaml`).
Edit the script, run `python3 hack/dashboards/homelab_overview.py`, commit both.
`--exprs` prints every query (one per line, tab-separated with its panel) for
checking against live Prometheus.
