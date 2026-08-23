// Package evals holds the triage cases the prompt is measured against.
//
// Every case is a real incident from a production GitOps repository, not an
// invented one. That matters: the failures worth catching are the ones where
// a pull request renders perfectly and breaks at runtime, and those have a
// particular shape that made-up examples do not reproduce.
package evals

// Case is one triage scenario.
type Case struct {
	Name string

	// Files are the repository fixture, path -> content.
	Files map[string]string

	// Subject is the bump: what moved, from where to where.
	Subject string

	// GateReport is what the pre-merge gate said, as a human would see it.
	GateReport string

	// WantClass is the correct classification.
	WantClass string

	// WantEdits maps key -> expected new value. Only checked for mechanical
	// cases. An answer may include these and no others.
	WantEdits map[string]string

	// EditFile is the path every expected edit targets.
	EditFile string
}

const addonsPath = "addons/environments/production/addons/addons.yaml"

// Cases are ordered roughly by how much judgement they need.
var Cases = []Case{
	{
		Name:    "metallb-frr-defaults-flip",
		Subject: "bump metallb chart 0.15.2 -> 0.16.0",
		Files: map[string]string{addonsPath: `metallb:
  enabled: true
  namespace: metallb-system
  chartName: metallb
  defaultVersion: 0.16.0
  valuesObject:
    speaker:
      frr:
        enabled: true
    frrk8s:
      enabled: true
`},
		GateReport: `The gate is RED.

Rendered diff, metallb 0.15.2 -> 0.16.0:
  removed  Container/speaker/frr
  removed  Container/speaker/frr-metrics
  added    DaemonSet/metallb-frr-k8s
  added    CustomResourceDefinition/frrconfigurations.frrk8s.metallb.io
  added    CustomResourceDefinition/frrnodestates.frrk8s.metallb.io
  added    ValidatingWebhookConfiguration/frr-k8s-validating-webhook

Chart 0.16.0 changed its defaults: speaker.frr.enabled now defaults false and
frrk8s.enabled now defaults true. This cluster is L2-only and does not use FRR
in any form.`,
		WantClass: "mechanical",
		EditFile:  addonsPath,
		WantEdits: map[string]string{
			"metallb.valuesObject.speaker.frr.enabled": "false",
			"metallb.valuesObject.frrk8s.enabled":      "false",
		},
	},
	{
		Name:    "argocd-networkpolicy-default-on",
		Subject: "bump argo-cd chart 9.4.3 -> 10.0.0",
		Files: map[string]string{addonsPath: `argocd:
  enabled: true
  namespace: argocd
  chartName: argo-cd
  defaultVersion: 10.0.0
  valuesObject:
    global:
      networkPolicy:
        create: true
`},
		GateReport: `The gate is RED.

Rendered diff, argo-cd 9.4.3 -> 10.0.0:
  added  NetworkPolicy/argocd-application-controller
  added  NetworkPolicy/argocd-repo-server
  added  NetworkPolicy/argocd-server
  added  NetworkPolicy/argocd-redis

Chart 10.0.0 flips global.networkPolicy.create to true by default. This
repository owns NetworkPolicies in a separate network-policies addon, plus a
Kyverno default-deny policy that generates one per namespace. Chart-authored
policies conflict with both.`,
		WantClass: "mechanical",
		EditFile:  addonsPath,
		WantEdits: map[string]string{
			"argocd.valuesObject.global.networkPolicy.create": "false",
		},
	},
	{
		Name:    "coupled-pin-gateway-api",
		Subject: "bump nginx-gateway-fabric chart 2.5.1 -> 2.6.7",
		Files: map[string]string{addonsPath: `gateway-api-crds:
  enabled: true
  type: manifest
  defaultVersion: v1.4.0
nginx-gateway-fabric:
  enabled: true
  namespace: nginx-gateway
  chartName: nginx-gateway-fabric
  defaultVersion: 2.6.7
`},
		GateReport: `The gate is RED.

nginx-gateway-fabric 2.6.7 requires Gateway API v1.5. The gateway-api-crds
addon is pinned at v1.4.0, so the controller will not start: it fails its
CRD version check at boot.

The exact version to move to is v1.5.1. The stored API versions in this
cluster are already v1, and Gateway API v1.5.1 still serves v1beta1, so this is
not a storage migration.`,
		WantClass: "mechanical",
		EditFile:  addonsPath,
		WantEdits: map[string]string{
			"gateway-api-crds.defaultVersion": "v1.5.1",
		},
	},
	{
		Name:    "metrics-port-moved-under-a-netpol",
		Subject: "bump metallb chart 0.15.2 -> 0.16.0 (metrics ports)",
		Files: map[string]string{"addons/cluster-roles/control-plane/addons/network-policies/metallb-system.yaml": `apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-monitoring-metallb
  namespace: metallb-system
spec:
  ingress:
    - ports:
        - protocol: TCP
          port: 7472
`},
		GateReport: `The gate is RED.

Rendered diff, metallb 0.15.2 -> 0.16.0:
  Service/metallb-controller  containerPort 7472 -> 9120
  Service/metallb-speaker     containerPort 7472 -> 9120

The NetworkPolicy allow-monitoring-metallb still names port 7472, so Prometheus
will be unable to scrape either component. Nothing reports an error -- scraping
simply stops.`,
		WantClass: "mechanical",
		EditFile:  "addons/cluster-roles/control-plane/addons/network-policies/metallb-system.yaml",
		WantEdits: map[string]string{
			"spec.ingress.0.ports.0.port": "9120",
		},
	},
	{
		// The same shape as the case above with one thing removed: the exact
		// version. A model that fills that gap from memory produces a change
		// that renders perfectly and is wrong.
		Name:    "coupled-pin-version-unstated",
		Subject: "bump nginx-gateway-fabric chart 2.5.1 -> 2.6.7",
		Files: map[string]string{addonsPath: `gateway-api-crds:
  enabled: true
  type: manifest
  defaultVersion: v1.4.0
nginx-gateway-fabric:
  enabled: true
  namespace: nginx-gateway
  chartName: nginx-gateway-fabric
  defaultVersion: 2.6.7
`},
		GateReport: `The gate is RED.

nginx-gateway-fabric 2.6.7 requires Gateway API v1.5 or newer. The
gateway-api-crds addon is pinned at v1.4.0, so the controller fails its CRD
version check at boot.

No specific patch release of Gateway API is named anywhere in this report.`,
		WantClass: "escalate",
	},
	{
		Name:    "authentik-illegal-version-skip",
		Subject: "bump authentik chart 2025.12.4 -> 2026.8.0",
		Files: map[string]string{addonsPath: `authentik:
  enabled: true
  namespace: authentik
  chartName: authentik
  defaultVersion: 2026.8.0
`},
		GateReport: `The gate is GREEN. The rendered diff shows only the image tag moving.

Upstream release notes state that authentik refuses to migrate across
major.minor releases in a single step: ensure_allowed_version() raises before
run_migrations(). The supported path is one release at a time.`,
		WantClass: "escalate",
	},
	{
		Name:    "external-secrets-api-version-removed",
		Subject: "bump external-secrets chart 1.9.4 -> 2.9.0",
		Files: map[string]string{addonsPath: `external-secrets:
  enabled: true
  namespace: external-secrets
  chartName: external-secrets
  defaultVersion: 2.9.0
`},
		GateReport: `The gate is RED.

Rendered diff, external-secrets 1.9.4 -> 2.9.0:
  apiVersion changed  CustomResourceDefinition/externalsecrets.external-secrets.io  v1beta1 -> v1
  v1beta1 is no longer served by default in 2.x.

39 ExternalSecret manifests across 29 files in this repository still declare
apiVersion external-secrets.io/v1beta1.`,
		WantClass: "escalate",
	},
	{
		Name:    "kyverno-drops-subcharts",
		Subject: "bump kyverno chart 3.2.6 -> 3.9.0",
		Files: map[string]string{addonsPath: `kyverno:
  enabled: true
  namespace: kyverno
  chartName: kyverno
  defaultVersion: 3.9.0
  valuesObject:
    cleanupJobs:
      admissionReports:
        enabled: true
    policyReportsCleanup:
      enabled: true
`},
		GateReport: `The gate is RED.

Rendered diff, kyverno 3.2.6 -> 3.9.0:
  removed  CronJob/kyverno-cleanup-admission-reports
  removed  CronJob/kyverno-cleanup-cluster-admission-reports
  values keys no longer read by the chart:
    cleanupJobs.admissionReports.enabled
    policyReportsCleanup.enabled

The chart drops both subcharts. Seven minors of generate-rule behaviour change
sit between these versions, under a webhook with failurePolicy: Fail.`,
		WantClass: "escalate",
	},
	{
		Name:    "unrelated-preexisting-failure",
		Subject: "bump qdrant chart 1.15.0 -> 1.15.1",
		Files: map[string]string{addonsPath: `qdrant:
  enabled: true
  namespace: ai
  chartName: qdrant
  defaultVersion: 1.15.1
`},
		GateReport: `The gate is RED.

Rendered diff, qdrant 1.15.0 -> 1.15.1: no resources added, removed or
changed apiVersion. Only the image tag moved.

Schema validation failed on a DIFFERENT addon: Application/open-webui declares
a field the CRD schema rejects. That file is not touched by this pull request
and the failure predates it.`,
		WantClass: "no_action",
	},
}
