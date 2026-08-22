package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"sigs.k8s.io/yaml"
)

// Config is .gitops-gate.yaml -- everything the gate needs to know about a
// repository it has never seen. The binary itself knows nothing about any
// particular layout; this file is the whole of that knowledge.
type Config struct {
	// Clusters is the path to the cluster inventory, relative to the repo root.
	Clusters string `json:"clusters"`

	// Sources describe how to obtain the Applications and ApplicationSets this
	// repository defines. A repository usually has more than one shape at
	// once -- some things committed as plain YAML, some rendered from a chart
	// -- so this is a list of strategies rather than a single mode.
	Sources []Source `json:"sources"`

	// Bootstraps is the older single-strategy form, kept working. It is
	// exactly equivalent to one `type: argocd-bootstrap` source each.
	Bootstraps []Bootstrap `json:"bootstraps"`

	// ValuesRef is the `ref:` name a multi-source Application gives its values
	// source, so `$values/…` paths map back to repo paths.
	ValuesRef string `json:"valuesRef"`

	// Concurrency caps parallel renders. Fleets are the reason this exists: a
	// fifty-cluster inventory is fifty chart renders per revision, and serial
	// execution turns a ninety-second gate into a coffee break.
	Concurrency int `json:"concurrency"`

	// Validate controls schema validation.
	Validate ValidateConfig `json:"validate"`

	// ClustersExport tunes `clusters export`.
	ClustersExport ClustersExportConfig `json:"clustersExport"`
}

// SourceType is how a source yields Applications and ApplicationSets.
type SourceType string

const (
	// SourceManifests reads committed YAML. Applications and ApplicationSets
	// checked into a directory is the most common ArgoCD layout there is, and
	// the one the first version of this gate could not read at all.
	SourceManifests SourceType = "manifests"

	// SourceHelm renders a chart, optionally once per matching cluster when
	// its values depend on cluster metadata.
	SourceHelm SourceType = "helm"

	// SourceKustomize builds an overlay.
	SourceKustomize SourceType = "kustomize"

	// SourceRendered reads manifests that are already rendered in git -- the
	// rendered-manifests pattern, whether produced by ArgoCD's source
	// hydrator, by Kargo's helm-template/kustomize-build promotion steps, or
	// by any CI job that commits its output.
	//
	// These are DEPLOYED OBJECTS, not Applications. That makes an
	// object-level diff possible: which Deployments, CRDs and NetworkPolicies
	// actually appear, vanish or change. It is the strongest signal available,
	// because it is the real answer rather than a reconstruction of it.
	SourceRendered SourceType = "rendered"

	// SourceArgoCDBootstrap derives a helm source from an app-of-apps
	// ApplicationSet -- the gitops-bridge shape, where cluster metadata on the
	// ArgoCD cluster Secret drives which chart is rendered and with what.
	SourceArgoCDBootstrap SourceType = "argocd-bootstrap"
)

// Source is one way to obtain manifests.
type Source struct {
	Name string     `json:"name"`
	Type SourceType `json:"type"`

	// Paths are glob patterns, for `manifests`.
	Paths []string `json:"paths"`

	// Path is a directory, for `kustomize` and `argocd-bootstrap`.
	Path string `json:"path"`

	// Chart and ValueFiles are for `helm`. Both may contain
	// {{ .name }}, {{ .labels.x }} and {{ .annotations.y }} placeholders,
	// resolved per cluster -- which is what makes a per-environment values
	// layout expressible without listing every combination by hand.
	Chart      string   `json:"chart"`
	ValueFiles []string `json:"valueFiles"`

	// Selector limits which clusters this source is rendered for. Omitted, a
	// helm source renders once with no cluster context; a manifests or
	// kustomize source is cluster-independent anyway.
	Selector *SourceSelector `json:"selector"`

	// ArgoCD names the ArgoCD instance this source belongs to, for fleets
	// running more than one. Clusters carry the same field; a source only
	// sees clusters whose value matches.
	ArgoCD string `json:"argocd"`

	// Scope decides which clusters the ApplicationSets from a per-cluster
	// render expand against.
	//
	//   fleet (default) -- the whole inventory. Correct for hub-and-spoke,
	//     where one ArgoCD holds every cluster and an ApplicationSet rendered
	//     under one cluster's values can still generate Applications for
	//     others. This is the gitops-bridge shape.
	//   cluster -- only the cluster it was rendered for. Correct where each
	//     cluster runs its own ArgoCD and therefore only ever sees itself.
	//
	// Getting this wrong is quiet rather than loud, which is why it is
	// explicit: under `fleet` a chart rendered per cluster yields the same
	// ApplicationSet name several times with different contents, and whichever
	// arrives first wins.
	Scope string `json:"scope"`
}

// SourceSelector is a label selector over the cluster inventory.
type SourceSelector struct {
	MatchLabels map[string]string `json:"matchLabels"`
}

type Bootstrap struct {
	// Path to the bootstrap ApplicationSet manifest, relative to the repo root.
	Path string `json:"path"`
	// Name is a short label used in output. Defaults to the file's base name.
	Name string `json:"name"`
}

type ClustersExportConfig struct {
	// IgnoreKeys are labels and annotations to drop from the exported
	// inventory because they churn without affecting any selector or
	// template -- a resync stamp, a content hash. A trailing `*` matches by
	// prefix. Without this, every export reports drift and a check that
	// always fails gets switched off.
	IgnoreKeys []string `json:"ignoreKeys"`

	// KnownAbsentLabels are label keys a selector matches on that no cluster
	// is expected to carry. Without this the gate refuses to render, on the
	// grounds that it usually means a stale inventory.
	KnownAbsentLabels []string `json:"knownAbsentLabels"`
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
	if c.Clusters == "" {
		return nil, fmt.Errorf("%s: `clusters` is required -- run `gitops-gate clusters export` to create one", path)
	}
	if c.ValuesRef == "" {
		c.ValuesRef = "values"
	}
	if c.Concurrency <= 0 {
		c.Concurrency = 8
	}

	// `bootstraps` is the older form. Fold it into sources so the rest of the
	// program only has one shape to understand.
	for i, b := range c.Bootstraps {
		if b.Path == "" {
			return nil, fmt.Errorf("%s: bootstraps[%d].path is required", path, i)
		}
		name := b.Name
		if name == "" {
			name = defaultName(b.Path)
		}
		c.Sources = append(c.Sources, Source{
			Name: name, Type: SourceArgoCDBootstrap, Path: b.Path,
		})
	}
	c.Bootstraps = nil

	if len(c.Sources) == 0 {
		return nil, fmt.Errorf("%s: at least one entry under `sources` is required", path)
	}
	for i := range c.Sources {
		if err := c.Sources[i].normalise(path, i); err != nil {
			return nil, err
		}
	}
	return &c, nil
}

func (s *Source) normalise(cfgPath string, i int) error {
	if s.Name == "" {
		switch {
		case s.Path != "":
			s.Name = defaultName(s.Path)
		case s.Chart != "":
			s.Name = defaultName(s.Chart)
		default:
			s.Name = fmt.Sprintf("sources[%d]", i)
		}
	}
	switch s.Type {
	case SourceManifests, SourceRendered:
		if len(s.Paths) == 0 {
			return fmt.Errorf("%s: source %q is type %s and needs `paths`", cfgPath, s.Name, s.Type)
		}
	case SourceHelm:
		if s.Chart == "" {
			return fmt.Errorf("%s: source %q is type helm and needs `chart`", cfgPath, s.Name)
		}
	case SourceKustomize, SourceArgoCDBootstrap:
		if s.Path == "" {
			return fmt.Errorf("%s: source %q is type %s and needs `path`", cfgPath, s.Name, s.Type)
		}
	case "":
		return fmt.Errorf("%s: source %q has no `type` (manifests, helm, kustomize or argocd-bootstrap)", cfgPath, s.Name)
	default:
		return fmt.Errorf("%s: source %q has unknown type %q", cfgPath, s.Name, s.Type)
	}
	return nil
}

func defaultName(p string) string {
	base := filepath.Base(p)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// matches reports whether a cluster is in this source's scope.
func (s *Source) matches(c Cluster) bool {
	if s.ArgoCD != "" && c.ArgoCD != s.ArgoCD {
		return false
	}
	if s.Selector == nil {
		return true
	}
	for k, v := range s.Selector.MatchLabels {
		if c.Labels[k] != v {
			return false
		}
	}
	return true
}
