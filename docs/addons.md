# Addons

This folder is the **addons layer** of the GitOps architecture. It is responsible for installing platform services via ApplicationSets.

## How the Addons Layer Works
1. **Bootstrap ApplicationSet** (created by Terraform) points ArgoCD at this repo.
2. The `addons/charts/application-sets` Helm chart renders one **ApplicationSet per addon**.
3. Each addon ApplicationSet generates one **Application per cluster** based on labels on the ArgoCD cluster Secret.
4. Each Application pulls Helm values from this repo using **multi‑source** and `$values/...` paths.

The chart is defined in:
- `addons/charts/application-sets/`

Key templates:
- `addons/charts/application-sets/templates/application-set.yaml`
- `addons/charts/application-sets/templates/_application_set.tpl`

## Folder Structure
- `addons/charts/`: ApplicationSet chart and shared helpers.
- `addons/cluster-roles/`: role‑based configs (control‑plane, vcluster).
- `addons/environments/`: environment overlays (production/staging/development).
- `addons/clusters/`: per‑cluster overrides and values.

## Value File Precedence
The ApplicationSet chart searches for values in this order:
1. `addons/default/addons/<addon>/values.yaml`
2. `addons/clusters/<environment>/addons/<addon>/values.yaml`
3. `addons/clusters/<cluster>/addons/<addon>/values.yaml`

`ignoreMissingValueFiles: true` is enabled so missing files are safe.

## Cluster Selection (Label‑Driven)
Each addon has a `selector` block in `addons/.../addons.yaml` that targets ArgoCD cluster Secrets. This lets you gate addons by label, e.g. `enable_kratix=true` or `cluster_role=control-plane`.

## Key Addons in This Repo
- **ArgoCD**: core GitOps engine.
- **cert‑manager**: TLS automation.
- **external‑dns**: DNS record management.
- **nginx‑gateway‑fabric**: Gateway API implementation.
- **kyverno**: policy engine with ArgoCD‑safe settings.
- **kube‑prometheus‑stack**: metrics, alerts, dashboards.
- **loki + promtail**: cluster log aggregation.

## Gateway Exposure
Routes for platform services live in:
- `addons/cluster-roles/control-plane/addons/observability-secrets/observability-httproutes.yaml`

## TLS Certificates
Gateway TLS certs are managed by cert‑manager. The wildcard cert used by the gateway is defined here:
- `addons/clusters/the-cluster/addons/nginx-gateway-fabric/certificate-wildcard-cluster-integratn-tech.yaml`

## Adding a New Addon (Practical Checklist)
1. Add the addon entry under the target `addons.yaml`.
2. Add values under the appropriate overlay folder.
3. Ensure the target cluster Secret has matching labels.
4. Sync the ApplicationSet and verify the generated Application.

## Operational Notes
- Prefer ClusterIP services behind the gateway.
- Keep values scoped to the smallest layer (cluster > environment > default).
- Use `valuesObject` for templated settings driven by cluster metadata.
