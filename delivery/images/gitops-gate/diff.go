package main

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

type Change struct {
	Kind    string `json:"kind"` // added | removed | version | moved
	Cluster string `json:"cluster"`
	App     string `json:"app"`
	AppSet  string `json:"appset,omitempty"`
	From    string `json:"from,omitempty"`
	To      string `json:"to,omitempty"`
	Detail  string `json:"detail,omitempty"`
}

type DiffResult struct {
	// Targeting changes block the merge: an Application appearing on or
	// vanishing from a cluster is the failure mode that reading a values diff
	// does not reveal.
	Targeting []Change `json:"targeting"`
	// Introduced covers whole ApplicationSets that did not exist before, and
	// does NOT block. Adding an addon is a deliberate act by the author of the
	// pull request; the dangerous case is an addon that already existed
	// quietly changing which clusters it reaches. Blocking on both would make
	// every new-addon PR red for a reason nobody needs to investigate, and a
	// check that is routinely overridden stops being a check.
	Introduced []Change `json:"introduced"`
	// Versions are reported, not blocked -- a version bump is the point.
	Versions []Change `json:"versions"`
	// Other covers a source moving between chart and path, or a project or
	// namespace change: not a targeting change, but not routine either.
	Other []Change `json:"other"`

	// Objects are resource-level differences, present only when the
	// repository renders manifests into git. This is the evidence a reviewer
	// -- or a triage agent -- actually needs: a version number says a chart
	// moved, whereas "removed two containers, added a DaemonSet and four
	// CRDs" says what will happen.
	Objects  []ObjectChange `json:"objects,omitempty"`
	Warnings []string       `json:"warnings,omitempty"`
}

func (d *DiffResult) Blocking() bool {
	if len(d.Targeting) > 0 || len(d.Other) > 0 {
		return true
	}
	// An API version moving under an existing resource is a migration, and
	// migrations are the class of change that renders perfectly and breaks at
	// runtime. Objects appearing or changing are reported but not blocked --
	// that is what a version bump legitimately does.
	for _, o := range d.Objects {
		if o.Kind == "apiVersion" {
			return true
		}
	}
	return false
}

func Diff(base, head *Table) *DiffResult {
	res := &DiffResult{}

	// ApplicationSets present before this change. An ApplicationSet missing
	// from this set is newly introduced, not newly leaking.
	baseAppSets := map[string]bool{}
	for _, r := range base.Rows {
		baseAppSets[r.AppSet] = true
	}

	baseByKey := map[string]Row{}
	for _, r := range base.Rows {
		baseByKey[r.Key()] = r
	}
	headByKey := map[string]Row{}
	for _, r := range head.Rows {
		headByKey[r.Key()] = r
	}

	// An app that vanishes from one cluster and appears on another is one
	// event, not two -- report it as a move so the reviewer sees the shape of
	// what happened rather than two unrelated-looking lines.
	removedByApp := map[string][]Row{}
	addedByApp := map[string][]Row{}

	for k, b := range baseByKey {
		if _, ok := headByKey[k]; !ok {
			removedByApp[b.AppSet] = append(removedByApp[b.AppSet], b)
		}
	}
	for k, h := range headByKey {
		if _, ok := baseByKey[k]; !ok {
			addedByApp[h.AppSet] = append(addedByApp[h.AppSet], h)
		}
	}

	for appset, removed := range removedByApp {
		added := addedByApp[appset]
		for len(removed) > 0 && len(added) > 0 {
			r, a := removed[0], added[0]
			removed, added = removed[1:], added[1:]
			res.Targeting = append(res.Targeting, Change{
				Kind: "moved", AppSet: appset, App: a.App,
				From: r.Cluster, To: a.Cluster,
				Detail: fmt.Sprintf("no longer targets %s, now targets %s", r.Cluster, a.Cluster),
			})
		}
		for _, r := range removed {
			res.Targeting = append(res.Targeting, Change{
				Kind: "removed", AppSet: appset, App: r.App, Cluster: r.Cluster,
				From: r.Describe(), Detail: "no longer generated for this cluster",
			})
		}
		addedByApp[appset] = added
	}
	for appset, added := range addedByApp {
		for _, a := range added {
			c := Change{
				AppSet: appset, App: a.App, Cluster: a.Cluster, To: a.Describe(),
			}
			if baseAppSets[appset] {
				c.Kind = "added"
				c.Detail = "newly generated for this cluster -- this addon already existed and has gained a cluster"
				res.Targeting = append(res.Targeting, c)
			} else {
				c.Kind = "introduced"
				c.Detail = "new addon, first appearance"
				res.Introduced = append(res.Introduced, c)
			}
		}
	}

	for k, h := range headByKey {
		b, ok := baseByKey[k]
		if !ok {
			continue
		}
		switch {
		case b.SourceType != h.SourceType:
			res.Other = append(res.Other, Change{
				Kind: "source-type", Cluster: h.Cluster, App: h.App, AppSet: h.AppSet,
				From: b.Describe(), To: h.Describe(),
				Detail: "the kind of source changed",
			})
		case b.Chart != h.Chart || b.ChartRepo != h.ChartRepo || b.Path != h.Path:
			res.Other = append(res.Other, Change{
				Kind: "source", Cluster: h.Cluster, App: h.App, AppSet: h.AppSet,
				From: b.Describe(), To: h.Describe(),
				Detail: "the source itself changed, not just its version",
			})
		case b.Project != h.Project:
			res.Other = append(res.Other, Change{
				Kind: "project", Cluster: h.Cluster, App: h.App, AppSet: h.AppSet,
				From: b.Project, To: h.Project, Detail: "ArgoCD project changed",
			})
		case b.Namespace != h.Namespace:
			res.Other = append(res.Other, Change{
				Kind: "namespace", Cluster: h.Cluster, App: h.App, AppSet: h.AppSet,
				From: b.Namespace, To: h.Namespace, Detail: "destination namespace changed",
			})
		case b.Version != h.Version:
			res.Versions = append(res.Versions, Change{
				Kind: "version", Cluster: h.Cluster, App: h.App, AppSet: h.AppSet,
				From: b.Version, To: h.Version,
			})
		}
	}

	res.Objects = diffObjects(base.Objects, head.Objects)

	sortChanges(res.Targeting)
	sortChanges(res.Introduced)
	sortChanges(res.Versions)
	sortChanges(res.Other)

	seen := map[string]bool{}
	for _, w := range append(append([]string{}, base.Warnings...), head.Warnings...) {
		if !seen[w] {
			seen[w] = true
			res.Warnings = append(res.Warnings, w)
		}
	}
	return res
}

func sortChanges(c []Change) {
	sort.Slice(c, func(i, j int) bool {
		if c[i].Cluster != c[j].Cluster {
			return c[i].Cluster < c[j].Cluster
		}
		return c[i].App < c[j].App
	})
}

// ReportMarker leads every report, and it is load-bearing rather than
// decorative. A triage agent finds the gate's verdict by looking for this
// string among the pull request's comments, and an adapter that publishes the
// report without it has published something nobody can find.
//
// It lives here, in the binary, for one reason: every CI adapter would
// otherwise have to remember an undocumented magic string, and three of the
// four did not. Emitting it means any adapter that posts the report verbatim
// is correct by construction. It renders as nothing in every markdown surface,
// including a CI job summary.
const ReportMarker = "<!-- gitops-gate -->"

// Report writes a markdown summary suitable for a pull-request comment or a CI
// job summary.
func (d *DiffResult) Report(w io.Writer) {
	fmt.Fprintf(w, "%s\n", ReportMarker)
	if len(d.Targeting) > 0 {
		fmt.Fprintf(w, "### Cluster targeting changed\n\n")
		fmt.Fprintf(w, "These Applications are generated for a different set of clusters than before. ")
		fmt.Fprintf(w, "A values-layer edit can do this without the text diff showing it.\n\n")
		fmt.Fprintf(w, "| Application | Change |\n|---|---|\n")
		for _, c := range d.Targeting {
			fmt.Fprintf(w, "| `%s` | %s |\n", c.App, c.Detail)
		}
		fmt.Fprintln(w)
	}
	if len(d.Other) > 0 {
		fmt.Fprintf(w, "### Source changed\n\n| Application | Cluster | From | To |\n|---|---|---|---|\n")
		for _, c := range d.Other {
			fmt.Fprintf(w, "| `%s` | %s | `%s` | `%s` |\n", c.App, c.Cluster, c.From, c.To)
		}
		fmt.Fprintln(w)
	}
	if len(d.Introduced) > 0 {
		fmt.Fprintf(w, "### New addons\n\n")
		fmt.Fprintf(w, "First appearance, so nothing changed underneath them. Listed for review, not blocking.\n\n")
		fmt.Fprintf(w, "| Application | Cluster | Source |\n|---|---|---|\n")
		for _, c := range d.Introduced {
			fmt.Fprintf(w, "| `%s` | %s | `%s` |\n", c.App, c.Cluster, c.To)
		}
		fmt.Fprintln(w)
	}
	if len(d.Objects) > 0 {
		var api, added, removed, changed []ObjectChange
		for _, o := range d.Objects {
			switch o.Kind {
			case "apiVersion":
				api = append(api, o)
			case "added":
				added = append(added, o)
			case "removed":
				removed = append(removed, o)
			default:
				changed = append(changed, o)
			}
		}
		fmt.Fprintf(w, "### Resources\n\n")
		if len(api) > 0 {
			fmt.Fprintf(w, "**API version changed** — this is a migration, not a bump.\n\n")
			for _, o := range api {
				fmt.Fprintf(w, "- `%s`: `%s` → `%s`\n", o.Object, o.From, o.To)
			}
			fmt.Fprintln(w)
		}
		for _, g := range []struct {
			label string
			items []ObjectChange
		}{{"Added", added}, {"Removed", removed}, {"Changed", changed}} {
			if len(g.items) == 0 {
				continue
			}
			fmt.Fprintf(w, "**%s (%d)**\n\n", g.label, len(g.items))
			for i, o := range g.items {
				if i == 12 {
					fmt.Fprintf(w, "- …and %d more\n", len(g.items)-12)
					break
				}
				fmt.Fprintf(w, "- `%s`\n", o.Object)
			}
			fmt.Fprintln(w)
		}
	}
	if len(d.Versions) > 0 {
		fmt.Fprintf(w, "### Versions\n\n| Application | Cluster | From | To |\n|---|---|---|---|\n")
		for _, c := range d.Versions {
			fmt.Fprintf(w, "| `%s` | %s | `%s` | `%s` |\n", c.App, c.Cluster, c.From, c.To)
		}
		fmt.Fprintln(w)
	}
	if len(d.Targeting) == 0 && len(d.Other) == 0 && len(d.Versions) == 0 &&
		len(d.Introduced) == 0 && len(d.Objects) == 0 {
		fmt.Fprintf(w, "No change to what gets deployed.\n\n")
	}
	if len(d.Warnings) > 0 {
		fmt.Fprintf(w, "### Not covered\n\n")
		fmt.Fprintf(w, "The gate could not expand the following, so the Applications they generate are **not** checked:\n\n")
		for _, warn := range d.Warnings {
			fmt.Fprintf(w, "- %s\n", strings.TrimSpace(warn))
		}
		fmt.Fprintln(w)
	}
}
