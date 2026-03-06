package deploy

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

// These tests complement the writer-related tests in translator_test.go.
// They target edge cases and additional behaviors not covered there.

func TestWriteResult_CreatesDeepNestedDirectories(t *testing.T) {
	repo := t.TempDir()
	result := &TranslateResult{
		WorkloadName:  "deep-app",
		TargetCluster: "prod",
		Namespace:     "prod",
		AddonsEntry:   map[string]interface{}{"enabled": true},
		Files: map[string][]byte{
			"workloads/prod/addons/deep-app/sub/nested/dir/file.yaml": []byte("nested: true\n"),
		},
	}

	_, err := WriteResult(result, repo)
	if err != nil {
		t.Fatalf("WriteResult: %v", err)
	}

	filePath := filepath.Join(repo, "workloads/prod/addons/deep-app/sub/nested/dir/file.yaml")
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("reading nested file: %v", err)
	}
	if string(data) != "nested: true\n" {
		t.Errorf("content = %q, want %q", string(data), "nested: true\n")
	}
}

func TestWriteResult_NoFilesStillWritesAddons(t *testing.T) {
	repo := t.TempDir()
	result := &TranslateResult{
		WorkloadName:  "empty-app",
		TargetCluster: "dev",
		Namespace:     "dev",
		AddonsEntry:   map[string]interface{}{"enabled": true},
		Files:         map[string][]byte{},
	}

	paths, err := WriteResult(result, repo)
	if err != nil {
		t.Fatalf("WriteResult: %v", err)
	}

	// Should still write addons.yaml
	found := false
	for _, p := range paths {
		if p == filepath.Join("workloads", "dev", "addons.yaml") {
			found = true
		}
	}
	if !found {
		t.Error("expected addons.yaml path in written paths")
	}

	addonsPath := filepath.Join(repo, "workloads", "dev", "addons.yaml")
	if _, err := os.Stat(addonsPath); os.IsNotExist(err) {
		t.Error("addons.yaml file should exist on disk")
	}
}

func TestWriteResult_VerifiesFileContent(t *testing.T) {
	repo := t.TempDir()
	yamlContent := "applicationName: verified\nnamespace: test\n"
	result := &TranslateResult{
		WorkloadName:  "verified",
		TargetCluster: "test",
		Namespace:     "test",
		AddonsEntry:   map[string]interface{}{"enabled": true},
		Files: map[string][]byte{
			"workloads/test/addons/verified/values.yaml": []byte(yamlContent),
		},
	}

	_, err := WriteResult(result, repo)
	if err != nil {
		t.Fatalf("WriteResult: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(repo, "workloads/test/addons/verified/values.yaml"))
	if err != nil {
		t.Fatalf("reading file: %v", err)
	}
	if string(data) != yamlContent {
		t.Errorf("content mismatch: got %q, want %q", string(data), yamlContent)
	}
}

func TestUpdateAddonsYAML_OverwritesExistingEntry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "addons.yaml")

	existing := map[string]interface{}{
		"sonarr": map[string]interface{}{"enabled": true, "namespace": "old-ns"},
	}
	data, _ := yaml.Marshal(existing)
	os.WriteFile(path, data, 0o644)

	entry := map[string]interface{}{"enabled": true, "namespace": "new-ns"}
	err := updateAddonsYAML(path, "sonarr", entry, "dev")
	if err != nil {
		t.Fatalf("updateAddonsYAML: %v", err)
	}

	updated, _ := os.ReadFile(path)
	var result map[string]interface{}
	yaml.Unmarshal(updated, &result)

	sonarr := result["sonarr"].(map[string]interface{})
	if sonarr["namespace"] != "new-ns" {
		t.Errorf("namespace = %v, want 'new-ns'", sonarr["namespace"])
	}
}

func TestUpdateAddonsYAML_CreatesParentDirectories(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "deep", "nested", "addons.yaml")

	entry := map[string]interface{}{"enabled": true}
	err := updateAddonsYAML(path, "app", entry, "cluster")
	if err != nil {
		t.Fatalf("updateAddonsYAML: %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("addons.yaml should be created with parent dirs")
	}
}

func TestRemoveWorkload_BothPathsReturned(t *testing.T) {
	repo := t.TempDir()
	cluster := "media"

	addonsDir := filepath.Join(repo, "workloads", cluster)
	os.MkdirAll(addonsDir, 0o755)
	addons := map[string]interface{}{
		"globalSelectors": map[string]interface{}{"cluster_name": cluster},
		"sonarr":          map[string]interface{}{"enabled": true},
		"radarr":          map[string]interface{}{"enabled": true},
	}
	data, _ := yaml.Marshal(addons)
	os.WriteFile(filepath.Join(addonsDir, "addons.yaml"), data, 0o644)

	valuesDir := filepath.Join(repo, "workloads", cluster, "addons", "sonarr")
	os.MkdirAll(valuesDir, 0o755)
	os.WriteFile(filepath.Join(valuesDir, "values.yaml"), []byte("key: val"), 0o644)

	removed, err := RemoveWorkload(repo, cluster, "sonarr")
	if err != nil {
		t.Fatalf("RemoveWorkload: %v", err)
	}

	// Should have 2 removed paths: addons.yaml update + values dir removal
	if len(removed) != 2 {
		t.Fatalf("expected 2 removed paths, got %d: %v", len(removed), removed)
	}

	// Verify radarr still exists
	updated, _ := os.ReadFile(filepath.Join(addonsDir, "addons.yaml"))
	var result map[string]interface{}
	yaml.Unmarshal(updated, &result)
	if _, ok := result["radarr"]; !ok {
		t.Error("radarr should still exist")
	}
	if _, ok := result["sonarr"]; ok {
		t.Error("sonarr should be removed")
	}
}

func TestRemoveWorkload_MissingAddonsFile(t *testing.T) {
	repo := t.TempDir()
	_, err := RemoveWorkload(repo, "missing-cluster", "app")
	if err == nil {
		t.Error("expected error when addons.yaml doesn't exist")
	}
}

func TestListWorkloads_ExcludesMetaKeys(t *testing.T) {
	repo := t.TempDir()
	cluster := "dev"

	addonsDir := filepath.Join(repo, "workloads", cluster)
	os.MkdirAll(addonsDir, 0o755)
	addons := map[string]interface{}{
		"globalSelectors":       map[string]interface{}{"cluster_name": cluster},
		"useAddonNameForValues": true,
		"appsetPrefix":          "dev",
		"myapp":                 map[string]interface{}{"enabled": true},
	}
	data, _ := yaml.Marshal(addons)
	os.WriteFile(filepath.Join(addonsDir, "addons.yaml"), data, 0o644)

	workloads, err := ListWorkloads(repo, cluster)
	if err != nil {
		t.Fatalf("ListWorkloads: %v", err)
	}

	if len(workloads) != 1 || workloads[0] != "myapp" {
		t.Errorf("expected [myapp], got %v", workloads)
	}
}

func TestListWorkloads_EmptyReturnsNilSlice(t *testing.T) {
	repo := t.TempDir()
	cluster := "empty"

	addonsDir := filepath.Join(repo, "workloads", cluster)
	os.MkdirAll(addonsDir, 0o755)
	addons := map[string]interface{}{
		"globalSelectors": map[string]interface{}{"cluster_name": cluster},
	}
	data, _ := yaml.Marshal(addons)
	os.WriteFile(filepath.Join(addonsDir, "addons.yaml"), data, 0o644)

	workloads, err := ListWorkloads(repo, cluster)
	if err != nil {
		t.Fatalf("ListWorkloads: %v", err)
	}
	if len(workloads) != 0 {
		t.Errorf("expected 0 workloads, got %d: %v", len(workloads), workloads)
	}
}

func TestListWorkloads_ReturnsSorted(t *testing.T) {
	repo := t.TempDir()
	cluster := "sorted"

	addonsDir := filepath.Join(repo, "workloads", cluster)
	os.MkdirAll(addonsDir, 0o755)
	addons := map[string]interface{}{
		"zebra": map[string]interface{}{"enabled": true},
		"alpha": map[string]interface{}{"enabled": true},
		"mongo": map[string]interface{}{"enabled": true},
	}
	data, _ := yaml.Marshal(addons)
	os.WriteFile(filepath.Join(addonsDir, "addons.yaml"), data, 0o644)

	workloads, err := ListWorkloads(repo, cluster)
	if err != nil {
		t.Fatalf("ListWorkloads: %v", err)
	}

	if len(workloads) != 3 {
		t.Fatalf("expected 3, got %d", len(workloads))
	}
	if workloads[0] != "alpha" || workloads[1] != "mongo" || workloads[2] != "zebra" {
		t.Errorf("workloads = %v, want [alpha mongo zebra]", workloads)
	}
}
