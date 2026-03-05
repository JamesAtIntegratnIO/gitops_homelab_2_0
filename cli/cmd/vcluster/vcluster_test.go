package vcluster

import (
	"testing"
)

func TestNewCmd_Structure(t *testing.T) {
	cmd := NewCmd()

	if cmd.Use != "vcluster" {
		t.Errorf("Use = %q, want 'vcluster'", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("Short should not be empty")
	}
	if cmd.Long == "" {
		t.Error("Long should not be empty")
	}

	// Check alias
	found := false
	for _, a := range cmd.Aliases {
		if a == "vc" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected alias 'vc', got %v", cmd.Aliases)
	}
}

func TestNewCmd_Subcommands(t *testing.T) {
	cmd := NewCmd()

	expectedSubcmds := []string{"list", "create", "status", "kubeconfig", "delete", "connect", "apps", "sync"}
	subcmds := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		subcmds[sub.Name()] = true
	}

	for _, name := range expectedSubcmds {
		if !subcmds[name] {
			t.Errorf("expected subcommand %q not found; got %v", name, subcmds)
		}
	}
}

func TestNewCmd_SubcommandCount(t *testing.T) {
	cmd := NewCmd()

	// Should have exactly 8 subcommands
	if got := len(cmd.Commands()); got != 8 {
		t.Errorf("expected 8 subcommands, got %d", got)
		for _, sub := range cmd.Commands() {
			t.Logf("  subcommand: %s", sub.Name())
		}
	}
}
