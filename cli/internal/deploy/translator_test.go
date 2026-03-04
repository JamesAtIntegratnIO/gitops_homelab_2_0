package deploy

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jamesatintegratnio/hctl/internal/score"
	"gopkg.in/yaml.v3"
)

func TestResolveVariableValue_Literal(t *testing.T) {
	result := resolveVariableValue("hello-world", nil)
	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}
	if m["value"] != "hello-world" {
		t.Errorf("value = %v, want 'hello-world'", m["value"])
	}
}

func TestResolveVariableValue_DirectSecretRef(t *testing.T) {
	result := resolveVariableValue("$(my-secret:password)", nil)
	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}
	vf, ok := m["valueFrom"].(map[string]interface{})
	if !ok {
		t.Fatal("expected valueFrom map")
	}
	ref, ok := vf["secretKeyRef"].(map[string]interface{})
	if !ok {
		t.Fatal("expected secretKeyRef map")
	}
	if ref["name"] != "my-secret" {
		t.Errorf("secretKeyRef name = %v, want 'my-secret'", ref["name"])
	}
	if ref["key"] != "password" {
		t.Errorf("secretKeyRef key = %v, want 'password'", ref["key"])
	}
}

func TestResolveVariableValue_ScoreResourceRef_SecretOutput(t *testing.T) {
	outputs := map[string]map[string]string{
		"db": {
			"host":     "$(db-secret:host)",
			"password": "$(db-secret:password)",
		},
	}

	result := resolveVariableValue("${resources.db.host}", outputs)
	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}
	vf, ok := m["valueFrom"].(map[string]interface{})
	if !ok {
		t.Fatal("expected valueFrom")
	}
	ref := vf["secretKeyRef"].(map[string]interface{})
	if ref["name"] != "db-secret" {
		t.Errorf("name = %v, want 'db-secret'", ref["name"])
	}
	if ref["key"] != "host" {
		t.Errorf("key = %v, want 'host'", ref["key"])
	}
}

func TestResolveVariableValue_ScoreResourceRef_LiteralOutput(t *testing.T) {
	outputs := map[string]map[string]string{
		"cache": {
			"host": "redis.default.svc.cluster.local",
		},
	}

	result := resolveVariableValue("${resources.cache.host}", outputs)
	m := result.(map[string]interface{})
	if m["value"] != "redis.default.svc.cluster.local" {
		t.Errorf("value = %v, want literal host", m["value"])
	}
}

func TestResolveVariableValue_UnresolvedReference(t *testing.T) {
	result := resolveVariableValue("${resources.missing.key}", nil)
	m := result.(map[string]interface{})
	if m["value"] != "${resources.missing.key}" {
		t.Errorf("expected unresolved placeholder, got %v", m["value"])
	}
}

func TestBuildContainerSpec_Basic(t *testing.T) {
	c := score.Container{
		Image:   "nginx:1.25",
		Command: []string{"nginx", "-g", "daemon off;"},
		Args:    []string{"-c", "/etc/nginx.conf"},
	}

	spec := buildContainerSpec("web", c, nil)
	if spec["name"] != "web" {
		t.Errorf("name = %v, want 'web'", spec["name"])
	}
	if spec["image"] != "nginx:1.25" {
		t.Errorf("image = %v, want 'nginx:1.25'", spec["image"])
	}
	cmd, ok := spec["command"].([]string)
	if !ok || len(cmd) != 3 {
		t.Errorf("expected 3 command entries, got %v", spec["command"])
	}
	args, ok := spec["args"].([]string)
	if !ok || len(args) != 2 {
		t.Errorf("expected 2 args entries, got %v", spec["args"])
	}
}

func TestBuildContainerSpec_WithVariables(t *testing.T) {
	c := score.Container{
		Image: "app:latest",
		Variables: map[string]string{
			"DB_HOST": "$(db-secret:host)",
			"PORT":    "8080",
		},
	}

	spec := buildContainerSpec("sidecar", c, nil)
	envList, ok := spec["env"].([]map[string]interface{})
	if !ok {
		t.Fatalf("expected env list, got %T", spec["env"])
	}
	if len(envList) != 2 {
		t.Fatalf("expected 2 env entries, got %d", len(envList))
	}

	// Variables are sorted by name
	if envList[0]["name"] != "DB_HOST" {
		t.Errorf("first env name = %v, want 'DB_HOST'", envList[0]["name"])
	}
	if envList[1]["name"] != "PORT" {
		t.Errorf("second env name = %v, want 'PORT'", envList[1]["name"])
	}
}

func TestUpdateAddonsYAML_NewFile(t *testing.T) {
	tmp := t.TempDir()
	addonsPath := filepath.Join(tmp, "addons.yaml")

	entry := map[string]interface{}{
		"enabled":   true,
		"namespace": "myapp",
	}

	err := updateAddonsYAML(addonsPath, "myapp", entry, "test-cluster")
	if err != nil {
		t.Fatalf("updateAddonsYAML: %v", err)
	}

	// Read back and verify
	data, err := os.ReadFile(addonsPath)
	if err != nil {
		t.Fatalf("reading back: %v", err)
	}

	var result map[string]interface{}
	if err := yaml.Unmarshal(data, &result); err != nil {
		t.Fatalf("parsing: %v", err)
	}

	// Should have globalSelectors and the workload entry
	gs, ok := result["globalSelectors"].(map[string]interface{})
	if !ok {
		t.Fatal("expected globalSelectors")
	}
	if gs["cluster_name"] != "test-cluster" {
		t.Errorf("cluster_name = %v, want 'test-cluster'", gs["cluster_name"])
	}

	app, ok := result["myapp"].(map[string]interface{})
	if !ok {
		t.Fatal("expected myapp entry")
	}
	if app["enabled"] != true {
		t.Errorf("myapp enabled = %v, want true", app["enabled"])
	}
}

func TestUpdateAddonsYAML_ExistingFile(t *testing.T) {
	tmp := t.TempDir()
	addonsPath := filepath.Join(tmp, "addons.yaml")

	// Write initial file
	initial := map[string]interface{}{
		"globalSelectors":       map[string]interface{}{"cluster_name": "my-cluster"},
		"useAddonNameForValues": true,
		"existing-app": map[string]interface{}{
			"enabled":   true,
			"namespace": "existing",
		},
	}
	data, _ := yaml.Marshal(initial)
	if err := os.WriteFile(addonsPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	// Add new workload
	entry := map[string]interface{}{
		"enabled":   true,
		"namespace": "new-app",
	}
	if err := updateAddonsYAML(addonsPath, "new-app", entry, "my-cluster"); err != nil {
		t.Fatalf("updateAddonsYAML: %v", err)
	}

	// Read back
	data, _ = os.ReadFile(addonsPath)
	var result map[string]interface{}
	yaml.Unmarshal(data, &result)

	// Both should exist
	if _, ok := result["existing-app"]; !ok {
		t.Error("existing-app entry should still exist")
	}
	if _, ok := result["new-app"]; !ok {
		t.Error("new-app entry should exist")
	}
}

func TestRemoveWorkload(t *testing.T) {
	tmp := t.TempDir()

	// Set up addons.yaml
	cluster := "test-cluster"
	addonsDir := filepath.Join(tmp, "workloads", cluster)
	os.MkdirAll(addonsDir, 0o755)

	content := map[string]interface{}{
		"globalSelectors": map[string]interface{}{"cluster_name": cluster},
		"myapp": map[string]interface{}{
			"enabled":   true,
			"namespace": "myapp",
		},
		"other-app": map[string]interface{}{
			"enabled":   true,
			"namespace": "other",
		},
	}
	data, _ := yaml.Marshal(content)
	os.WriteFile(filepath.Join(addonsDir, "addons.yaml"), data, 0o644)

	// Create values directory
	valuesDir := filepath.Join(addonsDir, "addons", "myapp")
	os.MkdirAll(valuesDir, 0o755)
	os.WriteFile(filepath.Join(valuesDir, "values.yaml"), []byte("test: true"), 0o644)

	// Remove
	removed, err := RemoveWorkload(tmp, cluster, "myapp")
	if err != nil {
		t.Fatalf("RemoveWorkload: %v", err)
	}

	if len(removed) != 2 {
		t.Errorf("expected 2 removed paths, got %d", len(removed))
	}

	// Verify addons.yaml no longer has myapp
	data, _ = os.ReadFile(filepath.Join(addonsDir, "addons.yaml"))
	var result map[string]interface{}
	yaml.Unmarshal(data, &result)
	if _, ok := result["myapp"]; ok {
		t.Error("myapp should have been removed from addons.yaml")
	}
	if _, ok := result["other-app"]; !ok {
		t.Error("other-app should still exist")
	}

	// Verify values directory is gone
	if _, err := os.Stat(valuesDir); !os.IsNotExist(err) {
		t.Error("values directory should have been removed")
	}
}

func TestRemoveWorkload_NotFound(t *testing.T) {
	tmp := t.TempDir()
	cluster := "test-cluster"
	addonsDir := filepath.Join(tmp, "workloads", cluster)
	os.MkdirAll(addonsDir, 0o755)

	content := map[string]interface{}{
		"existing": map[string]interface{}{"enabled": true},
	}
	data, _ := yaml.Marshal(content)
	os.WriteFile(filepath.Join(addonsDir, "addons.yaml"), data, 0o644)

	_, err := RemoveWorkload(tmp, cluster, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent workload")
	}
}

func TestListWorkloads(t *testing.T) {
	tmp := t.TempDir()
	cluster := "test-cluster"
	addonsDir := filepath.Join(tmp, "workloads", cluster)
	os.MkdirAll(addonsDir, 0o755)

	content := map[string]interface{}{
		"globalSelectors":       map[string]interface{}{"cluster_name": cluster},
		"useAddonNameForValues": true,
		"app-a": map[string]interface{}{
			"enabled":   true,
			"namespace": "a",
		},
		"app-b": map[string]interface{}{
			"enabled":   false,
			"namespace": "b",
		},
		"app-c": map[string]interface{}{
			"enabled":   true,
			"namespace": "c",
		},
	}
	data, _ := yaml.Marshal(content)
	os.WriteFile(filepath.Join(addonsDir, "addons.yaml"), data, 0o644)

	workloads, err := ListWorkloads(tmp, cluster)
	if err != nil {
		t.Fatalf("ListWorkloads: %v", err)
	}

	// Should only include enabled workloads, sorted
	if len(workloads) != 2 {
		t.Fatalf("expected 2 workloads, got %d: %v", len(workloads), workloads)
	}
	if workloads[0] != "app-a" || workloads[1] != "app-c" {
		t.Errorf("workloads = %v, want [app-a, app-c]", workloads)
	}
}

func TestListWorkloads_NoFile(t *testing.T) {
	_, err := ListWorkloads("/nonexistent", "cluster")
	if err == nil {
		t.Error("expected error for missing addons.yaml")
	}
}

func TestSecretRefRegex(t *testing.T) {
	tests := []struct {
		input string
		match bool
		name  string
		key   string
	}{
		{"$(my-secret:password)", true, "my-secret", "password"},
		{"$(db-creds:host)", true, "db-creds", "host"},
		{"literal-value", false, "", ""},
		{"${resources.db.host}", false, "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			matches := secretRefRegex.FindStringSubmatch(tt.input)
			if tt.match {
				if len(matches) != 3 {
					t.Fatalf("expected match, got %v", matches)
				}
				if matches[1] != tt.name {
					t.Errorf("name = %q, want %q", matches[1], tt.name)
				}
				if matches[2] != tt.key {
					t.Errorf("key = %q, want %q", matches[2], tt.key)
				}
			} else {
				if len(matches) > 0 {
					t.Errorf("expected no match, got %v", matches)
				}
			}
		})
	}
}

func TestScoreVarRegex(t *testing.T) {
	tests := []struct {
		input   string
		match   bool
		resName string
		resKey  string
	}{
		{"${resources.db.host}", true, "db", "host"},
		{"${resources.cache.port}", true, "cache", "port"},
		{"literal", false, "", ""},
		{"$(secret:key)", false, "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			matches := scoreVarRegex.FindStringSubmatch(tt.input)
			if tt.match {
				if len(matches) != 3 {
					t.Fatalf("expected match, got %v", matches)
				}
				if matches[1] != tt.resName {
					t.Errorf("resName = %q, want %q", matches[1], tt.resName)
				}
				if matches[2] != tt.resKey {
					t.Errorf("resKey = %q, want %q", matches[2], tt.resKey)
				}
			} else {
				if len(matches) > 0 {
					t.Errorf("expected no match, got %v", matches)
				}
			}
		})
	}
}

func TestWriteResult(t *testing.T) {
	tmp := t.TempDir()

	result := &TranslateResult{
		WorkloadName:  "myapp",
		TargetCluster: "test-cluster",
		Namespace:     "myapp",
		StakaterValues: map[string]interface{}{
			"applicationName": "myapp",
		},
		AddonsEntry: map[string]interface{}{
			"enabled":   true,
			"namespace": "myapp",
		},
		Files: map[string][]byte{
			filepath.Join("workloads", "test-cluster", "addons", "myapp", "values.yaml"): []byte("applicationName: myapp\n"),
		},
	}

	paths, err := WriteResult(result, tmp)
	if err != nil {
		t.Fatalf("WriteResult: %v", err)
	}

	if len(paths) < 2 {
		t.Errorf("expected at least 2 written paths, got %d", len(paths))
	}

	// Verify values file was written
	valuesPath := filepath.Join(tmp, "workloads", "test-cluster", "addons", "myapp", "values.yaml")
	if _, err := os.Stat(valuesPath); os.IsNotExist(err) {
		t.Error("values.yaml should have been created")
	}

	// Verify addons.yaml was created
	addonsPath := filepath.Join(tmp, "workloads", "test-cluster", "addons.yaml")
	if _, err := os.Stat(addonsPath); os.IsNotExist(err) {
		t.Error("addons.yaml should have been created")
	}
}
