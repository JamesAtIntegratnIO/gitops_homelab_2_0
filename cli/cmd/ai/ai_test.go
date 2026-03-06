package ai

import (
	"testing"
)

func TestNewCmd(t *testing.T) {
	cmd := NewCmd()
	if cmd.Use != "ai" {
		t.Errorf("expected Use 'ai', got %q", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("expected non-empty Short description")
	}
	if cmd.Long == "" {
		t.Error("expected non-empty Long description")
	}

	// Verify subcommands are registered
	subNames := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		subNames[sub.Name()] = true
	}
	if !subNames["reindex"] {
		t.Error("missing 'reindex' subcommand")
	}
}

func TestReindexCmd_Structure(t *testing.T) {
	cmd := NewCmd()

	// Find reindex subcommand
	var found bool
	for _, sub := range cmd.Commands() {
		if sub.Name() == "reindex" {
			found = true

			if sub.Short == "" {
				t.Error("reindex: expected non-empty Short description")
			}

			// Check flags exist
			waitFlag := sub.Flags().Lookup("wait")
			if waitFlag == nil {
				t.Error("missing --wait flag on reindex command")
			}
			timeoutFlag := sub.Flags().Lookup("timeout")
			if timeoutFlag == nil {
				t.Error("missing --timeout flag on reindex command")
			}

			// Check shorthand flags
			if waitFlag != nil && waitFlag.Shorthand != "w" {
				t.Errorf("--wait shorthand = %q, want 'w'", waitFlag.Shorthand)
			}
			if timeoutFlag != nil && timeoutFlag.Shorthand != "t" {
				t.Errorf("--timeout shorthand = %q, want 't'", timeoutFlag.Shorthand)
			}

			break
		}
	}
	if !found {
		t.Fatal("reindex command not found")
	}
}

func TestConstants(t *testing.T) {
	if aiNamespace != "ai" {
		t.Errorf("aiNamespace = %q, want 'ai'", aiNamespace)
	}
	if cronJobName != "git-indexer" {
		t.Errorf("cronJobName = %q, want 'git-indexer'", cronJobName)
	}
	if jobPrefixManual != "git-indexer-manual-" {
		t.Errorf("jobPrefixManual = %q, want 'git-indexer-manual-'", jobPrefixManual)
	}
}
