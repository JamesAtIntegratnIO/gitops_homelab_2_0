package tui

import (
	"testing"
)

// TestGitUIAdapterType verifies GitUIAdapter is constructable and its methods
// exist with correct signatures. The actual Confirm method delegates to an
// interactive bubbletea prompt and can't be tested non-interactively.
func TestGitUIAdapterType(t *testing.T) {
	adapter := GitUIAdapter{}

	// Verify PrintSuccess and PrintDim don't panic with empty strings.
	// These write to stdout, which is acceptable in tests.
	adapter.PrintSuccess("")
	adapter.PrintDim("")
	adapter.PrintSuccess("test success message")
	adapter.PrintDim("test dim message")
}

// TestGitUIAdapterImplementsInterface verifies the adapter has the expected
// method signatures by calling them through a local interface mirror.
// This ensures the adapter doesn't accidentally break the git.UI contract.
type uiInterface interface {
	Confirm(prompt string) (bool, error)
	PrintSuccess(msg string)
	PrintDim(msg string)
}

func TestGitUIAdapterSatisfiesInterface(t *testing.T) {
	var _ uiInterface = GitUIAdapter{}
}
