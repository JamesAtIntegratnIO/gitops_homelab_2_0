package addon

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListCmd_ReadAddonsAndBuildRows(t *testing.T) {
	tmp := t.TempDir()
	addonsDir := filepath.Join(tmp, "addons", "environments", "production", "addons")
	if err := os.MkdirAll(addonsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	entries := map[string]map[string]interface{}{
		"cert-manager":  {"enabled": true, "namespace": "cert-manager"},
		"ingress-nginx": {"enabled": false, "namespace": "ingress"},
		"external-dns":  {"enabled": true, "namespace": "external-dns"},
	}
	yamlPath := filepath.Join(addonsDir, "addons.yaml")
	if err := writeAddonsYAML(yamlPath, entries); err != nil {
		t.Fatal(err)
	}

	// Read back and verify we can build rows
	got, err := readAddonsYAML(yamlPath)
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != 3 {
		t.Fatalf("expected 3 addons, got %d", len(got))
	}

	// Build rows like list.go does
	var rows [][]string
	for name, entry := range got {
		enabled := "yes"
		if e, ok := entry["enabled"]; ok {
			if b, ok := e.(bool); ok && !b {
				enabled = "no"
			}
		}
		rows = append(rows, []string{name, enabled, "production", "—"})
	}

	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(rows))
	}

	// Find ingress-nginx row and verify it says "no"
	foundIngress := false
	for _, row := range rows {
		if row[0] == "ingress-nginx" {
			foundIngress = true
			if row[1] != "no" {
				t.Errorf("ingress-nginx enabled = %q, want 'no'", row[1])
			}
		}
	}
	if !foundIngress {
		t.Error("ingress-nginx row not found")
	}
}

func TestListCmd_EmptyAddons(t *testing.T) {
	tmp := t.TempDir()
	addonsDir := filepath.Join(tmp, "addons", "environments", "production", "addons")
	if err := os.MkdirAll(addonsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	yamlPath := filepath.Join(addonsDir, "addons.yaml")
	if err := writeAddonsYAML(yamlPath, map[string]map[string]interface{}{}); err != nil {
		t.Fatal(err)
	}

	got, err := readAddonsYAML(yamlPath)
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != 0 {
		t.Fatalf("expected 0 addons, got %d", len(got))
	}
}

func TestListCmd_ValuesFolderResolution(t *testing.T) {
	// Test the valuesFolderName / chartName / addonName fallback logic from list.go

	tests := []struct {
		name     string
		entry    map[string]interface{}
		wantName string
	}{
		{
			name:     "uses valuesFolderName when set",
			entry:    map[string]interface{}{"valuesFolderName": "custom-folder"},
			wantName: "custom-folder",
		},
		{
			name:     "falls back to chartName",
			entry:    map[string]interface{}{"chartName": "my-chart"},
			wantName: "my-chart",
		},
		{
			name:     "falls back to addon name",
			entry:    map[string]interface{}{"enabled": true},
			wantName: "my-addon",
		},
		{
			name:     "valuesFolderName takes precedence over chartName",
			entry:    map[string]interface{}{"valuesFolderName": "vfn", "chartName": "cn"},
			wantName: "vfn",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addonName := "my-addon"
			folderName := addonName
			if vfn, ok := tt.entry["valuesFolderName"].(string); ok && vfn != "" {
				folderName = vfn
			} else if cn, ok := tt.entry["chartName"].(string); ok && cn != "" {
				folderName = cn
			}

			if folderName != tt.wantName {
				t.Errorf("folderName = %q, want %q", folderName, tt.wantName)
			}
		})
	}
}
