package main

import (
	"fmt"
	"os"
	"path/filepath"

	"sigs.k8s.io/yaml"
)

// Config is .gitops-gate.yaml -- everything the gate needs to know about a
// repository it has never seen. The binary itself knows nothing about any
// particular layout; this file is the whole of that knowledge.
type Config struct {
	// Clusters is the path to the cluster inventory, relative to the repo root.
	Clusters string `json:"clusters"`

	// Bootstraps are the ApplicationSets that generate the Applications that
	// render the ApplicationSets that generate everything else. Two levels is
	// the common "app of apps of addons" shape.
	Bootstraps []Bootstrap `json:"bootstraps"`

	// ValuesRef is the `ref:` name the bootstrap uses for the values source,
	// so `$values/...` paths in valueFiles can be mapped back to repo paths.
	ValuesRef string `json:"valuesRef"`

	// Validate controls schema validation.
	Validate ValidateConfig `json:"validate"`

	// ClustersExport tunes `clusters export`.
	ClustersExport ClustersExportConfig `json:"clustersExport"`
}

type ClustersExportConfig struct {
	// IgnoreKeys are labels and annotations to drop from the exported
	// inventory because they churn without affecting any selector or template
	// -- a resync stamp, a content hash. A trailing `*` matches by prefix.
	//
	// Without this, every export reports drift, and a check that always fails
	// gets switched off.
	IgnoreKeys []string `json:"ignoreKeys"`
}

type Bootstrap struct {
	// Path to the bootstrap ApplicationSet manifest, relative to the repo root.
	Path string `json:"path"`
	// Name is a short label used in output. Defaults to the file's base name.
	Name string `json:"name"`
}

type ValidateConfig struct {
	// Enabled turns kubeconform on. Requires the binary on PATH.
	Enabled bool `json:"enabled"`
	// SchemaLocations are passed to kubeconform as -schema-location.
	SchemaLocations []string `json:"schemaLocations"`
	// IgnoreMissingSchemas is almost always required: CRDs from smaller
	// projects are not in any published schema catalogue, and without this a
	// single unknown kind fails the whole run.
	IgnoreMissingSchemas bool `json:"ignoreMissingSchemas"`
	// SkipKinds are kinds to skip entirely.
	SkipKinds []string `json:"skipKinds"`
}

func LoadConfig(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	var c Config
	if err := yaml.UnmarshalStrict(raw, &c); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if len(c.Bootstraps) == 0 {
		return nil, fmt.Errorf("%s: at least one entry under `bootstraps` is required", path)
	}
	if c.Clusters == "" {
		return nil, fmt.Errorf("%s: `clusters` is required -- run `gitops-gate clusters export` to create one", path)
	}
	if c.ValuesRef == "" {
		c.ValuesRef = "values"
	}
	for i := range c.Bootstraps {
		if c.Bootstraps[i].Path == "" {
			return nil, fmt.Errorf("%s: bootstraps[%d].path is required", path, i)
		}
		if c.Bootstraps[i].Name == "" {
			base := filepath.Base(c.Bootstraps[i].Path)
			c.Bootstraps[i].Name = base[:len(base)-len(filepath.Ext(base))]
		}
	}
	return &c, nil
}
