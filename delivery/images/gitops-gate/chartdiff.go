package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// ChartDiff renders every Application whose chart version changed, at both
// versions, and diffs the resources.
//
// This is the difference between "cert-manager moved from v1.21.1 to v1.22.0"
// and "that bump adds four CRDs, removes a container, and moves a Service
// port". Every incident worth gating on is of the second kind: a pull request
// that renders fine and breaks at runtime. The version alone cannot show it,
// and a reviewer -- or a triage agent -- reading only the version is reasoning
// from far less than they appear to have.
//
// It costs a chart pull and two renders per changed Application, so it runs
// only for rows whose version actually moved, which on a typical bump pull
// request is one.
func ChartDiff(repoRoot string, cfg *Config, base, head *Table) ([]Object, []Object, []string) {
	type pair struct{ before, after Row }

	baseByKey := map[string]Row{}
	for _, r := range base.Rows {
		baseByKey[r.Key()] = r
	}

	var pairs []pair
	for _, h := range head.Rows {
		b, ok := baseByKey[h.Key()]
		if !ok || h.SourceType != "helm" || b.SourceType != "helm" {
			continue
		}
		if b.Version == h.Version || b.Chart != h.Chart || b.ChartRepo != h.ChartRepo {
			continue
		}
		pairs = append(pairs, pair{before: b, after: h})
	}
	if len(pairs) == 0 {
		return nil, nil, nil
	}

	var (
		mu       sync.Mutex
		beforeOb []Object
		afterOb  []Object
		warnings []string
		wg       sync.WaitGroup
	)
	sem := make(chan struct{}, cfg.Concurrency)

	for _, p := range pairs {
		wg.Add(1)
		go func(p pair) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			b, errB := renderChartVersion(repoRoot, p.before)
			a, errA := renderChartVersion(repoRoot, p.after)

			mu.Lock()
			defer mu.Unlock()
			// A chart that cannot be pulled is reported, never silently
			// skipped: "no resource changes" and "we could not look" must not
			// read identically.
			if errB != nil || errA != nil {
				err := errB
				if err == nil {
					err = errA
				}
				warnings = append(warnings, fmt.Sprintf(
					"%s: could not render %s at both versions, so its resource changes are NOT covered: %v",
					p.after.App, p.after.Chart, err))
				return
			}
			beforeOb = append(beforeOb, b...)
			afterOb = append(afterOb, a...)
		}(p)
	}
	wg.Wait()
	return beforeOb, afterOb, warnings
}

// renderChartVersion renders one Application's chart at its pinned version,
// with the value files and inline values that Application actually uses.
func renderChartVersion(repoRoot string, r Row) ([]Object, error) {
	args := []string{"template", releaseNameFor(r), chartRef(r), "--version", r.Version}
	if !strings.HasPrefix(r.ChartRepo, "oci://") && r.ChartRepo != "" {
		args = append(args, "--repo", r.ChartRepo)
	}

	for _, vf := range r.ValueFiles {
		// `$values/x` refers to the multi-source values ref, which is this
		// repository. A file that does not exist for this Application is
		// normal -- ArgoCD's ignoreMissingValueFiles behaves the same way.
		clean := vf
		if i := strings.Index(clean, "/"); strings.HasPrefix(clean, "$") && i > 0 {
			clean = clean[i+1:]
		}
		full := filepath.Join(repoRoot, clean)
		if _, err := os.Stat(full); err == nil {
			args = append(args, "-f", full)
		}
	}

	if strings.TrimSpace(r.ValuesInline) != "" {
		f, err := os.CreateTemp("", "inline-*.yaml")
		if err != nil {
			return nil, err
		}
		defer os.Remove(f.Name())
		if _, err := f.WriteString(r.ValuesInline); err != nil {
			f.Close()
			return nil, err
		}
		f.Close()
		args = append(args, "-f", f.Name())
	}

	// The Application's destination namespace, or namespaced resources render
	// into helm's default and the report names a namespace nothing deploys to.
	if r.Namespace != "" {
		args = append(args, "--namespace", r.Namespace)
	}

	// CRDs are the point of several of these diffs -- a chart that starts or
	// stops shipping one is exactly what a version bump hides.
	args = append(args, "--include-crds")

	out, err := run(repoRoot, "helm", args...)
	if err != nil {
		return nil, err
	}
	objs, err := parseStream(out)
	if err != nil {
		return nil, err
	}

	var result []Object
	for _, o := range objs {
		if obj, ok := objectFrom(r.App, r.Cluster, r.Namespace, o); ok {
			result = append(result, obj)
		}
	}
	return result, nil
}

// chartRef builds what `helm template` needs: an OCI URL renders directly,
// a classic repo needs --repo rather than a pre-added repository, so nothing
// has to mutate the runner's helm config.
func chartRef(r Row) string {
	if strings.HasPrefix(r.ChartRepo, "oci://") {
		return strings.TrimRight(r.ChartRepo, "/") + "/" + r.Chart
	}
	return r.Chart
}

func releaseNameFor(r Row) string {
	if r.Chart != "" {
		return r.Chart
	}
	return "release"
}
