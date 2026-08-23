package edits

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sample = `# MetalLB, L2 only.
metallb:
  enabled: true
  defaultVersion: 0.16.0
  valuesObject:
    speaker:
      frr:
        enabled: true      # keep FRR off; this cluster is L2-only
    frrk8s:
      enabled: true
  containers:
    - image: "quay.io/metallb/controller:v0.15.2"
`

func repo(t *testing.T, files map[string]string) string {
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

func openPolicy() Policy { return Policy{Allow: []string{"addons/**"}} }

// The real case, in the exact shape the live model produced.
func TestAppliesScalarEditsInPlace(t *testing.T) {
	root := repo(t, map[string]string{"addons/values.yaml": sample})

	res, err := Apply(root, openPolicy(), []Edit{
		{Path: "addons/values.yaml", Key: "metallb.valuesObject.speaker.frr.enabled", From: "true", To: "false"},
		{Path: "addons/values.yaml", Key: "metallb.valuesObject.frrk8s.enabled", From: "true", To: "false"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Applied) != 2 || len(res.Rejected) != 0 {
		t.Fatalf("want 2 applied 0 rejected, got %d/%d: %+v", len(res.Applied), len(res.Rejected), res.Rejected)
	}

	got, _ := os.ReadFile(filepath.Join(root, "addons/values.yaml"))
	s := string(got)

	// The trailing comment must survive -- Kargo's own yaml-update deletes
	// them, and losing one silently removes the note explaining the value.
	if !strings.Contains(s, "enabled: false      # keep FRR off; this cluster is L2-only") {
		t.Errorf("indentation or trailing comment not preserved:\n%s", s)
	}
	// Only the two target lines may change.
	if !strings.Contains(s, "# MetalLB, L2 only.") || !strings.Contains(s, "defaultVersion: 0.16.0") {
		t.Errorf("unrelated content changed:\n%s", s)
	}
	if strings.Count(s, "enabled: true") != 1 { // metallb.enabled stays
		t.Errorf("wrong number of `enabled: true` left:\n%s", s)
	}
}

// The optimistic-concurrency check: a model working from a stale or imagined
// view of the file must change nothing rather than the wrong line.
func TestRejectsWhenFromDoesNotMatch(t *testing.T) {
	root := repo(t, map[string]string{"addons/values.yaml": sample})
	before, _ := os.ReadFile(filepath.Join(root, "addons/values.yaml"))

	res, err := Apply(root, openPolicy(), []Edit{
		{Path: "addons/values.yaml", Key: "metallb.defaultVersion", From: "0.15.2", To: "0.17.0"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Applied) != 0 || len(res.Rejected) != 1 {
		t.Fatalf("a mismatched `from` must be refused, got %+v", res)
	}
	if !strings.Contains(res.Rejected[0].Reason, "refusing to overwrite") {
		t.Errorf("reason should say why: %q", res.Rejected[0].Reason)
	}
	after, _ := os.ReadFile(filepath.Join(root, "addons/values.yaml"))
	if string(before) != string(after) {
		t.Error("file changed despite the edit being rejected")
	}
}

// The load-bearing test. Every one of these is a way to make a red gate green
// without fixing anything, and all are refused regardless of the allowlist.
func TestAlwaysDeniesTheGateAndThePolicy(t *testing.T) {
	forbidden := []string{
		".github/workflows/validate-addons.yaml",
		".gitops-gate.yaml",
		".gitops-gate/clusters.yaml",
		"delivery/images/bosun/prompt.go",
		"delivery/charts/kargo-pipelines/values.yaml",
		"addons/cluster-roles/control-plane/addons/kargo-projects/values.yaml",
		".gitlab-ci.yml",
		"bitbucket-pipelines.yml",
	}
	files := map[string]string{}
	for _, f := range forbidden {
		files[f] = "key: value\n"
	}
	root := repo(t, files)

	// Deliberately the most permissive allowlist possible.
	policy := Policy{Allow: []string{"**/*", "*"}}
	for _, f := range forbidden {
		res, err := Apply(root, policy, []Edit{{Path: f, Key: "key", From: "value", To: "pwned"}})
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Applied) != 0 {
			t.Errorf("%s was written even though it is always denied", f)
		}
		if len(res.Rejected) != 1 || !strings.Contains(res.Rejected[0].Reason, "denied") {
			t.Errorf("%s: want a denial, got %+v", f, res.Rejected)
		}
	}
}

func TestRejectsPathOutsideTheAllowlist(t *testing.T) {
	root := repo(t, map[string]string{"other/thing.yaml": "key: value\n"})
	res, _ := Apply(root, Policy{Allow: []string{"addons/**"}},
		[]Edit{{Path: "other/thing.yaml", Key: "key", From: "value", To: "x"}})
	if len(res.Applied) != 0 || len(res.Rejected) != 1 {
		t.Fatalf("want rejection, got %+v", res)
	}
	if !strings.Contains(res.Rejected[0].Reason, "not in the allowlist") {
		t.Errorf("unexpected reason: %q", res.Rejected[0].Reason)
	}
}

func TestRejectsTraversalOutOfTheRepository(t *testing.T) {
	root := repo(t, map[string]string{"addons/values.yaml": sample})
	res, _ := Apply(root, Policy{Allow: []string{"**/*"}},
		[]Edit{{Path: "../../../etc/passwd", Key: "key", From: "", To: "x"}})
	if len(res.Applied) != 0 {
		t.Fatal("traversal escaped the repository")
	}
}

func TestRejectsAKeyThatDoesNotExist(t *testing.T) {
	root := repo(t, map[string]string{"addons/values.yaml": sample})
	res, _ := Apply(root, openPolicy(), []Edit{
		{Path: "addons/values.yaml", Key: "metallb.valuesObject.nope.enabled", From: "true", To: "false"},
	})
	if len(res.Applied) != 0 || !strings.Contains(res.Rejected[0].Reason, "not found") {
		t.Fatalf("edits change values, they never add them: %+v", res)
	}
}

// The shape the model produced BEFORE the prompt spelled out the contract: a
// file path in `key`, a multi-line blob in `from`. It must fail closed.
func TestRejectsAMalformedEditShape(t *testing.T) {
	root := repo(t, map[string]string{"addons/values.yaml": sample})
	res, _ := Apply(root, openPolicy(), []Edit{{
		Path: "addons/values.yaml",
		Key:  "addons/values.yaml",
		From: "speaker:\n  frr:\n    enabled: true",
		To:   "speaker:\n  frr:\n    enabled: false",
	}})
	if len(res.Applied) != 0 {
		t.Fatal("a malformed edit was applied")
	}
}

func TestRejectsNonScalarTarget(t *testing.T) {
	root := repo(t, map[string]string{"addons/values.yaml": sample})
	res, _ := Apply(root, openPolicy(), []Edit{
		{Path: "addons/values.yaml", Key: "metallb.valuesObject", From: "", To: "x"},
	})
	if len(res.Applied) != 0 || !strings.Contains(res.Rejected[0].Reason, "not a scalar") {
		t.Fatalf("want a not-a-scalar rejection, got %+v", res)
	}
}

func TestAppliesInsideAList(t *testing.T) {
	root := repo(t, map[string]string{"addons/values.yaml": sample})
	res, err := Apply(root, openPolicy(), []Edit{{
		Path: "addons/values.yaml", Key: "metallb.containers.0.image",
		From: "quay.io/metallb/controller:v0.15.2",
		To:   "quay.io/metallb/controller:v0.16.0",
	}})
	if err != nil || len(res.Applied) != 1 {
		t.Fatalf("want 1 applied, got %+v %v", res, err)
	}
	got, _ := os.ReadFile(filepath.Join(root, "addons/values.yaml"))
	// The surrounding quotes are part of the line, not the value, so they stay.
	if !strings.Contains(string(got), `- image: "quay.io/metallb/controller:v0.16.0"`) {
		t.Errorf("quoting style not preserved:\n%s", got)
	}
}

// A model told "requires Gateway API v1.5" will confidently write v1.5.0 when
// the answer was v1.5.1 -- observed on a live model, with the prompt
// explicitly forbidding it. So the guarantee lives in code.
func TestRefusesAnInventedVersion(t *testing.T) {
	root := repo(t, map[string]string{"addons/values.yaml": "gateway-api-crds:\n  defaultVersion: v1.4.0\n"})
	policy := Policy{
		Allow:    []string{"addons/**"},
		Evidence: "nginx-gateway-fabric 2.6.7 requires Gateway API v1.5 or newer.",
	}

	res, err := Apply(root, policy, []Edit{
		{Path: "addons/values.yaml", Key: "gateway-api-crds.defaultVersion", From: "v1.4.0", To: "v1.5.0"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Applied) != 0 {
		t.Fatal("an uncorroborated version was written")
	}
	if !strings.Contains(res.Rejected[0].Reason, "invented version") {
		t.Errorf("reason should name the cause: %q", res.Rejected[0].Reason)
	}
}

func TestAllowsAVersionThatAppearsInTheEvidence(t *testing.T) {
	root := repo(t, map[string]string{"addons/values.yaml": "gateway-api-crds:\n  defaultVersion: v1.4.0\n"})
	policy := Policy{
		Allow:    []string{"addons/**"},
		Evidence: "The exact version to move to is v1.5.1.",
	}
	res, _ := Apply(root, policy, []Edit{
		{Path: "addons/values.yaml", Key: "gateway-api-crds.defaultVersion", From: "v1.4.0", To: "v1.5.1"},
	})
	if len(res.Applied) != 1 {
		t.Fatalf("a corroborated version must be allowed: %+v", res.Rejected)
	}
}

// Booleans and ports must not be caught by the corroboration check -- "false"
// rarely appears in a failure report, and rejecting toggles would break the
// most common mechanical fix there is.
func TestCorroborationIgnoresNonVersionValues(t *testing.T) {
	root := repo(t, map[string]string{"addons/values.yaml": sample})
	policy := Policy{Allow: []string{"addons/**"}, Evidence: "chart 0.16.0 flips the default"}

	res, _ := Apply(root, policy, []Edit{
		{Path: "addons/values.yaml", Key: "metallb.valuesObject.frrk8s.enabled", From: "true", To: "false"},
	})
	if len(res.Applied) != 1 {
		t.Fatalf("a boolean toggle must not need corroboration: %+v", res.Rejected)
	}
}
