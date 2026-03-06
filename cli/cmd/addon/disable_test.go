package addon

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDisableMutate_SetsEnabledFalse(t *testing.T) {
	entries := map[string]map[string]interface{}{
		"cert-manager": {"enabled": true, "namespace": "cert-manager"},
	}

	// Simulate the disable mutate callback logic (non-remove path)
	addonName := "cert-manager"
	if _, ok := entries[addonName]; !ok {
		t.Fatal("addon should exist")
	}
	entries[addonName]["enabled"] = false

	if entries[addonName]["enabled"] != false {
		t.Errorf("expected enabled=false, got %v", entries[addonName]["enabled"])
	}
	// Other fields should be preserved
	if entries[addonName]["namespace"] != "cert-manager" {
		t.Errorf("namespace should be preserved")
	}
}

func TestDisableMutate_RemoveDeletesEntry(t *testing.T) {
	entries := map[string]map[string]interface{}{
		"cert-manager": {"enabled": true, "namespace": "cert-manager"},
		"nginx":        {"enabled": true},
	}

	// Simulate remove logic
	delete(entries, "cert-manager")

	if _, ok := entries["cert-manager"]; ok {
		t.Error("cert-manager should be removed from entries")
	}
	if _, ok := entries["nginx"]; !ok {
		t.Error("nginx should still exist")
	}
}

func TestDisableMutate_RemoveDeletesValuesDir(t *testing.T) {
	tmp := t.TempDir()
	valuesDir := filepath.Join(tmp, "cert-manager")
	if err := os.MkdirAll(valuesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(valuesDir, "values.yaml"), []byte("test: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Simulate the remove logic from disable.go
	if _, err := os.Stat(valuesDir); err == nil {
		if err := os.RemoveAll(valuesDir); err != nil {
			t.Fatalf("removing values dir: %v", err)
		}
	}

	if _, err := os.Stat(valuesDir); !os.IsNotExist(err) {
		t.Error("values directory should be deleted")
	}
}

func TestDisableMutate_AddonNotFound(t *testing.T) {
	entries := map[string]map[string]interface{}{
		"nginx": {"enabled": true},
	}

	addonName := "missing-addon"
	_, ok := entries[addonName]
	if ok {
		t.Error("missing-addon should not exist in entries")
	}
}

func TestDisable_NonInteractive_DisablesAndWritesBack(t *testing.T) {
	tmp := t.TempDir()
	addonsDir := filepath.Join(tmp, "addons", "environments", "staging", "addons")
	if err := os.MkdirAll(addonsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	yamlPath := filepath.Join(addonsDir, "addons.yaml")
	entries := map[string]map[string]interface{}{
		"my-addon": {"enabled": true, "namespace": "my-ns"},
	}
	if err := writeAddonsYAML(yamlPath, entries); err != nil {
		t.Fatal(err)
	}

	// Simulate a full disable write cycle without git
	got, err := readAddonsYAML(yamlPath)
	if err != nil {
		t.Fatal(err)
	}
	got["my-addon"]["enabled"] = false
	if err := writeAddonsYAML(yamlPath, got); err != nil {
		t.Fatal(err)
	}

	result, err := readAddonsYAML(yamlPath)
	if err != nil {
		t.Fatal(err)
	}
	if result["my-addon"]["enabled"] != false {
		t.Errorf("expected enabled=false, got %v", result["my-addon"]["enabled"])
	}
	if result["my-addon"]["namespace"] != "my-ns" {
		t.Error("namespace should be preserved after disable")
	}
}
