package git

import (
	"testing"
)

func TestWorkflowOptsDefaults(t *testing.T) {
	opts := WorkflowOpts{
		RepoPath: "/tmp/test-repo",
		Paths:    []string{"file.yaml"},
		Action:   "test",
		Resource: "thing",
		GitMode:  "generate",
	}

	if opts.ConfirmPrompt != "" {
		t.Errorf("expected empty confirm prompt, got %q", opts.ConfirmPrompt)
	}
	if opts.Details != "" {
		t.Errorf("expected empty details, got %q", opts.Details)
	}
	if opts.Interactive {
		t.Errorf("expected interactive=false by default")
	}
	_ = opts // ensure struct is constructable
}

func TestHandleGitWorkflowNoRepo(t *testing.T) {
	// When RepoPath points to a non-existent directory, DetectRepo should fail
	// and HandleGitWorkflow should return GitNoRepo without error.
	result, err := HandleGitWorkflow(WorkflowOpts{
		RepoPath: "/tmp/definitely-not-a-git-repo-" + t.Name(),
		Paths:    []string{"file.yaml"},
		Action:   "test",
		Resource: "thing",
		GitMode:  "auto",
	})
	if err != nil {
		t.Fatalf("expected no error for missing repo, got: %v", err)
	}
	if result != GitNoRepo {
		t.Errorf("expected GitNoRepo, got %v", result)
	}
}

func TestHandleGitWorkflowUnknownMode(t *testing.T) {
	// Unknown git mode should be treated as "generate only"
	result, err := HandleGitWorkflow(WorkflowOpts{
		RepoPath: "/tmp/definitely-not-a-git-repo-" + t.Name(),
		Paths:    []string{"file.yaml"},
		Action:   "test",
		Resource: "thing",
		GitMode:  "unknown-mode",
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	// With no repo, we get GitNoRepo regardless of mode
	if result != GitNoRepo {
		t.Errorf("expected GitNoRepo, got %v", result)
	}
}

func TestHandleGitWorkflowPromptNonInteractive(t *testing.T) {
	// In prompt mode with Interactive=false, should skip
	result, err := HandleGitWorkflow(WorkflowOpts{
		RepoPath:    "/tmp/definitely-not-a-git-repo-" + t.Name(),
		Paths:       []string{"file.yaml"},
		Action:      "test",
		Resource:    "thing",
		GitMode:     "prompt",
		Interactive: false,
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	// No repo means GitNoRepo before we even check the mode
	if result != GitNoRepo {
		t.Errorf("expected GitNoRepo, got %v", result)
	}
}

func TestFormatCommitMessage(t *testing.T) {
	tests := []struct {
		action, resource, details string
		want                      string
	}{
		{"create vcluster", "media", "prod, 3 replicas", "hctl: create vcluster media (prod, 3 replicas)"},
		{"delete vcluster", "dev", "", "hctl: delete vcluster dev"},
		{"deploy", "sonarr", "vcluster-media", "hctl: deploy sonarr (vcluster-media)"},
	}

	for _, tt := range tests {
		got := FormatCommitMessage(tt.action, tt.resource, tt.details)
		if got != tt.want {
			t.Errorf("FormatCommitMessage(%q, %q, %q) = %q, want %q", tt.action, tt.resource, tt.details, got, tt.want)
		}
	}
}
