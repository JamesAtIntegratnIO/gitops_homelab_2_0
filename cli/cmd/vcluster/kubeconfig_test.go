package vcluster

import (
	"os"
	"path/filepath"
	"testing"
)

func TestKubeconfigCmd_Structure(t *testing.T) {
	cmd := newKubeconfigCmd()

	if cmd.Use != "kubeconfig [name]" {
		t.Errorf("Use = %q, want 'kubeconfig [name]'", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("Short should not be empty")
	}
	if cmd.RunE == nil {
		t.Error("RunE should be set")
	}
}

func TestKubeconfigCmd_RequiresExactlyOneArg(t *testing.T) {
	cmd := newKubeconfigCmd()

	if err := cmd.Args(cmd, []string{}); err == nil {
		t.Error("kubeconfig with 0 args should fail")
	}
	if err := cmd.Args(cmd, []string{"my-cluster"}); err != nil {
		t.Errorf("kubeconfig with 1 arg should pass: %v", err)
	}
	if err := cmd.Args(cmd, []string{"a", "b"}); err == nil {
		t.Error("kubeconfig with 2 args should fail")
	}
}

func TestKubeconfigCmd_HasOutputFlag(t *testing.T) {
	cmd := newKubeconfigCmd()

	f := cmd.Flags().Lookup("output")
	if f == nil {
		t.Fatal("expected --output flag")
	}
	if f.Shorthand != "o" {
		t.Errorf("expected shorthand 'o', got %q", f.Shorthand)
	}
}

func TestWriteFile_WritesData(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "kubeconfig.yaml")

	data := []byte("apiVersion: v1\nkind: Config\n")
	if err := writeFile(path, data); err != nil {
		t.Fatalf("writeFile: %v", err)
	}

	readBack, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading back file: %v", err)
	}
	if string(readBack) != string(data) {
		t.Errorf("data mismatch: got %q, want %q", string(readBack), string(data))
	}

	// Verify file permissions (0600)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	perm := info.Mode().Perm()
	if perm != 0o600 {
		t.Errorf("file permissions = %o, want 0600", perm)
	}
}

func TestConnectCmd_Structure(t *testing.T) {
	cmd := newConnectCmd()

	if cmd.Use != "connect [name]" {
		t.Errorf("Use = %q, want 'connect [name]'", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("Short should not be empty")
	}
	if cmd.RunE == nil {
		t.Error("RunE should be set")
	}

	// Requires exactly 1 arg
	if err := cmd.Args(cmd, []string{}); err == nil {
		t.Error("connect with 0 args should fail")
	}
	if err := cmd.Args(cmd, []string{"my-cluster"}); err != nil {
		t.Errorf("connect with 1 arg should pass: %v", err)
	}
}
