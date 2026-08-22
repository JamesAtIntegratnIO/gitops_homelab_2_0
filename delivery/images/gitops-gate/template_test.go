package main

import "testing"

func TestFastTemplateResolvesClusterPaths(t *testing.T) {
	data := Cluster{
		Name:   "hub",
		Labels: map[string]string{"environment": "production", "cluster_role": "control-plane"},
	}.TemplateData(nil)

	got, err := renderFastTemplate(
		"$values/addons/environments/{{metadata.labels.environment}}/addons/addons.yaml", data)
	if err != nil {
		t.Fatal(err)
	}
	want := "$values/addons/environments/production/addons/addons.yaml"
	if got != want {
		t.Fatalf("want %q, got %q", want, got)
	}
}

// A placeholder with no value must fail loudly. Rendering it as an empty string
// produces a path that silently resolves to the wrong values layer.
func TestFastTemplateFailsOnMissingKey(t *testing.T) {
	data := Cluster{Name: "hub"}.TemplateData(nil)
	if _, err := renderFastTemplate("{{metadata.labels.nope}}", data); err == nil {
		t.Fatal("a missing key must be an error, not an empty string")
	}
}

func TestGoTemplateRendersValuesAndName(t *testing.T) {
	data := Cluster{
		Name:   "hub",
		Labels: map[string]string{"environment": "production"},
	}.TemplateData(map[string]string{"chart": "cert-manager", "addonChartVersion": "v1.2.3"})

	out, err := renderGoTemplate(map[string]any{
		"metadata": map[string]any{"name": "{{ .values.chart }}-{{ .name }}"},
		"spec":     map[string]any{"targetRevision": "{{ .values.addonChartVersion }}"},
	}, data)
	if err != nil {
		t.Fatal(err)
	}
	m := out.(map[string]any)
	meta := m["metadata"].(map[string]any)
	if meta["name"] != "cert-manager-hub" {
		t.Fatalf("want cert-manager-hub, got %v", meta["name"])
	}
}

func TestSplitPathHandlesQuotedIndex(t *testing.T) {
	data := map[string]any{"metadata": map[string]any{
		"labels": map[string]any{"platform.example/team": "core"},
	}}
	got, ok := lookupPath(data, `metadata.labels["platform.example/team"]`)
	if !ok || got != "core" {
		t.Fatalf("want core, got %q ok=%v", got, ok)
	}
}
