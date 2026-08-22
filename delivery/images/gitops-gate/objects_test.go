package main

import (
	"strings"
	"testing"
)

func obj(kind, ns, name, apiVersion, hash string) Object {
	return Object{Kind: kind, Namespace: ns, Name: name, APIVersion: apiVersion, Hash: hash}
}

// The signal the Application table cannot carry. A chart moving 0.15.2 -> 0.16.0
// is one version row; what it DOES is remove two containers and add a
// DaemonSet, four CRDs and a webhook.
func TestObjectDiffReportsAddedRemovedAndChanged(t *testing.T) {
	base := []Object{
		obj("Deployment", "metallb-system", "controller", "apps/v1", "aaa"),
		obj("DaemonSet", "metallb-system", "speaker", "apps/v1", "bbb"),
	}
	head := []Object{
		obj("Deployment", "metallb-system", "controller", "apps/v1", "aaa"),
		obj("DaemonSet", "metallb-system", "speaker", "apps/v1", "CHANGED"),
		obj("DaemonSet", "metallb-system", "frr-k8s", "apps/v1", "ccc"),
	}

	got := map[string]string{}
	for _, c := range diffObjects(base, head) {
		got[c.Object] = c.Kind
	}
	if got["DaemonSet/frr-k8s in metallb-system"] != "added" {
		t.Errorf("new DaemonSet must be reported added: %v", got)
	}
	if got["DaemonSet/speaker in metallb-system"] != "changed" {
		t.Errorf("modified DaemonSet must be reported changed: %v", got)
	}
	if _, ok := got["Deployment/controller in metallb-system"]; ok {
		t.Errorf("an unchanged object must not be reported: %v", got)
	}
}

// An apiVersion moving under an existing resource is a MIGRATION. Reporting it
// as one removal plus one addition hides the very thing a reviewer needs, and
// it is the single most reliable indicator that a bump needs a human.
func TestApiVersionChangeIsItsOwnKindAndBlocks(t *testing.T) {
	base := []Object{obj("CustomResourceDefinition", "", "externalsecrets.external-secrets.io", "apiextensions.k8s.io/v1beta1", "x")}
	head := []Object{obj("CustomResourceDefinition", "", "externalsecrets.external-secrets.io", "apiextensions.k8s.io/v1", "y")}

	changes := diffObjects(base, head)
	if len(changes) != 1 {
		t.Fatalf("want one change, not an add plus a remove: %+v", changes)
	}
	if changes[0].Kind != "apiVersion" {
		t.Fatalf("want kind=apiVersion, got %q", changes[0].Kind)
	}

	d := &DiffResult{Objects: changes}
	if !d.Blocking() {
		t.Error("an API version migration must block")
	}
}

// Objects appearing or changing is what a legitimate version bump does, so it
// is reported and must not block on its own.
func TestObjectAdditionsAloneDoNotBlock(t *testing.T) {
	d := &DiffResult{Objects: []ObjectChange{
		{Kind: "added", Object: "DaemonSet/frr-k8s"},
		{Kind: "changed", Object: "Deployment/controller"},
	}}
	if d.Blocking() {
		t.Error("added and changed resources are the point of a bump; they must not block")
	}
}

func TestObjectIDIgnoresApiVersionSoMigrationsAreVisible(t *testing.T) {
	a := obj("CustomResourceDefinition", "", "thing", "apiextensions.k8s.io/v1beta1", "x")
	b := obj("CustomResourceDefinition", "", "thing", "apiextensions.k8s.io/v1", "y")
	if a.ID() != b.ID() {
		t.Fatal("the same resource under two API versions must share an ID")
	}
}

func TestObjectFromRejectsNonResources(t *testing.T) {
	for name, in := range map[string]map[string]any{
		"no kind":       {"apiVersion": "v1", "metadata": map[string]any{"name": "x"}},
		"no apiVersion": {"kind": "ConfigMap", "metadata": map[string]any{"name": "x"}},
		"no name":       {"kind": "ConfigMap", "apiVersion": "v1"},
	} {
		if _, ok := objectFrom("s", "", "", in); ok {
			t.Errorf("%s: must be rejected", name)
		}
	}
}

func TestObjectReportNamesTheMigrationExplicitly(t *testing.T) {
	d := &DiffResult{Objects: []ObjectChange{{
		Kind: "apiVersion", Object: "CustomResourceDefinition/externalsecrets.external-secrets.io",
		From: "apiextensions.k8s.io/v1beta1", To: "apiextensions.k8s.io/v1",
	}}}
	var b strings.Builder
	d.Report(&b)
	out := b.String()
	if !strings.Contains(out, "migration, not a bump") {
		t.Errorf("the report must say what an apiVersion change means:\n%s", out)
	}
	if !strings.Contains(out, "v1beta1") || !strings.Contains(out, "→ `apiextensions.k8s.io/v1`") {
		t.Errorf("both versions must appear:\n%s", out)
	}
}

// The rendered-manifests pattern end to end: manifests already in git, diffed
// as objects. ArgoCD's source hydrator produces exactly this shape, and so
// does any CI job that commits its render.
func TestRenderedSourceProducesAnObjectDiff(t *testing.T) {
	before := writeRepo(t, map[string]string{"rendered/prod/manifest.yaml": `
apiVersion: apps/v1
kind: Deployment
metadata: {name: controller, namespace: metallb-system}
spec: {replicas: 1}
---
apiVersion: apps/v1
kind: DaemonSet
metadata: {name: speaker, namespace: metallb-system}
spec: {revisionHistoryLimit: 10}
`})
	after := writeRepo(t, map[string]string{"rendered/prod/manifest.yaml": `
apiVersion: apps/v1
kind: Deployment
metadata: {name: controller, namespace: metallb-system}
spec: {replicas: 1}
---
apiVersion: apps/v1
kind: DaemonSet
metadata: {name: frr-k8s, namespace: metallb-system}
spec: {revisionHistoryLimit: 10}
`})
	cfg := &Config{Concurrency: 2, Sources: []Source{
		{Name: "hydrated", Type: SourceRendered, Paths: []string{"rendered/**/manifest.yaml"}},
	}}
	inv := fleet(t, before, []Cluster{{Name: "prod"}})

	b, err := Render(before, cfg, inv)
	if err != nil {
		t.Fatal(err)
	}
	h, err := Render(after, cfg, inv)
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Objects) != 2 || len(h.Objects) != 2 {
		t.Fatalf("want 2 objects each side, got %d and %d", len(b.Objects), len(h.Objects))
	}

	kinds := map[string]string{}
	for _, c := range Diff(b, h).Objects {
		kinds[c.Object] = c.Kind
	}
	if kinds["DaemonSet/speaker in metallb-system"] != "removed" {
		t.Errorf("want speaker removed, got %v", kinds)
	}
	if kinds["DaemonSet/frr-k8s in metallb-system"] != "added" {
		t.Errorf("want frr-k8s added, got %v", kinds)
	}
}

// Whether a chart stamps metadata.namespace varies between versions of the
// SAME chart -- podinfo omits it at 6.7.0 and sets it at 6.14.1. Keying on the
// raw value made every object in the chart read as one removal plus one
// addition, which buried the actual change under a full-set churn.
func TestNamespaceDefaultsToTheApplicationDestination(t *testing.T) {
	without := map[string]any{
		"apiVersion": "apps/v1", "kind": "Deployment",
		"metadata": map[string]any{"name": "podinfo"},
	}
	with := map[string]any{
		"apiVersion": "apps/v1", "kind": "Deployment",
		"metadata": map[string]any{"name": "podinfo", "namespace": "podinfo-tenant"},
	}

	a, ok := objectFrom("app", "local", "podinfo-tenant", without)
	if !ok {
		t.Fatal("object was rejected")
	}
	b, ok := objectFrom("app", "local", "podinfo-tenant", with)
	if !ok {
		t.Fatal("object was rejected")
	}
	if a.ID() != b.ID() {
		t.Fatalf("same resource got two identities:\n  %s\n  %s", a.ID(), b.ID())
	}
}

// A destination-less render must not invent one, or committed manifests for a
// cluster-scoped resource would acquire a namespace they do not have.
func TestNamespaceIsNotInventedWithoutADestination(t *testing.T) {
	o, ok := objectFrom("app", "local", "", map[string]any{
		"apiVersion": "rbac.authorization.k8s.io/v1", "kind": "ClusterRole",
		"metadata": map[string]any{"name": "reader"},
	})
	if !ok {
		t.Fatal("object was rejected")
	}
	if o.Namespace != "" {
		t.Fatalf("namespace = %q, want empty", o.Namespace)
	}
}

// Helm test hooks are never applied by a sync, and charts routinely give them
// a random name -- so they appeared as the same three pods added AND removed
// on every render. Other hooks are applied and must still be reported.
func TestHelmTestHooksAreExcludedButOtherHooksAreNot(t *testing.T) {
	hooked := func(v string) map[string]any {
		return map[string]any{
			"apiVersion": "v1", "kind": "Pod",
			"metadata": map[string]any{
				"name":        "podinfo-grpc-test-lxlnc",
				"annotations": map[string]any{"helm.sh/hook": v},
			},
		}
	}
	for _, v := range []string{"test", "test-success", "test-failure",
		"test-success,hook-succeeded"} {
		if _, ok := objectFrom("app", "local", "ns", hooked(v)); ok {
			t.Errorf("helm.sh/hook %q was reported as a deployed resource", v)
		}
	}
	for _, v := range []string{"pre-install", "post-upgrade"} {
		if _, ok := objectFrom("app", "local", "ns", hooked(v)); !ok {
			t.Errorf("helm.sh/hook %q was dropped; it IS applied by a sync", v)
		}
	}
}
