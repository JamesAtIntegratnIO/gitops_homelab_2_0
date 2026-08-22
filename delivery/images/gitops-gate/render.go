package main

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

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
	// Collect every source concurrently. On a fleet this is the difference
	// between a gate that runs inside a pull request and one nobody waits for:
	// fifty clusters is fifty chart renders, and they do not depend on each
	// other.
	type result struct {
		idx   int
		batch []docs
		err   error
	}
	sem := make(chan struct{}, cfg.Concurrency)
	results := make(chan result, len(cfg.Sources))
	var wg sync.WaitGroup

	for i := range cfg.Sources {
		wg.Add(1)
		go func(i int, src Source) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			b, err := collect(repoRoot, cfg, inv, src)
			results <- result{idx: i, batch: b, err: err}
		}(i, cfg.Sources[i])
	}
	wg.Wait()
	close(results)

	ordered := make([][]docs, len(cfg.Sources))
	for r := range results {
		if r.err != nil {
			return nil, r.err
		}
		ordered[r.idx] = r.batch
	}

	table := &Table{}
	seenSelectorKeys := map[string]bool{}

	for _, batch := range ordered {
		for _, d := range batch {
			if d.bootstrapRow != nil {
				table.Rows = append(table.Rows, *d.bootstrapRow)
			}
			for _, obj := range d.objects {
				kind, _ := obj["kind"].(string)
				switch kind {
				case "ApplicationSet":
					if gens, err := generatorsOf(obj); err == nil {
						for _, k := range selectorKeys(gens) {
							seenSelectorKeys[k] = true
						}
					}
					// Under `scope: cluster` the ApplicationSet is expanded
					// against only the cluster it was rendered for.
					scoped := inv
					if d.cluster != nil && d.scope == "cluster" {
						scoped = &Inventory{Clusters: []Cluster{*d.cluster}}
					}
					rows, warns, err := expandAppSet(obj, scoped)
					if err != nil {
						return nil, err
					}
					table.Warnings = append(table.Warnings, warns...)
					table.Rows = append(table.Rows, rows...)

				default:
					// Anything that is not an Application or ApplicationSet
					// is a resource the cluster will end up with. Recording
					// these is what makes an object-level diff possible.
					cluster := ""
					if d.cluster != nil {
						cluster = d.cluster.Name
					}
					if o, ok := objectFrom(d.source, cluster, "", obj); ok {
						table.Objects = append(table.Objects, o)
					}

				case "Application":
					// A plain Application needs no expansion: it targets one
					// destination and is already the thing ArgoCD will create.
					// Reading these is why a repository that commits
					// Applications directly -- an extremely common layout --
					// is covered at all.
					row, err := rowFromPlainApplication(d.source, obj, inv)
					if err != nil {
						table.Warnings = append(table.Warnings,
							fmt.Sprintf("%s: %v", d.source, err))
						continue
					}
					table.Rows = append(table.Rows, row)
				}
			}
		}
	}

	keys := make([]string, 0, len(seenSelectorKeys))
	for k := range seenSelectorKeys {
		keys = append(keys, k)
	}
	if err := inv.Validate(keys, cfg.ClustersExport.KnownAbsentLabels); err != nil {
		return nil, err
	}

	table.Rows = dedupeRows(table.Rows)
	table.Warnings = dedupeStrings(table.Warnings)
	table.Sort()
	return table, nil
}

// rowFromPlainApplication reads a committed Application. Its destination names
// a cluster by `name` or by `server`; both are resolved against the inventory
// so the row keys the same way an ApplicationSet-generated one does.
func rowFromPlainApplication(source string, obj map[string]any, inv *Inventory) (Row, error) {
	row, err := rowFromApp(source, "", obj)
	if err != nil {
		return Row{}, err
	}
	spec, _ := obj["spec"].(map[string]any)
	dest, _ := spec["destination"].(map[string]any)

	name, _ := dest["name"].(string)
	server, _ := dest["server"].(string)
	switch {
	case name != "":
		row.Cluster = name
	case server != "":
		row.Cluster = server
		for _, c := range inv.Clusters {
			if c.Server == server {
				row.Cluster = c.Name
				break
			}
		}
	default:
		return Row{}, fmt.Errorf("Application %q has no destination", row.App)
	}
	return row, nil
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

	// An Application template carries EITHER `sources` (multi-source, used
	// when values come from a second repo via `ref:`) or `source` (singular).
	// The canonical gitops-bridge bootstrap uses the singular form, so reading
	// only the plural made the gate unable to parse the most common bootstrap
	// there is.
	sources, _ := tspec["sources"].([]any)
	if len(sources) == 0 {
		if single, ok := tspec["source"].(map[string]any); ok {
			sources = []any{single}
		}
	}
	if len(sources) == 0 {
		return "", nil, fmt.Errorf("no .spec.template.spec.source or .sources")
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

// helmRender renders a chart and returns every object in the output.
func helmRender(repoRoot, chartPath string, valueFiles []string) ([]map[string]any, error) {
	stream, err := helmTemplateRaw(repoRoot, chartPath, valueFiles)
	if err != nil {
		return nil, err
	}
	return parseStream(stream)
}

// collectBootstrap handles the gitops-bridge shape: an app-of-apps
// ApplicationSet whose template points at a chart, with the path and the value
// files templated from metadata on each ArgoCD cluster Secret.
//
// It is one source type rather than the model, because plenty of repositories
// have no such layer -- but where it exists it is doing real work, and reading
// it is how the gate learns which values each cluster is rendered with without
// being told twice.
func collectBootstrap(repoRoot string, cfg *Config, inv *Inventory, s Source) ([]docs, error) {
	raw, err := os.ReadFile(filepath.Join(repoRoot, s.Path))
	if err != nil {
		return nil, fmt.Errorf("source %q: reading %s: %w", s.Name, s.Path, err)
	}
	bs, err := unmarshalMap(raw)
	if err != nil {
		return nil, fmt.Errorf("source %q: parsing %s: %w", s.Name, s.Path, err)
	}

	gens, err := generatorsOf(bs)
	if err != nil {
		return nil, fmt.Errorf("source %q: %w", s.Name, err)
	}
	params, _, err := expandGenerators(gens, inv)
	if err != nil {
		return nil, fmt.Errorf("source %q: %w", s.Name, err)
	}
	if len(params) == 0 {
		return nil, fmt.Errorf(
			"source %q matched no cluster in the inventory.\n\n"+
				"Nothing it generates can be checked, so the comparison would be made\n"+
				"against an empty set and report no change.\n\n"+
				"Almost always a stale inventory: re-run `gitops-gate clusters export`.\n"+
				"If it genuinely targets no cluster here, remove it from .gitops-gate.yaml\n"+
				"rather than leaving the gate blind to it", s.Name)
	}

	var out []docs
	for i := range params {
		p := params[i]
		chartPath, valueFiles, err := bootstrapSource(bs, p, cfg)
		if err != nil {
			return nil, fmt.Errorf("source %q (cluster %s): %w", s.Name, p.Cluster.Name, err)
		}
		// A bootstrap's source path is EITHER a chart OR a directory of
		// committed manifests that ArgoCD walks with `directory.recurse`.
		//
		// The canonical gitops-bridge bootstrap is the second kind -- it
		// points at a directory and applies every ApplicationSet YAML it
		// finds. Assuming a chart made the gate blind to that entire pattern,
		// which is the one most people using this actually run. Detect by
		// looking for Chart.yaml, exactly as ArgoCD decides.
		objs, err := renderBootstrapPath(repoRoot, chartPath, valueFiles)
		if err != nil {
			return nil, fmt.Errorf("source %q (cluster %s): %w", s.Name, p.Cluster.Name, err)
		}

		// The bootstrap Application itself is a row: a cluster appearing or
		// disappearing here means it started or stopped receiving addons
		// entirely, which is the largest targeting change there is.
		c := p.Cluster
		d := docs{source: s.Name, cluster: &c, objects: objs}
		if row, ok := bootstrapRow(bs, p, s.Name, chartPath); ok {
			d.bootstrapRow = &row
		}
		out = append(out, d)
	}
	return out, nil
}

// renderBootstrapPath resolves a bootstrap's source path the way ArgoCD does:
// a directory containing Chart.yaml is a chart, anything else is a directory of
// manifests to be read recursively.
func renderBootstrapPath(repoRoot, path string, valueFiles []string) ([]map[string]any, error) {
	full := filepath.Join(repoRoot, path)
	if _, err := os.Stat(filepath.Join(full, "Chart.yaml")); err == nil {
		return helmRender(repoRoot, path, valueFiles)
	}
	info, err := os.Stat(full)
	if err != nil {
		return nil, fmt.Errorf("source path %s: %w", path, err)
	}
	if !info.IsDir() {
		raw, err := os.ReadFile(full)
		if err != nil {
			return nil, err
		}
		return parseStream(raw)
	}
	return readDirRecursive(full)
}

// readDirRecursive reads every YAML manifest under a directory, matching
// ArgoCD's `directory.recurse: true`.
func readDirRecursive(dir string) ([]map[string]any, error) {
	var out []map[string]any
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		switch filepath.Ext(p) {
		case ".yaml", ".yml", ".json":
		default:
			return nil
		}
		raw, readErr := os.ReadFile(p)
		if readErr != nil {
			return nil
		}
		objs, parseErr := parseStream(raw)
		if parseErr != nil {
			return fmt.Errorf("%s: %w", p, parseErr)
		}
		out = append(out, objs...)
		return nil
	})
	return out, err
}
