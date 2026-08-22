package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"sigs.k8s.io/yaml"
)

// Render walks both levels of the ApplicationSet hierarchy and returns the flat
// set of Applications the cluster would end up with.
//
//	bootstrap ApplicationSet
//	  -> one Application per matching cluster
//	     -> renders the factory chart with that cluster's values layers
//	        -> N ApplicationSets
//	           -> one Application each per matching cluster   <- what we return
func Render(repoRoot string, cfg *Config, inv *Inventory) (*Table, error) {
	table := &Table{}

	for _, b := range cfg.Bootstraps {
		bsPath := filepath.Join(repoRoot, b.Path)
		raw, err := os.ReadFile(bsPath)
		if err != nil {
			return nil, fmt.Errorf("reading bootstrap %s: %w", b.Path, err)
		}

		var bs map[string]any
		if err := yaml.Unmarshal(raw, &bs); err != nil {
			return nil, fmt.Errorf("parsing bootstrap %s: %w", b.Path, err)
		}

		gens, err := generatorsOf(bs)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", b.Path, err)
		}
		params, warns, err := expandGenerators(gens, inv)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", b.Path, err)
		}
		for _, w := range warns {
			table.Warnings = append(table.Warnings, b.Name+": "+w)
		}

		for _, p := range params {
			chartPath, valueFiles, err := bootstrapSource(bs, p, cfg)
			if err != nil {
				return nil, fmt.Errorf("%s (cluster %s): %w", b.Path, p.Cluster.Name, err)
			}

			// The bootstrap Application itself is a row. A cluster appearing
			// or disappearing here means it started or stopped receiving
			// addons at all -- the largest targeting change there is, and one
			// that would otherwise show up only as ~30 unexplained additions.
			if row, ok := bootstrapRow(bs, p, b.Name, chartPath); ok {
				table.Rows = append(table.Rows, row)
			}

			appsets, err := helmTemplate(repoRoot, chartPath, valueFiles)
			if err != nil {
				return nil, fmt.Errorf("%s (cluster %s): %w", b.Path, p.Cluster.Name, err)
			}

			for _, as := range appsets {
				rows, warns, err := expandAppSet(as, inv)
				if err != nil {
					return nil, fmt.Errorf("%s (cluster %s): %w", b.Path, p.Cluster.Name, err)
				}
				for _, w := range warns {
					table.Warnings = append(table.Warnings, w)
				}
				table.Rows = append(table.Rows, rows...)
			}
		}
	}

	table.Rows = dedupeRows(table.Rows)
	table.Warnings = dedupeStrings(table.Warnings)
	table.Sort()
	return table, nil
}

func generatorsOf(obj map[string]any) ([]generatorSpec, error) {
	spec, _ := obj["spec"].(map[string]any)
	if spec == nil {
		return nil, fmt.Errorf("no .spec")
	}
	raw, err := yaml.Marshal(spec["generators"])
	if err != nil {
		return nil, err
	}
	var gens []generatorSpec
	if err := yaml.Unmarshal(raw, &gens); err != nil {
		return nil, fmt.Errorf("parsing generators: %w", err)
	}
	return gens, nil
}

// bootstrapSource resolves the chart path and value files for one cluster.
// Bootstrap ApplicationSets predate goTemplate mode, so placeholders here are
// the {{metadata.labels.environment}} dialect.
func bootstrapSource(bs map[string]any, p Param, cfg *Config) (string, []string, error) {
	spec, _ := bs["spec"].(map[string]any)
	tmpl, _ := spec["template"].(map[string]any)
	if tmpl == nil {
		return "", nil, fmt.Errorf("no .spec.template")
	}
	tspec, _ := tmpl["spec"].(map[string]any)
	sources, _ := tspec["sources"].([]any)
	if len(sources) == 0 {
		return "", nil, fmt.Errorf("no .spec.template.spec.sources")
	}

	data := p.Cluster.TemplateData(p.Values)

	var chartPath string
	var valueFiles []string

	for _, s := range sources {
		src, _ := s.(map[string]any)
		if src == nil {
			continue
		}
		pathTpl, _ := src["path"].(string)
		if pathTpl == "" {
			continue
		}
		resolved, err := renderFastTemplate(pathTpl, data)
		if err != nil {
			return "", nil, fmt.Errorf("resolving source path %q: %w", pathTpl, err)
		}
		chartPath = resolved

		helm, _ := src["helm"].(map[string]any)
		if helm == nil {
			continue
		}
		vfs, _ := helm["valueFiles"].([]any)
		for _, vf := range vfs {
			s, _ := vf.(string)
			if s == "" {
				continue
			}
			r, err := renderFastTemplate(s, data)
			if err != nil {
				return "", nil, fmt.Errorf("resolving valueFile %q: %w", s, err)
			}
			// `$values/x` refers to the ref'd source, which is this repo.
			r = strings.TrimPrefix(r, "$"+cfg.ValuesRef+"/")
			valueFiles = append(valueFiles, r)
		}
	}

	if chartPath == "" {
		return "", nil, fmt.Errorf("no source with a `path` -- the gate cannot render this bootstrap")
	}
	return chartPath, valueFiles, nil
}

// helmTemplateRaw renders the factory chart and returns the raw manifest
// stream. Missing value files are skipped, matching the
// ignoreMissingValueFiles the bootstraps set -- a values layer that does not
// exist for a given cluster is normal, not an error.
func helmTemplateRaw(repoRoot, chartPath string, valueFiles []string) ([]byte, error) {
	args := []string{"template", "gate", filepath.Join(repoRoot, chartPath)}
	for _, vf := range valueFiles {
		full := filepath.Join(repoRoot, vf)
		if _, err := os.Stat(full); err != nil {
			continue // ignoreMissingValueFiles: true
		}
		args = append(args, "-f", full)
	}

	cmd := exec.Command("helm", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("helm template %s: %w\n%s", chartPath, err, stderr.String())
	}
	return stdout.Bytes(), nil
}

func unmarshalMap(raw []byte) (map[string]any, error) {
	var m map[string]any
	if err := yaml.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// helmTemplate renders the factory chart and returns every ApplicationSet in
// the output.
func helmTemplate(repoRoot, chartPath string, valueFiles []string) ([]map[string]any, error) {
	stream, err := helmTemplateRaw(repoRoot, chartPath, valueFiles)
	if err != nil {
		return nil, err
	}

	var out []map[string]any
	for _, doc := range splitYAML(stream) {
		var obj map[string]any
		if err := yaml.Unmarshal(doc, &obj); err != nil {
			return nil, fmt.Errorf("parsing helm output: %w", err)
		}
		if obj == nil {
			continue
		}
		if kind, _ := obj["kind"].(string); kind == "ApplicationSet" {
			out = append(out, obj)
		}
	}
	return out, nil
}

func splitYAML(b []byte) [][]byte {
	parts := bytes.Split(b, []byte("\n---"))
	var out [][]byte
	for _, p := range parts {
		if len(bytes.TrimSpace(p)) == 0 {
			continue
		}
		out = append(out, p)
	}
	return out
}

func dedupeRows(rows []Row) []Row {
	seen := map[string]bool{}
	var out []Row
	for _, r := range rows {
		k := r.AppSet + "\x00" + r.Key()
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, r)
	}
	return out
}

func dedupeStrings(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// bootstrapRow describes the Application a bootstrap ApplicationSet generates
// for one cluster.
func bootstrapRow(bs map[string]any, p Param, appsetName, chartPath string) (Row, bool) {
	spec, _ := bs["spec"].(map[string]any)
	tmpl, _ := spec["template"].(map[string]any)
	if tmpl == nil {
		return Row{}, false
	}
	meta, _ := tmpl["metadata"].(map[string]any)
	nameTpl, _ := meta["name"].(string)
	if nameTpl == "" {
		return Row{}, false
	}
	name, err := renderFastTemplate(nameTpl, p.Cluster.TemplateData(p.Values))
	if err != nil {
		return Row{}, false
	}

	row := Row{
		AppSet:     appsetName,
		Cluster:    p.Cluster.Name,
		App:        name,
		SourceType: "path",
		Path:       chartPath,
	}
	tspec, _ := tmpl["spec"].(map[string]any)
	if tspec != nil {
		row.Project, _ = tspec["project"].(string)
		if dest, ok := tspec["destination"].(map[string]any); ok {
			row.Namespace, _ = dest["namespace"].(string)
		}
	}
	return row, true
}
