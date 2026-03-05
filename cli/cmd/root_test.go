package cmd

import (
	"testing"
)

func TestRootCommand_Structure(t *testing.T) {
	// Use sync.Once to safely ensure setup — matches Execute()'s pattern
	setupOnce.Do(setupCommands)

	if rootCmd.Use != "hctl" {
		t.Errorf("root Use = %q, want 'hctl'", rootCmd.Use)
	}
	if rootCmd.Short == "" {
		t.Error("root Short should not be empty")
	}
	if rootCmd.Long == "" {
		t.Error("root Long should not be empty")
	}
	if !rootCmd.SilenceUsage {
		t.Error("root SilenceUsage should be true")
	}
	if !rootCmd.SilenceErrors {
		t.Error("root SilenceErrors should be true")
	}
}

func TestRootCommand_PersistentFlags(t *testing.T) {
	setupOnce.Do(setupCommands)

	flags := []struct {
		name     string
		wantType string
	}{
		{"config", "string"},
		{"non-interactive", "bool"},
		{"output", "string"},
		{"verbose", "bool"},
		{"quiet", "bool"},
	}

	for _, f := range flags {
		t.Run(f.name, func(t *testing.T) {
			flag := rootCmd.PersistentFlags().Lookup(f.name)
			if flag == nil {
				t.Fatalf("persistent flag %q not found", f.name)
			}
			if flag.Value.Type() != f.wantType {
				t.Errorf("flag %q type = %q, want %q", f.name, flag.Value.Type(), f.wantType)
			}
		})
	}
}

func TestRootCommand_Subcommands(t *testing.T) {
	setupOnce.Do(setupCommands)

	expected := []string{
		"version", "status", "diagnose", "reconcile", "context",
		"alerts", "completion", "vcluster", "deploy", "addon",
		"scale", "secret", "up", "down", "open", "logs", "doctor",
		"trace", "init", "ai",
	}

	subNames := make(map[string]bool)
	for _, sub := range rootCmd.Commands() {
		subNames[sub.Name()] = true
	}

	for _, name := range expected {
		if !subNames[name] {
			t.Errorf("missing subcommand %q", name)
		}
	}
}


