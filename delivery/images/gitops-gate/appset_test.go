package main

import (
	"strings"
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

// The inventory cannot detect that it is out of date -- only comparing against
// the live cluster can. What it CAN do is refuse to answer when a selector
// asks about a label it has never seen, because the alternative is a silently
// diminished render that then reports "no change".
func TestValidateRefusesUnknownSelectorLabels(t *testing.T) {
	inv := testInventory()
	err := inv.Validate([]string{"cluster_role", "aws_cluster_name"}, nil)
	if err == nil {
		t.Fatal("a selector on a label no cluster carries must be refused")
	}
	if !strings.Contains(err.Error(), "aws_cluster_name") {
		t.Errorf("the error must name the offending label: %v", err)
	}
	if strings.Contains(err.Error(), "cluster_role") {
		t.Errorf("a label that IS present must not be reported: %v", err)
	}
}

func TestValidateAcceptsDeliberatelyAbsentLabels(t *testing.T) {
	inv := testInventory()
	if err := inv.Validate([]string{"aws_cluster_name"}, []string{"aws_cluster_name"}); err != nil {
		t.Fatalf("an explicitly known-absent label must be allowed: %v", err)
	}
}

func TestValidatePassesWhenEverySelectorKeyIsKnown(t *testing.T) {
	inv := testInventory()
	if err := inv.Validate([]string{"cluster_role", "environment"}, nil); err != nil {
		t.Fatalf("want no error, got %v", err)
	}
}

// selectorKeys has to reach inside a merge generator, or a stale-inventory
// check would skip exactly the addons that use per-environment overrides.
func TestSelectorKeysReachesInsideMergeGenerators(t *testing.T) {
	gens := parseGens(t, `
- merge:
    mergeKeys: [server]
    generators:
      - clusters:
          selector:
            matchLabels:
              cluster_role: control-plane
      - clusters:
          selector:
            matchExpressions:
              - key: environment
                operator: In
                values: [production]
`)
	got := selectorKeys(gens)
	want := map[string]bool{"cluster_role": true, "environment": true}
	if len(got) != len(want) {
		t.Fatalf("want %v, got %v", want, got)
	}
	for _, k := range got {
		if !want[k] {
			t.Errorf("unexpected key %q", k)
		}
	}
}
