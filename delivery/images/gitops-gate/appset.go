package main

import (
	"fmt"
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// Param is one expanded generator result: the cluster it resolved to, plus the
// `values` block that generator attached. One Param becomes one Application.
type Param struct {
	Cluster Cluster
	Values  map[string]string
}

// generatorSpec is the subset of ApplicationSet generators this gate
// understands. Anything else is reported rather than silently ignored --
// a generator we cannot expand is a blind spot, and a blind spot that
// announces itself is survivable.
type generatorSpec struct {
	Clusters *clustersGenerator `json:"clusters,omitempty"`
	Merge    *mergeGenerator    `json:"merge,omitempty"`
	List     *listGenerator     `json:"list,omitempty"`
	Git      map[string]any     `json:"git,omitempty"`
	Matrix   map[string]any     `json:"matrix,omitempty"`
}

type clustersGenerator struct {
	Selector *metav1.LabelSelector `json:"selector,omitempty"`
	Values   map[string]string     `json:"values,omitempty"`
}

type mergeGenerator struct {
	MergeKeys  []string        `json:"mergeKeys"`
	Generators []generatorSpec `json:"generators"`
}

type listGenerator struct {
	Elements []map[string]string `json:"elements"`
}

// expandGenerators turns a generator list into the set of Applications it would
// produce, given the cluster inventory.
//
// This is the heart of the targeting check. A selector is evaluated against
// live cluster labels, which means an addon's scope can change without its own
// definition changing at all -- someone relabels a cluster, or adds one, and an
// addon silently starts or stops targeting it. Reading the addon definition
// tells you nothing; only expanding the generator does.
func expandGenerators(gens []generatorSpec, inv *Inventory) ([]Param, []string, error) {
	var out []Param
	var warnings []string

	for i, g := range gens {
		switch {
		case g.Clusters != nil:
			ps, err := expandClusters(g.Clusters, inv)
			if err != nil {
				return nil, nil, fmt.Errorf("generators[%d].clusters: %w", i, err)
			}
			out = append(out, ps...)

		case g.Merge != nil:
			ps, warns, err := expandMerge(g.Merge, inv)
			if err != nil {
				return nil, nil, fmt.Errorf("generators[%d].merge: %w", i, err)
			}
			warnings = append(warnings, warns...)
			out = append(out, ps...)

		case g.List != nil:
			warnings = append(warnings, fmt.Sprintf("generators[%d]: list generator is not expanded; its Applications are not covered by the targeting check", i))

		case g.Git != nil:
			warnings = append(warnings, fmt.Sprintf("generators[%d]: git generator is not expanded (it needs repository contents at both revisions); its Applications are not covered by the targeting check", i))

		case g.Matrix != nil:
			warnings = append(warnings, fmt.Sprintf("generators[%d]: matrix generator is not expanded; its Applications are not covered by the targeting check", i))

		default:
			warnings = append(warnings, fmt.Sprintf("generators[%d]: unrecognised generator; not covered by the targeting check", i))
		}
	}
	return out, warnings, nil
}

func expandClusters(g *clustersGenerator, inv *Inventory) ([]Param, error) {
	sel := labels.Everything()
	if g.Selector != nil {
		s, err := metav1.LabelSelectorAsSelector(g.Selector)
		if err != nil {
			return nil, fmt.Errorf("invalid selector: %w", err)
		}
		sel = s
	}

	var out []Param
	for _, c := range inv.Clusters {
		if !sel.Matches(labels.Set(c.Labels)) {
			continue
		}
		vals := map[string]string{}
		for k, v := range g.Values {
			vals[k] = v
		}
		out = append(out, Param{Cluster: c, Values: vals})
	}
	return out, nil
}

// expandMerge reproduces ApplicationSet's merge semantics: the first generator
// establishes the base set, and each subsequent generator overrides values for
// the params it matches, keyed by mergeKeys. Params produced only by a later
// generator are discarded -- merge is a left join, not a union.
func expandMerge(g *mergeGenerator, inv *Inventory) ([]Param, []string, error) {
	if len(g.Generators) == 0 {
		return nil, nil, nil
	}
	if len(g.MergeKeys) == 0 {
		return nil, nil, fmt.Errorf("mergeKeys is required")
	}

	base, warnings, err := expandGenerators(g.Generators[:1], inv)
	if err != nil {
		return nil, nil, err
	}

	index := map[string]int{}
	for i, p := range base {
		index[mergeKeyFor(p, g.MergeKeys)] = i
	}

	for _, sub := range g.Generators[1:] {
		overrides, warns, err := expandGenerators([]generatorSpec{sub}, inv)
		if err != nil {
			return nil, nil, err
		}
		warnings = append(warnings, warns...)
		for _, o := range overrides {
			i, ok := index[mergeKeyFor(o, g.MergeKeys)]
			if !ok {
				// No base param with this key: ApplicationSet drops it.
				continue
			}
			for k, v := range o.Values {
				base[i].Values[k] = v
			}
		}
	}
	return base, warnings, nil
}

func mergeKeyFor(p Param, keys []string) string {
	sorted := append([]string(nil), keys...)
	sort.Strings(sorted)
	out := ""
	for _, k := range sorted {
		switch k {
		case "server":
			out += "server=" + p.Cluster.Server + ";"
		case "name":
			out += "name=" + p.Cluster.Name + ";"
		default:
			if v, ok := p.Values[k]; ok {
				out += k + "=" + v + ";"
			} else if v, ok := p.Cluster.Labels[k]; ok {
				out += k + "=" + v + ";"
			} else {
				out += k + "=<unset>;"
			}
		}
	}
	return out
}

// selectorKeys returns every label key a generator list matches on, at any
// depth. Feeding these to Inventory.Validate is what converts a stale fixture
// from a wrong answer into a refusal.
func selectorKeys(gens []generatorSpec) []string {
	seen := map[string]bool{}
	var walk func([]generatorSpec)
	walk = func(gs []generatorSpec) {
		for _, g := range gs {
			if g.Clusters != nil && g.Clusters.Selector != nil {
				for k := range g.Clusters.Selector.MatchLabels {
					seen[k] = true
				}
				for _, e := range g.Clusters.Selector.MatchExpressions {
					seen[e.Key] = true
				}
			}
			if g.Merge != nil {
				walk(g.Merge.Generators)
			}
		}
	}
	walk(gens)

	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
