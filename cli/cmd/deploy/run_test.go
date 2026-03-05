package deploy

import (
	"testing"

	"github.com/jamesatintegratnio/hctl/internal/git"
)

func TestGitWorkflowStep_AllModes(t *testing.T) {
	tests := []struct {
		gitMode   string
		wantTitle string
	}{
		{"auto", "Committing and pushing"},
		{"generate", "Committing changes"},
		{"stage-only", "Staging files"},
		{"", "Staging files"},
		{"unknown-mode", "Staging files"},
	}

	for _, tt := range tests {
		t.Run("mode_"+tt.gitMode, func(t *testing.T) {
			step := gitWorkflowStep(git.WorkflowOpts{
				RepoPath: "/tmp/fake",
				Action:   "deploy",
				Resource: "my-app",
				GitMode:  tt.gitMode,
			})
			if step.Title != tt.wantTitle {
				t.Errorf("Title = %q, want %q", step.Title, tt.wantTitle)
			}
			if step.Run == nil {
				t.Error("Step Run function should not be nil")
			}
		})
	}
}

func TestGitWorkflowStep_RunWithNoRepo(t *testing.T) {
	// When there's no git repo, the step should still return successfully
	step := gitWorkflowStep(git.WorkflowOpts{
		RepoPath: "/nonexistent/path/that/does/not/exist",
		Paths:    []string{"file.yaml"},
		Action:   "deploy",
		Resource: "test",
		GitMode:  "auto",
	})

	result, err := step.Run()
	if err != nil {
		t.Fatalf("Run should not error when no git repo found, got: %v", err)
	}
	if result != "no git repo detected" {
		t.Errorf("result = %q, want 'no git repo detected'", result)
	}
}

func TestNewDeployRunCmd_FlagDefaults(t *testing.T) {
	cmd := newDeployRunCmd()

	// --dry-run defaults to false
	if cmd.Flags().Lookup("dry-run").DefValue != "false" {
		t.Errorf("--dry-run default should be 'false'")
	}

	// --file defaults to score.yaml
	if cmd.Flags().Lookup("file").DefValue != "score.yaml" {
		t.Errorf("--file default should be 'score.yaml'")
	}

	// --watch defaults to false
	if cmd.Flags().Lookup("watch").DefValue != "false" {
		t.Errorf("--watch default should be 'false'")
	}

	// --timeout defaults to 5m0s
	if cmd.Flags().Lookup("timeout").DefValue != "5m0s" {
		t.Errorf("--timeout default should be '5m0s'")
	}

	// --file has -f shorthand
	fileFlag := cmd.Flags().Lookup("file")
	if fileFlag.Shorthand != "f" {
		t.Errorf("--file shorthand = %q, want 'f'", fileFlag.Shorthand)
	}

	// --watch has -w shorthand
	watchFlag := cmd.Flags().Lookup("watch")
	if watchFlag.Shorthand != "w" {
		t.Errorf("--watch shorthand = %q, want 'w'", watchFlag.Shorthand)
	}
}
