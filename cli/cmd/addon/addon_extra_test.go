package addon

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jamesatintegratnio/hctl/internal/config"
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

// --- Functional tests for disable.go ---

func TestDisableCmd_RepoPathRequired(t *testing.T) {
	// newAddonDisableCmd().RunE should fail when RepoPath is empty in config
	cmd := newAddonDisableCmd()
	cmd.SetArgs([]string{"test-addon"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("disable should error when repo path is not set")
	}
}

func TestDisableCmd_AddonNotFound(t *testing.T) {
	tmp := t.TempDir()
	// Create addons.yaml with some addons but not "missing-addon"
	addonsDir := filepath.Join(tmp, "addons", "environments", "production", "addons")
	if err := os.MkdirAll(addonsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	entries := map[string]map[string]interface{}{
		"existing-addon": {"enabled": true},
	}
	if err := writeAddonsYAML(filepath.Join(addonsDir, "addons.yaml"), entries); err != nil {
		t.Fatal(err)
	}

	// Set config with the temp repo path and non-interactive
	cfg := &config.Config{
		RepoPath:    tmp,
		Interactive: false,
	}
	config.Set(cfg)
	defer config.Set(config.Default())

	cmd := newAddonDisableCmd()
	cmd.SetArgs([]string{"missing-addon"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("disable should error when addon is not found in addons.yaml")
	}
}

func TestDisableCmd_DisablesAddon(t *testing.T) {
	tmp := t.TempDir()
	addonsDir := filepath.Join(tmp, "addons", "environments", "production", "addons")
	if err := os.MkdirAll(addonsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	entries := map[string]map[string]interface{}{
		"my-addon": {"enabled": true, "namespace": "my-ns"},
	}
	if err := writeAddonsYAML(filepath.Join(addonsDir, "addons.yaml"), entries); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		RepoPath:    tmp,
		Interactive: false,
	}
	config.Set(cfg)
	defer config.Set(config.Default())

	cmd := newAddonDisableCmd()
	cmd.SetArgs([]string{"my-addon"})
	// Ignore error from git operations (no git repo in temp dir)
	_ = cmd.Execute()

	// Verify the addon was disabled in addons.yaml
	got, err := readAddonsYAML(filepath.Join(addonsDir, "addons.yaml"))
	if err != nil {
		t.Fatalf("reading addons.yaml: %v", err)
	}
	if got["my-addon"]["enabled"] != false {
		t.Errorf("addon should be disabled, got enabled=%v", got["my-addon"]["enabled"])
	}
}

// --- Functional tests for enable.go ---

func TestEnableCmd_RepoPathRequired(t *testing.T) {
	cfg := &config.Config{
		RepoPath:    "",
		Interactive: false,
	}
	config.Set(cfg)
	defer config.Set(config.Default())

	cmd := newAddonEnableCmd()
	cmd.SetArgs([]string{"test-addon"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("enable should error when repo path is not set")
	}
}

func TestEnableCmd_CreatesNewAddon(t *testing.T) {
	tmp := t.TempDir()
	addonsDir := filepath.Join(tmp, "addons", "environments", "production", "addons")
	if err := os.MkdirAll(addonsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Start with empty addons.yaml
	if err := writeAddonsYAML(filepath.Join(addonsDir, "addons.yaml"), map[string]map[string]interface{}{}); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		RepoPath:    tmp,
		Interactive: false,
	}
	config.Set(cfg)
	defer config.Set(config.Default())

	cmd := newAddonEnableCmd()
	cmd.SetArgs([]string{"new-addon"})
	// Ignore git errors
	_ = cmd.Execute()

	got, err := readAddonsYAML(filepath.Join(addonsDir, "addons.yaml"))
	if err != nil {
		t.Fatalf("reading addons.yaml: %v", err)
	}
	addon, ok := got["new-addon"]
	if !ok {
		t.Fatal("new-addon should be created in addons.yaml")
	}
	if addon["enabled"] != true {
		t.Errorf("new addon should be enabled, got enabled=%v", addon["enabled"])
	}
}

func TestEnableCmd_ReEnablesExisting(t *testing.T) {
	tmp := t.TempDir()
	addonsDir := filepath.Join(tmp, "addons", "environments", "production", "addons")
	if err := os.MkdirAll(addonsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	entries := map[string]map[string]interface{}{
		"my-addon": {"enabled": false, "namespace": "my-ns"},
	}
	if err := writeAddonsYAML(filepath.Join(addonsDir, "addons.yaml"), entries); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		RepoPath:    tmp,
		Interactive: false,
	}
	config.Set(cfg)
	defer config.Set(config.Default())

	cmd := newAddonEnableCmd()
	cmd.SetArgs([]string{"my-addon"})
	_ = cmd.Execute()

	got, err := readAddonsYAML(filepath.Join(addonsDir, "addons.yaml"))
	if err != nil {
		t.Fatalf("reading addons.yaml: %v", err)
	}
	if got["my-addon"]["enabled"] != true {
		t.Errorf("addon should be re-enabled, got enabled=%v", got["my-addon"]["enabled"])
	}
}

// --- Functional tests for list.go ---

func TestListCmd_RepoPathRequired(t *testing.T) {
	cfg := &config.Config{
		RepoPath:    "",
		Interactive: false,
	}
	config.Set(cfg)
	defer config.Set(config.Default())

	cmd := newAddonListCmd()
	err := cmd.Execute()
	if err == nil {
		t.Fatal("list should error when repo path is not set")
	}
}

func TestListCmd_EnvironmentFlagDefault(t *testing.T) {
	cmd := newAddonListCmd()
	envFlag := cmd.Flags().Lookup("environment")
	if envFlag == nil {
		t.Fatal("missing --environment flag")
	}
	// Default should be empty (code defaults to "production")
	if envFlag.DefValue != "" {
		t.Errorf("--environment default = %q, want empty string", envFlag.DefValue)
	}
}

// --- Functional tests for status.go ---

func TestStatusCmd_RequiresExactlyOneArg(t *testing.T) {
	cmd := newAddonStatusCmd()

	if err := cmd.Args(cmd, []string{}); err == nil {
		t.Error("status should require exactly 1 arg")
	}
	if err := cmd.Args(cmd, []string{"addon-name"}); err != nil {
		t.Errorf("status with 1 arg should pass: %v", err)
	}
	if err := cmd.Args(cmd, []string{"a", "b"}); err == nil {
		t.Error("status should reject 2 args")
	}
}

func TestStatusCmd_HasRunE(t *testing.T) {
	cmd := newAddonStatusCmd()
	if cmd.RunE == nil {
		t.Error("status RunE should be set")
	}
}

func TestStatusCmd_UseAndShort(t *testing.T) {
	cmd := newAddonStatusCmd()
	if cmd.Use != "status [addon]" {
		t.Errorf("Use = %q, want 'status [addon]'", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("Short should not be empty")
	}
}
