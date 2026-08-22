package main

import (
	"fmt"

	"sigs.k8s.io/yaml"
)

// expandAppSet turns one ApplicationSet into the Applications it generates.
func expandAppSet(as map[string]any, inv *Inventory) ([]Row, []string, error) {
	meta, _ := as["metadata"].(map[string]any)
	asName, _ := meta["name"].(string)

	spec, _ := as["spec"].(map[string]any)
	if spec == nil {
		return nil, nil, fmt.Errorf("ApplicationSet %s has no .spec", asName)
	}

	goTemplate, _ := spec["goTemplate"].(bool)

	gens, err := generatorsOf(as)
	if err != nil {
		return nil, nil, fmt.Errorf("ApplicationSet %s: %w", asName, err)
	}
	params, warnings, err := expandGenerators(gens, inv)
	if err != nil {
		return nil, nil, fmt.Errorf("ApplicationSet %s: %w", asName, err)
	}
	for i, w := range warnings {
		warnings[i] = asName + ": " + w
	}

	// An ApplicationSet that matches no cluster generates nothing. Usually
	// that is deliberate -- a disabled addon, or one aimed at a cluster type
	// this installation does not run.
	//
	// But it is also what a stale inventory looks like, and that case is
	// dangerous: labels drift, selectors stop matching, the render shrinks,
	// and the gate compares two diminished sets and reports no change. One
	// missing label took a real render from 62 Applications to 7. Nothing
	// inside the inventory file can detect that the file is out of date --
	// only `clusters export -check` against the live cluster can -- so this
	// warning is the in-band signal, and a page of them means look at the
	// inventory before believing the verdict.
	if len(params) == 0 {
		warnings = append(warnings, fmt.Sprintf(
			"%s: matched no cluster, so it generates nothing", asName))
	}

	tmpl, _ := spec["template"].(map[string]any)
	if tmpl == nil {
		return nil, warnings, fmt.Errorf("ApplicationSet %s has no .spec.template", asName)
	}

	var rows []Row
	for _, p := range params {
		data := p.Cluster.TemplateData(p.Values)

		var rendered any
		if goTemplate {
			rendered, err = renderGoTemplate(tmpl, data)
			if err != nil {
				// A template that will not render is a real finding, not a
				// reason to abandon the whole run -- report it and continue so
				// the reviewer sees every problem at once.
				warnings = append(warnings, fmt.Sprintf("%s (cluster %s): template did not render: %v", asName, p.Cluster.Name, err))
				continue
			}
		} else {
			raw, err := yaml.Marshal(tmpl)
			if err != nil {
				return nil, warnings, err
			}
			s, err := renderFastTemplate(string(raw), data)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("%s (cluster %s): template did not render: %v", asName, p.Cluster.Name, err))
				continue
			}
			if err := yaml.Unmarshal([]byte(s), &rendered); err != nil {
				return nil, warnings, err
			}
		}

		row, err := rowFromApp(asName, p.Cluster.Name, rendered)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("%s (cluster %s): %v", asName, p.Cluster.Name, err))
			continue
		}
		rows = append(rows, row)
	}

	return rows, warnings, nil
}

func rowFromApp(asName, clusterName string, app any) (Row, error) {
	m, _ := app.(map[string]any)
	if m == nil {
		return Row{}, fmt.Errorf("rendered Application is not a mapping")
	}
	meta, _ := m["metadata"].(map[string]any)
	spec, _ := m["spec"].(map[string]any)
	if meta == nil || spec == nil {
		return Row{}, fmt.Errorf("rendered Application has no metadata or spec")
	}

	row := Row{
		AppSet:  asName,
		Cluster: clusterName,
	}
	row.App, _ = meta["name"].(string)
	row.Project, _ = spec["project"].(string)

	if dest, ok := spec["destination"].(map[string]any); ok {
		row.Namespace, _ = dest["namespace"].(string)
	}

	// An Application has either `source` or `sources`. The interesting one is
	// whichever carries a chart or a path -- the first source in these charts
	// is a bare `ref: values` with neither.
	var sources []any
	if s, ok := spec["sources"].([]any); ok {
		sources = s
	} else if s, ok := spec["source"].(map[string]any); ok {
		sources = []any{s}
	}

	for _, s := range sources {
		src, _ := s.(map[string]any)
		if src == nil {
			continue
		}
		chart, _ := src["chart"].(string)
		path, _ := src["path"].(string)
		switch {
		case chart != "":
			row.SourceType = "helm"
			row.Chart = chart
			row.ChartRepo, _ = src["repoURL"].(string)
			row.Version, _ = src["targetRevision"].(string)
			row.ValueFiles, row.ValuesInline = helmValues(src)
			return row, nil
		case path != "":
			row.SourceType = "path"
			row.Path = path
			row.Version, _ = src["targetRevision"].(string)
			// Keep looking: a manifest-source Application can carry both a
			// values ref and a path, and a later source may be the chart.
		}
	}

	if row.SourceType == "" {
		return Row{}, fmt.Errorf("no source with a chart or path")
	}
	return row, nil
}

// helmValues extracts what a source renders its chart with: the value files it
// layers, and any inline valuesObject serialised back to YAML.
//
// The inline block matters more than it looks. It is where a repository pins
// the chart defaults it depends on -- `global.networkPolicy.create: false`,
// `speaker.frr.enabled: false` -- so rendering without it reproduces upstream
// defaults rather than this cluster's configuration, and the diff describes a
// cluster nobody has.
func helmValues(src map[string]any) ([]string, string) {
	helm, _ := src["helm"].(map[string]any)
	if helm == nil {
		return nil, ""
	}

	var files []string
	if raw, ok := helm["valueFiles"].([]any); ok {
		for _, f := range raw {
			if s, ok := f.(string); ok {
				files = append(files, s)
			}
		}
	}

	inline := ""
	if vo, ok := helm["valuesObject"]; ok && vo != nil {
		if b, err := yaml.Marshal(vo); err == nil {
			inline = string(b)
		}
	} else if v, ok := helm["values"].(string); ok {
		inline = v
	}
	return files, inline
}
