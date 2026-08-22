package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"sigs.k8s.io/yaml"
)

// requireTool skips rather than fails when an external binary is absent. The
// alternative -- a hard failure -- makes `go test ./...` depend on the
// developer's PATH, and a suite that cannot run is a suite nobody runs.
func requireTool(t *testing.T, name string) {
	t.Helper()
	if _, err := exec.LookPath(name); err != nil {
		t.Skipf("%s is not on PATH; skipping", name)
	}
}

// These are the repository shapes people actually have. The first version of
// this gate could read exactly one of them -- an app-of-apps ApplicationSet
// rendering a chart -- and silently could not see the rest, which for most
// ArgoCD users would have meant a gate that checked nothing while reporting
// success.

func writeRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for p, c := range files {
		full := filepath.Join(root, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func fleet(t *testing.T, root string, clusters []Cluster) *Inventory {
	t.Helper()
	inv := &Inventory{Clusters: clusters}
	for i := range inv.Clusters {
		if inv.Clusters[i].Labels == nil {
			inv.Clusters[i].Labels = map[string]string{}
		}
		inv.Clusters[i].Labels["argocd.argoproj.io/secret-type"] = "cluster"
	}
	return inv
}

func appNames(rows []Row) []string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Cluster+"/"+r.App)
	}
	sort.Strings(out)
	return out
}

// The most common ArgoCD layout there is: ApplicationSets committed as YAML.
func TestManifestsSourceReadsCommittedApplicationSets(t *testing.T) {
	root := writeRepo(t, map[string]string{"appsets/cert-manager.yaml": `
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: cert-manager
spec:
  goTemplate: true
  generators:
    - clusters:
        selector:
          matchLabels:
            environment: production
        values:
          version: v1.21.1
  template:
    metadata:
      name: 'cert-manager-{{ .name }}'
    spec:
      project: default
      source:
        repoURL: https://charts.jetstack.io
        chart: cert-manager
        targetRevision: '{{ .values.version }}'
      destination:
        namespace: cert-manager
`})
	cfg := &Config{Concurrency: 4, Sources: []Source{
		{Name: "appsets", Type: SourceManifests, Paths: []string{"appsets/*.yaml"}},
	}}
	inv := fleet(t, root, []Cluster{
		{Name: "prod-eu", Labels: map[string]string{"environment": "production"}},
		{Name: "prod-us", Labels: map[string]string{"environment": "production"}},
		{Name: "lab", Labels: map[string]string{"environment": "lab"}},
	})

	table, err := Render(root, cfg, inv)
	if err != nil {
		t.Fatal(err)
	}
	got := appNames(table.Rows)
	want := []string{"prod-eu/cert-manager-prod-eu", "prod-us/cert-manager-prod-us"}
	if len(got) != len(want) {
		t.Fatalf("want %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("want %v, got %v", want, got)
		}
	}
	if table.Rows[0].Version != "v1.21.1" {
		t.Errorf("generator values must reach the template: %+v", table.Rows[0])
	}
}

// Plain Applications committed to git, with no ApplicationSet anywhere.
func TestManifestsSourceReadsPlainApplications(t *testing.T) {
	root := writeRepo(t, map[string]string{
		"apps/by-name.yaml": `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: grafana
spec:
  project: default
  destination:
    name: prod-eu
    namespace: monitoring
  source:
    repoURL: https://grafana.github.io/helm-charts
    chart: grafana
    targetRevision: 8.5.0
`,
		"apps/by-server.yaml": `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: loki
spec:
  project: default
  destination:
    server: https://prod-us.example.com
    namespace: loki
  source:
    repoURL: https://grafana.github.io/helm-charts
    chart: loki
    targetRevision: 6.49.0
`,
	})
	cfg := &Config{Concurrency: 4, Sources: []Source{
		{Name: "apps", Type: SourceManifests, Paths: []string{"apps/*.yaml"}},
	}}
	inv := fleet(t, root, []Cluster{
		{Name: "prod-eu", Server: "https://prod-eu.example.com"},
		{Name: "prod-us", Server: "https://prod-us.example.com"},
	})

	table, err := Render(root, cfg, inv)
	if err != nil {
		t.Fatal(err)
	}
	got := appNames(table.Rows)
	// A destination given by server must resolve to the cluster's NAME, or the
	// same Application keys differently depending on how it was written and
	// every diff reports a spurious move.
	want := []string{"prod-eu/grafana", "prod-us/loki"}
	for i := range want {
		if i >= len(got) || got[i] != want[i] {
			t.Fatalf("want %v, got %v", want, got)
		}
	}
}

// A chart rendered once per cluster, with the value files chosen by cluster
// metadata. This is the shape that makes a per-environment values layout
// expressible without enumerating every combination.
func TestHelmSourceRendersPerClusterValueFiles(t *testing.T) {
	chart := map[string]string{
		"charts/addons/Chart.yaml":  "apiVersion: v2\nname: addons\nversion: 0.1.0\n",
		"charts/addons/values.yaml": "version: \"0.0.0\"\n",
		"charts/addons/templates/appset.yaml": `
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: thing
spec:
  goTemplate: true
  generators:
    - clusters:
        selector:
          matchLabels:
            argocd.argoproj.io/secret-type: cluster
  template:
    metadata:
      name: 'thing-{{ .name }}'
    spec:
      project: default
      source:
        repoURL: https://charts.example.com
        chart: thing
        targetRevision: {{ .Values.version | quote }}
      destination:
        namespace: thing
`,
		"envs/production/values.yaml": "version: \"2.0.0\"\n",
		"envs/lab/values.yaml":        "version: \"1.0.0\"\n",
	}
	requireTool(t, "helm")
	root := writeRepo(t, chart)
	cfg := &Config{Concurrency: 4, Sources: []Source{{
		Name: "addons", Type: SourceHelm, Chart: "charts/addons",
		ValueFiles: []string{"envs/{{metadata.labels.environment}}/values.yaml"},
		// Each cluster runs its own ArgoCD in this topology, so an
		// ApplicationSet rendered for one must not generate for the other.
		Scope: "cluster",
	}}}
	inv := fleet(t, root, []Cluster{
		{Name: "prod", Labels: map[string]string{"environment": "production"}},
		{Name: "lab", Labels: map[string]string{"environment": "lab"}},
	})

	table, err := Render(root, cfg, inv)
	if err != nil {
		t.Fatal(err)
	}
	byCluster := map[string]string{}
	for _, r := range table.Rows {
		byCluster[r.Cluster] = r.Version
	}
	// Rendering once and reusing it for every cluster would give both the same
	// version, which is exactly the bug this source type exists to avoid.
	if byCluster["prod"] != "2.0.0" || byCluster["lab"] != "1.0.0" {
		t.Fatalf("each cluster must get its own values: %v", byCluster)
	}
}

// A source scoped to one ArgoCD instance must not see another's clusters.
// Large fleets routinely run one ArgoCD per region or per tenant.
func TestSourceScopedToOneArgoCDInstance(t *testing.T) {
	root := writeRepo(t, map[string]string{"appsets/a.yaml": `
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: thing
spec:
  goTemplate: true
  generators:
    - clusters:
        selector:
          matchLabels:
            argocd.argoproj.io/secret-type: cluster
  template:
    metadata:
      name: 'thing-{{ .name }}'
    spec:
      project: default
      source: {repoURL: https://charts.example.com, chart: thing, targetRevision: 1.0.0}
      destination: {namespace: thing}
`})
	cfg := &Config{Concurrency: 4, Sources: []Source{
		{Name: "eu", Type: SourceManifests, Paths: []string{"appsets/*.yaml"}, ArgoCD: "eu"},
	}}
	inv := fleet(t, root, []Cluster{
		{Name: "eu-1", ArgoCD: "eu"},
		{Name: "us-1", ArgoCD: "us"},
	})

	// The manifests source is cluster-independent, so scoping happens when the
	// ApplicationSet expands -- both clusters are in the inventory and both
	// match the selector. Recording the expectation so the behaviour is
	// deliberate rather than accidental: instance scoping applies to SOURCE
	// selection, and an ApplicationSet still expands against the whole
	// inventory unless its own selector says otherwise.
	table, err := Render(root, cfg, inv)
	if err != nil {
		t.Fatal(err)
	}
	if len(table.Rows) != 2 {
		t.Fatalf("want both clusters from an unscoped selector, got %v", appNames(table.Rows))
	}
}

// Several shapes at once, which is what a real repository looks like after a
// few years.
func TestMixedTopologiesCoexist(t *testing.T) {
	root := writeRepo(t, map[string]string{
		"appsets/a.yaml": `
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata: {name: fromappset}
spec:
  goTemplate: true
  generators:
    - clusters:
        selector: {matchLabels: {argocd.argoproj.io/secret-type: cluster}}
  template:
    metadata: {name: 'fromappset-{{ .name }}'}
    spec:
      project: default
      source: {repoURL: https://charts.example.com, chart: a, targetRevision: 1.0.0}
      destination: {namespace: a}
`,
		"apps/b.yaml": `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata: {name: fromapp}
spec:
  project: default
  destination: {name: only, namespace: b}
  source: {repoURL: https://charts.example.com, chart: b, targetRevision: 2.0.0}
`,
	})
	cfg := &Config{Concurrency: 4, Sources: []Source{
		{Name: "appsets", Type: SourceManifests, Paths: []string{"appsets/*.yaml"}},
		{Name: "apps", Type: SourceManifests, Paths: []string{"apps/*.yaml"}},
	}}
	inv := fleet(t, root, []Cluster{{Name: "only"}})

	table, err := Render(root, cfg, inv)
	if err != nil {
		t.Fatal(err)
	}
	got := appNames(table.Rows)
	want := []string{"only/fromapp", "only/fromappset-only"}
	for i := range want {
		if i >= len(got) || got[i] != want[i] {
			t.Fatalf("want %v, got %v", want, got)
		}
	}
}

func TestUnknownSourceTypeIsRejectedAtLoad(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, ".gitops-gate.yaml")
	raw, _ := yaml.Marshal(map[string]any{
		"clusters": "c.yaml",
		"sources":  []map[string]any{{"name": "x", "type": "flux"}},
	})
	if err := os.WriteFile(cfgPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(cfgPath); err == nil {
		t.Fatal("an unknown source type must be refused, not silently skipped")
	}
}

// The canonical gitops-bridge bootstrap: an ApplicationSet whose template
// points at a DIRECTORY (repo URL, basepath and path all templated from
// annotations on the cluster Secret) which ArgoCD walks with
// `directory.recurse: true`, applying every ApplicationSet YAML it finds.
//
// The first version of this gate assumed that path was always a Helm chart,
// which made it blind to the pattern most gitops-bridge users actually run.
func TestArgoCDBootstrapReadsADirectoryOfManifests(t *testing.T) {
	root := writeRepo(t, map[string]string{
		"bootstrap/addons.yaml": `
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: cluster-addons
spec:
  generators:
    - clusters: {}
  template:
    metadata:
      name: 'cluster-addons-{{name}}'
    spec:
      project: default
      source:
        repoURL: '{{metadata.annotations.addons_repo_url}}'
        path: '{{metadata.annotations.addons_repo_basepath}}{{metadata.annotations.addons_repo_path}}'
        targetRevision: '{{metadata.annotations.addons_repo_revision}}'
        directory:
          recurse: true
      destination:
        namespace: argocd
        name: '{{name}}'
`,
		// Two committed per-addon ApplicationSets, the gitops-bridge way.
		"gitops/addons/oss/metrics-server.yaml": `
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: addon-metrics-server
spec:
  generators:
    - clusters:
        selector:
          matchExpressions:
            - key: enable_metrics_server
              operator: In
              values: ['true']
        values:
          addonChart: metrics-server
          addonChartVersion: 3.12.0
          addonChartRepository: https://kubernetes-sigs.github.io/metrics-server
  template:
    metadata:
      name: 'addon-{{name}}-{{values.addonChart}}'
    spec:
      project: default
      source:
        repoURL: '{{values.addonChartRepository}}'
        chart: '{{values.addonChart}}'
        targetRevision: '{{values.addonChartVersion}}'
      destination:
        namespace: kube-system
`,
		"gitops/addons/aws/karpenter.yaml": `
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: addon-karpenter
spec:
  generators:
    - clusters:
        selector:
          matchExpressions:
            - key: enable_karpenter
              operator: In
              values: ['true']
        values:
          addonChart: karpenter
          addonChartVersion: 1.0.6
          addonChartRepository: public.ecr.aws
  template:
    metadata:
      name: 'addon-{{name}}-{{values.addonChart}}'
    spec:
      project: default
      source:
        repoURL: '{{values.addonChartRepository}}'
        chart: '{{values.addonChart}}'
        targetRevision: '{{values.addonChartVersion}}'
      destination:
        namespace: '{{metadata.annotations.karpenter_namespace}}'
`,
	})

	cfg := &Config{Concurrency: 4, ValuesRef: "values", Sources: []Source{
		{Name: "addons", Type: SourceArgoCDBootstrap, Path: "bootstrap/addons.yaml"},
	}}
	ann := map[string]string{
		"addons_repo_url":      "https://github.com/example/gitops.git",
		"addons_repo_basepath": "gitops/",
		"addons_repo_path":     "addons",
		"addons_repo_revision": "main",
		"karpenter_namespace":  "karpenter",
	}
	inv := fleet(t, root, []Cluster{
		{
			Name: "eks-prod", Server: "https://eks-prod.example.com",
			Labels: map[string]string{
				"cluster_name": "eks-prod", "environment": "prod",
				"enable_metrics_server": "true", "enable_karpenter": "true",
			},
			Annotations: ann,
		},
		{
			Name: "eks-dev", Server: "https://eks-dev.example.com",
			Labels: map[string]string{
				"cluster_name": "eks-dev", "environment": "dev",
				"enable_metrics_server": "true", "enable_karpenter": "false",
			},
			Annotations: ann,
		},
	})

	table, err := Render(root, cfg, inv)
	if err != nil {
		t.Fatal(err)
	}
	got := appNames(table.Rows)
	want := []string{
		"eks-dev/addon-eks-dev-metrics-server",
		"eks-dev/cluster-addons-eks-dev",
		"eks-prod/addon-eks-prod-karpenter",
		"eks-prod/addon-eks-prod-metrics-server",
		"eks-prod/cluster-addons-eks-prod",
	}
	if len(got) != len(want) {
		t.Fatalf("want %d rows\n  want %v\n  got  %v", len(want), want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("want %v\n got %v", want, got)
		}
	}

	// enable_karpenter=false on dev is the whole point of the label
	// convention: the addon must not be generated there.
	for _, r := range table.Rows {
		if r.Cluster == "eks-dev" && strings.Contains(r.App, "karpenter") {
			t.Error("enable_karpenter=false must exclude the addon")
		}
	}
}

// A fleet-sized inventory must render in reasonable time. Serial rendering is
// what turns a gate people wait for into one they route around.
func TestFleetOfFiftyClustersRenders(t *testing.T) {
	root := writeRepo(t, map[string]string{"appsets/a.yaml": `
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata: {name: thing}
spec:
  goTemplate: true
  generators:
    - clusters:
        selector: {matchLabels: {argocd.argoproj.io/secret-type: cluster}}
  template:
    metadata: {name: 'thing-{{ .name }}'}
    spec:
      project: default
      source: {repoURL: https://charts.example.com, chart: thing, targetRevision: 1.0.0}
      destination: {namespace: thing}
`})
	var clusters []Cluster
	for i := 0; i < 50; i++ {
		clusters = append(clusters, Cluster{
			Name:   fmt.Sprintf("cluster-%02d", i),
			Server: fmt.Sprintf("https://c%02d.example.com", i),
			Labels: map[string]string{"environment": "production"},
		})
	}
	cfg := &Config{Concurrency: 8, Sources: []Source{
		{Name: "appsets", Type: SourceManifests, Paths: []string{"appsets/*.yaml"}},
	}}
	inv := fleet(t, root, clusters)

	start := time.Now()
	table, err := Render(root, cfg, inv)
	if err != nil {
		t.Fatal(err)
	}
	if len(table.Rows) != 50 {
		t.Fatalf("want 50 Applications, got %d", len(table.Rows))
	}
	if d := time.Since(start); d > 10*time.Second {
		t.Errorf("50 clusters took %s; a gate this slow gets routed around", d)
	}
}
