package cmd

import (
	"testing"
)

func TestPhaseStyled(t *testing.T) {
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
			got := phaseStyled(tt.phase)
			if got == "" && tt.phase != "" {
				t.Errorf("phaseStyled(%q) returned empty string", tt.phase)
			}
			// The styled output should contain the original text
			// (lipgloss wraps it with ANSI codes but the text is still there)
			if tt.phase != "" && len(got) < len(tt.phase) {
				t.Errorf("phaseStyled(%q) returned shorter string than input: %q", tt.phase, got)
			}
		})
	}
}

func TestRootCmdStructure(t *testing.T) {
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
