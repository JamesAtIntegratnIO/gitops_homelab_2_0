package addon

import (
	"os"
	"path/filepath"
	"testing"
)

// --- Additional helper tests ---

func TestResolveLayerPaths_EnvironmentDifferentEnvs(t *testing.T) {
	tests := []struct {
		env      string
		wantPath string
	}{
		{"production", filepath.Join("/repo", "addons", "environments", "production", "addons", "addons.yaml")},
		{"staging", filepath.Join("/repo", "addons", "environments", "staging", "addons", "addons.yaml")},
		{"development", filepath.Join("/repo", "addons", "environments", "development", "addons", "addons.yaml")},
	}

	for _, tt := range tests {
		t.Run(tt.env, func(t *testing.T) {
			path, _, err := resolveLayerPaths("/repo", "environment", tt.env, "", "", "test-addon")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if path != tt.wantPath {
				t.Errorf("path = %q, want %q", path, tt.wantPath)
			}
		})
	}
}

func TestResolveLayerPaths_ValuesDirContainsAddonName(t *testing.T) {
	_, valuesDir, err := resolveLayerPaths("/repo", "environment", "prod", "", "", "cert-manager")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(valuesDir) != "cert-manager" {
		t.Errorf("values dir should end with addon name, got %q", filepath.Base(valuesDir))
	}
}

func TestWriteAddonsYAML_OverwritesExisting(t *testing.T) {
	tmp := t.TempDir()
	yamlPath := filepath.Join(tmp, "addons.yaml")

	// Write initial
	entries1 := map[string]map[string]interface{}{
		"addon-a": {"enabled": true},
	}
	if err := writeAddonsYAML(yamlPath, entries1); err != nil {
		t.Fatal(err)
	}

	// Overwrite
	entries2 := map[string]map[string]interface{}{
		"addon-b": {"enabled": false},
	}
	if err := writeAddonsYAML(yamlPath, entries2); err != nil {
		t.Fatal(err)
	}

	// Read back
	got, err := readAddonsYAML(yamlPath)
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := got["addon-a"]; ok {
		t.Error("addon-a should not exist after overwrite")
	}
	if _, ok := got["addon-b"]; !ok {
		t.Error("addon-b should exist after overwrite")
	}
}

func TestReadAddonsYAML_EmptyFile(t *testing.T) {
	tmp := t.TempDir()
	yamlPath := filepath.Join(tmp, "addons.yaml")
	if err := os.WriteFile(yamlPath, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	entries, err := readAddonsYAML(yamlPath)
	if err != nil {
		t.Fatalf("empty YAML should not error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for empty file, got %d", len(entries))
	}
}

func TestReadAddonsYAML_ScalarValues(t *testing.T) {
	tmp := t.TempDir()
	yamlPath := filepath.Join(tmp, "addons.yaml")
	// YAML with scalar value (string) not map — should be silently skipped
	content := `
addon-a:
  enabled: true
some-scalar: "just a string"
`
	if err := os.WriteFile(yamlPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	entries, err := readAddonsYAML(yamlPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := entries["addon-a"]; !ok {
		t.Error("addon-a should be present")
	}
	if _, ok := entries["some-scalar"]; ok {
		t.Error("scalar values should be skipped")
	}
}

func TestWriteAddonsYAML_PreservesComplexValues(t *testing.T) {
	tmp := t.TempDir()
	yamlPath := filepath.Join(tmp, "addons.yaml")

	entries := map[string]map[string]interface{}{
		"my-addon": {
			"enabled":         true,
			"namespace":       "my-ns",
			"chartRepository": "https://charts.example.com",
			"chartName":       "my-chart",
			"defaultVersion":  "1.2.3",
		},
	}

	if err := writeAddonsYAML(yamlPath, entries); err != nil {
		t.Fatal(err)
	}

	got, err := readAddonsYAML(yamlPath)
	if err != nil {
		t.Fatal(err)
	}

	addon := got["my-addon"]
	if addon["namespace"] != "my-ns" {
		t.Errorf("namespace = %v, want 'my-ns'", addon["namespace"])
	}
	if addon["chartName"] != "my-chart" {
		t.Errorf("chartName = %v, want 'my-chart'", addon["chartName"])
	}
}

// --- Command structure tests ---

func TestEnableCmd_Structure(t *testing.T) {
	cmd := NewCmd()
	for _, sub := range cmd.Commands() {
		if sub.Name() == "enable" {
			if sub.Short == "" {
				t.Error("enable: expected non-empty Short")
			}
			// Should require exactly 1 arg
			if err := sub.Args(sub, []string{}); err == nil {
				t.Error("enable should require exactly 1 arg")
			}
			if err := sub.Args(sub, []string{"addon-name"}); err != nil {
				t.Errorf("enable with 1 arg should pass: %v", err)
			}

			// Check flags exist
			expectedFlags := []string{"environment", "cluster", "cluster-role", "namespace", "chart-repo", "chart-name", "version", "layer"}
			for _, f := range expectedFlags {
				if sub.Flags().Lookup(f) == nil {
					t.Errorf("missing flag --%s on enable command", f)
				}
			}
			return
		}
	}
	t.Fatal("enable subcommand not found")
}

func TestDisableCmd_Structure(t *testing.T) {
	cmd := NewCmd()
	for _, sub := range cmd.Commands() {
		if sub.Name() == "disable" {
			if sub.Short == "" {
				t.Error("disable: expected non-empty Short")
			}
			// Should require exactly 1 arg
			if err := sub.Args(sub, []string{}); err == nil {
				t.Error("disable should require exactly 1 arg")
			}
			if err := sub.Args(sub, []string{"addon-name"}); err != nil {
				t.Errorf("disable with 1 arg should pass: %v", err)
			}

			// Check flags
			expectedFlags := []string{"environment", "cluster", "cluster-role", "layer", "remove"}
			for _, f := range expectedFlags {
				if sub.Flags().Lookup(f) == nil {
					t.Errorf("missing flag --%s on disable command", f)
				}
			}
			return
		}
	}
	t.Fatal("disable subcommand not found")
}

func TestStatusCmd_Structure(t *testing.T) {
	cmd := NewCmd()
	for _, sub := range cmd.Commands() {
		if sub.Name() == "status" {
			if sub.Short == "" {
				t.Error("status: expected non-empty Short")
			}
			// Should require exactly 1 arg
			if err := sub.Args(sub, []string{}); err == nil {
				t.Error("status should require exactly 1 arg")
			}
			if err := sub.Args(sub, []string{"addon-name"}); err != nil {
				t.Errorf("status with 1 arg should pass: %v", err)
			}
			return
		}
	}
	t.Fatal("status subcommand not found")
}

func TestListCmd_Structure(t *testing.T) {
	cmd := NewCmd()
	for _, sub := range cmd.Commands() {
		if sub.Name() == "list" {
			// Check alias
			found := false
			for _, a := range sub.Aliases {
				if a == "ls" {
					found = true
				}
			}
			if !found {
				t.Error("list: should have 'ls' alias")
			}

			// Check --environment flag
			envFlag := sub.Flags().Lookup("environment")
			if envFlag == nil {
				t.Error("list: missing --environment flag")
			}
			return
		}
	}
	t.Fatal("list subcommand not found")
}
