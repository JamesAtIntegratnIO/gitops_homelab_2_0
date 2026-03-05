package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// mockUI captures calls to the UI interface for assertion.
type mockUI struct {
	confirmAnswer bool
	confirmErr    error
	successMsgs   []string
	dimMsgs       []string
}

func (m *mockUI) Confirm(_ string) (bool, error) { return m.confirmAnswer, m.confirmErr }
func (m *mockUI) PrintSuccess(msg string)         { m.successMsgs = append(m.successMsgs, msg) }
func (m *mockUI) PrintDim(msg string)             { m.dimMsgs = append(m.dimMsgs, msg) }

// setupWorkflowRepo creates a temp git repo with a dirty file for testing.
func setupWorkflowRepo(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "checkout", "-b", "main"},
		{"git", "commit", "--allow-empty", "-m", "initial"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("setup %v: %s: %v", args, out, err)
		}
	}

	// Create a file to commit
	filePath := "test-file.yaml"
	absPath := filepath.Join(dir, filePath)
	if err := os.WriteFile(absPath, []byte("key: value\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir, filePath
}

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

// ---------------------------------------------------------------------------
// HandleGitWorkflow with real repos and mock UI
// ---------------------------------------------------------------------------

func TestHandleGitWorkflow_GenerateMode_WithRepo(t *testing.T) {
	dir, filePath := setupWorkflowRepo(t)

	ui := &mockUI{}
	result, err := HandleGitWorkflow(WorkflowOpts{
		RepoPath: dir,
		Paths:    []string{filePath},
		Action:   "deploy",
		Resource: "myapp",
		GitMode:  "generate",
		UI:       ui,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != GitCommittedLocal {
		t.Errorf("expected GitCommittedLocal, got %v", result)
	}
	if len(ui.successMsgs) != 1 {
		t.Errorf("expected 1 success message, got %d", len(ui.successMsgs))
	}

	// Verify the repo is clean (file was committed)
	repo := &Repo{Root: dir}
	clean, _ := repo.IsClean()
	if !clean {
		t.Error("repo should be clean after generate mode commit")
	}
}

func TestHandleGitWorkflow_PromptMode_Confirmed(t *testing.T) {
	dir, filePath := setupWorkflowRepo(t)

	ui := &mockUI{confirmAnswer: true}
	// Push will fail (no remote) but commit should succeed
	result, err := HandleGitWorkflow(WorkflowOpts{
		RepoPath:    dir,
		Paths:       []string{filePath},
		Action:      "deploy",
		Resource:    "myapp",
		GitMode:     "prompt",
		Interactive: true,
		UI:          ui,
	})
	// The error is expected because there's no remote to push to
	if err == nil {
		t.Log("Note: push succeeded (unexpected in test but not fatal)")
	}
	// Result should be GitPushFailed because commit succeeded but push failed
	if result != GitPushFailed {
		t.Errorf("expected GitPushFailed, got %v", result)
	}
}

func TestHandleGitWorkflow_PromptMode_Declined(t *testing.T) {
	dir, filePath := setupWorkflowRepo(t)

	ui := &mockUI{confirmAnswer: false}
	result, err := HandleGitWorkflow(WorkflowOpts{
		RepoPath:    dir,
		Paths:       []string{filePath},
		Action:      "deploy",
		Resource:    "myapp",
		GitMode:     "prompt",
		Interactive: true,
		UI:          ui,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != GitStaged {
		t.Errorf("expected GitStaged, got %v", result)
	}
	if len(ui.dimMsgs) == 0 {
		t.Error("expected a dim message about skipping git commit")
	}
}

func TestHandleGitWorkflow_PromptMode_NoUI(t *testing.T) {
	dir, filePath := setupWorkflowRepo(t)

	result, err := HandleGitWorkflow(WorkflowOpts{
		RepoPath:    dir,
		Paths:       []string{filePath},
		Action:      "deploy",
		Resource:    "myapp",
		GitMode:     "prompt",
		Interactive: true,
		UI:          nil, // no UI = skip
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != GitSkipped {
		t.Errorf("expected GitSkipped when UI is nil, got %v", result)
	}
}

func TestHandleGitWorkflow_DefaultMode(t *testing.T) {
	dir, filePath := setupWorkflowRepo(t)

	ui := &mockUI{}
	result, err := HandleGitWorkflow(WorkflowOpts{
		RepoPath: dir,
		Paths:    []string{filePath},
		Action:   "deploy",
		Resource: "myapp",
		GitMode:  "", // default/unknown mode
		UI:       ui,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != GitSkipped {
		t.Errorf("expected GitSkipped for default mode, got %v", result)
	}
	if len(ui.dimMsgs) == 0 {
		t.Error("expected a dim message about generate-only")
	}
}

func TestHandleGitWorkflow_AutoMode_WithRepo(t *testing.T) {
	dir, filePath := setupWorkflowRepo(t)

	ui := &mockUI{}
	// Auto mode will try commit + push. Push fails (no remote).
	result, err := HandleGitWorkflow(WorkflowOpts{
		RepoPath: dir,
		Paths:    []string{filePath},
		Action:   "deploy",
		Resource: "myapp",
		GitMode:  "auto",
		UI:       ui,
	})
	// Push should fail, so we expect an error
	if err == nil {
		t.Log("Note: auto mode push succeeded unexpectedly")
	}
	if result != GitPushFailed {
		t.Errorf("expected GitPushFailed, got %v", result)
	}
}

func TestGitResultConstants(t *testing.T) {
	t.Parallel()
	// Verify the constants are distinct
	results := []GitResult{GitCommitted, GitCommittedLocal, GitPushFailed, GitStaged, GitSkipped, GitNoRepo}
	seen := make(map[GitResult]bool)
	for _, r := range results {
		if seen[r] {
			t.Errorf("duplicate GitResult value: %d", r)
		}
		seen[r] = true
	}
}
