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
	// Versions are reported, not blocked -- a version bump is the point.
	Versions []Change `json:"versions"`
	// Other covers a source moving between chart and path, or a project or
	// namespace change: not a targeting change, but not routine either.
	Other    []Change `json:"other"`
	Warnings []string `json:"warnings,omitempty"`
}

func (d *DiffResult) Blocking() bool { return len(d.Targeting) > 0 || len(d.Other) > 0 }

func Diff(base, head *Table) *DiffResult {
	res := &DiffResult{}

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
			res.Targeting = append(res.Targeting, Change{
				Kind: "added", AppSet: appset, App: a.App, Cluster: a.Cluster,
				To: a.Describe(), Detail: "newly generated for this cluster",
			})
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

	sortChanges(res.Targeting)
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

// Report writes a markdown summary suitable for a pull-request comment or a CI
// job summary.
func (d *DiffResult) Report(w io.Writer) {
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
	if len(d.Versions) > 0 {
		fmt.Fprintf(w, "### Versions\n\n| Application | Cluster | From | To |\n|---|---|---|---|\n")
		for _, c := range d.Versions {
			fmt.Fprintf(w, "| `%s` | %s | `%s` | `%s` |\n", c.App, c.Cluster, c.From, c.To)
		}
		fmt.Fprintln(w)
	}
	if len(d.Targeting) == 0 && len(d.Other) == 0 && len(d.Versions) == 0 {
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
