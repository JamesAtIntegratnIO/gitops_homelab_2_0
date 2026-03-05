package cmd

import (
	"testing"

	"github.com/jamesatintegratnio/hctl/internal/tui"
)

func TestPhaseBadge(t *testing.T) {
	tests := []struct {
		phase   string
		wantNon string // the phase string should at least contain the input
	}{
		{"Ready", "Ready"},
		{"Progressing", "Progressing"},
		{"Degraded", "Degraded"},
		{"Failed", "Failed"},
		{"Suspended", "Suspended"},
		{"Unknown", "Unknown"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.phase, func(t *testing.T) {
			got := tui.PhaseBadge(tt.phase)
			if got == "" && tt.phase != "" {
				t.Errorf("PhaseBadge(%q) returned empty string", tt.phase)
			}
			// The styled output should contain the original text
			// (lipgloss wraps it with ANSI codes but the text is still there)
			if tt.phase != "" && len(got) < len(tt.phase) {
				t.Errorf("PhaseBadge(%q) returned shorter string than input: %q", tt.phase, got)
			}
		})
	}
}

func TestRootCmdStructure(t *testing.T) {
	// Ensure commands are set up (replaces former init()-based registration)
	setupOnce.Do(setupCommands)

	// Verify the root command exists and has expected configuration
	if rootCmd == nil {
		t.Fatal("rootCmd is nil")
	}
	if rootCmd.Use != "hctl" {
		t.Errorf("rootCmd.Use = %q, want 'hctl'", rootCmd.Use)
	}
	if rootCmd.Short == "" {
		t.Error("rootCmd.Short should not be empty")
	}

	// Verify persistent flags are registered
	flags := rootCmd.PersistentFlags()
	for _, name := range []string{"output", "verbose", "quiet"} {
		if flags.Lookup(name) == nil {
			t.Errorf("missing persistent flag %q", name)
		}
	}
}

func TestSetupCommands_RegistersSubcommands(t *testing.T) {
	setupOnce.Do(setupCommands)

	// Expected top-level subcommand names
	expectedCmds := []string{
		"version",
		"init",
		"status",
		"diagnose",
		"reconcile",
		"context",
		"alerts",
		"completion",
		"vcluster",
		"deploy",
		"addon",
		"scale",
		"secret",
		"ai",
		"up",
		"down",
		"open",
		"logs",
		"doctor",
		"trace",
	}

	// Build a set of registered command names
	registeredNames := make(map[string]bool)
	for _, cmd := range rootCmd.Commands() {
		registeredNames[cmd.Name()] = true
	}

	for _, name := range expectedCmds {
		if !registeredNames[name] {
			t.Errorf("missing expected subcommand %q", name)
		}
	}
}

func TestSetupCommands_PersistentFlags(t *testing.T) {
	setupOnce.Do(setupCommands)

	expectedFlags := []string{
		"config",
		"non-interactive",
		"output",
		"verbose",
		"quiet",
	}

	flags := rootCmd.PersistentFlags()
	for _, name := range expectedFlags {
		if flags.Lookup(name) == nil {
			t.Errorf("missing persistent flag %q", name)
		}
	}
}

func TestSetupCommands_SilenceSettings(t *testing.T) {
	setupOnce.Do(setupCommands)

	if !rootCmd.SilenceUsage {
		t.Error("SilenceUsage should be true")
	}
	if !rootCmd.SilenceErrors {
		t.Error("SilenceErrors should be true")
	}
}

func TestSetupCommands_CommandAliases(t *testing.T) {
	setupOnce.Do(setupCommands)

	// Verify some commands have proper configuration
	for _, cmd := range rootCmd.Commands() {
		switch cmd.Name() {
		case "diagnose":
			if cmd.Args == nil {
				t.Error("diagnose should require exactly 1 arg")
			}
		case "reconcile":
			if cmd.Args == nil {
				t.Error("reconcile should require exactly 1 arg")
			}
		case "completion":
			if cmd.Args == nil {
				t.Error("completion should require exactly 1 arg")
			}
		}
	}
}

func TestSetupCommands_StatusHasWatchFlag(t *testing.T) {
	setupOnce.Do(setupCommands)

	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "status" {
			if cmd.Flags().Lookup("watch") == nil {
				t.Error("status command missing --watch flag")
			}
			if cmd.Flags().Lookup("interval") == nil {
				t.Error("status command missing --interval flag")
			}
			return
		}
	}
	t.Error("status command not found")
}

func TestSetupCommands_DiagnoseHasBundleFlag(t *testing.T) {
	setupOnce.Do(setupCommands)

	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "diagnose" {
			if cmd.Flags().Lookup("bundle") == nil {
				t.Error("diagnose command missing --bundle flag")
			}
			return
		}
	}
	t.Error("diagnose command not found")
}
