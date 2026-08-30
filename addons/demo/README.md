# Delivery-flow fixtures

Files here are **applied by nothing**. No Application or ApplicationSet points
at this directory; ArgoCD never sees it. What does see it is bosun's
repository scan: a manifest here declaring an API version a chart bump stops
serving counts as a consumer, blocks the bump, and gets migrated by the agent
like any real manifest would.

That is the point. This directory holds manifests in deliberately old shapes,
ported from bosun's local proving ground (`local/` in that repository), so the
repair paths can be watched running against this repository without putting a
demo workload on the cluster.

`legacy-external-secret.yaml` is the structural case: external-secrets
v1alpha1 spelled `dataFrom` as a flat `[{key, property, version}]`, which v1
replaced with `[{extract: {…}}]`. Swapping the apiVersion line alone leaves a
document the apiserver prunes those fields from silently — so the swap is not
enough, and the agent has to ask the model for the reshape, validated before a
byte lands.

Delete this directory whenever the demo has served its purpose; nothing will
notice.
