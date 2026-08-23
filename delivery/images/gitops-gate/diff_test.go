package main

import (
	"strings"
	"testing"
)

func row(cluster, app, version string) Row {
	return Row{
		AppSet: "addons", Cluster: cluster, App: app,
		SourceType: "helm", Chart: "thing", ChartRepo: "https://charts.example",
		Version: version, Project: "default", Namespace: "ns",
	}
}

// A version bump is the whole point of the pipeline. It must be reported and
// must not block, or every automated merge parks forever.
func TestVersionChangeIsReportedNotBlocking(t *testing.T) {
	base := &Table{Rows: []Row{row("hub", "thing-hub", "1.0.0")}}
	head := &Table{Rows: []Row{row("hub", "thing-hub", "1.1.0")}}

	d := Diff(base, head)
	if d.Blocking() {
		t.Fatal("a version change must not block")
	}
	if len(d.Versions) != 1 || d.Versions[0].To != "1.1.0" {
		t.Fatalf("want one version change to 1.1.0, got %+v", d.Versions)
	}
}

func TestNewClusterTargetBlocks(t *testing.T) {
	base := &Table{Rows: []Row{row("hub", "thing-hub", "1.0.0")}}
	head := &Table{Rows: []Row{
		row("hub", "thing-hub", "1.0.0"),
		row("tenant", "thing-tenant", "1.0.0"),
	}}

	d := Diff(base, head)
	if !d.Blocking() {
		t.Fatal("an app newly targeting a cluster must block")
	}
	if len(d.Targeting) != 1 || d.Targeting[0].Kind != "added" {
		t.Fatalf("want one added targeting change, got %+v", d.Targeting)
	}
}

func TestDroppedClusterTargetBlocks(t *testing.T) {
	base := &Table{Rows: []Row{
		row("hub", "thing-hub", "1.0.0"),
		row("tenant", "thing-tenant", "1.0.0"),
	}}
	head := &Table{Rows: []Row{row("hub", "thing-hub", "1.0.0")}}

	d := Diff(base, head)
	if !d.Blocking() {
		t.Fatal("an app no longer targeting a cluster must block -- that is a silent uninstall")
	}
	if d.Targeting[0].Kind != "removed" {
		t.Fatalf("want a removal, got %+v", d.Targeting)
	}
}

// An app leaving one cluster and arriving at another is one event. Reporting it
// as an unrelated add plus an unrelated remove buries the actual shape.
func TestMoveBetweenClustersIsOneChange(t *testing.T) {
	base := &Table{Rows: []Row{row("hub", "thing-hub", "1.0.0")}}
	head := &Table{Rows: []Row{row("tenant", "thing-tenant", "1.0.0")}}

	d := Diff(base, head)
	if len(d.Targeting) != 1 {
		t.Fatalf("want a single move, got %d changes: %+v", len(d.Targeting), d.Targeting)
	}
	if d.Targeting[0].Kind != "moved" {
		t.Fatalf("want kind=moved, got %q", d.Targeting[0].Kind)
	}
}

// A chart swapped underneath an unchanged Application name is not a version
// bump, and must not be waved through as one.
func TestChartSwapBlocks(t *testing.T) {
	base := &Table{Rows: []Row{row("hub", "thing-hub", "1.0.0")}}
	swapped := row("hub", "thing-hub", "1.0.0")
	swapped.Chart = "something-else"
	head := &Table{Rows: []Row{swapped}}

	d := Diff(base, head)
	if !d.Blocking() {
		t.Fatal("changing the chart behind an app must block")
	}
}

func TestNoChangeIsNotBlocking(t *testing.T) {
	base := &Table{Rows: []Row{row("hub", "thing-hub", "1.0.0")}}
	head := &Table{Rows: []Row{row("hub", "thing-hub", "1.0.0")}}
	if d := Diff(base, head); d.Blocking() || len(d.Versions) != 0 {
		t.Fatalf("identical renders must produce nothing, got %+v", d)
	}
}

// Adding a new addon is a deliberate act by the pull request's author. Blocking
// on it would make every new-addon PR red for a reason nobody needs to look
// into, and a check that is routinely overridden stops working as a check.
func TestNewAddonIsReportedNotBlocking(t *testing.T) {
	base := &Table{Rows: []Row{row("hub", "existing-hub", "1.0.0")}}
	newRow := row("hub", "brand-new-hub", "1.0.0")
	newRow.AppSet = "brand-new"
	head := &Table{Rows: []Row{row("hub", "existing-hub", "1.0.0"), newRow}}

	d := Diff(base, head)
	if d.Blocking() {
		t.Fatal("a brand-new addon must not block")
	}
	if len(d.Introduced) != 1 || d.Introduced[0].Kind != "introduced" {
		t.Fatalf("want one introduced change, got %+v", d.Introduced)
	}
	if len(d.Targeting) != 0 {
		t.Fatalf("a new addon is not a targeting change, got %+v", d.Targeting)
	}
}

// The distinction that matters: an addon that already existed gaining a cluster
// is the leak, and must still block.
func TestExistingAddonGainingAClusterStillBlocks(t *testing.T) {
	base := &Table{Rows: []Row{row("hub", "thing-hub", "1.0.0")}}
	head := &Table{Rows: []Row{
		row("hub", "thing-hub", "1.0.0"),
		row("tenant", "thing-tenant", "1.0.0"), // same AppSet, new cluster
	}}

	d := Diff(base, head)
	if !d.Blocking() {
		t.Fatal("an existing addon reaching a new cluster must block -- this is the leak")
	}
	if len(d.Introduced) != 0 {
		t.Fatalf("must not be classified as a new addon, got %+v", d.Introduced)
	}
}

// The marker is a contract with whatever reads the gate's verdict off a pull
// request -- a triage agent finds the report by searching comments for it.
//
// It is asserted on BOTH a blocking report and a green one. Before this test
// existed the marker was prepended by one shell script in the local proving
// ground and by no adapter at all, so the agent could never have found a
// report published by CI. A report that leads with anything else is a report
// nothing can locate, which is indistinguishable from no gate at all.
func TestReportLeadsWithTheMarker(t *testing.T) {
	for _, tc := range []struct {
		name       string
		base, head *Table
	}{
		{
			name: "blocking",
			base: &Table{Rows: []Row{row("hub", "thing-hub", "1.0.0")}},
			head: &Table{Rows: []Row{
				row("hub", "thing-hub", "1.0.0"),
				row("tenant", "thing-tenant", "1.0.0"),
			}},
		},
		{
			name: "no change at all",
			base: &Table{Rows: []Row{row("hub", "thing-hub", "1.0.0")}},
			head: &Table{Rows: []Row{row("hub", "thing-hub", "1.0.0")}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var b strings.Builder
			Diff(tc.base, tc.head).Report(&b)

			got := b.String()
			if !strings.HasPrefix(got, ReportMarker+"\n") {
				t.Fatalf("report must lead with %q, got:\n%s", ReportMarker, got)
			}
			if strings.Count(got, ReportMarker) != 1 {
				t.Fatalf("marker must appear exactly once, got %d", strings.Count(got, ReportMarker))
			}
		})
	}
}
