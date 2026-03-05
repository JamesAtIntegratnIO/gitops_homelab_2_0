package vcluster

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDeleteCmd_Structure(t *testing.T) {
	cmd := newDeleteCmd()

	if cmd.Use != "delete [name]" {
		t.Errorf("Use = %q, want 'delete [name]'", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("Short should not be empty")
	}
	if cmd.Long == "" {
		t.Error("Long should not be empty")
	}
	if cmd.RunE == nil {
		t.Error("RunE should be set")
	}
}

func TestDeleteCmd_RequiresExactlyOneArg(t *testing.T) {
	cmd := newDeleteCmd()

	if err := cmd.Args(cmd, []string{}); err == nil {
		t.Error("delete with 0 args should fail")
	}
	if err := cmd.Args(cmd, []string{"my-cluster"}); err != nil {
		t.Errorf("delete with 1 arg should pass: %v", err)
	}
	if err := cmd.Args(cmd, []string{"a", "b"}); err == nil {
		t.Error("delete with 2 args should fail")
	}
}

func TestDeleteCmd_FilePathResolution(t *testing.T) {
	tmp := t.TempDir()

	// Simulate the file path logic from the delete command
	name := "test-cluster"
	filePath := filepath.Join(tmp, "platform", "vclusters", name+".yaml")

	// File doesn't exist yet
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Error("file should not exist before creation")
	}

	// Create the file to test the path is correct
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filePath, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Verify path components
	if filepath.Base(filePath) != "test-cluster.yaml" {
		t.Errorf("expected filename 'test-cluster.yaml', got %q", filepath.Base(filePath))
	}

	// Verify relative path calculation
	relPath, err := filepath.Rel(tmp, filePath)
	if err != nil {
		t.Fatal(err)
	}
	expected := filepath.Join("platform", "vclusters", "test-cluster.yaml")
	if relPath != expected {
		t.Errorf("relPath = %q, want %q", relPath, expected)
	}
}
