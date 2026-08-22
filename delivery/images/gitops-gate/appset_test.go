package main

import (
	"testing"

	"sigs.k8s.io/yaml"
)

func testInventory() *Inventory {
	return &Inventory{Clusters: []Cluster{
		{
			Name: "hub", Server: "https://hub",
			Labels: map[string]string{
				"argocd.argoproj.io/secret-type": "cluster",
				"cluster_role":                   "control-plane",
				"environment":                    "production",
			},
		},
		{
			Name: "tenant", Server: "https://tenant",
			Labels: map[string]string{
				"argocd.argoproj.io/secret-type": "cluster",
				"cluster_role":                   "vcluster",
				"environment":                    "production",
			},
		},
	}}
}

func parseGens(t *testing.T, src string) []generatorSpec {
	t.Helper()
	var gens []generatorSpec
	if err := yaml.Unmarshal([]byte(src), &gens); err != nil {
		t.Fatalf("parsing generators: %v", err)
	}
	return gens
}

func names(ps []Param) []string {
	out := make([]string, 0, len(ps))
	for _, p := range ps {
		out = append(out, p.Cluster.Name)
	}
	return out
}

func TestClustersGeneratorMatchLabels(t *testing.T) {
	gens := parseGens(t, `
- clusters:
    selector:
      matchLabels:
        cluster_role: control-plane
`)
	got, _, err := expandGenerators(gens, testInventory())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Cluster.Name != "hub" {
		t.Fatalf("want [hub], got %v", names(got))
	}
}

// The incident this whole gate exists for: an addon with no cluster_role
// exclusion targets every cluster, vclusters included. Reading the addon
// definition does not reveal it; expanding the selector does.
func TestClustersGeneratorWithoutExclusionHitsEveryCluster(t *testing.T) {
	gens := parseGens(t, `
- clusters:
    selector:
      matchLabels:
        argocd.argoproj.io/secret-type: cluster
`)
	got, _, err := expandGenerators(gens, testInventory())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want both clusters, got %v", names(got))
	}
}

func TestClustersGeneratorMatchExpressionsNotIn(t *testing.T) {
	gens := parseGens(t, `
- clusters:
    selector:
      matchLabels:
        argocd.argoproj.io/secret-type: cluster
      matchExpressions:
        - key: cluster_role
          operator: NotIn
          values: ['vcluster']
`)
	got, _, err := expandGenerators(gens, testInventory())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Cluster.Name != "hub" {
		t.Fatalf("want [hub], got %v", names(got))
	}
}

// merge is a left join: the first generator sets the population, later ones
// only override values for params already present. A cluster that only the
// override matches must not be added.
func TestMergeGeneratorIsALeftJoin(t *testing.T) {
	gens := parseGens(t, `
- merge:
    mergeKeys: [server]
    generators:
      - clusters:
          selector:
            matchLabels:
              cluster_role: control-plane
          values:
            addonChartVersion: "1.0.0"
      - clusters:
          selector:
            matchLabels:
              argocd.argoproj.io/secret-type: cluster
          values:
            addonChartVersion: "2.0.0"
`)
	got, _, err := expandGenerators(gens, testInventory())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("merge must not widen the population; got %v", names(got))
	}
	if got[0].Values["addonChartVersion"] != "2.0.0" {
		t.Fatalf("later generator should override the value, got %q", got[0].Values["addonChartVersion"])
	}
}

func TestUnexpandableGeneratorWarnsRatherThanSilentlyPassing(t *testing.T) {
	gens := parseGens(t, `
- git:
    repoURL: https://example.invalid/repo.git
`)
	got, warns, err := expandGenerators(gens, testInventory())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("git generator should produce nothing, got %v", names(got))
	}
	if len(warns) != 1 {
		t.Fatalf("an unexpandable generator must warn -- a silent blind spot reads as full coverage; got %v", warns)
	}
}
