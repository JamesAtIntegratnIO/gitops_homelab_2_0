package addon

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewCmd(t *testing.T) {
	cmd := NewCmd()
	if cmd.Use != "addon" {
		t.Errorf("expected Use 'addon', got %q", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("expected non-empty Short description")
	}

	// Verify all subcommands are registered
	subNames := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		subNames[sub.Name()] = true
	}
	for _, expected := range []string{"list", "status", "enable", "disable"} {
		if !subNames[expected] {
			t.Errorf("missing subcommand %q", expected)
		}
	}
}

func TestResolveLayerPaths_Environment(t *testing.T) {
	addonsPath, valuesDir, err := resolveLayerPaths("/repo", "environment", "production", "", "", "my-addon")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantAddons := filepath.Join("/repo", "addons", "environments", "production", "addons", "addons.yaml")
	if addonsPath != wantAddons {
		t.Errorf("addonsPath = %q, want %q", addonsPath, wantAddons)
	}
	wantValues := filepath.Join("/repo", "addons", "environments", "production", "addons", "my-addon")
	if valuesDir != wantValues {
		t.Errorf("valuesDir = %q, want %q", valuesDir, wantValues)
	}
}

func TestResolveLayerPaths_ClusterRole(t *testing.T) {
	addonsPath, valuesDir, err := resolveLayerPaths("/repo", "cluster-role", "staging", "control-plane", "", "cert-manager")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantAddons := filepath.Join("/repo", "addons", "cluster-roles", "control-plane", "addons", "addons.yaml")
	if addonsPath != wantAddons {
		t.Errorf("addonsPath = %q, want %q", addonsPath, wantAddons)
	}
	wantValues := filepath.Join("/repo", "addons", "cluster-roles", "control-plane", "addons", "cert-manager")
	if valuesDir != wantValues {
		t.Errorf("valuesDir = %q, want %q", valuesDir, wantValues)
	}
}

func TestResolveLayerPaths_ClusterRoleMissingFlag(t *testing.T) {
	_, _, err := resolveLayerPaths("/repo", "cluster-role", "prod", "", "", "addon")
	if err == nil {
		t.Error("expected error when --cluster-role is empty")
	}
}

func TestResolveLayerPaths_Cluster(t *testing.T) {
	addonsPath, valuesDir, err := resolveLayerPaths("/repo", "cluster", "prod", "", "the-cluster", "nginx")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantAddons := filepath.Join("/repo", "addons", "clusters", "the-cluster", "addons", "addons.yaml")
	if addonsPath != wantAddons {
		t.Errorf("addonsPath = %q, want %q", addonsPath, wantAddons)
	}
	wantValues := filepath.Join("/repo", "addons", "clusters", "the-cluster", "addons", "nginx")
	if valuesDir != wantValues {
		t.Errorf("valuesDir = %q, want %q", valuesDir, wantValues)
	}
}

func TestResolveLayerPaths_ClusterMissingFlag(t *testing.T) {
	_, _, err := resolveLayerPaths("/repo", "cluster", "prod", "", "", "addon")
	if err == nil {
		t.Error("expected error when --cluster is empty")
	}
}

func TestResolveLayerPaths_InvalidLayer(t *testing.T) {
	_, _, err := resolveLayerPaths("/repo", "invalid", "prod", "", "", "addon")
	if err == nil {
		t.Error("expected error for invalid layer")
	}
}

func TestReadWriteAddonsYAML(t *testing.T) {
	tmp := t.TempDir()
	yamlPath := filepath.Join(tmp, "addons.yaml")

	// Write
	entries := map[string]map[string]interface{}{
		"cert-manager": {
			"enabled":   true,
			"namespace": "cert-manager",
		},
		"ingress-nginx": {
			"enabled":   false,
			"namespace": "ingress",
		},
	}
	if err := writeAddonsYAML(yamlPath, entries); err != nil {
		t.Fatalf("writeAddonsYAML: %v", err)
	}

	// Read back
	got, err := readAddonsYAML(yamlPath)
	if err != nil {
		t.Fatalf("readAddonsYAML: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(got))
	}

	cm, ok := got["cert-manager"]
	if !ok {
		t.Fatal("missing cert-manager entry")
	}
	if cm["enabled"] != true {
		t.Errorf("cert-manager enabled = %v, want true", cm["enabled"])
	}
	if cm["namespace"] != "cert-manager" {
		t.Errorf("cert-manager namespace = %v, want cert-manager", cm["namespace"])
	}

	ing, ok := got["ingress-nginx"]
	if !ok {
		t.Fatal("missing ingress-nginx entry")
	}
	if ing["enabled"] != false {
		t.Errorf("ingress-nginx enabled = %v, want false", ing["enabled"])
	}
}

func TestReadAddonsYAML_NotExist(t *testing.T) {
	_, err := readAddonsYAML("/nonexistent/path/addons.yaml")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestReadAddonsYAML_InvalidYAML(t *testing.T) {
	tmp := t.TempDir()
	yamlPath := filepath.Join(tmp, "addons.yaml")
	if err := os.WriteFile(yamlPath, []byte("{{invalid yaml"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := readAddonsYAML(yamlPath)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestWriteAddonsYAML_CreatesDir(t *testing.T) {
	tmp := t.TempDir()
	nested := filepath.Join(tmp, "deep", "nested", "addons.yaml")

	entries := map[string]map[string]interface{}{
		"test-addon": {"enabled": true},
	}
	if err := writeAddonsYAML(nested, entries); err != nil {
		t.Fatalf("writeAddonsYAML failed: %v", err)
	}

	if _, err := os.Stat(nested); os.IsNotExist(err) {
		t.Error("expected file to be created")
	}
}
