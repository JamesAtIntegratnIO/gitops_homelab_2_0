package deploy

import (
	"testing"

	"github.com/jamesatintegratnio/hctl/internal/git"
)

func TestNewDeployStatusCmd_Structure(t *testing.T) {
	cmd := newDeployStatusCmd()
	if cmd.Use != "status [workload]" {
		t.Errorf("Use = %q, want 'status [workload]'", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("Short should not be empty")
	}

	// Verify flags
	if cmd.Flags().Lookup("cluster") == nil {
		t.Error("missing --cluster flag")
	}

	// Should accept 0 or 1 args
	if err := cmd.Args(cmd, []string{}); err != nil {
		t.Errorf("should accept 0 args: %v", err)
	}
	if err := cmd.Args(cmd, []string{"myapp"}); err != nil {
		t.Errorf("should accept 1 arg: %v", err)
	}
	if err := cmd.Args(cmd, []string{"a", "b"}); err == nil {
		t.Error("should reject 2 args")
	}
}

func TestNewDeployRunCmd_Structure(t *testing.T) {
	cmd := newDeployRunCmd()
	if cmd.Use != "run" {
		t.Errorf("Use = %q, want 'run'", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("Short should not be empty")
	}

	// Verify all flags exist
	expectedFlags := []string{"cluster", "dry-run", "file", "watch", "timeout"}
	for _, name := range expectedFlags {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("missing --%s flag", name)
		}
	}

	// Verify flag defaults
	fileFlag := cmd.Flags().Lookup("file")
	if fileFlag.DefValue != "score.yaml" {
		t.Errorf("--file default = %q, want 'score.yaml'", fileFlag.DefValue)
	}

	dryRunFlag := cmd.Flags().Lookup("dry-run")
	if dryRunFlag.DefValue != "false" {
		t.Errorf("--dry-run default = %q, want 'false'", dryRunFlag.DefValue)
	}

	watchFlag := cmd.Flags().Lookup("watch")
	if watchFlag.DefValue != "false" {
		t.Errorf("--watch default = %q, want 'false'", watchFlag.DefValue)
	}

	timeoutFlag := cmd.Flags().Lookup("timeout")
	if timeoutFlag.DefValue != "5m0s" {
		t.Errorf("--timeout default = %q, want '5m0s'", timeoutFlag.DefValue)
	}
}

func TestNewDeployInitCmd_Structure(t *testing.T) {
	cmd := newDeployInitCmd()
	if cmd.Use != "init" {
		t.Errorf("Use = %q, want 'init'", cmd.Use)
	}

	// Verify flags
	if cmd.Flags().Lookup("cluster") == nil {
		t.Error("missing --cluster flag")
	}
	if cmd.Flags().Lookup("template") == nil {
		t.Error("missing --template flag")
	}

	templateFlag := cmd.Flags().Lookup("template")
	if templateFlag.DefValue != "web" {
		t.Errorf("--template default = %q, want 'web'", templateFlag.DefValue)
	}
}

func TestGitWorkflowStep_TitleByMode(t *testing.T) {
	tests := []struct {
		gitMode   string
		wantTitle string
	}{
		{"auto", "Committing and pushing"},
		{"generate", "Committing changes"},
		{"stage-only", "Staging files"},
		{"", "Staging files"},
	}

	for _, tt := range tests {
		t.Run(tt.gitMode, func(t *testing.T) {
			step := gitWorkflowStep(git.WorkflowOpts{
				RepoPath: "/tmp/fake",
				GitMode:  tt.gitMode,
			})
			if step.Title != tt.wantTitle {
				t.Errorf("Title = %q, want %q", step.Title, tt.wantTitle)
			}
		})
	}
}
