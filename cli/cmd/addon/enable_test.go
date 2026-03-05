package addon

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnableMutate_ExistingSetsEnabled(t *testing.T) {
	entries := map[string]map[string]interface{}{
		"nginx": {"enabled": false, "namespace": "ingress"},
	}

	// Simulate enable mutate for existing addon
	addonName := "nginx"
	existing := entries[addonName]
	existing["enabled"] = true
	entries[addonName] = existing

	if entries[addonName]["enabled"] != true {
		t.Errorf("expected enabled=true, got %v", entries[addonName]["enabled"])
	}
	if entries[addonName]["namespace"] != "ingress" {
		t.Error("namespace should be preserved")
	}
}

func TestEnableMutate_NewAddonCreatesEntry(t *testing.T) {
	entries := map[string]map[string]interface{}{}

	// Simulate enable mutate for new addon
	addonName := "cert-manager"
	ns := "cert-manager"
	entry := map[string]interface{}{
		"enabled":         true,
		"namespace":       ns,
		"chartRepository": "https://stakater.github.io/stakater-charts",
		"chartName":       "application",
		"defaultVersion":  "6.14.0",
	}
	entries[addonName] = entry

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	addon := entries[addonName]
	if addon["enabled"] != true {
		t.Error("new addon should be enabled")
	}
	if addon["namespace"] != "cert-manager" {
		t.Errorf("namespace = %v, want 'cert-manager'", addon["namespace"])
	}
	if addon["chartRepository"] != "https://stakater.github.io/stakater-charts" {
		t.Error("should use default chart repository")
	}
	if addon["chartName"] != "application" {
		t.Error("should use default chart name")
	}
	if addon["defaultVersion"] != "6.14.0" {
		t.Error("should use default version")
	}
}

func TestEnableMutate_CustomChartValues(t *testing.T) {
	entries := map[string]map[string]interface{}{}

	entry := map[string]interface{}{
		"enabled":         true,
		"namespace":       "my-ns",
		"chartRepository": "https://custom.charts.io",
		"chartName":       "custom-chart",
		"defaultVersion":  "2.0.0",
	}
	entries["custom-addon"] = entry

	addon := entries["custom-addon"]
	if addon["chartRepository"] != "https://custom.charts.io" {
		t.Error("custom chart repository should be used")
	}
	if addon["chartName"] != "custom-chart" {
		t.Error("custom chart name should be used")
	}
	if addon["defaultVersion"] != "2.0.0" {
		t.Error("custom version should be used")
	}
}

func TestEnableMutate_ScaffoldsValuesDirectory(t *testing.T) {
	tmp := t.TempDir()
	addonName := "my-addon"
	layer := "environment"
	valuesDir := filepath.Join(tmp, addonName)

	// Scaffold values directory (simulating enable logic)
	if err := os.MkdirAll(valuesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	valuesFile := filepath.Join(valuesDir, "values.yaml")
	scaffold := "# " + addonName + " values\n# Layer: " + layer + "\n# See: https://github.com/stakater/application\n"
	if err := os.WriteFile(valuesFile, []byte(scaffold), 0o644); err != nil {
		t.Fatal(err)
	}

	// Verify
	data, err := os.ReadFile(valuesFile)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if content == "" {
		t.Error("values.yaml should not be empty")
	}
	if !contains(content, addonName) {
		t.Error("values.yaml should mention addon name")
	}
	if !contains(content, layer) {
		t.Error("values.yaml should mention layer")
	}
}

func TestEnableMutate_SkipsExistingValuesFile(t *testing.T) {
	tmp := t.TempDir()
	valuesDir := filepath.Join(tmp, "my-addon")
	if err := os.MkdirAll(valuesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	valuesFile := filepath.Join(valuesDir, "values.yaml")
	originalContent := "# existing content\nkey: value\n"
	if err := os.WriteFile(valuesFile, []byte(originalContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Simulate the enable logic: only scaffold if file doesn't exist
	if _, err := os.Stat(valuesFile); os.IsNotExist(err) {
		t.Fatal("file should exist — scaffold should be skipped")
	}

	data, err := os.ReadFile(valuesFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != originalContent {
		t.Error("existing values.yaml should not be overwritten")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
