package main

import (
	"strings"
	"testing"
)

// Only rows whose version actually moved are rendered. A chart pull and two
// renders per Application is real cost, and on a typical bump pull request
// exactly one row qualifies.
func TestChartDiffOnlyConsidersVersionChanges(t *testing.T) {
	mk := func(app, chart, version, sourceType string) Row {
		return Row{
			Cluster: "prod", App: app, Chart: chart, ChartRepo: "https://charts.example.com",
			Version: version, SourceType: sourceType,
		}
	}
	base := &Table{Rows: []Row{
		mk("same", "a", "1.0.0", "helm"),
		mk("moved", "b", "1.0.0", "helm"),
		mk("path-source", "c", "1.0.0", "path"),
	}}
	head := &Table{Rows: []Row{
		mk("same", "a", "1.0.0", "helm"),
		mk("moved", "b", "2.0.0", "helm"),
		mk("path-source", "c", "2.0.0", "path"),
	}}

	// No helm on PATH in this test; what matters is which pairs it selects,
	// which is observable through the warnings it emits for failed renders.
	cfg := &Config{Concurrency: 2}
	_, _, warns := ChartDiff(t.TempDir(), cfg, base, head)

	joined := strings.Join(warns, "\n")
	if strings.Contains(joined, "same") {
		t.Error("an unchanged version must not be rendered")
	}
	if strings.Contains(joined, "path-source") {
		t.Error("a path source has no chart to render")
	}
}

// A chart that cannot be pulled must be reported. "No resource changes" and
// "we could not look" must never read the same.
func TestChartDiffReportsWhatItCouldNotRender(t *testing.T) {
	row := func(v string) Row {
		return Row{
			Cluster: "prod", App: "thing", Chart: "nonexistent-chart-xyz",
			ChartRepo: "https://charts.invalid.example", Version: v, SourceType: "helm",
		}
	}
	base := &Table{Rows: []Row{row("1.0.0")}}
	head := &Table{Rows: []Row{row("2.0.0")}}

	before, after, warns := ChartDiff(t.TempDir(), &Config{Concurrency: 1}, base, head)
	if len(before) != 0 || len(after) != 0 {
		t.Fatal("a failed render must contribute no objects")
	}
	if len(warns) != 1 || !strings.Contains(warns[0], "NOT covered") {
		t.Fatalf("the failure must be surfaced, got %v", warns)
	}
}

// Helm stamps chart and app version onto every object it renders. Hashing
// those makes a bump report every resource as changed -- measured at 101 of
// 105 on one cert-manager bump -- burying the handful that really changed.
func TestVersionStampsDoNotCountAsChanges(t *testing.T) {
	withStamp := func(chart, version string) map[string]any {
		return map[string]any{
			"apiVersion": "apps/v1", "kind": "Deployment",
			"metadata": map[string]any{
				"name": "controller", "namespace": "x",
				"labels": map[string]any{
					"helm.sh/chart":             chart,
					"app.kubernetes.io/version": version,
					"app.kubernetes.io/name":    "controller",
				},
			},
			"spec": map[string]any{"replicas": 1},
		}
	}
	a, _ := objectFrom("s", "prod", "", withStamp("cert-manager-1.19.3", "v1.19.3"))
	b, _ := objectFrom("s", "prod", "", withStamp("cert-manager-1.21.1", "v1.21.1"))
	if a.Hash != b.Hash {
		t.Fatal("a version stamp alone must not read as a changed resource")
	}

	// A real change still registers.
	changed := withStamp("cert-manager-1.21.1", "v1.21.1")
	changed["spec"] = map[string]any{"replicas": 3}
	c, _ := objectFrom("s", "prod", "", changed)
	if a.Hash == c.Hash {
		t.Fatal("a real spec change must register")
	}
}

// The same resource changing identically on several clusters is one finding.
func TestIdenticalChangesAcrossClustersCollapse(t *testing.T) {
	base := []Object{
		{Cluster: "a", Kind: "Deployment", Namespace: "x", Name: "c", APIVersion: "apps/v1", Hash: "1"},
		{Cluster: "b", Kind: "Deployment", Namespace: "x", Name: "c", APIVersion: "apps/v1", Hash: "1"},
	}
	head := []Object{
		{Cluster: "a", Kind: "Deployment", Namespace: "x", Name: "c", APIVersion: "apps/v1", Hash: "2"},
		{Cluster: "b", Kind: "Deployment", Namespace: "x", Name: "c", APIVersion: "apps/v1", Hash: "2"},
	}
	got := diffObjects(base, head)
	if len(got) != 1 {
		t.Fatalf("want one collapsed change, got %d: %+v", len(got), got)
	}
	if !strings.Contains(got[0].Cluster, "a") || !strings.Contains(got[0].Cluster, "b") {
		t.Errorf("both clusters must be named: %q", got[0].Cluster)
	}
}
