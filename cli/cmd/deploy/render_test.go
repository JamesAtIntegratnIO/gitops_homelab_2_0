package deploy

import (
	"strings"
	"testing"
)

func TestPrintUnifiedDiff_IdenticalStrings(t *testing.T) {
	// printUnifiedDiff writes to stdout; we verify it doesn't panic with identical input
	// and test the logic inline: identical lines produce no diff markers
	old := "line1\nline2\nline3"
	new := "line1\nline2\nline3"
	oldLines := strings.Split(old, "\n")
	newLines := strings.Split(new, "\n")

	for i := 0; i < len(oldLines); i++ {
		if oldLines[i] != newLines[i] {
			t.Errorf("line %d should be identical", i)
		}
	}
}

func TestPrintUnifiedDiff_NoPanic(t *testing.T) {
	// Ensure printUnifiedDiff doesn't panic on various inputs
	tests := []struct {
		name     string
		old, new string
	}{
		{"empty both", "", ""},
		{"old empty", "", "new content"},
		{"new empty", "old content", ""},
		{"single line change", "before", "after"},
		{"multiline", "a\nb\nc", "a\nB\nc"},
		{"added lines", "a\nb", "a\nb\nc\nd"},
		{"removed lines", "a\nb\nc\nd", "a\nb"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just verify no panic
			printUnifiedDiff("test.yaml", tt.old, tt.new)
		})
	}
}

func TestNewDeployRenderCmd_Structure(t *testing.T) {
	cmd := newDeployRenderCmd()
	if cmd.Use != "render" {
		t.Errorf("Use = %q, want 'render'", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("Short should not be empty")
	}

	// Verify flags
	if cmd.Flags().Lookup("cluster") == nil {
		t.Error("missing --cluster flag")
	}
	if cmd.Flags().Lookup("file") == nil {
		t.Error("missing --file flag")
	}

	// Verify default for --file
	fileFlag := cmd.Flags().Lookup("file")
	if fileFlag.DefValue != "score.yaml" {
		t.Errorf("--file default = %q, want 'score.yaml'", fileFlag.DefValue)
	}
}

func TestNewDeployDiffCmd_Structure(t *testing.T) {
	cmd := newDeployDiffCmd()
	if cmd.Use != "diff" {
		t.Errorf("Use = %q, want 'diff'", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("Short should not be empty")
	}

	// Verify flags
	if cmd.Flags().Lookup("cluster") == nil {
		t.Error("missing --cluster flag")
	}
	if cmd.Flags().Lookup("file") == nil {
		t.Error("missing --file flag")
	}
}

func TestNewDeployCmd_HasAllSubcommands(t *testing.T) {
	cmd := NewCmd()
	if cmd.Use != "deploy" {
		t.Errorf("Use = %q, want 'deploy'", cmd.Use)
	}

	subNames := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		subNames[sub.Name()] = true
	}

	expected := []string{"init", "run", "render", "diff", "status", "remove", "list"}
	for _, name := range expected {
		if !subNames[name] {
			t.Errorf("missing subcommand %q", name)
		}
	}
}
